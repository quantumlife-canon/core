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
	"quantumlife/internal/held"
	"quantumlife/internal/interest"
	"quantumlife/internal/proof"
	"quantumlife/internal/surface"
	"quantumlife/internal/interruptions"
	"quantumlife/internal/loop"
	"quantumlife/internal/obligations"
	"quantumlife/internal/todayquietly"
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
	identityRepo      *identity.InMemoryRepository  // Phase 13.1: Identity graph
	interestStore     *interest.Store               // Phase 18.1: Interest capture
	todayEngine       *todayquietly.Engine          // Phase 18.2: Today, quietly
	preferenceStore   *todayquietly.PreferenceStore // Phase 18.2: Preference capture
	heldEngine        *held.Engine                  // Phase 18.3: Held, not shown
	heldStore         *held.SummaryStore            // Phase 18.3: Summary store
	surfaceEngine     *surface.Engine               // Phase 18.4: Quiet Shift
	surfaceStore      *surface.ActionStore          // Phase 18.4: Action store
	proofEngine       *proof.Engine                 // Phase 18.5: Quiet Proof
	proofAckStore     *proof.AckStore               // Phase 18.5: Ack store
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
	PolicySet *policy.PolicySet
	// Phase 18: Circle detail
	CircleDetail   *loop.CircleResult
	CirclePolicies []circlePolicyInfo
	CirclePolicy   *circlePolicyInfo
	// Phase 18.1: Interest capture
	InterestSubmitted bool
	InterestMessage   string
	// Phase 18.2: Today, quietly
	TodayPage           *todayquietly.TodayQuietlyPage
	PreferenceSubmitted bool
	PreferenceMessage string
	// Phase 18.3: Held, not shown
	HeldSummary *held.HeldSummary
	// Phase 18.4: Quiet Shift
	SurfaceCue         *surface.SurfaceCue
	SurfacePage        *surface.SurfacePage
	SurfaceActionDone  bool
	SurfaceActionMessage string
	// Phase 18.5: Quiet Proof
	ProofSummary *proof.ProofSummary
	ProofCue     *proof.ProofCue
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

	// Create finance executor adapter (Phase 17b)
	// Uses mock connector by default - NO real money moves
	financeExecutor := execexecutor.NewFinanceExecutorAdapter(
		clk,
		emitter,
		func() string { return fmt.Sprintf("fin-%d", clk.Now().UnixNano()) },
		execexecutor.DefaultFinanceExecutorAdapterConfig(),
	)

	execExecutor := execexecutor.NewExecutor(clk, emitter).
		WithEmailExecutor(emailExecutor).
		WithCalendarExecutor(calExecutor).
		WithFinanceExecutor(financeExecutor)

	// Parse templates
	tmpl := template.Must(template.New("").Funcs(template.FuncMap{
		"formatTime": func(t time.Time) string {
			return t.Format("2006-01-02 15:04:05")
		},
		// Phase 18: Template helpers
		"hasPrefix": strings.HasPrefix,
		"slice": func(s string, start, end int) string {
			if start < 0 || end > len(s) || start >= end {
				if len(s) > 0 {
					return s[:1]
				}
				return ""
			}
			return s[start:end]
		},
	}).Parse(templates))

	// Create interest store (Phase 18.1)
	interestStore := interest.NewStore(
		interest.WithClock(clk.Now),
	)

	// Create today quietly engine and store (Phase 18.2)
	todayEngine := todayquietly.NewEngine(clk.Now)
	preferenceStore := todayquietly.NewPreferenceStore(
		todayquietly.WithStoreClock(clk.Now),
	)

	// Create held engine and store (Phase 18.3)
	heldEngine := held.NewEngine(clk.Now)
	heldStore := held.NewSummaryStore(
		held.WithStoreClock(clk.Now),
	)

	// Create surface engine and store (Phase 18.4)
	surfaceEngine := surface.NewEngine(clk.Now)
	surfaceStore := surface.NewActionStore(
		surface.WithStoreClock(clk.Now),
	)

	// Create proof engine and store (Phase 18.5)
	proofEngine := proof.NewEngine()
	proofAckStore := proof.NewAckStore(128)

	// Create server
	server := &Server{
		engine:            engine,
		templates:         tmpl,
		eventEmitter:      emitter,
		clk:               clk,
		execRouter:        execRouter,
		execExecutor:      execExecutor,
		multiCircleConfig: multiCfg,
		identityRepo:      identityRepo,    // Phase 13.1
		interestStore:     interestStore,   // Phase 18.1
		todayEngine:       todayEngine,     // Phase 18.2
		preferenceStore:   preferenceStore, // Phase 18.2
		heldEngine:        heldEngine,      // Phase 18.3
		heldStore:         heldStore,       // Phase 18.3
		surfaceEngine:     surfaceEngine,   // Phase 18.4
		surfaceStore:      surfaceStore,    // Phase 18.4
		proofEngine:       proofEngine,     // Phase 18.5
		proofAckStore:     proofAckStore,   // Phase 18.5
	}

	// Set up routes
	mux := http.NewServeMux()

	// Phase 18: Static files
	mux.Handle("/static/", http.StripPrefix("/static/", http.FileServer(http.Dir("cmd/quantumlife-web/static"))))

	// Phase 18: Public routes
	mux.HandleFunc("/", server.handleLanding)
	mux.HandleFunc("/interest", server.handleInterest)           // Phase 18.1: Interest capture
	mux.HandleFunc("/today", server.handleToday)                 // Phase 18.2: Today, quietly
	mux.HandleFunc("/today/preference", server.handlePreference) // Phase 18.2: Preference capture
	mux.HandleFunc("/held", server.handleHeld)                   // Phase 18.3: Held, not shown
	mux.HandleFunc("/surface", server.handleSurface)             // Phase 18.4: Quiet Shift
	mux.HandleFunc("/surface/hold", server.handleSurfaceHold)    // Phase 18.4: Hold action
	mux.HandleFunc("/surface/why", server.handleSurfaceWhy)      // Phase 18.4: Why action
	mux.HandleFunc("/surface/prefer", server.handleSurfacePrefer) // Phase 18.4: Prefer show_all
	mux.HandleFunc("/proof", server.handleProof)                  // Phase 18.5: Quiet Proof
	mux.HandleFunc("/proof/dismiss", server.handleProofDismiss)   // Phase 18.5: Dismiss proof
	mux.HandleFunc("/demo", server.handleDemo)

	// Phase 18: App routes (authenticated)
	mux.HandleFunc("/app", server.handleAppHome)
	mux.HandleFunc("/app/", server.handleAppHome)
	mux.HandleFunc("/app/circle/", server.handleAppCircle)
	mux.HandleFunc("/app/drafts", server.handleAppDrafts)
	mux.HandleFunc("/app/draft/", server.handleAppDraft)
	mux.HandleFunc("/app/people", server.handleAppPeople)
	mux.HandleFunc("/app/policies", server.handleAppPolicies)

	// Legacy routes (redirect to new app routes)
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

// ============================================================================
// Phase 18: Product Language System - Landing, Demo, and App Handlers
// ============================================================================

// handleLanding serves the public landing page.
// This explains the category, not the features.
func (s *Server) handleLanding(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}

	data := templateData{
		Title:       "The Moment",
		CurrentTime: s.clk.Now().Format("2006-01-02 15:04"),
	}

	s.render(w, "moment", data)
}

