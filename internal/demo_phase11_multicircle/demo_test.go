// Package demo_phase11_multicircle demonstrates Phase 11: Real Data Quiet Loop (Multi-account).
//
// This demo tests multi-circle configuration loading, ingestion, routing, and loop execution.
// It has two modes:
// - mock: Uses mock adapters and deterministic clock
// - real: Requires OAuth tokens; skips with clear message if not available
//
// Run: go test -v ./internal/demo_phase11_multicircle/...
//
// Reference: docs/ADR/ADR-0026-phase11-multicircle-real-loop.md
package demo_phase11_multicircle

import (
	"testing"
	"time"

	"quantumlife/internal/config"
	"quantumlife/internal/drafts"
	"quantumlife/internal/drafts/calendar"
	"quantumlife/internal/drafts/commerce"
	"quantumlife/internal/drafts/email"
	"quantumlife/internal/drafts/review"
	"quantumlife/internal/ingestion"
	"quantumlife/internal/interruptions"
	"quantumlife/internal/loop"
	"quantumlife/internal/obligations"
	"quantumlife/internal/routing"
	"quantumlife/pkg/clock"
	"quantumlife/pkg/domain/draft"
	domainevents "quantumlife/pkg/domain/events"
	"quantumlife/pkg/domain/feedback"
	"quantumlife/pkg/domain/identity"
	"quantumlife/pkg/events"
)

// mockEmitter captures events for testing.
type mockEmitter struct {
	events []events.Event
}

func (e *mockEmitter) Emit(event events.Event) {
	e.events = append(e.events, event)
}

// mockIdentityRepo provides a mock identity repository.
type mockIdentityRepo struct{}

func (m *mockIdentityRepo) GetByID(id identity.EntityID) (identity.Entity, error) { return nil, nil }
func (m *mockIdentityRepo) IsHighPriority(id identity.EntityID) bool              { return false }

func TestConfigLoading_Determinism(t *testing.T) {
	content := `
[circle:work]
name = Work
email = google:work@company.com:email:read
calendar = google:primary:calendar:read

[circle:personal]
name = Personal
email = google:me@gmail.com:email:read

[routing]
work_domains = company.com
personal_domains = gmail.com
`
	fixedTime := time.Date(2025, 1, 15, 10, 0, 0, 0, time.UTC)

	// Load twice and verify same hash
	cfg1, err := config.LoadFromString(content, fixedTime)
	if err != nil {
		t.Fatalf("failed to load config 1: %v", err)
	}

	cfg2, err := config.LoadFromString(content, fixedTime)
	if err != nil {
		t.Fatalf("failed to load config 2: %v", err)
	}

	if cfg1.Hash() != cfg2.Hash() {
		t.Errorf("config hashes not deterministic: %s != %s", cfg1.Hash(), cfg2.Hash())
	}

	t.Logf("Config hash: %s", cfg1.Hash()[:16])
	t.Logf("Circle count: %d", len(cfg1.Circles))
	t.Logf("Work domains: %v", cfg1.Routing.WorkDomains)
}

func TestRouting_Determinism(t *testing.T) {
	content := `
[circle:work]
name = Work
email = google:work@company.com

[circle:personal]
name = Personal
email = google:me@gmail.com

[circle:family]
name = Family

[routing]
work_domains = company.com
family_members = spouse@gmail.com
`
	fixedTime := time.Date(2025, 1, 15, 10, 0, 0, 0, time.UTC)
	cfg, _ := config.LoadFromString(content, fixedTime)
	router := routing.NewRouter(cfg)

	tests := []struct {
		name         string
		senderDomain string
		accountEmail string
		wantCircle   identity.EntityID
	}{
		{"work domain", "company.com", "unknown@other.com", "work"},
		{"work email receiver", "random.com", "work@company.com", "work"},
		{"personal email receiver", "random.com", "me@gmail.com", "personal"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			event := domainevents.NewEmailMessageEvent("google", "msg-123", tt.accountEmail, fixedTime, fixedTime)
			event.SenderDomain = tt.senderDomain
			event.From = domainevents.EmailAddress{Address: "sender@" + tt.senderDomain}

			// Run 10 times to verify determinism
			for i := 0; i < 10; i++ {
				result := router.RouteEmailToCircle(event)
				if result != tt.wantCircle {
					t.Errorf("run %d: RouteEmailToCircle() = %q, want %q", i, result, tt.wantCircle)
				}
			}
		})
	}
}

