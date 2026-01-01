// Package main provides the web server for QuantumLife.
//
// CRITICAL: Uses stdlib only (net/http + html/template).
// CRITICAL: No goroutines in request handlers.
// CRITICAL: Loop runs synchronously per request.
// CRITICAL: Graceful shutdown is command-layer only (not in internal/ or pkg/).
//
// Reference: docs/ADR/ADR-0023-phase6-quiet-loop-web.md
package main

import (
	"context"
	"flag"
	"fmt"
	"html/template"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	calexec "quantumlife/internal/calendar/execution"
	"quantumlife/internal/config"
	mockcal "quantumlife/internal/connectors/calendar/write/providers/mock"
	mockemail "quantumlife/internal/connectors/email/write/providers/mock"
	"quantumlife/internal/drafts"
	"quantumlife/internal/drafts/calendar"
	"quantumlife/internal/drafts/commerce"
	"quantumlife/internal/drafts/email"
	"quantumlife/internal/drafts/review"
	emailexec "quantumlife/internal/email/execution"
	"quantumlife/internal/execexecutor"
	"quantumlife/internal/execrouter"
	"quantumlife/internal/interruptions"
	"quantumlife/internal/loop"
	"quantumlife/internal/obligations"
	"quantumlife/pkg/clock"
	"quantumlife/pkg/domain/draft"
	domainevents "quantumlife/pkg/domain/events"
	"quantumlife/pkg/domain/feedback"
	"quantumlife/pkg/domain/identity"
	"quantumlife/pkg/domain/obligation"
	"quantumlife/pkg/domain/policy"
	"quantumlife/pkg/events"
)

var (
	addr       = flag.String("addr", ":8080", "HTTP listen address")
	mockData   = flag.Bool("mock", true, "Use mock data")
	configPath = flag.String("config", "configs/circles/default.qlconf", "Path to circle configuration file")
)

// Server handles HTTP requests.
type Server struct {
	engine            *loop.Engine
	templates         *template.Template
	eventEmitter      *eventLogger
	clk               clock.Clock
	execRouter        *execrouter.Router
	execExecutor      *execexecutor.Executor
	multiCircleConfig *config.MultiCircleConfig
	identityRepo      *identity.InMemoryRepository // Phase 13.1: Identity graph
}

// eventLogger logs events.
type eventLogger struct {
	events []events.Event
}

func (l *eventLogger) Emit(event events.Event) {
	l.events = append(l.events, event)
	log.Printf("[EVENT] %s: %v", event.Type, event.Metadata)
}

// templateData holds data for templates.
type templateData struct {
	Title            string
	CurrentTime      string
	RunResult        *loop.RunResult
	NeedsYou         *loop.NeedsYouSummary
	Circles          []loop.CircleResult
	Draft            *draft.Draft
	PendingDrafts    []draft.Draft
	FeedbackStats    *feedback.FeedbackStats
	CalendarExecHist []calexec.Envelope
	EmailExecHist    []emailexec.Envelope
	Message          string
	Error            string
	ExecOutcome      *execexecutor.ExecutionOutcome
	CircleConfigs    []circleConfigInfo
	ConfigHash       string
	ConfigPath       string
	// Phase 13.1: People UI
	People        []personInfo
	Person        *personInfo
	IdentityStats *identityStats
	// Phase 14: Policy UI
	PolicySet      *policy.PolicySet
	CirclePolicies []circlePolicyInfo
	CirclePolicy   *circlePolicyInfo
}

// personInfo contains person data for display. Phase 13.1.
type personInfo struct {
	ID            string
	Label         string
	PrimaryEmail  string
	IsVIP         bool
	IsHousehold   bool
	EdgeCount     int
	Organizations []string
	Households    []string
}

// identityStats contains identity graph statistics for display. Phase 13.1.
type identityStats struct {
	PersonCount       int
	OrganizationCount int
	HouseholdCount    int
	EdgeCount         int
}

// circlePolicyInfo contains policy data for display. Phase 14.
type circlePolicyInfo struct {
	CircleID         string
	RegretThreshold  int
	NotifyThreshold  int
	UrgentThreshold  int
	DailyNotifyQuota int
	DailyQueuedQuota int
	HasHoursPolicy   bool
	HoursInfo        string
}

// circleConfigInfo contains config info for display.
type circleConfigInfo struct {
	ID                   string
	Name                 string
	EmailCount           int
	CalendarCount        int
	FinanceCount         int
	EmailIntegrations    []string
	CalendarIntegrations []string
	FinanceIntegrations  []string
}

// mockIdentityRepo implements IdentityRepository for obligations engine.
type mockIdentityRepo struct{}

func (m *mockIdentityRepo) GetByID(id identity.EntityID) (identity.Entity, error) {
	return nil, nil
}

func (m *mockIdentityRepo) IsHighPriority(id identity.EntityID) bool {
	return false
}