// handleInterest handles POST /interest for email capture.
// Phase 18.1: The Moment - a single interaction that earns permission.
func (s *Server) handleInterest(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	email := strings.TrimSpace(r.FormValue("email"))
	if email == "" {
		// Emit invalid event
		s.eventEmitter.Emit(events.Event{
			Type:      events.Phase18_1InterestInvalid,
			Timestamp: s.clk.Now(),
			Metadata:  map[string]string{"reason": "empty_email"},
		})
		// Render page with subtle error
		data := templateData{
			Title:             "The Moment",
			CurrentTime:       s.clk.Now().Format("2006-01-02 15:04"),
			InterestSubmitted: false,
			InterestMessage:   "An email address is needed.",
		}
		s.render(w, "moment", data)
		return
	}

	// Basic email validation
	if !strings.Contains(email, "@") || !strings.Contains(email, ".") {
		s.eventEmitter.Emit(events.Event{
			Type:      events.Phase18_1InterestInvalid,
			Timestamp: s.clk.Now(),
			Metadata:  map[string]string{"reason": "invalid_format"},
		})
		data := templateData{
			Title:             "The Moment",
			CurrentTime:       s.clk.Now().Format("2006-01-02 15:04"),
			InterestSubmitted: false,
			InterestMessage:   "That doesn't look like an email address.",
		}
		s.render(w, "moment", data)
		return
	}

	// Register interest
	isNew, err := s.interestStore.Register(email, "web")
	if err != nil {
		log.Printf("Interest registration error: %v", err)
	}

	if isNew {
		// Emit registered event
		s.eventEmitter.Emit(events.Event{
			Type:      events.Phase18_1InterestRegistered,
			Timestamp: s.clk.Now(),
			Metadata:  map[string]string{"source": "web"},
		})
	} else {
		// Emit duplicate event
		s.eventEmitter.Emit(events.Event{
			Type:      events.Phase18_1InterestDuplicate,
			Timestamp: s.clk.Now(),
			Metadata:  map[string]string{"source": "web"},
		})
	}

	// Same response whether new or duplicate - no information leakage
	data := templateData{
		Title:             "The Moment",
		CurrentTime:       s.clk.Now().Format("2006-01-02 15:04"),
		InterestSubmitted: true,
		InterestMessage:   "Noted. We'll be in touch when this is real.",
	}
	s.render(w, "moment", data)
}

// handleToday serves the "Today, quietly." page.
// Phase 18.2: Recognition + Suppression + Preference
func (s *Server) handleToday(w http.ResponseWriter, r *http.Request) {
	// Build projection input from current state
	// For now, use default input (can be wired to loop results later)
	input := todayquietly.DefaultInput()

	// Generate page deterministically
	page := s.todayEngine.Generate(input)

	// Emit rendered event
	s.eventEmitter.Emit(events.Event{
		Type:      events.Phase18_2TodayRendered,
		Timestamp: s.clk.Now(),
		Metadata: map[string]string{
			"page_hash": page.PageHash,
		},
	})

	// Emit suppression demonstrated event
	s.eventEmitter.Emit(events.Event{
		Type:      events.Phase18_2SuppressionDemonstrated,
		Timestamp: s.clk.Now(),
		Metadata: map[string]string{
			"suppressed_title": page.SuppressedInsight.Title,
		},
	})

	// Phase 18.4: Build surface cue
	// Get user preference from store (default to quiet)
	pref := s.preferenceStore.LatestPreference()
	if pref == "" {
		pref = "quiet"
	}

	surfaceInput := surface.SurfaceInput{
		HeldCategories: map[surface.Category]surface.MagnitudeBucket{
			surface.CategoryMoney: surface.MagnitudeAFew,
			surface.CategoryTime:  surface.MagnitudeAFew,
			surface.CategoryWork:  surface.MagnitudeSeveral,
		},
		UserPreference:    pref,
		SuppressedFinance: true,
		SuppressedWork:    true,
		Now:               s.clk.Now(),
	}
	surfaceCue := s.surfaceEngine.BuildCue(surfaceInput)

	// Emit surface cue computed event
	if surfaceCue.Available {
		s.eventEmitter.Emit(events.Event{
			Type:      events.Phase18_4SurfaceCueComputed,
			Timestamp: s.clk.Now(),
			Metadata: map[string]string{
				"cue_hash":  surfaceCue.Hash,
				"available": "true",
			},
		})
	}

	// Phase 18.5: Build proof cue
	// Proof shows restraint - how much we chose not to interrupt
	proofInput := proof.ProofInput{
		SuppressedByCategory: map[proof.Category]int{
			proof.CategoryMoney: 2,
			proof.CategoryTime:  1,
			proof.CategoryWork:  3,
		},
		PreferenceQuiet: pref == "quiet",
		Period:          "week",
	}
	proofSummary := s.proofEngine.BuildProof(proofInput)
	hasRecentAck := s.proofAckStore.HasRecent(proofSummary.Hash)
	proofCue := s.proofEngine.BuildCue(proofSummary, hasRecentAck)

	// Emit proof computed event
	s.eventEmitter.Emit(events.Event{
		Type:      events.Phase18_5ProofComputed,
		Timestamp: s.clk.Now(),
		Metadata: map[string]string{
			"proof_hash": proofSummary.Hash,
			"magnitude":  string(proofSummary.Magnitude),
		},
	})

	// Phase 18.5.1: Single whisper rule
	// Show at most ONE whisper cue on /today.
	// Priority: surface cue > proof cue
	// If surface is available, hide proof cue (proof accessible via /surface).
	var displaySurfaceCue *surface.SurfaceCue
	var displayProofCue *proof.ProofCue
	if surfaceCue.Available {
		displaySurfaceCue = &surfaceCue
		// Proof cue hidden - accessible via /surface link
	} else if proofCue.Available {
		displayProofCue = &proofCue
	}

	data := templateData{
		Title:       "Today, quietly.",
		CurrentTime: s.clk.Now().Format("2006-01-02 15:04"),
		TodayPage:   &page,
		SurfaceCue:  displaySurfaceCue,
		ProofCue:    displayProofCue,
	}

	s.render(w, "today", data)
}

// handlePreference handles POST /today/preference for preference capture.
// Phase 18.2: Preference capture with confirmation.
func (s *Server) handlePreference(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	mode := strings.TrimSpace(r.FormValue("mode"))
	if mode != "quiet" && mode != "show_all" {
		// Invalid mode, redirect back
		http.Redirect(w, r, "/today", http.StatusFound)
		return
	}

	// Record preference
	isNew, err := s.preferenceStore.Record(mode, "web")
	if err != nil {
		log.Printf("Preference recording error: %v", err)
	}

	// Emit event
	s.eventEmitter.Emit(events.Event{
		Type:      events.Phase18_2PreferenceRecorded,
		Timestamp: s.clk.Now(),
		Metadata: map[string]string{
			"mode":   mode,
			"is_new": fmt.Sprintf("%t", isNew),
			"source": "web",
		},
	})

	// Generate page for confirmation
	input := todayquietly.DefaultInput()
	page := s.todayEngine.Generate(input)

	data := templateData{
		Title:               "Today, quietly.",
		CurrentTime:         s.clk.Now().Format("2006-01-02 15:04"),
		TodayPage:           &page,
		PreferenceSubmitted: true,
		PreferenceMessage:   todayquietly.ConfirmationMessage(mode),
	}

	s.render(w, "today", data)
}