func TestMultiCircleRunner_MockMode(t *testing.T) {
	fixedTime := time.Date(2025, 1, 15, 10, 0, 0, 0, time.UTC)
	clk := clock.NewFixed(fixedTime)

	// Create config
	content := `
[circle:work]
name = Work
email = google:work@company.com

[circle:personal]
name = Personal
email = google:me@gmail.com

[routing]
work_domains = company.com
`
	cfg, err := config.LoadFromString(content, fixedTime)
	if err != nil {
		t.Fatalf("failed to load config: %v", err)
	}

	// Create stores
	eventStore := domainevents.NewInMemoryEventStore()
	draftStore := draft.NewInMemoryStore()
	feedbackStore := feedback.NewMemoryStore()
	identityRepo := identity.NewInMemoryRepository()

	// Create circles in identity repo
	gen := identity.NewGenerator()
	workCircle := gen.CircleFromName("owner-1", "Work", fixedTime)
	personalCircle := gen.CircleFromName("owner-1", "Personal", fixedTime)
	identityRepo.Store(workCircle)
	identityRepo.Store(personalCircle)

	// Populate mock events using the generated circle IDs
	workEmail := domainevents.NewEmailMessageEvent("google", "msg-work-001", "work@company.com", fixedTime, fixedTime.Add(-1*time.Hour))
	workEmail.SetCircleID(workCircle.ID())
	workEmail.Subject = "Important Work Email"
	workEmail.IsRead = false
	workEmail.IsImportant = true
	eventStore.Store(workEmail)

	personalEmail := domainevents.NewEmailMessageEvent("google", "msg-personal-001", "me@gmail.com", fixedTime, fixedTime.Add(-2*time.Hour))
	personalEmail.SetCircleID(personalCircle.ID())
	personalEmail.Subject = "Personal Newsletter"
	personalEmail.IsRead = true
	eventStore.Store(personalEmail)

	// Create loop engine
	emitter := &mockEmitter{}
	oblConfig := obligations.DefaultConfig()
	obligationEngine := obligations.NewEngine(oblConfig, clk, &mockIdentityRepo{})

	intConfig := interruptions.DefaultConfig()
	interruptionEngine := interruptions.NewEngine(intConfig, clk,
		interruptions.NewInMemoryDeduper(),
		interruptions.NewInMemoryQuotaStore())

	draftPolicy := draft.DefaultDraftPolicy()
	draftEngine := drafts.NewEngine(draftStore, draftPolicy,
		email.NewDefaultEngine(),
		calendar.NewDefaultEngine(),
		commerce.NewDefaultEngine())

	reviewService := review.NewService(draftStore)

	engine := &loop.Engine{
		Clock:              clk,
		IdentityRepo:       identityRepo,
		EventStore:         eventStore,
		ObligationEngine:   obligationEngine,
		InterruptionEngine: interruptionEngine,
		DraftEngine:        draftEngine,
		DraftStore:         draftStore,
		ReviewService:      reviewService,
		FeedbackStore:      feedbackStore,
		EventEmitter:       emitter,
	}

	// Create multi-circle runner
	runner := loop.NewMultiCircleRunner(engine, clk, cfg).WithEventEmitter(emitter)

	// Create multi-runner for ingestion
	multiRunner := ingestion.NewMultiRunner(clk, eventStore)
	runner.WithMultiRunner(multiRunner)

	// Run the loop
	result := runner.Run(loop.MultiCircleRunOptions{
		RunIngestion:          false, // Skip ingestion since we populated mock events
		ExecuteApprovedDrafts: false,
	})

	t.Logf("RunID: %s", result.RunID)
	t.Logf("Hash: %s", result.Hash[:16])
	t.Logf("Circles processed: %d", len(result.LoopResult.Circles))
	t.Logf("NeedsYou total: %d", result.LoopResult.NeedsYou.TotalItems)
	t.Logf("IsQuiet: %t", result.LoopResult.NeedsYou.IsQuiet)

	// Verify determinism by creating a completely independent second runner
	// with identical initial state (all stores fresh)
	eventStore2 := domainevents.NewInMemoryEventStore()
	draftStore2 := draft.NewInMemoryStore()
	feedbackStore2 := feedback.NewMemoryStore()
	identityRepo2 := identity.NewInMemoryRepository()

	// Use same generator seed by creating from same name at same time
	gen2 := identity.NewGenerator()
	workCircle2 := gen2.CircleFromName("owner-1", "Work", fixedTime)
	personalCircle2 := gen2.CircleFromName("owner-1", "Personal", fixedTime)
	identityRepo2.Store(workCircle2)
	identityRepo2.Store(personalCircle2)

	workEmail2 := domainevents.NewEmailMessageEvent("google", "msg-work-001", "work@company.com", fixedTime, fixedTime.Add(-1*time.Hour))
	workEmail2.SetCircleID(workCircle2.ID())
	workEmail2.Subject = "Important Work Email"
	workEmail2.IsRead = false
	workEmail2.IsImportant = true
	eventStore2.Store(workEmail2)

	personalEmail2 := domainevents.NewEmailMessageEvent("google", "msg-personal-001", "me@gmail.com", fixedTime, fixedTime.Add(-2*time.Hour))
	personalEmail2.SetCircleID(personalCircle2.ID())
	personalEmail2.Subject = "Personal Newsletter"
	personalEmail2.IsRead = true
	eventStore2.Store(personalEmail2)

	oblConfig2 := obligations.DefaultConfig()
	obligationEngine2 := obligations.NewEngine(oblConfig2, clk, &mockIdentityRepo{})

	intConfig2 := interruptions.DefaultConfig()
	interruptionEngine2 := interruptions.NewEngine(intConfig2, clk,
		interruptions.NewInMemoryDeduper(),
		interruptions.NewInMemoryQuotaStore())

	draftPolicy2 := draft.DefaultDraftPolicy()
	draftEngine2 := drafts.NewEngine(draftStore2, draftPolicy2,
		email.NewDefaultEngine(),
		calendar.NewDefaultEngine(),
		commerce.NewDefaultEngine())

	reviewService2 := review.NewService(draftStore2)
	emitter2 := &mockEmitter{}

	engine2 := &loop.Engine{
		Clock:              clk,
		IdentityRepo:       identityRepo2,
		EventStore:         eventStore2,
		ObligationEngine:   obligationEngine2,
		InterruptionEngine: interruptionEngine2,
		DraftEngine:        draftEngine2,
		DraftStore:         draftStore2,
		ReviewService:      reviewService2,
		FeedbackStore:      feedbackStore2,
		EventEmitter:       emitter2,
	}

	runner2 := loop.NewMultiCircleRunner(engine2, clk, cfg).WithEventEmitter(emitter2)
	result2 := runner2.Run(loop.MultiCircleRunOptions{
		RunIngestion:          false,
		ExecuteApprovedDrafts: false,
	})

	// Same inputs should produce same NeedsYou hash
	if result.LoopResult.NeedsYou.Hash != result2.LoopResult.NeedsYou.Hash {
		t.Errorf("NeedsYou hash not deterministic: %s != %s",
			result.LoopResult.NeedsYou.Hash, result2.LoopResult.NeedsYou.Hash)
	}
}