func main() {
	flag.Parse()

	// Create clock (real time for production web server)
	clk := clock.NewReal()

	// Load multi-circle configuration (Phase 11)
	var multiCfg *config.MultiCircleConfig
	if *configPath != "" {
		cfg, err := config.LoadFromFile(*configPath, clk.Now())
		if err != nil {
			log.Printf("Warning: failed to load config from %s: %v (using default)", *configPath, err)
			multiCfg = config.DefaultConfig(clk.Now())
		} else {
			multiCfg = cfg
			log.Printf("Loaded config from %s (hash: %s)", *configPath, cfg.Hash()[:16])
		}
	} else {
		multiCfg = config.DefaultConfig(clk.Now())
	}

	// Create event logger
	emitter := &eventLogger{}

	// Create stores
	draftStore := draft.NewInMemoryStore()
	feedbackStore := feedback.NewMemoryStore()
	identityRepo := identity.NewInMemoryRepository()
	eventStore := domainevents.NewInMemoryEventStore()

	// Create circles for demo
	gen := identity.NewGenerator()
	now := clk.Now()
	personalCircle := gen.CircleFromName("owner-1", "Personal", now)
	workCircle := gen.CircleFromName("owner-1", "Work", now)
	financeCircle := gen.CircleFromName("owner-1", "Finance", now)
	identityRepo.Store(personalCircle)
	identityRepo.Store(workCircle)
	identityRepo.Store(financeCircle)

	// Populate mock events if requested
	if *mockData {
		populateMockEvents(eventStore, now, personalCircle.ID(), workCircle.ID(), financeCircle.ID())
	}

	// Create obligations engine
	oblConfig := obligations.DefaultConfig()
	oblIdentityRepo := &mockIdentityRepo{}
	obligationEngine := obligations.NewEngine(oblConfig, clk, oblIdentityRepo)

	// Create interruptions engine
	intConfig := interruptions.DefaultConfig()
	dedupStore := interruptions.NewInMemoryDeduper()
	quotaStore := interruptions.NewInMemoryQuotaStore()
	interruptionEngine := interruptions.NewEngine(intConfig, clk, dedupStore, quotaStore)

	// Create drafts engine
	draftPolicy := draft.DefaultDraftPolicy()
	emailEngine := email.NewDefaultEngine()
	calendarEngine := calendar.NewDefaultEngine()
	commerceEngine := commerce.NewDefaultEngine()
	draftEngine := drafts.NewEngine(draftStore, draftPolicy, emailEngine, calendarEngine, commerceEngine)

	// Create review service
	reviewService := review.NewService(draftStore)

	// Create calendar executor
	calMockWriter := mockcal.NewWriter(
		mockcal.WithClock(clk.Now),
	)
	calExecutor := calexec.NewExecutor(calexec.ExecutorConfig{
		EnvelopeStore:   calexec.NewMemoryStore(),
		FreshnessPolicy: calexec.NewDefaultFreshnessPolicy(),
		Clock:           clk.Now,
	})
	calExecutor.RegisterWriter("mock", calMockWriter)

	// Create email executor
	emailMockWriter := mockemail.NewWriter(
		mockemail.WithClock(clk.Now),
	)
	emailExecutor := emailexec.NewExecutor(
		emailexec.WithExecutorClock(clk.Now),
		emailexec.WithWriter("mock", emailMockWriter),
		emailexec.WithEventEmitter(emitter),
	)

	// Create loop engine
	engine := &loop.Engine{
		Clock:              clk,
		IdentityRepo:       identityRepo,
		EventStore:         eventStore,
		ObligationEngine:   obligationEngine,
		InterruptionEngine: interruptionEngine,
		DraftEngine:        draftEngine,
		DraftStore:         draftStore,
		ReviewService:      reviewService,
		CalendarExecutor:   calExecutor,
		EmailExecutor:      emailExecutor,
		FeedbackStore:      feedbackStore,
		EventEmitter:       emitter,
	}

	// Create Phase 10 execution routing components
	execRouter := execrouter.NewRouter(clk, emitter)
	execExecutor := execexecutor.NewExecutor(clk, emitter).
		WithEmailExecutor(emailExecutor).
		WithCalendarExecutor(calExecutor)

	// Parse templates
	tmpl := template.Must(template.New("").Funcs(template.FuncMap{
		"formatTime": func(t time.Time) string {
			return t.Format("2006-01-02 15:04:05")
		},
	}).Parse(templates))

	// Create server
	server := &Server{
		engine:            engine,
		templates:         tmpl,
		eventEmitter:      emitter,
		clk:               clk,
		execRouter:        execRouter,
		execExecutor:      execExecutor,
		multiCircleConfig: multiCfg,
		identityRepo:      identityRepo, // Phase 13.1
	}

	// Set up routes
	mux := http.NewServeMux()
	mux.HandleFunc("/", server.handleHome)
	mux.HandleFunc("/circles", server.handleCircles)
	mux.HandleFunc("/circle/", server.handleCircle)
	mux.HandleFunc("/needs-you", server.handleNeedsYou)
	mux.HandleFunc("/draft/", server.handleDraft)
	mux.HandleFunc("/execute/", server.handleExecute)
	mux.HandleFunc("/history", server.handleHistory)
	mux.HandleFunc("/run/daily", server.handleRunDaily)
	mux.HandleFunc("/feedback", server.handleFeedback)
	mux.HandleFunc("/people", server.handlePeople)          // Phase 13.1
	mux.HandleFunc("/people/", server.handlePerson)         // Phase 13.1
	mux.HandleFunc("/policies", server.handlePolicies)      // Phase 14
	mux.HandleFunc("/policies/", server.handlePolicyDetail) // Phase 14

	// Create HTTP server with explicit configuration
	httpServer := &http.Server{
		Addr:    *addr,
		Handler: mux,
	}

	// Channel to signal server shutdown complete
	shutdownComplete := make(chan struct{})

	// Goroutine to handle graceful shutdown on signals
	// NOTE: This goroutine is ONLY in the command layer (main.go).
	// Core packages (internal/, pkg/) remain synchronous with no goroutines.
	go func() {
		sigChan := make(chan os.Signal, 1)
		signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
		<-sigChan

		// Print shutdown message to stdout (not log, for clean output)
		fmt.Println("quantumlife-web: shutting down")

		// Create shutdown context with 3-second timeout
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()

		// Gracefully shutdown the server
		if err := httpServer.Shutdown(ctx); err != nil {
			log.Printf("shutdown error: %v", err)
		}

		close(shutdownComplete)
	}()

	log.Printf("Starting QuantumLife Web on %s", *addr)
	log.Printf("Mock data: %v", *mockData)

	// Start the server (blocks until shutdown)
	if err := httpServer.ListenAndServe(); err != http.ErrServerClosed {
		log.Fatalf("server error: %v", err)
	}

	// Wait for shutdown to complete
	<-shutdownComplete
}