// handleHeld serves the "Held, not shown" page.
// Phase 18.3: The Proof of Care
func (s *Server) handleHeld(w http.ResponseWriter, r *http.Request) {
	// Build held input from current state
	// For now, use default input (can be wired to loop results later)
	input := held.DefaultInput()

	// Generate summary deterministically
	summary := s.heldEngine.Generate(input)

	// Record summary hash (for replay verification)
	if err := s.heldStore.Record(summary); err != nil {
		log.Printf("Held store error: %v", err)
	}

	// Emit computed event
	s.eventEmitter.Emit(events.Event{
		Type:      events.Phase18_3HeldComputed,
		Timestamp: s.clk.Now(),
		Metadata: map[string]string{
			"summary_hash": summary.Hash,
			"magnitude":    summary.Magnitude,
		},
	})

	// Emit presented event
	s.eventEmitter.Emit(events.Event{
		Type:      events.Phase18_3HeldPresented,
		Timestamp: s.clk.Now(),
		Metadata: map[string]string{
			"summary_hash": summary.Hash,
		},
	})

	data := templateData{
		Title:       "Held",
		CurrentTime: s.clk.Now().Format("2006-01-02 15:04"),
		HeldSummary: &summary,
	}

	s.render(w, "held", data)
}

// handleSurface serves the surface page showing one abstract item.
// Phase 18.4: Quiet Shift - view surfaced item.
func (s *Server) handleSurface(w http.ResponseWriter, r *http.Request) {
	// Get user preference
	pref := s.preferenceStore.LatestPreference()
	if pref == "" {
		pref = "quiet"
	}

	// Build surface input
	surfaceInput := surface.SurfaceInput{
		HeldCategories: map[surface.Category]surface.MagnitudeBucket{
			surface.CategoryMoney: surface.MagnitudeAFew,
			surface.CategoryTime:  surface.MagnitudeAFew,
			surface.CategoryWork:  surface.MagnitudeSeveral,
		},
		UserPreference:    pref,
		SuppressedFinance: true,
		SuppressedWork:    true,
		Now:               s.clk.Now(),
	}

	// Check if ?why=1 query param is present
	showExplain := r.URL.Query().Get("why") == "1"

	// Generate surface page
	surfacePage := s.surfaceEngine.BuildSurfacePage(surfaceInput, showExplain)

	// Record view action
	if err := s.surfaceStore.RecordViewed("", surfacePage.Item.ItemKeyHash); err != nil {
		log.Printf("Surface store error: %v", err)
	}

	// Emit page rendered event
	s.eventEmitter.Emit(events.Event{
		Type:      events.Phase18_4SurfacePageRendered,
		Timestamp: s.clk.Now(),
		Metadata: map[string]string{
			"page_hash":     surfacePage.Hash,
			"item_category": string(surfacePage.Item.Category),
			"show_explain":  fmt.Sprintf("%t", showExplain),
		},
	})

	// Emit viewed action event
	s.eventEmitter.Emit(events.Event{
		Type:      events.Phase18_4SurfaceActionViewed,
		Timestamp: s.clk.Now(),
		Metadata: map[string]string{
			"item_key_hash": surfacePage.Item.ItemKeyHash,
		},
	})

	data := templateData{
		Title:       "Something you could look at",
		CurrentTime: s.clk.Now().Format("2006-01-02 15:04"),
		SurfacePage: &surfacePage,
	}

	s.render(w, "surface", data)
}

// handleSurfaceHold handles POST /surface/hold - marks item as held.
// Phase 18.4: Hold action.
func (s *Server) handleSurfaceHold(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	itemKeyHash := r.FormValue("item_key_hash")

	// Record hold action
	if err := s.surfaceStore.RecordHeld("", itemKeyHash); err != nil {
		log.Printf("Surface store error: %v", err)
	}

	// Emit held action event
	s.eventEmitter.Emit(events.Event{
		Type:      events.Phase18_4SurfaceActionHeld,
		Timestamp: s.clk.Now(),
		Metadata: map[string]string{
			"item_key_hash": itemKeyHash,
		},
	})

	// Redirect to /today with confirmation
	http.Redirect(w, r, "/today?held=1", http.StatusFound)
}

// handleSurfaceWhy handles POST /surface/why - shows explainability.
// Phase 18.4: Why action (redirects to surface with ?why=1).
func (s *Server) handleSurfaceWhy(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	itemKeyHash := r.FormValue("item_key_hash")

	// Record why action
	if err := s.surfaceStore.RecordWhy("", itemKeyHash); err != nil {
		log.Printf("Surface store error: %v", err)
	}

	// Emit why action event
	s.eventEmitter.Emit(events.Event{
		Type:      events.Phase18_4SurfaceActionWhy,
		Timestamp: s.clk.Now(),
		Metadata: map[string]string{
			"item_key_hash": itemKeyHash,
		},
	})

	// Redirect to /surface with ?why=1
	http.Redirect(w, r, "/surface?why=1", http.StatusFound)
}

// handleSurfacePrefer handles POST /surface/prefer - sets preference to show_all.
// Phase 18.4: Prefer show_all action.
func (s *Server) handleSurfacePrefer(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	itemKeyHash := r.FormValue("item_key_hash")

	// Record prefer action
	if err := s.surfaceStore.RecordPreferShowAll("", itemKeyHash); err != nil {
		log.Printf("Surface store error: %v", err)
	}

	// Update preference to show_all (reuse Phase 18.2 store)
	if _, err := s.preferenceStore.Record("show_all", "surface"); err != nil {
		log.Printf("Preference store error: %v", err)
	}

	// Emit prefer action event
	s.eventEmitter.Emit(events.Event{
		Type:      events.Phase18_4SurfaceActionPreferShowAll,
		Timestamp: s.clk.Now(),
		Metadata: map[string]string{
			"item_key_hash": itemKeyHash,
		},
	})

	// Redirect to /today
	http.Redirect(w, r, "/today", http.StatusFound)
}

// handleProof serves the "Quiet, kept." proof page.
// Phase 18.5: Quiet Proof - Restraint Ledger
func (s *Server) handleProof(w http.ResponseWriter, r *http.Request) {
	// Get user preference
	pref := s.preferenceStore.LatestPreference()
	if pref == "" {
		pref = "quiet"
	}

	// Build proof input
	proofInput := proof.ProofInput{
		SuppressedByCategory: map[proof.Category]int{
			proof.CategoryMoney: 2,
			proof.CategoryTime:  1,
			proof.CategoryWork:  3,
		},
		PreferenceQuiet: pref == "quiet",
		Period:          "week",
	}

	// Generate proof summary
	proofSummary := s.proofEngine.BuildProof(proofInput)

	// Record viewed acknowledgement
	if err := s.proofAckStore.Record(proof.AckViewed, proofSummary.Hash, s.clk.Now()); err != nil {
		log.Printf("Proof ack store error: %v", err)
	}

	// Emit proof viewed event
	s.eventEmitter.Emit(events.Event{
		Type:      events.Phase18_5ProofViewed,
		Timestamp: s.clk.Now(),
		Metadata: map[string]string{
			"proof_hash": proofSummary.Hash,
			"magnitude":  string(proofSummary.Magnitude),
		},
	})

	data := templateData{
		Title:        "Quiet, kept.",
		CurrentTime:  s.clk.Now().Format("2006-01-02 15:04"),
		ProofSummary: &proofSummary,
	}

	s.render(w, "proof", data)
}