func TestMultiCircleRunner_CircleSpecificRun(t *testing.T) {
	fixedTime := time.Date(2025, 1, 15, 10, 0, 0, 0, time.UTC)
	clk := clock.NewFixed(fixedTime)

	content := `
[circle:work]
name = Work

[circle:personal]
name = Personal
`
	cfg, _ := config.LoadFromString(content, fixedTime)

	// Create minimal engine components
	eventStore := domainevents.NewInMemoryEventStore()
	draftStore := draft.NewInMemoryStore()
	feedbackStore := feedback.NewMemoryStore()
	identityRepo := identity.NewInMemoryRepository()

	gen := identity.NewGenerator()
	workCircle := gen.CircleFromName("owner-1", "Work", fixedTime)
	personalCircle := gen.CircleFromName("owner-1", "Personal", fixedTime)
	identityRepo.Store(workCircle)
	identityRepo.Store(personalCircle)

	emitter := &mockEmitter{}
	engine := &loop.Engine{
		Clock:         clk,
		IdentityRepo:  identityRepo,
		EventStore:    eventStore,
		DraftStore:    draftStore,
		FeedbackStore: feedbackStore,
		EventEmitter:  emitter,
	}

	runner := loop.NewMultiCircleRunner(engine, clk, cfg)

	// Run for work circle only (use generated ID)
	result := runner.Run(loop.MultiCircleRunOptions{
		CircleID: workCircle.ID(),
	})

	// Should only process work circle
	if len(result.LoopResult.Circles) != 1 {
		t.Errorf("expected 1 circle result for work-only run, got %d", len(result.LoopResult.Circles))
	}
	if len(result.LoopResult.Circles) > 0 && result.LoopResult.Circles[0].CircleID != workCircle.ID() {
		t.Errorf("expected work circle %s, got %s", workCircle.ID(), result.LoopResult.Circles[0].CircleID)
	}

	t.Logf("Circle-specific run hash: %s", result.Hash[:16])
}