// populateMockEvents creates realistic mock events.
func populateMockEvents(store *domainevents.InMemoryEventStore, now time.Time, personal, work, finance identity.EntityID) {
	// Work: Unread important email
	importantEmail := domainevents.NewEmailMessageEvent("gmail", "msg-100", "self@work.com", now, now.Add(-3*time.Hour))
	importantEmail.Circle = work
	importantEmail.Subject = "URGENT: Approval needed - Q1 Budget Review"
	importantEmail.BodyPreview = "Please review and approve the attached budget by Friday."
	importantEmail.From = domainevents.EmailAddress{Address: "cfo@company.com", Name: "Sarah CFO"}
	importantEmail.IsRead = false
	importantEmail.IsImportant = true
	importantEmail.SenderDomain = "company.com"
	store.Store(importantEmail)

	// Work: Unresponded calendar invite
	meetingInvite := domainevents.NewCalendarEventEvent("google", "cal-work", "evt-001", "self@work.com", now, now)
	meetingInvite.Circle = work
	meetingInvite.Title = "Quarterly Review Meeting"
	meetingInvite.StartTime = now.Add(4 * time.Hour)
	meetingInvite.EndTime = now.Add(5 * time.Hour)
	meetingInvite.MyResponseStatus = domainevents.RSVPNeedsAction
	meetingInvite.AttendeeCount = 10
	store.Store(meetingInvite)

	// Personal: School event needing decision
	schoolEvent := domainevents.NewCalendarEventEvent("google", "cal-personal", "evt-002", "self@personal.com", now, now)
	schoolEvent.Circle = personal
	schoolEvent.Title = "Parent-Teacher Conference"
	schoolEvent.StartTime = now.Add(6 * time.Hour)
	schoolEvent.EndTime = now.Add(7 * time.Hour)
	schoolEvent.MyResponseStatus = domainevents.RSVPNeedsAction
	store.Store(schoolEvent)

	// Finance: Balance check (healthy)
	balance := domainevents.NewBalanceEvent("truelayer", "acc-300", now, now)
	balance.Circle = finance
	balance.AccountType = "CHECKING"
	balance.Institution = "Bank"
	balance.AvailableMinor = 150000 // £1500
	balance.CurrentMinor = 155000
	balance.Currency = "GBP"
	store.Store(balance)

	log.Printf("Populated %d mock events", 4)
}

// handleHome shows the home page ("Nothing Needs You" or needs-you summary).
func (s *Server) handleHome(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}

	// Run the loop
	result := s.engine.Run(context.Background(), loop.RunOptions{
		IncludeMockData: *mockData,
	})

	data := templateData{
		Title:       "QuantumLife",
		CurrentTime: s.clk.Now().Format("2006-01-02 15:04:05"),
		RunResult:   &result,
		NeedsYou:    &result.NeedsYou,
		Circles:     result.Circles,
	}

	s.render(w, "home", data)
}

// handleCircles lists all circles.
func (s *Server) handleCircles(w http.ResponseWriter, r *http.Request) {
	result := s.engine.Run(context.Background(), loop.RunOptions{
		IncludeMockData: *mockData,
	})

	// Build circle config info (Phase 11)
	var circleConfigs []circleConfigInfo
	if s.multiCircleConfig != nil {
		for _, circleID := range s.multiCircleConfig.CircleIDs() {
			circle := s.multiCircleConfig.GetCircle(circleID)
			if circle == nil {
				continue
			}
			info := circleConfigInfo{
				ID:            string(circle.ID),
				Name:          circle.Name,
				EmailCount:    len(circle.EmailIntegrations),
				CalendarCount: len(circle.CalendarIntegrations),
				FinanceCount:  len(circle.FinanceIntegrations),
			}
			for _, e := range circle.EmailIntegrations {
				info.EmailIntegrations = append(info.EmailIntegrations, e.Provider+":"+e.Identifier)
			}
			for _, c := range circle.CalendarIntegrations {
				info.CalendarIntegrations = append(info.CalendarIntegrations, c.Provider+":"+c.CalendarID)
			}
			for _, f := range circle.FinanceIntegrations {
				info.FinanceIntegrations = append(info.FinanceIntegrations, f.Provider+":"+f.Identifier)
			}
			circleConfigs = append(circleConfigs, info)
		}
	}

	data := templateData{
		Title:         "Circles",
		CurrentTime:   s.clk.Now().Format("2006-01-02 15:04:05"),
		Circles:       result.Circles,
		CircleConfigs: circleConfigs,
		ConfigPath:    *configPath,
	}
	if s.multiCircleConfig != nil {
		data.ConfigHash = s.multiCircleConfig.Hash()[:16]
	}

	s.render(w, "circles", data)
}

// handleCircle shows a single circle.
func (s *Server) handleCircle(w http.ResponseWriter, r *http.Request) {
	circleID := strings.TrimPrefix(r.URL.Path, "/circle/")
	if circleID == "" {
		http.Redirect(w, r, "/circles", http.StatusFound)
		return
	}

	result := s.engine.Run(context.Background(), loop.RunOptions{
		CircleID:        identity.EntityID(circleID),
		IncludeMockData: *mockData,
	})

	if len(result.Circles) == 0 {
		http.NotFound(w, r)
		return
	}

	data := templateData{
		Title:       fmt.Sprintf("Circle: %s", result.Circles[0].CircleName),
		CurrentTime: s.clk.Now().Format("2006-01-02 15:04:05"),
		Circles:     result.Circles,
	}

	s.render(w, "circle", data)
}

// handleNeedsYou shows items that need attention.
func (s *Server) handleNeedsYou(w http.ResponseWriter, r *http.Request) {
	result := s.engine.Run(context.Background(), loop.RunOptions{
		IncludeMockData: *mockData,
	})

	data := templateData{
		Title:       "Needs You",
		CurrentTime: s.clk.Now().Format("2006-01-02 15:04:05"),
		NeedsYou:    &result.NeedsYou,
	}

	s.render(w, "needs-you", data)
}

