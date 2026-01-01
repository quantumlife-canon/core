// Package main provides the web server for QuantumLife.
//
// CRITICAL: Uses stdlib only (net/http + html/template).
// CRITICAL: No goroutines in request handlers.
// CRITICAL: Loop runs synchronously per request.
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
	"strings"
	"time"

	"quantumlife/internal/calendar/execution"
	mockcal "quantumlife/internal/connectors/calendar/write/providers/mock"
	"quantumlife/internal/drafts"
	"quantumlife/internal/drafts/calendar"
	"quantumlife/internal/drafts/email"
	"quantumlife/internal/drafts/review"
	"quantumlife/internal/interruptions"
	"quantumlife/internal/loop"
	"quantumlife/internal/obligations"
	"quantumlife/pkg/clock"
	"quantumlife/pkg/domain/draft"
	domainevents "quantumlife/pkg/domain/events"
	"quantumlife/pkg/domain/feedback"
	"quantumlife/pkg/domain/identity"
	"quantumlife/pkg/domain/obligation"
	"quantumlife/pkg/events"
)

var (
	addr     = flag.String("addr", ":8080", "HTTP listen address")
	mockData = flag.Bool("mock", true, "Use mock data")
)

// Server handles HTTP requests.
type Server struct {
	engine       *loop.Engine
	templates    *template.Template
	eventEmitter *eventLogger
	clk          clock.Clock
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
	Title         string
	CurrentTime   string
	RunResult     *loop.RunResult
	NeedsYou      *loop.NeedsYouSummary
	Circles       []loop.CircleResult
	Draft         *draft.Draft
	PendingDrafts []draft.Draft
	FeedbackStats *feedback.FeedbackStats
	ExecutionHist []execution.Envelope
	Message       string
	Error         string
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
	draftEngine := drafts.NewEngine(draftStore, draftPolicy, emailEngine, calendarEngine)

	// Create review service
	reviewService := review.NewService(draftStore)

	// Create calendar executor
	mockWriter := mockcal.NewWriter(
		mockcal.WithClock(clk.Now),
	)
	executor := execution.NewExecutor(execution.ExecutorConfig{
		EnvelopeStore:   execution.NewMemoryStore(),
		FreshnessPolicy: execution.NewDefaultFreshnessPolicy(),
		Clock:           clk.Now,
	})
	executor.RegisterWriter("mock", mockWriter)

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
		CalendarExecutor:   executor,
		FeedbackStore:      feedbackStore,
		EventEmitter:       emitter,
	}

	// Parse templates
	tmpl := template.Must(template.New("").Funcs(template.FuncMap{
		"formatTime": func(t time.Time) string {
			return t.Format("2006-01-02 15:04:05")
		},
	}).Parse(templates))

	// Create server
	server := &Server{
		engine:       engine,
		templates:    tmpl,
		eventEmitter: emitter,
		clk:          clk,
	}

	// Set up routes
	mux := http.NewServeMux()
	mux.HandleFunc("/", server.handleHome)
	mux.HandleFunc("/circles", server.handleCircles)
	mux.HandleFunc("/circle/", server.handleCircle)
	mux.HandleFunc("/needs-you", server.handleNeedsYou)
	mux.HandleFunc("/draft/", server.handleDraft)
	mux.HandleFunc("/history", server.handleHistory)
	mux.HandleFunc("/run/daily", server.handleRunDaily)
	mux.HandleFunc("/feedback", server.handleFeedback)

	log.Printf("Starting QuantumLife Web on %s", *addr)
	log.Printf("Mock data: %v", *mockData)
	if err := http.ListenAndServe(*addr, mux); err != nil {
		log.Fatal(err)
	}
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

	data := templateData{
		Title:       "Circles",
		CurrentTime: s.clk.Now().Format("2006-01-02 15:04:05"),
		Circles:     result.Circles,
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
	history := s.engine.GetExecutionHistory()

	data := templateData{
		Title:         "Execution History",
		CurrentTime:   s.clk.Now().Format("2006-01-02 15:04:05"),
		ExecutionHist: history,
	}

	s.render(w, "history", data)
}

// handleRunDaily triggers a daily loop run.
func (s *Server) handleRunDaily(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	result := s.engine.Run(context.Background(), loop.RunOptions{
		IncludeMockData:       *mockData,
		ExecuteApprovedDrafts: true,
	})

	data := templateData{
		Title:       "Run Complete",
		CurrentTime: s.clk.Now().Format("2006-01-02 15:04:05"),
		RunResult:   &result,
		Message:     fmt.Sprintf("Daily run completed. RunID: %s, Duration: %v", result.RunID, result.CompletedAt.Sub(result.StartedAt)),
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
`