func TestIngestionRunner_SortedOrder(t *testing.T) {
	fixedTime := time.Date(2025, 1, 15, 10, 0, 0, 0, time.UTC)
	clk := clock.NewFixed(fixedTime)

	content := `
[circle:zebra]
name = Zebra
email = google:zebra@test.com

[circle:alpha]
name = Alpha
email = google:alpha@test.com

[circle:mango]
name = Mango
email = google:mango@test.com
`
	cfg, _ := config.LoadFromString(content, fixedTime)
	eventStore := domainevents.NewInMemoryEventStore()

	runner := ingestion.NewMultiRunner(clk, eventStore)

	// Run without adapters - should still process in sorted order
	result, err := runner.Run(cfg, ingestion.DefaultMultiRunOptions())
	if err != nil {
		t.Fatalf("run failed: %v", err)
	}

	// Verify circles are in sorted order
	expectedOrder := []identity.EntityID{"alpha", "mango", "zebra"}
	for i, receipt := range result.CircleReceipts {
		if receipt.CircleID != expectedOrder[i] {
			t.Errorf("circle %d: expected %s, got %s", i, expectedOrder[i], receipt.CircleID)
		}
	}

	t.Logf("Ingestion result hash: %s", result.Hash[:16])
}

func TestCircleConfigInfos(t *testing.T) {
	fixedTime := time.Date(2025, 1, 15, 10, 0, 0, 0, time.UTC)
	clk := clock.NewFixed(fixedTime)

	content := `
[circle:work]
name = Work
email = google:work@company.com
calendar = google:primary
calendar = microsoft:secondary

[circle:personal]
name = Personal
email = google:me@gmail.com
`
	cfg, _ := config.LoadFromString(content, fixedTime)

	identityRepo := identity.NewInMemoryRepository()
	engine := &loop.Engine{
		Clock:        clk,
		IdentityRepo: identityRepo,
	}

	runner := loop.NewMultiCircleRunner(engine, clk, cfg)
	infos := runner.GetCircleConfigInfos()

	if len(infos) != 2 {
		t.Fatalf("expected 2 circle infos, got %d", len(infos))
	}

	// Find work circle
	var workInfo *loop.CircleConfigInfo
	for i := range infos {
		if infos[i].ID == "work" {
			workInfo = &infos[i]
			break
		}
	}

	if workInfo == nil {
		t.Fatal("work circle info not found")
	}

	if workInfo.EmailCount != 1 {
		t.Errorf("expected 1 email integration, got %d", workInfo.EmailCount)
	}
	if workInfo.CalendarCount != 2 {
		t.Errorf("expected 2 calendar integrations, got %d", workInfo.CalendarCount)
	}

	t.Logf("Circle infos retrieved: %d circles", len(infos))
}