// handleDraft handles draft viewing and approval/rejection.
func (s *Server) handleDraft(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimPrefix(r.URL.Path, "/draft/")
	parts := strings.Split(path, "/")

	if len(parts) == 0 || parts[0] == "" {
		// List pending drafts
		pending := s.engine.GetPendingDrafts()
		data := templateData{
			Title:         "Pending Drafts",
			CurrentTime:   s.clk.Now().Format("2006-01-02 15:04:05"),
			PendingDrafts: pending,
		}
		s.render(w, "drafts", data)
		return
	}

	draftID := draft.DraftID(parts[0])

	// Handle approve/reject
	if len(parts) >= 2 && r.Method == http.MethodPost {
		action := parts[1]
		reason := r.FormValue("reason")

		d, found := s.engine.GetDraft(draftID)
		if !found {
			http.NotFound(w, r)
			return
		}

		var err error
		switch action {
		case "approve":
			err = s.engine.ApproveDraft(draftID, d.CircleID, reason)
		case "reject":
			err = s.engine.RejectDraft(draftID, d.CircleID, reason)
		default:
			http.Error(w, "unknown action", http.StatusBadRequest)
			return
		}

		if err != nil {
			data := templateData{
				Title: "Error",
				Error: err.Error(),
			}
			s.render(w, "error", data)
			return
		}

		http.Redirect(w, r, "/draft/", http.StatusFound)
		return
	}

	// Show draft details
	d, found := s.engine.GetDraft(draftID)
	if !found {
		http.NotFound(w, r)
		return
	}

	data := templateData{
		Title:       fmt.Sprintf("Draft: %s", draftID),
		CurrentTime: s.clk.Now().Format("2006-01-02 15:04:05"),
		Draft:       &d,
	}

	s.render(w, "draft", data)
}

// handleHistory shows execution history.
func (s *Server) handleHistory(w http.ResponseWriter, r *http.Request) {
	calHistory := s.engine.GetExecutionHistory()
	emailHistory := s.engine.GetEmailExecutionHistory()

	data := templateData{
		Title:            "Execution History",
		CurrentTime:      s.clk.Now().Format("2006-01-02 15:04:05"),
		CalendarExecHist: calHistory,
		EmailExecHist:    emailHistory,
	}

	s.render(w, "history", data)
}

// handleRunDaily triggers a daily loop run.
// Supports optional circle parameter: POST /run/daily?circle=<id>
func (s *Server) handleRunDaily(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Check for circle-specific run (Phase 11)
	circleID := identity.EntityID(r.URL.Query().Get("circle"))

	opts := loop.RunOptions{
		IncludeMockData:       *mockData,
		ExecuteApprovedDrafts: true,
	}
	if circleID != "" {
		opts.CircleID = circleID
	}

	result := s.engine.Run(context.Background(), opts)

	var message string
	if circleID != "" {
		message = fmt.Sprintf("Daily run completed for circle %s. RunID: %s, Duration: %v", circleID, result.RunID, result.CompletedAt.Sub(result.StartedAt))
	} else {
		message = fmt.Sprintf("Daily run completed (all circles). RunID: %s, Duration: %v", result.RunID, result.CompletedAt.Sub(result.StartedAt))
	}

	data := templateData{
		Title:       "Run Complete",
		CurrentTime: s.clk.Now().Format("2006-01-02 15:04:05"),
		RunResult:   &result,
		Message:     message,
	}

	s.render(w, "run-result", data)
}

// handleFeedback records feedback for an item.
func (s *Server) handleFeedback(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	targetType := feedback.FeedbackTargetType(r.FormValue("target_type"))
	targetID := r.FormValue("target_id")
	circleID := identity.EntityID(r.FormValue("circle_id"))
	signal := feedback.FeedbackSignal(r.FormValue("signal"))
	reason := r.FormValue("reason")

	_, err := s.engine.RecordFeedback(targetType, targetID, circleID, signal, reason)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	http.Redirect(w, r, r.FormValue("redirect"), http.StatusFound)
}

// handleExecute handles draft execution (Phase 10).
// POST /execute/:draft_id - execute an approved draft
func (s *Server) handleExecute(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Extract draft ID from URL
	draftID := strings.TrimPrefix(r.URL.Path, "/execute/")
	if draftID == "" {
		http.Error(w, "draft ID required", http.StatusBadRequest)
		return
	}

	// Get the draft
	d, found := s.engine.GetDraft(draft.DraftID(draftID))
	if !found {
		http.NotFound(w, r)
		return
	}

	// Draft must be approved for execution
	if d.Status != draft.StatusApproved {
		data := templateData{
			Title:       "Execution Blocked",
			CurrentTime: s.clk.Now().Format("2006-01-02 15:04:05"),
			Error:       fmt.Sprintf("Draft must be approved for execution. Current status: %s", d.Status),
			Draft:       &d,
		}
		s.render(w, "exec-result", data)
		return
	}

	// Build execution intent from draft
	intent, err := s.execRouter.BuildIntentFromDraft(&d)
	if err != nil {
		data := templateData{
			Title:       "Execution Failed",
			CurrentTime: s.clk.Now().Format("2006-01-02 15:04:05"),
			Error:       fmt.Sprintf("Failed to build execution intent: %v", err),
			Draft:       &d,
		}
		s.render(w, "exec-result", data)
		return
	}

	// Execute the intent
	traceID := fmt.Sprintf("web-exec-%s-%d", draftID, s.clk.Now().UnixNano())
	outcome := s.execExecutor.ExecuteIntent(context.Background(), intent, traceID)

	// Render the result
	data := templateData{
		Title:       "Execution Result",
		CurrentTime: s.clk.Now().Format("2006-01-02 15:04:05"),
		Draft:       &d,
		ExecOutcome: &outcome,
	}

	if outcome.Success {
		data.Message = fmt.Sprintf("Execution succeeded! Intent ID: %s, Provider Response: %s",
			outcome.IntentID, outcome.ProviderResponseID)
	} else if outcome.Blocked {
		data.Error = fmt.Sprintf("Execution blocked: %s", outcome.BlockedReason)
	} else {
		data.Error = fmt.Sprintf("Execution failed: %s", outcome.Error)
	}

	s.render(w, "exec-result", data)
}