// handleProofDismiss handles POST /proof/dismiss - dismisses the proof.
// Phase 18.5: Dismiss proof action.
func (s *Server) handleProofDismiss(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	proofHash := r.FormValue("proof_hash")

	// Record dismissed acknowledgement
	if err := s.proofAckStore.Record(proof.AckDismissed, proofHash, s.clk.Now()); err != nil {
		log.Printf("Proof ack store error: %v", err)
	}

	// Emit proof dismissed event
	s.eventEmitter.Emit(events.Event{
		Type:      events.Phase18_5ProofDismissed,
		Timestamp: s.clk.Now(),
		Metadata: map[string]string{
			"proof_hash": proofHash,
		},
	})

	// Redirect to /today
	http.Redirect(w, r, "/today", http.StatusFound)
}

// handleDemo serves the deterministic demo page.
// Same seed = same output, always.
func (s *Server) handleDemo(w http.ResponseWriter, r *http.Request) {
	// Run the loop with demo context
	result := s.engine.Run(context.Background(), loop.RunOptions{
		IncludeMockData: true,
	})

	data := templateData{
		Title:       "Demo",
		CurrentTime: s.clk.Now().Format("2006-01-02 15:04"),
		RunResult:   &result,
		NeedsYou:    &result.NeedsYou,
		Circles:     result.Circles,
	}

	s.render(w, "demo", data)
}

// handleAppHome shows the app home page ("Nothing Needs You" or items).
func (s *Server) handleAppHome(w http.ResponseWriter, r *http.Request) {
	// Check for exact paths
	if r.URL.Path != "/app" && r.URL.Path != "/app/" {
		http.NotFound(w, r)
		return
	}

	// Run the loop
	result := s.engine.Run(context.Background(), loop.RunOptions{
		IncludeMockData: *mockData,
	})

	data := templateData{
		Title:       "Home",
		CurrentTime: s.clk.Now().Format("2006-01-02 15:04"),
		RunResult:   &result,
		NeedsYou:    &result.NeedsYou,
		Circles:     result.Circles,
	}

	s.render(w, "app-home", data)
}

// handleAppCircle shows a circle detail page.
func (s *Server) handleAppCircle(w http.ResponseWriter, r *http.Request) {
	circleID := strings.TrimPrefix(r.URL.Path, "/app/circle/")
	if circleID == "" {
		http.Redirect(w, r, "/app", http.StatusFound)
		return
	}

	// Run the loop
	result := s.engine.Run(context.Background(), loop.RunOptions{
		IncludeMockData: *mockData,
	})

	// Find the specific circle
	var circleResult *loop.CircleResult
	for i := range result.Circles {
		if string(result.Circles[i].CircleID) == circleID {
			circleResult = &result.Circles[i]
			break
		}
	}

	// Get people from identity repo
	var people []personInfo
	if s.identityRepo != nil {
		persons := s.identityRepo.ListPersons()
		for _, p := range persons {
			people = append(people, personInfo{
				ID:    string(p.ID()),
				Label: s.identityRepo.PersonLabel(p.ID()),
			})
		}
	}

	data := templateData{
		Title:        "Circle: " + circleID,
		CurrentTime:  s.clk.Now().Format("2006-01-02 15:04"),
		RunResult:    &result,
		Circles:      result.Circles,
		People:       people,
		CircleDetail: circleResult,
	}

	s.render(w, "app-circle", data)
}

// handleAppDrafts shows all pending drafts.
func (s *Server) handleAppDrafts(w http.ResponseWriter, r *http.Request) {
	// Run the loop to get pending drafts
	result := s.engine.Run(context.Background(), loop.RunOptions{
		IncludeMockData: *mockData,
	})

	data := templateData{
		Title:         "Drafts",
		CurrentTime:   s.clk.Now().Format("2006-01-02 15:04"),
		PendingDrafts: result.NeedsYou.PendingDrafts,
	}

	s.render(w, "app-drafts", data)
}

// handleAppDraft shows a specific draft for review.
func (s *Server) handleAppDraft(w http.ResponseWriter, r *http.Request) {
	draftID := strings.TrimPrefix(r.URL.Path, "/app/draft/")
	if draftID == "" {
		http.Redirect(w, r, "/app/drafts", http.StatusFound)
		return
	}

	// Find the draft
	var foundDraft *draft.Draft
	result := s.engine.Run(context.Background(), loop.RunOptions{
		IncludeMockData: *mockData,
	})

	for i := range result.NeedsYou.PendingDrafts {
		if string(result.NeedsYou.PendingDrafts[i].DraftID) == draftID {
			foundDraft = &result.NeedsYou.PendingDrafts[i]
			break
		}
	}

	data := templateData{
		Title:       "Review Draft",
		CurrentTime: s.clk.Now().Format("2006-01-02 15:04"),
		Draft:       foundDraft,
	}

	s.render(w, "app-draft", data)
}

// handleAppPeople shows the identity graph.
func (s *Server) handleAppPeople(w http.ResponseWriter, r *http.Request) {
	var people []personInfo
	var stats *identityStats

	if s.identityRepo != nil {
		persons := s.identityRepo.ListPersons()
		for _, p := range persons {
			people = append(people, personInfo{
				ID:           string(p.ID()),
				Label:        s.identityRepo.PersonLabel(p.ID()),
				PrimaryEmail: s.identityRepo.PrimaryEmail(p.ID()),
			})
		}
		stats = &identityStats{
			PersonCount:       s.identityRepo.CountByType(identity.EntityTypePerson),
			OrganizationCount: s.identityRepo.CountByType(identity.EntityTypeOrganization),
		}
	}

	data := templateData{
		Title:         "People",
		CurrentTime:   s.clk.Now().Format("2006-01-02 15:04"),
		People:        people,
		IdentityStats: stats,
	}

	s.render(w, "app-people", data)
}

// handleAppPolicies shows circle policies.
func (s *Server) handleAppPolicies(w http.ResponseWriter, r *http.Request) {
	var circlePolicies []circleConfigInfo

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
			circlePolicies = append(circlePolicies, info)
		}
	}

	data := templateData{
		Title:         "Policies",
		CurrentTime:   s.clk.Now().Format("2006-01-02 15:04"),
		CircleConfigs: circlePolicies,
	}

	s.render(w, "app-policies", data)
}