// handlePeople lists all people in the identity graph. Phase 13.1.
func (s *Server) handlePeople(w http.ResponseWriter, r *http.Request) {
	var people []personInfo
	var stats *identityStats

	if s.identityRepo != nil {
		// Get all persons in deterministic order
		persons := s.identityRepo.ListPersons()
		for _, p := range persons {
			info := personInfo{
				ID:           string(p.ID()),
				Label:        s.identityRepo.PersonLabel(p.ID()),
				PrimaryEmail: s.identityRepo.PrimaryEmail(p.ID()),
				IsHousehold:  s.identityRepo.IsHouseholdMember(p.ID()),
				EdgeCount:    len(s.identityRepo.GetPersonEdgesSorted(p.ID())),
			}

			// Get organizations
			orgs := s.identityRepo.GetPersonOrganizations(p.ID())
			for _, org := range orgs {
				info.Organizations = append(info.Organizations, org.NormalizedName)
			}

			// Get households
			hhs := s.identityRepo.GetPersonHouseholds(p.ID())
			for _, hh := range hhs {
				info.Households = append(info.Households, hh.Name)
			}

			people = append(people, info)
		}

		// Get stats
		stats = &identityStats{
			PersonCount:       s.identityRepo.CountByType(identity.EntityTypePerson),
			OrganizationCount: s.identityRepo.CountByType(identity.EntityTypeOrganization),
			HouseholdCount:    s.identityRepo.CountByType(identity.EntityTypeHousehold),
			EdgeCount:         s.identityRepo.EdgeCount(),
		}
	}

	data := templateData{
		Title:         "People",
		CurrentTime:   s.clk.Now().Format("2006-01-02 15:04:05"),
		People:        people,
		IdentityStats: stats,
	}

	s.render(w, "people", data)
}

// handlePerson shows details for a single person. Phase 13.1.
func (s *Server) handlePerson(w http.ResponseWriter, r *http.Request) {
	personID := strings.TrimPrefix(r.URL.Path, "/people/")
	if personID == "" {
		http.Redirect(w, r, "/people", http.StatusFound)
		return
	}

	if s.identityRepo == nil {
		http.NotFound(w, r)
		return
	}

	entity, err := s.identityRepo.Get(identity.EntityID(personID))
	if err != nil {
		http.NotFound(w, r)
		return
	}

	person, ok := entity.(*identity.Person)
	if !ok {
		http.NotFound(w, r)
		return
	}

	info := &personInfo{
		ID:           string(person.ID()),
		Label:        s.identityRepo.PersonLabel(person.ID()),
		PrimaryEmail: s.identityRepo.PrimaryEmail(person.ID()),
		IsHousehold:  s.identityRepo.IsHouseholdMember(person.ID()),
		EdgeCount:    len(s.identityRepo.GetPersonEdgesSorted(person.ID())),
	}

	// Get organizations
	orgs := s.identityRepo.GetPersonOrganizations(person.ID())
	for _, org := range orgs {
		info.Organizations = append(info.Organizations, org.NormalizedName)
	}

	// Get households
	hhs := s.identityRepo.GetPersonHouseholds(person.ID())
	for _, hh := range hhs {
		info.Households = append(info.Households, hh.Name)
	}

	data := templateData{
		Title:       fmt.Sprintf("Person: %s", info.Label),
		CurrentTime: s.clk.Now().Format("2006-01-02 15:04:05"),
		Person:      info,
	}

	s.render(w, "person", data)
}

// handlePolicies lists all circle policies. Phase 14.
func (s *Server) handlePolicies(w http.ResponseWriter, r *http.Request) {
	// Get default policy set for demo
	now := s.clk.Now()
	ps := policy.DefaultPolicySet(now)

	var policies []circlePolicyInfo
	for _, cp := range ps.Circles {
		info := circlePolicyInfo{
			CircleID:         cp.CircleID,
			RegretThreshold:  cp.RegretThreshold,
			NotifyThreshold:  cp.NotifyThreshold,
			UrgentThreshold:  cp.UrgentThreshold,
			DailyNotifyQuota: cp.DailyNotifyQuota,
			DailyQueuedQuota: cp.DailyQueuedQuota,
		}
		if cp.Hours != nil {
			info.HasHoursPolicy = true
			info.HoursInfo = fmt.Sprintf("Weekdays: %d, %d:00-%d:00",
				cp.Hours.AllowedWeekdays, cp.Hours.StartMinute/60, cp.Hours.EndMinute/60)
		}
		policies = append(policies, info)
	}

	// Sort for determinism
	for i := 0; i < len(policies); i++ {
		for j := i + 1; j < len(policies); j++ {
			if policies[i].CircleID > policies[j].CircleID {
				policies[i], policies[j] = policies[j], policies[i]
			}
		}
	}

	data := templateData{
		Title:          "Policies",
		CurrentTime:    s.clk.Now().Format("2006-01-02 15:04:05"),
		PolicySet:      &ps,
		CirclePolicies: policies,
	}

	s.render(w, "policies", data)
}

// handlePolicyDetail shows/edits a single circle policy. Phase 14.
func (s *Server) handlePolicyDetail(w http.ResponseWriter, r *http.Request) {
	circleID := strings.TrimPrefix(r.URL.Path, "/policies/")

	// Check for edit action
	if strings.HasSuffix(circleID, "/edit") {
		circleID = strings.TrimSuffix(circleID, "/edit")
		if r.Method == http.MethodPost {
			s.handlePolicyEdit(w, r, circleID)
			return
		}
	}

	if circleID == "" {
		http.Redirect(w, r, "/policies", http.StatusFound)
		return
	}

	// Get default policy set for demo
	now := s.clk.Now()
	ps := policy.DefaultPolicySet(now)

	cp := ps.GetCircle(circleID)
	if cp == nil {
		http.NotFound(w, r)
		return
	}

	info := &circlePolicyInfo{
		CircleID:         cp.CircleID,
		RegretThreshold:  cp.RegretThreshold,
		NotifyThreshold:  cp.NotifyThreshold,
		UrgentThreshold:  cp.UrgentThreshold,
		DailyNotifyQuota: cp.DailyNotifyQuota,
		DailyQueuedQuota: cp.DailyQueuedQuota,
	}
	if cp.Hours != nil {
		info.HasHoursPolicy = true
		info.HoursInfo = fmt.Sprintf("Weekdays: %d, %d:00-%d:00",
			cp.Hours.AllowedWeekdays, cp.Hours.StartMinute/60, cp.Hours.EndMinute/60)
	}

	data := templateData{
		Title:        fmt.Sprintf("Policy: %s", circleID),
		CurrentTime:  s.clk.Now().Format("2006-01-02 15:04:05"),
		CirclePolicy: info,
	}

	s.render(w, "policy-detail", data)
}

// handlePolicyEdit handles POST to update a circle policy. Phase 14.
func (s *Server) handlePolicyEdit(w http.ResponseWriter, r *http.Request, circleID string) {
	// Parse form values
	regretThreshold := parseIntOr(r.FormValue("regret_threshold"), 30)
	notifyThreshold := parseIntOr(r.FormValue("notify_threshold"), 50)
	urgentThreshold := parseIntOr(r.FormValue("urgent_threshold"), 75)
	dailyNotifyQuota := parseIntOr(r.FormValue("daily_notify_quota"), 10)
	dailyQueuedQuota := parseIntOr(r.FormValue("daily_queued_quota"), 50)

	// Validate
	if regretThreshold < 0 || regretThreshold > 100 ||
		notifyThreshold < 0 || notifyThreshold > 100 ||
		urgentThreshold < 0 || urgentThreshold > 100 {
		http.Error(w, "thresholds must be 0-100", http.StatusBadRequest)
		return
	}
	if urgentThreshold < notifyThreshold || notifyThreshold < regretThreshold {
		http.Error(w, "thresholds must be monotonic: urgent >= notify >= regret", http.StatusBadRequest)
		return
	}

	// For demo, just show the updated values (no actual persistence in web demo)
	log.Printf("[Phase14] Policy update for %s: regret=%d, notify=%d, urgent=%d, daily_notify=%d, daily_queued=%d",
		circleID, regretThreshold, notifyThreshold, urgentThreshold, dailyNotifyQuota, dailyQueuedQuota)

	http.Redirect(w, r, "/policies/"+circleID, http.StatusFound)
}

// parseIntOr parses an int or returns the default.
func parseIntOr(s string, def int) int {
	if s == "" {
		return def
	}
	var result int
	for _, c := range s {
		if c >= '0' && c <= '9' {
			result = result*10 + int(c-'0')
		}
	}
	if result == 0 && s != "0" {
		return def
	}
	return result
}

// render executes a template.
func (s *Server) render(w http.ResponseWriter, name string, data templateData) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := s.templates.ExecuteTemplate(w, name, data); err != nil {
		log.Printf("template error: %v", err)
		http.Error(w, "internal server error", http.StatusInternalServerError)
	}
}

// generateMockDraftsFromObligations creates drafts from obligations for demo purposes.
func generateMockDraftsFromObligations(engine *drafts.Engine, circleID identity.EntityID, now time.Time) {
	// Create a mock obligation that would generate a draft
	obl := obligation.NewObligation(
		circleID,
		"email-mock-001",
		"email",
		obligation.ObligationReply,
		now,
	).WithReason("Unread email from manager").
		WithEvidence(obligation.EvidenceKeySender, "manager@company.com").
		WithEvidence(obligation.EvidenceKeySubject, "Q1 Budget Review")

	engine.Process(circleID, "", obl, now)
}