// ============================================================================
// Legacy Handlers (Phase 1-17)
// ============================================================================

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
// Phase 18: Product Language System - uses external CSS files.
const templates = `
{{/* ================================================================
     Phase 18: Base template with external CSS
     ================================================================ */}}
{{define "base18"}}
<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="utf-8">
    <meta name="viewport" content="width=device-width, initial-scale=1">
    <title>{{.Title}} - QuantumLife</title>
    <link rel="stylesheet" href="/static/tokens.css">
    <link rel="stylesheet" href="/static/reset.css">
    <link rel="stylesheet" href="/static/app.css">
</head>
<body>
    <div class="page">
        {{template "page-content" .}}
    </div>
</body>
</html>
{{end}}

{{/* ================================================================
     Landing Page - Public
     ================================================================ */}}
{{define "landing"}}
{{template "base18" .}}
{{end}}

{{define "landing-content"}}
<div class="hero">
    <h1 class="hero-title">Nothing Needs You</h1>
    <p class="hero-subtitle">
        QuantumLife is a personal operating system that handles life administration
        in the background. When it's working, you don't notice.
    </p>
    <div class="hero-cta">
        <a href="/demo" class="btn btn-primary">Try the Demo</a>
        <a href="/app" class="btn btn-secondary">Enter App</a>
    </div>
</div>
<p class="category-line">
    Not a todo list. Not an inbox. Not an assistant. A system that handles, so you don't have to.
</p>
{{end}}

{{/* ================================================================
     Phase 18.1: The Moment - Emotional landing that earns trust
     ================================================================ */}}
{{define "moment"}}
{{template "base18" .}}
{{end}}

{{define "moment-content"}}
<div class="moment">
    {{/* ═══════════════════════════════════════════════════════════
         Moment 1: Arrival
         ═══════════════════════════════════════════════════════════ */}}
    <section class="moment-section moment-arrival">
        <h1 class="moment-headline">Nothing needs you.</h1>
        <p class="moment-subtext">QuantumLife exists so you don't have to keep checking.</p>
    </section>

    {{/* ═══════════════════════════════════════════════════════════
         Moment 2: Recognition
         ═══════════════════════════════════════════════════════════ */}}
    <section class="moment-section moment-recognition">
        <p class="moment-para">
            You already manage more than most systems understand.
            Emails, family, money, work, health.
            None of them agree on what matters now.
        </p>
        <p class="moment-emphasis">
            QuantumLife doesn't add tasks.<br>
            It removes unnecessary ones.
        </p>
    </section>

    {{/* ═══════════════════════════════════════════════════════════
         Moment 3: The Promise
         ═══════════════════════════════════════════════════════════ */}}
    <section class="moment-section moment-promise">
        <ul class="moment-pillars">
            <li><strong>Calm</strong> — nothing interrupts you without reason</li>
            <li><strong>Certainty</strong> — every action is explainable</li>
            <li><strong>Consent</strong> — nothing acts without you</li>
        </ul>
        <p class="moment-regret">
            QuantumLife only surfaces what creates future regret if ignored.
        </p>
    </section>

    {{/* ═══════════════════════════════════════════════════════════
         Moment 4: Permission (The Only Interaction)
         ═══════════════════════════════════════════════════════════ */}}
    <section class="moment-section moment-permission">
        {{if .InterestSubmitted}}
        <p class="moment-confirmation">{{.InterestMessage}}</p>
        {{else}}
        <p class="moment-question">
            Would you like a life where nothing needs you — unless it truly does?
        </p>
        <form action="/interest" method="POST" class="moment-form">
            <label for="email" class="moment-label">Early access (no spam, no automation, no urgency)</label>
            {{if .InterestMessage}}
            <p class="moment-error">{{.InterestMessage}}</p>
            {{end}}
            <div class="moment-input-row">
                <input type="email" id="email" name="email" class="moment-input" placeholder="you@example.com" required>
                <button type="submit" class="moment-button">Notify me when this is real.</button>
            </div>
        </form>
        {{end}}
    </section>

    {{/* Subtle link to Today, quietly */}}
    <footer class="moment-footer">
        <a href="/today" class="moment-subtle-link">See what today looks like</a>
    </footer>
</div>
{{end}}

{{/* ================================================================
     Phase 18.2: Today, quietly - Recognition + Suppression + Preference
     ================================================================ */}}
{{define "today"}}
{{template "base18" .}}
{{end}}

{{define "today-content"}}
<div class="today-quietly">
    {{/* Header */}}
    <header class="today-header">
        <h1 class="today-title">{{.TodayPage.Title}}</h1>
        <p class="today-subtitle">{{.TodayPage.Subtitle}}</p>
    </header>

    {{/* Preference confirmation (if submitted) */}}
    {{if .PreferenceSubmitted}}
    <div class="today-confirmation">
        <p class="today-confirmation-text">{{.PreferenceMessage}}</p>
    </div>
    {{end}}

    {{/* Recognition sentence */}}
    <section class="today-section today-recognition">
        <p class="today-recognition-text">{{.TodayPage.Recognition}}</p>
    </section>

    {{/* Three quiet observations */}}
    <section class="today-section today-observations">
        <ul class="today-observations-list">
            {{range .TodayPage.Observations}}
            <li class="today-observation">{{.Text}}</li>
            {{end}}
        </ul>
    </section>

    {{/* Suppressed insight */}}
    <section class="today-section today-suppressed">
        <div class="today-suppressed-divider"></div>
        <p class="today-suppressed-title">{{.TodayPage.SuppressedInsight.Title}}</p>
        <p class="today-suppressed-reason">{{.TodayPage.SuppressedInsight.Reason}}</p>
    </section>

    {{/* Permission pivot (preference form) */}}
    {{if not .PreferenceSubmitted}}
    <section class="today-section today-permission">
        <p class="today-permission-prompt">{{.TodayPage.PermissionPivot.Prompt}}</p>
        <form action="/today/preference" method="POST" class="today-preference-form">
            {{range .TodayPage.PermissionPivot.Choices}}
            <label class="today-preference-option">
                <input type="radio" name="mode" value="{{.Mode}}" {{if .IsDefault}}checked{{end}}>
                <span class="today-preference-label">{{.Label}}</span>
            </label>
            {{end}}
            <button type="submit" class="today-preference-button">Save preference</button>
        </form>
    </section>
    {{end}}

    {{/* Phase 18.4: Quiet Shift - Subtle availability cue */}}
    {{if and .SurfaceCue .SurfaceCue.Available}}
    <section class="quiet-shift">
        <p class="quiet-shift-cue">{{.SurfaceCue.CueText}}</p>
        <a href="/surface" class="quiet-shift-link">{{.SurfaceCue.LinkText}}</a>
    </section>
    {{end}}

    {{/* Phase 18.5: Quiet Proof - Restraint ledger cue */}}
    {{if and .ProofCue .ProofCue.Available}}
    <section class="quiet-proof-cue">
        <p class="quiet-proof-cue-text">{{.ProofCue.CueText}}</p>
        <a href="/proof" class="quiet-proof-cue-link">{{.ProofCue.LinkText}}</a>
    </section>
    {{end}}

    {{/* Subtle links */}}
    <footer class="today-footer">
        <a href="/held" class="today-subtle-link">What are you holding for me?</a>
        <span class="today-footer-divider">·</span>
        <a href="/" class="today-back-link">Back to home</a>
    </footer>
</div>
{{end}}

{{/* ================================================================
     Phase 18.3: Held, not shown - The Proof of Care
     ================================================================ */}}
{{define "held"}}
{{template "base18" .}}
{{end}}

{{define "held-content"}}
<div class="held">
    <header class="held-header">
        <h1 class="held-title">Held, quietly.</h1>
    </header>

    <section class="held-statement">
        <p class="held-statement-text">{{.HeldSummary.Statement}}</p>
    </section>

    {{if .HeldSummary.Categories}}
    <section class="held-categories">
        <ul class="held-categories-list">
            {{range .HeldSummary.Categories}}
            <li class="held-category">{{.Category}}</li>
            {{end}}
        </ul>
    </section>
    {{end}}

    <section class="held-reassurance">
        <p class="held-reassurance-text">We're watching, so you don't have to.</p>
    </section>

    <footer class="held-footer">
        <a href="/today" class="held-back-link">Back to today</a>
    </footer>
</div>
{{end}}

{{/* ================================================================
     Phase 18.4: Quiet Shift - Subtle Availability
     ================================================================ */}}
{{define "surface"}}
{{template "base18" .}}
{{end}}

{{define "surface-content"}}
<div class="surface">
    <header class="surface-header">
        <h1 class="surface-title">{{.SurfacePage.Title}}</h1>
        <p class="surface-subtitle">{{.SurfacePage.Subtitle}}</p>
    </header>

    <section class="surface-item">
        <div class="surface-item-category">{{.SurfacePage.Item.Category}}</div>
        <p class="surface-item-reason">{{.SurfacePage.Item.ReasonSummary}}</p>
        <p class="surface-item-horizon">Relevant: {{.SurfacePage.Item.Horizon}}</p>
    </section>

    {{if .SurfacePage.ShowExplain}}
    <section class="surface-explain">
        <h2 class="surface-explain-title">Why we noticed</h2>
        <ul class="surface-explain-list">
            {{range .SurfacePage.Item.Explain}}
            <li class="surface-explain-item">{{.Text}}</li>
            {{end}}
        </ul>
    </section>
    {{end}}

    <section class="surface-actions">
        <form action="/surface/hold" method="POST" class="surface-action-form">
            <input type="hidden" name="item_key_hash" value="{{.SurfacePage.Item.ItemKeyHash}}">
            <button type="submit" class="surface-action-button surface-action-hold">Hold this for later</button>
        </form>

        {{if not .SurfacePage.ShowExplain}}
        <form action="/surface/why" method="POST" class="surface-action-form">
            <input type="hidden" name="item_key_hash" value="{{.SurfacePage.Item.ItemKeyHash}}">
            <button type="submit" class="surface-action-button surface-action-why">Show me why</button>
        </form>
        {{end}}

        <form action="/surface/prefer" method="POST" class="surface-action-form">
            <input type="hidden" name="item_key_hash" value="{{.SurfacePage.Item.ItemKeyHash}}">
            <button type="submit" class="surface-action-button surface-action-prefer">I want to see everything</button>
        </form>
    </section>

    {{/* Phase 18.5.1: Subtle proof link - routes proof from /surface */}}
    <section class="surface-proof-link">
        <a href="/proof" class="surface-proof-link-text">Quiet, kept.</a>
    </section>

    <footer class="surface-footer">
        <a href="/today" class="surface-back-link">Back to today</a>
    </footer>
</div>
{{end}}

{{/* ================================================================
     Phase 18.5: Quiet Proof - Restraint Ledger
     ================================================================ */}}
{{define "proof"}}
{{template "base18" .}}
{{end}}

{{define "proof-content"}}
<div class="proof">
    <header class="proof-header">
        <h1 class="proof-title">Quiet, kept.</h1>
        <p class="proof-subtitle">Proof that silence is intentional.</p>
    </header>

    {{if eq .ProofSummary.Magnitude "nothing"}}
    <section class="proof-nothing">
        <p class="proof-nothing-text">Nothing was held.</p>
    </section>
    {{else}}
    <section class="proof-statement">
        <p class="proof-statement-text">{{.ProofSummary.Statement}}</p>
    </section>

    {{if .ProofSummary.Categories}}
    <section class="proof-categories">
        <ul class="proof-categories-list">
            {{range .ProofSummary.Categories}}
            <li class="proof-category">{{.}}</li>
            {{end}}
        </ul>
    </section>
    {{end}}

    {{if .ProofSummary.WhyLine}}
    <section class="proof-why">
        <p class="proof-why-text">{{.ProofSummary.WhyLine}}</p>
    </section>
    {{end}}

    <section class="proof-actions">
        <form action="/proof/dismiss" method="POST" class="proof-dismiss-form">
            <input type="hidden" name="proof_hash" value="{{.ProofSummary.Hash}}">
            <button type="submit" class="proof-dismiss-link">Dismiss</button>
        </form>
    </section>
    {{end}}

    <footer class="proof-footer">
        <a href="/today" class="proof-back-link">Back to today</a>
    </footer>
</div>
{{end}}

{{define "page-content"}}
{{if eq .Title "Held"}}
    {{template "held-content" .}}
{{else if eq .Title "Something you could look at"}}
    {{template "surface-content" .}}
{{else if eq .Title "Quiet, kept."}}
    {{template "proof-content" .}}
{{else if eq .Title "Today, quietly."}}
    {{template "today-content" .}}
{{else if eq .Title "The Moment"}}
    {{template "moment-content" .}}
{{else if eq .Title "Nothing Needs You"}}
    {{template "landing-content" .}}
{{else if eq .Title "Demo"}}
    {{template "demo-content" .}}
{{else if eq .Title "Home"}}
    <header class="header">
        <div class="header-inner container">
            <a href="/app" class="header-logo">QuantumLife</a>
            <nav class="header-nav">
                <a href="/app" class="header-nav-link header-nav-link--active">Home</a>
                <a href="/app/drafts" class="header-nav-link">Drafts</a>
                <a href="/app/people" class="header-nav-link">People</a>
                <a href="/app/policies" class="header-nav-link">Policies</a>
            </nav>
        </div>
    </header>
    <div class="page-content">
        <div class="container">
            {{template "app-home-content" .}}
        </div>
    </div>
    <footer class="footer">
        <div class="container footer-inner">
            <span class="footer-text">QuantumLife</span>
            <div class="footer-links">
                <a href="#" class="footer-link">Audit trail</a>
            </div>
        </div>
    </footer>
{{else if hasPrefix .Title "Circle:"}}
    <header class="header">
        <div class="header-inner container">
            <a href="/app" class="header-logo">QuantumLife</a>
            <nav class="header-nav">
                <a href="/app" class="header-nav-link">Home</a>
                <a href="/app/drafts" class="header-nav-link">Drafts</a>
                <a href="/app/people" class="header-nav-link">People</a>
                <a href="/app/policies" class="header-nav-link">Policies</a>
            </nav>
        </div>
    </header>
    <div class="page-content">
        <div class="container">
            {{template "app-circle-content" .}}
        </div>
    </div>
{{else if eq .Title "Drafts"}}
    <header class="header">
        <div class="header-inner container">
            <a href="/app" class="header-logo">QuantumLife</a>
            <nav class="header-nav">
                <a href="/app" class="header-nav-link">Home</a>
                <a href="/app/drafts" class="header-nav-link header-nav-link--active">Drafts</a>
                <a href="/app/people" class="header-nav-link">People</a>
                <a href="/app/policies" class="header-nav-link">Policies</a>
            </nav>
        </div>
    </header>
    <div class="page-content">
        <div class="container">
            {{template "app-drafts-content" .}}
        </div>
    </div>
{{else if eq .Title "Review Draft"}}
    <header class="header">
        <div class="header-inner container">
            <a href="/app" class="header-logo">QuantumLife</a>
            <nav class="header-nav">
                <a href="/app" class="header-nav-link">Home</a>
                <a href="/app/drafts" class="header-nav-link header-nav-link--active">Drafts</a>
                <a href="/app/people" class="header-nav-link">People</a>
                <a href="/app/policies" class="header-nav-link">Policies</a>
            </nav>
        </div>
    </header>
    <div class="page-content">
        <div class="container">
            {{template "app-draft-content" .}}
        </div>
    </div>
{{else if eq .Title "People"}}
    <header class="header">
        <div class="header-inner container">
            <a href="/app" class="header-logo">QuantumLife</a>
            <nav class="header-nav">
                <a href="/app" class="header-nav-link">Home</a>
                <a href="/app/drafts" class="header-nav-link">Drafts</a>
                <a href="/app/people" class="header-nav-link header-nav-link--active">People</a>
                <a href="/app/policies" class="header-nav-link">Policies</a>
            </nav>
        </div>
    </header>
    <div class="page-content">
        <div class="container">
            {{template "app-people-content" .}}
        </div>
    </div>
{{else if eq .Title "Policies"}}
    <header class="header">
        <div class="header-inner container">
            <a href="/app" class="header-logo">QuantumLife</a>
            <nav class="header-nav">
                <a href="/app" class="header-nav-link">Home</a>
                <a href="/app/drafts" class="header-nav-link">Drafts</a>
                <a href="/app/people" class="header-nav-link">People</a>
                <a href="/app/policies" class="header-nav-link header-nav-link--active">Policies</a>
            </nav>
        </div>
    </header>
    <div class="page-content">
        <div class="container">
            {{template "app-policies-content" .}}
        </div>
    </div>
{{else}}
    {{template "legacy-content" .}}
{{end}}
{{end}}

{{/* ================================================================
     Demo Page - Public, Deterministic
     ================================================================ */}}
{{define "demo"}}
{{template "base18" .}}
{{end}}

{{define "demo-content"}}
<header class="header">
    <div class="header-inner container">
        <a href="/" class="header-logo">QuantumLife</a>
        <span class="demo-badge">Demo</span>
    </div>
</header>
<div class="page-content">
    <div class="container">
        <div class="demo-notice">
            This is a deterministic demo. Every time you run it, you see the same output. This is simulated data.
        </div>

        {{if .NeedsYou}}
            {{if .NeedsYou.IsQuiet}}
            <div class="empty-state">
                <h2 class="empty-state-title">Nothing needs you.</h2>
                <p class="empty-state-body">QuantumLife handled everything this week.</p>
            </div>
            {{else}}
            <div class="section">
                <h2 class="section-title">{{.NeedsYou.TotalItems}} item(s) need you</h2>
                {{range .NeedsYou.PendingDrafts}}
                <a href="#" class="needs-you-item needs-you-item--needs-you">
                    <div class="needs-you-item-title">{{.DraftType}}</div>
                    <div class="needs-you-item-meta">Circle: {{.CircleID}}</div>
                </a>
                {{end}}
            </div>
            {{end}}
        {{end}}

        {{if .Circles}}
        <div class="section">
            <h2 class="section-title">Your Circles</h2>
            <div class="circles-row">
                {{range .Circles}}
                <div class="circle-card">
                    <div class="circle-card-title">{{.CircleName}}</div>
                    <div class="circle-card-meta">{{.ObligationCount}} obligations</div>
                    {{if gt .DraftCount 0}}
                    <span class="circle-card-badge">{{.DraftCount}} drafts</span>
                    {{end}}
                </div>
                {{end}}
            </div>
        </div>
        {{end}}

        <div class="mt-8 text-center">
            <a href="/app" class="btn btn-primary">Enter App</a>
        </div>
    </div>
</div>
{{end}}

{{/* ================================================================
     App Home - "Nothing Needs You" or items
     ================================================================ */}}
{{define "app-home"}}
{{template "base18" .}}
{{end}}

{{define "app-home-content"}}
{{if .NeedsYou}}
    {{if .NeedsYou.IsQuiet}}
    <div class="empty-state">
        <h2 class="empty-state-title">Nothing needs you.</h2>
        <p class="empty-state-body">QuantumLife handled everything this week.</p>
    </div>
    {{else}}
    <div class="section">
        <h2 class="section-title">{{.NeedsYou.TotalItems}} item(s) need you</h2>
        {{range .NeedsYou.PendingDrafts}}
        <a href="/app/draft/{{.DraftID}}" class="needs-you-item needs-you-item--needs-you">
            <div class="needs-you-item-title">{{.DraftType}}</div>
            <div class="needs-you-item-meta">Circle: {{.CircleID}} | Created: {{formatTime .CreatedAt}}</div>
        </a>
        {{end}}
        {{range .NeedsYou.ActiveInterruptions}}
        <div class="needs-you-item needs-you-item--urgent">
            <div class="needs-you-item-title">{{.Level}}</div>
            <div class="needs-you-item-meta">{{.Trigger}}</div>
        </div>
        {{end}}
    </div>
    {{end}}
{{end}}

{{if .Circles}}
<div class="section">
    <h2 class="section-title">Your Circles</h2>
    <div class="circles-row">
        {{range .Circles}}
        <a href="/app/circle/{{.CircleID}}" class="circle-card">
            <div class="circle-card-title">{{.CircleName}}</div>
            <div class="circle-card-meta">{{.ObligationCount}} obligations</div>
            {{if gt .DraftCount 0}}
            <span class="circle-card-badge">{{.DraftCount}} drafts</span>
            {{end}}
        </a>
        {{end}}
    </div>
</div>
{{end}}
{{end}}

{{/* ================================================================
     App Circle Detail
     ================================================================ */}}
{{define "app-circle"}}
{{template "base18" .}}
{{end}}

{{define "app-circle-content"}}
<h2 class="section-title">{{.Title}}</h2>

{{if .CircleDetail}}
<div class="card mb-4">
    <div class="card-meta">
        {{.CircleDetail.ObligationCount}} obligations |
        {{.CircleDetail.InterruptionCount}} interruptions |
        {{.CircleDetail.DraftCount}} drafts
    </div>
</div>
{{end}}

{{if .People}}
<div class="section">
    <h3 class="section-title">People in this circle</h3>
    {{range .People}}
    <div class="identity-card">
        <div class="identity-avatar">{{slice .Label 0 1}}</div>
        <div class="identity-info">
            <div class="identity-name">{{.Label}}</div>
            <div class="identity-role">{{if .PrimaryEmail}}{{.PrimaryEmail}}{{end}}</div>
        </div>
    </div>
    {{end}}
</div>
{{end}}

<div class="mt-4">
    <a href="/app" class="btn btn-secondary">Back to Home</a>
</div>
{{end}}

{{/* ================================================================
     App Drafts List
     ================================================================ */}}
{{define "app-drafts"}}
{{template "base18" .}}
{{end}}

{{define "app-drafts-content"}}
<h2 class="section-title">Drafts</h2>

{{if .PendingDrafts}}
<div class="section">
    {{range .PendingDrafts}}
    <div class="draft-card">
        <div class="draft-card-action">{{.DraftType}}</div>
        <div class="draft-card-meta">Circle: {{.CircleID}} | Created: {{formatTime .CreatedAt}}</div>
        <div class="draft-card-actions">
            <a href="/app/draft/{{.DraftID}}" class="btn btn-primary">Review</a>
        </div>
    </div>
    {{end}}
</div>
{{else}}
<div class="empty-state">
    <h2 class="empty-state-title">No drafts.</h2>
    <p class="empty-state-body">Proposed actions will appear here.</p>
</div>
{{end}}
{{end}}

{{/* ================================================================
     App Draft Detail
     ================================================================ */}}
{{define "app-draft"}}
{{template "base18" .}}
{{end}}

{{define "app-draft-content"}}
{{if .Draft}}
<div class="draft-card">
    <div class="draft-card-action">{{.Draft.DraftType}}</div>
    <div class="draft-card-meta">
        Circle: {{.Draft.CircleID}}<br>
        Created: {{formatTime .Draft.CreatedAt}}<br>
        Expires: {{formatTime .Draft.ExpiresAt}}
    </div>

    <div class="explain-panel">
        <div class="explain-panel-title">Why am I seeing this?</div>
        <p class="explain-panel-body">
            This needs your approval before it can proceed.
            {{if .Draft.SourceObligationID}}From obligation: {{.Draft.SourceObligationID}}{{end}}
        </p>
    </div>

    {{if eq .Draft.Status "proposed"}}
    <div class="draft-card-actions mt-4">
        <form method="POST" action="/draft/{{.Draft.DraftID}}/approve" style="display: inline;">
            <input type="hidden" name="reason" value="approved via web">
            <button type="submit" class="btn btn-primary">Approve</button>
        </form>
        <form method="POST" action="/draft/{{.Draft.DraftID}}/reject" style="display: inline;">
            <input type="hidden" name="reason" value="rejected via web">
            <button type="submit" class="btn btn-secondary">Reject</button>
        </form>
    </div>
    {{end}}
    {{if eq .Draft.Status "approved"}}
    <div class="draft-card-actions mt-4">
        <form method="POST" action="/execute/{{.Draft.DraftID}}" style="display: inline;">
            <button type="submit" class="btn btn-primary">Execute</button>
        </form>
    </div>
    {{end}}
</div>
{{else}}
<div class="empty-state">
    <h2 class="empty-state-title">Draft not found.</h2>
    <p class="empty-state-body"><a href="/app/drafts">Back to drafts</a></p>
</div>
{{end}}
{{end}}

{{/* ================================================================
     App People
     ================================================================ */}}
{{define "app-people"}}
{{template "base18" .}}
{{end}}

{{define "app-people-content"}}
<h2 class="section-title">People</h2>

{{if .IdentityStats}}
<p class="text-secondary mb-4">{{.IdentityStats.PersonCount}} people known</p>
{{end}}

{{if .People}}
<div class="section">
    {{range .People}}
    <div class="identity-card">
        <div class="identity-avatar">{{slice .Label 0 1}}</div>
        <div class="identity-info">
            <div class="identity-name">{{.Label}}</div>
            <div class="identity-role">{{if .PrimaryEmail}}{{.PrimaryEmail}}{{end}}</div>
        </div>
    </div>
    {{end}}
</div>
{{else}}
<div class="empty-state">
    <h2 class="empty-state-title">No people yet.</h2>
    <p class="empty-state-body">Identities will appear as you connect accounts.</p>
</div>
{{end}}
{{end}}

{{/* ================================================================
     App Policies
     ================================================================ */}}
{{define "app-policies"}}
{{template "base18" .}}
{{end}}

{{define "app-policies-content"}}
<h2 class="section-title">Policies</h2>

{{if .CircleConfigs}}
<div class="section">
    {{range .CircleConfigs}}
    <div class="policy-card">
        <div class="policy-card-title">{{.Name}}</div>
        <div class="policy-card-description">
            {{.EmailCount}} email | {{.CalendarCount}} calendar | {{.FinanceCount}} finance
        </div>
    </div>
    {{end}}
</div>
{{else}}
<div class="empty-state">
    <h2 class="empty-state-title">No policies defined.</h2>
    <p class="empty-state-body">Default policies apply.</p>
</div>
{{end}}
{{end}}

{{/* ================================================================
     Legacy base template (Phase 1-17 compatibility)
     ================================================================ */}}
{{define "base"}}
<!DOCTYPE html>
<html>
<head>
    <meta charset="utf-8">
    <meta name="viewport" content="width=device-width, initial-scale=1">
    <title>{{.Title}} - QuantumLife</title>
    <link rel="stylesheet" href="/static/tokens.css">
    <link rel="stylesheet" href="/static/reset.css">
    <link rel="stylesheet" href="/static/app.css">
    <style>
        /* Legacy inline styles for backwards compatibility */
        .circle-tile { display: inline-block; background: var(--color-level-ambient); padding: 15px 25px; border-radius: 8px; margin: 5px; }
        .circle-tile h3 { margin-bottom: 5px; }
        .draft-item { border-bottom: 1px solid var(--color-border-subtle); padding: 15px 0; }
        .draft-item:last-child { border-bottom: none; }
        .status-badge { display: inline-block; padding: 2px 8px; border-radius: 4px; font-size: 12px; }
        .status-proposed { background: var(--color-level-needs-you); color: var(--color-text-primary); }
        .status-approved { background: #e8f5e9; color: #2e7d32; }
        .status-rejected { background: #ffebee; color: #c62828; }
        .quiet { text-align: center; padding: 60px 20px; }
        .quiet h2 { color: var(--color-success); font-size: 2rem; margin-bottom: 10px; }
        .needs-you-legacy { border-left: 4px solid var(--color-warning); }
        .needs-you-legacy h2 { color: var(--color-warning); }
        .actions form { display: inline; }
        .form-group { margin-bottom: 15px; }
        .form-group label { display: block; margin-bottom: 5px; font-weight: 500; }
        .form-group input, .form-group textarea { width: 100%; padding: 8px; border: var(--input-border); border-radius: var(--input-radius); }
        .message { color: var(--color-success); background: #e8f5e9; padding: 15px; border-radius: 4px; }
    </style>
</head>
<body>
    <header class="header">
        <div class="header-inner container">
            <a href="/" class="header-logo">QuantumLife</a>
            <nav class="header-nav">
                <a href="/" class="header-nav-link">Home</a>
                <a href="/circles" class="header-nav-link">Circles</a>
                <a href="/people" class="header-nav-link">People</a>
                <a href="/policies" class="header-nav-link">Policies</a>
                <a href="/needs-you" class="header-nav-link">Needs You</a>
                <a href="/draft/" class="header-nav-link">Drafts</a>
                <a href="/history" class="header-nav-link">History</a>
            </nav>
        </div>
    </header>
    <main class="container page-content">
        {{template "content" .}}
    </main>
    <footer class="footer">
        <div class="container footer-inner">
            <span class="footer-text">{{.CurrentTime}} | Deterministic. Synchronous. Quiet.</span>
        </div>
    </footer>
</body>
</html>
{{end}}

{{define "legacy-content"}}
{{template "content" .}}
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