// templates contains all HTML templates.
const templates = `
{{define "base"}}
<!DOCTYPE html>
<html>
<head>
    <meta charset="utf-8">
    <meta name="viewport" content="width=device-width, initial-scale=1">
    <title>{{.Title}} - QuantumLife</title>
    <style>
        * { box-sizing: border-box; margin: 0; padding: 0; }
        body { font-family: system-ui, sans-serif; line-height: 1.6; background: #f5f5f5; color: #333; }
        .container { max-width: 800px; margin: 0 auto; padding: 20px; }
        header { background: #2d2d2d; color: white; padding: 20px 0; margin-bottom: 20px; }
        header .container { display: flex; justify-content: space-between; align-items: center; }
        header h1 { font-size: 1.5rem; }
        header nav a { color: white; text-decoration: none; margin-left: 20px; }
        header nav a:hover { text-decoration: underline; }
        .card { background: white; border-radius: 8px; padding: 20px; margin-bottom: 20px; box-shadow: 0 2px 4px rgba(0,0,0,0.1); }
        .quiet { text-align: center; padding: 60px 20px; }
        .quiet h2 { color: #4caf50; font-size: 2rem; margin-bottom: 10px; }
        .quiet p { color: #666; }
        .needs-you { border-left: 4px solid #ff9800; }
        .needs-you h2 { color: #ff9800; }
        .circle-tile { display: inline-block; background: #e3f2fd; padding: 15px 25px; border-radius: 8px; margin: 5px; }
        .circle-tile h3 { margin-bottom: 5px; }
        .draft-item { border-bottom: 1px solid #eee; padding: 15px 0; }
        .draft-item:last-child { border-bottom: none; }
        .btn { display: inline-block; padding: 8px 16px; border-radius: 4px; text-decoration: none; cursor: pointer; border: none; font-size: 14px; }
        .btn-primary { background: #2196f3; color: white; }
        .btn-success { background: #4caf50; color: white; }
        .btn-danger { background: #f44336; color: white; }
        .btn-secondary { background: #757575; color: white; }
        .btn:hover { opacity: 0.9; }
        .status-badge { display: inline-block; padding: 2px 8px; border-radius: 4px; font-size: 12px; }
        .status-proposed { background: #fff3e0; color: #e65100; }
        .status-approved { background: #e8f5e9; color: #2e7d32; }
        .status-rejected { background: #ffebee; color: #c62828; }
        .meta { color: #666; font-size: 14px; margin-top: 10px; }
        .actions { margin-top: 15px; }
        .actions form { display: inline; }
        .form-group { margin-bottom: 15px; }
        .form-group label { display: block; margin-bottom: 5px; font-weight: 500; }
        .form-group input, .form-group textarea { width: 100%; padding: 8px; border: 1px solid #ddd; border-radius: 4px; }
        .error { color: #c62828; background: #ffebee; padding: 15px; border-radius: 4px; }
        .message { color: #2e7d32; background: #e8f5e9; padding: 15px; border-radius: 4px; }
        footer { text-align: center; padding: 20px; color: #666; font-size: 14px; }
    </style>
</head>
<body>
    <header>
        <div class="container">
            <h1>QuantumLife</h1>
            <nav>
                <a href="/">Home</a>
                <a href="/circles">Circles</a>
                <a href="/people">People</a>
                <a href="/policies">Policies</a>
                <a href="/needs-you">Needs You</a>
                <a href="/draft/">Drafts</a>
                <a href="/history">History</a>
            </nav>
        </div>
    </header>
    <main class="container">
        {{template "content" .}}
    </main>
    <footer>
        <p>{{.CurrentTime}} | Deterministic. Synchronous. Quiet.</p>
    </footer>
</body>
</html>
{{end}}

{{define "home"}}
{{template "base" .}}
{{end}}

{{define "content"}}
{{if .NeedsYou}}
    {{if .NeedsYou.IsQuiet}}
    <div class="card quiet">
        <h2>Nothing Needs You</h2>
        <p>All caught up. Enjoy the quiet.</p>
    </div>
    {{else}}
    <div class="card needs-you">
        <h2>{{.NeedsYou.TotalItems}} Item(s) Need Your Attention</h2>
        {{if .NeedsYou.PendingDrafts}}
        <h3 style="margin-top: 15px;">Pending Drafts</h3>
        {{range .NeedsYou.PendingDrafts}}
        <div class="draft-item">
            <strong>{{.DraftType}}</strong>
            <span class="status-badge status-proposed">Proposed</span>
            <div class="meta">Circle: {{.CircleID}} | Created: {{formatTime .CreatedAt}}</div>
            <a href="/draft/{{.DraftID}}" class="btn btn-primary" style="margin-top: 10px;">Review</a>
        </div>
        {{end}}
        {{end}}
        {{if .NeedsYou.ActiveInterruptions}}
        <h3 style="margin-top: 15px;">Active Interruptions</h3>
        {{range .NeedsYou.ActiveInterruptions}}
        <div class="draft-item">
            <strong>{{.Level}}</strong> - {{.Trigger}}
            <div class="meta">Circle: {{.CircleID}}</div>
        </div>
        {{end}}
        {{end}}
    </div>
    {{end}}
{{end}}

{{if .Circles}}
<h2 style="margin-bottom: 15px;">Your Circles</h2>
{{range .Circles}}
<div class="circle-tile">
    <h3>{{.CircleName}}</h3>
    <div class="meta">
        {{.ObligationCount}} obligations |
        {{.InterruptionCount}} interruptions |
        {{.DraftCount}} drafts
    </div>
    <a href="/circle/{{.CircleID}}" class="btn btn-secondary" style="margin-top: 10px; font-size: 12px;">View</a>
</div>
{{end}}
{{end}}

{{if .ExecOutcome}}
<div class="card">
    <h2>Execution Outcome</h2>
    {{if .ExecOutcome.Success}}
    <p class="message">Execution succeeded!</p>
    <div class="meta">
        <p><strong>Intent ID:</strong> {{.ExecOutcome.IntentID}}</p>
        <p><strong>Envelope ID:</strong> {{.ExecOutcome.EnvelopeID}}</p>
        <p><strong>Provider Response:</strong> {{.ExecOutcome.ProviderResponseID}}</p>
    </div>
    {{else if .ExecOutcome.Blocked}}
    <p class="error">Execution blocked: {{.ExecOutcome.BlockedReason}}</p>
    {{else}}
    <p class="error">Execution failed: {{.ExecOutcome.Error}}</p>
    {{end}}
</div>
{{end}}

{{if .Draft}}
<div class="card">
    <h2>Draft: {{.Draft.DraftType}}</h2>
    <span class="status-badge status-{{.Draft.Status}}">{{.Draft.Status}}</span>
    <div class="meta">
        <p><strong>ID:</strong> {{.Draft.DraftID}}</p>
        <p><strong>Circle:</strong> {{.Draft.CircleID}}</p>
        <p><strong>Created:</strong> {{formatTime .Draft.CreatedAt}}</p>
        <p><strong>Expires:</strong> {{formatTime .Draft.ExpiresAt}}</p>
        {{if .Draft.SourceObligationID}}
        <p><strong>From Obligation:</strong> {{.Draft.SourceObligationID}}</p>
        {{end}}
    </div>

    {{if eq .Draft.Status "proposed"}}
    <div class="actions" style="margin-top: 20px;">
        <form method="POST" action="/draft/{{.Draft.DraftID}}/approve" style="display: inline;">
            <input type="hidden" name="reason" value="approved via web">
            <button type="submit" class="btn btn-success">Approve</button>
        </form>
        <form method="POST" action="/draft/{{.Draft.DraftID}}/reject" style="display: inline; margin-left: 10px;">
            <input type="hidden" name="reason" value="rejected via web">
            <button type="submit" class="btn btn-danger">Reject</button>
        </form>
    </div>
    {{end}}
    {{if eq .Draft.Status "approved"}}
    <div class="actions" style="margin-top: 20px;">
        <form method="POST" action="/execute/{{.Draft.DraftID}}" style="display: inline;">
            <button type="submit" class="btn btn-primary">Execute</button>
        </form>
        <p style="margin-top: 10px; color: #666; font-size: 14px;">
            Policy Hash: {{.Draft.PolicySnapshotHash}}<br>
            View Hash: {{.Draft.ViewSnapshotHash}}
        </p>
    </div>
    {{end}}
</div>
{{end}}

{{if .PendingDrafts}}
<div class="card">
    <h2>Pending Drafts</h2>
    {{range .PendingDrafts}}
    <div class="draft-item">
        <strong>{{.DraftType}}</strong>
        <span class="status-badge status-proposed">Proposed</span>
        <div class="meta">Circle: {{.CircleID}} | Created: {{formatTime .CreatedAt}}</div>
        <a href="/draft/{{.DraftID}}" class="btn btn-primary" style="margin-top: 10px;">Review</a>
    </div>
    {{end}}
</div>
{{end}}

{{if .People}}
<div class="card">
    <h2>People</h2>
    {{if .IdentityStats}}
    <div class="meta" style="margin-bottom: 15px;">
        {{.IdentityStats.PersonCount}} people |
        {{.IdentityStats.OrganizationCount}} organizations |
        {{.IdentityStats.HouseholdCount}} households |
        {{.IdentityStats.EdgeCount}} edges
    </div>
    {{end}}
    {{range .People}}
    <div class="draft-item">
        <strong>{{.Label}}</strong>
        {{if .IsHousehold}}<span class="status-badge status-proposed">Household</span>{{end}}
        <div class="meta">
            {{if .PrimaryEmail}}Email: {{.PrimaryEmail}}{{end}}
            {{if .Organizations}} | Orgs: {{range .Organizations}}{{.}} {{end}}{{end}}
            {{if .Households}} | Households: {{range .Households}}{{.}} {{end}}{{end}}
            | {{.EdgeCount}} edges
        </div>
        <a href="/people/{{.ID}}" class="btn btn-secondary" style="margin-top: 10px; font-size: 12px;">View</a>
    </div>
    {{end}}
</div>
{{end}}

{{if .Person}}
<div class="card">
    <h2>Person: {{.Person.Label}}</h2>
    {{if .Person.IsHousehold}}<span class="status-badge status-proposed">Household Member</span>{{end}}
    <div class="meta">
        <p><strong>ID:</strong> {{.Person.ID}}</p>
        {{if .Person.PrimaryEmail}}<p><strong>Primary Email:</strong> {{.Person.PrimaryEmail}}</p>{{end}}
        <p><strong>Edge Count:</strong> {{.Person.EdgeCount}}</p>
        {{if .Person.Organizations}}
        <p><strong>Organizations:</strong> {{range .Person.Organizations}}{{.}} {{end}}</p>
        {{end}}
        {{if .Person.Households}}
        <p><strong>Households:</strong> {{range .Person.Households}}{{.}} {{end}}</p>
        {{end}}
    </div>
</div>
{{end}}

<div class="card" style="margin-top: 20px;">
    <h3>Run Daily Loop</h3>
    <p style="margin: 10px 0;">Trigger a full daily loop run (synchronous).</p>
    <form method="POST" action="/run/daily">
        <button type="submit" class="btn btn-primary">Run Now</button>
    </form>
</div>
{{end}}

{{define "circles"}}
{{template "base" .}}
{{end}}

{{define "circle"}}
{{template "base" .}}
{{end}}

{{define "needs-you"}}
{{template "base" .}}
{{end}}

{{define "drafts"}}
{{template "base" .}}
{{end}}

{{define "draft"}}
{{template "base" .}}
{{end}}

{{define "history"}}
{{template "base" .}}
{{end}}

{{define "run-result"}}
{{template "base" .}}
{{end}}

{{define "error"}}
{{template "base" .}}
{{end}}

{{define "exec-result"}}
{{template "base" .}}
{{end}}

{{define "people"}}
{{template "base" .}}
{{end}}

{{define "person"}}
{{template "base" .}}
{{end}}

{{define "policies"}}
{{template "base" .}}
{{end}}

{{define "policies-content"}}
<div class="card">
    <h2>Circle Policies</h2>
    <p class="meta">Policy Hash: {{if .PolicySet}}{{.PolicySet.Hash}}{{else}}N/A{{end}}</p>
</div>
{{range .CirclePolicies}}
<div class="card">
    <h3><a href="/policies/{{.CircleID}}">{{.CircleID}}</a></h3>
    <table style="width:100%; margin-top:10px;">
        <tr><td>Regret Threshold:</td><td>{{.RegretThreshold}}</td></tr>
        <tr><td>Notify Threshold:</td><td>{{.NotifyThreshold}}</td></tr>
        <tr><td>Urgent Threshold:</td><td>{{.UrgentThreshold}}</td></tr>
        <tr><td>Daily Notify Quota:</td><td>{{.DailyNotifyQuota}}</td></tr>
        <tr><td>Daily Queued Quota:</td><td>{{.DailyQueuedQuota}}</td></tr>
        {{if .HasHoursPolicy}}<tr><td>Hours Policy:</td><td>{{.HoursInfo}}</td></tr>{{end}}
    </table>
</div>
{{else}}
<div class="card">
    <p>No policies configured.</p>
</div>
{{end}}
{{end}}

{{define "policy-detail"}}
{{template "base" .}}
{{end}}

{{define "policy-detail-content"}}
{{if .CirclePolicy}}
<div class="card">
    <h2>Policy: {{.CirclePolicy.CircleID}}</h2>
    <form method="POST" action="/policies/{{.CirclePolicy.CircleID}}/edit">
        <div class="form-group">
            <label>Regret Threshold (0-100)</label>
            <input type="number" name="regret_threshold" value="{{.CirclePolicy.RegretThreshold}}" min="0" max="100">
        </div>
        <div class="form-group">
            <label>Notify Threshold (0-100)</label>
            <input type="number" name="notify_threshold" value="{{.CirclePolicy.NotifyThreshold}}" min="0" max="100">
        </div>
        <div class="form-group">
            <label>Urgent Threshold (0-100)</label>
            <input type="number" name="urgent_threshold" value="{{.CirclePolicy.UrgentThreshold}}" min="0" max="100">
        </div>
        <div class="form-group">
            <label>Daily Notify Quota</label>
            <input type="number" name="daily_notify_quota" value="{{.CirclePolicy.DailyNotifyQuota}}" min="0">
        </div>
        <div class="form-group">
            <label>Daily Queued Quota</label>
            <input type="number" name="daily_queued_quota" value="{{.CirclePolicy.DailyQueuedQuota}}" min="0">
        </div>
        {{if .CirclePolicy.HasHoursPolicy}}
        <p class="meta">Hours: {{.CirclePolicy.HoursInfo}}</p>
        {{end}}
        <button type="submit" class="btn btn-primary">Update Policy</button>
        <a href="/policies" class="btn btn-secondary">Back to Policies</a>
    </form>
</div>
{{else}}
<div class="card error">
    <p>Policy not found.</p>
</div>
{{end}}
{{end}}
`
