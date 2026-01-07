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
	"crypto/sha256"
	"encoding/hex"
	"flag"
	"fmt"
	"html/template"
	"log"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"syscall"
	"time"

	calexec "quantumlife/internal/calendar/execution"
	"quantumlife/internal/commerceingest"
	internalcommerceobserver "quantumlife/internal/commerceobserver"
	"quantumlife/internal/config"
	"quantumlife/internal/connectors/auth"
	"quantumlife/internal/connectors/auth/impl_inmem"
	mockcal "quantumlife/internal/connectors/calendar/write/providers/mock"
	mockemail "quantumlife/internal/connectors/email/write/providers/mock"
	internaldeviceidentity "quantumlife/internal/deviceidentity"
	"quantumlife/internal/drafts"
	"quantumlife/internal/drafts/calendar"
	"quantumlife/internal/drafts/commerce"
	"quantumlife/internal/drafts/email"
	"quantumlife/internal/drafts/review"
	emailexec "quantumlife/internal/email/execution"
	"quantumlife/internal/execexecutor"
	"quantumlife/internal/execrouter"
	"quantumlife/internal/financemirror"
	internalfirstaction "quantumlife/internal/firstaction"
	internalfirstminutes "quantumlife/internal/firstminutes"
	"quantumlife/internal/held"
	gmailread "quantumlife/internal/integrations/gmail_read"
	"quantumlife/internal/interest"
	"quantumlife/internal/interruptions"
	internalinvitation "quantumlife/internal/invitation"
	"quantumlife/internal/journey"
	"quantumlife/internal/loop"
	"quantumlife/internal/mirror"
	"quantumlife/internal/mode"
	"quantumlife/internal/oauth"
	"quantumlife/internal/obligations"
	"quantumlife/internal/persist"
	"quantumlife/internal/proof"
	internalquietmirror "quantumlife/internal/quietmirror"
	internalreality "quantumlife/internal/reality"
	internalreplay "quantumlife/internal/replay"
	rulepackengine "quantumlife/internal/rulepack"
	"quantumlife/internal/shadowcalibration"
	shadowdiffengine "quantumlife/internal/shadowdiff"
	shadowgate "quantumlife/internal/shadowgate"
	"quantumlife/internal/shadowllm"
	"quantumlife/internal/shadowllm/providers/azureopenai"
	"quantumlife/internal/shadowllm/stub"
	"quantumlife/internal/shadowview"
	"quantumlife/internal/surface"
	"quantumlife/internal/todayquietly"
	trustengine "quantumlife/internal/trust"
	trustactionengine "quantumlife/internal/trustaction"
	"quantumlife/internal/undoableexec"
	"quantumlife/pkg/clock"
	"quantumlife/pkg/domain/approvaltoken"
	domaincommerceobserver "quantumlife/pkg/domain/commerceobserver"
	pkgconfig "quantumlife/pkg/domain/config"
	"quantumlife/pkg/domain/connection"
	domaindeviceidentity "quantumlife/pkg/domain/deviceidentity"
	"quantumlife/pkg/domain/draft"
	domainevents "quantumlife/pkg/domain/events"
	"quantumlife/pkg/domain/feedback"
	domainfinancemirror "quantumlife/pkg/domain/financemirror"
	domainfirstaction "quantumlife/pkg/domain/firstaction"
	domainfirstminutes "quantumlife/pkg/domain/firstminutes"
	"quantumlife/pkg/domain/identity"
	domaininvitation "quantumlife/pkg/domain/invitation"
	domainmirror "quantumlife/pkg/domain/mirror"
	"quantumlife/pkg/domain/obligation"
	"quantumlife/pkg/domain/policy"
	quietmirror "quantumlife/pkg/domain/quietmirror"
	domainreality "quantumlife/pkg/domain/reality"
	domainrulepack "quantumlife/pkg/domain/rulepack"
	"quantumlife/pkg/domain/runlog"
	"quantumlife/pkg/domain/shadowdiff"
	domainshadowgate "quantumlife/pkg/domain/shadowgate"
	domainshadow "quantumlife/pkg/domain/shadowllm"
	domainshadowview "quantumlife/pkg/domain/shadowview"
	"quantumlife/pkg/domain/suppress"
	domaintrust "quantumlife/pkg/domain/trust"
	domainundoableexec "quantumlife/pkg/domain/undoableexec"
	"quantumlife/pkg/events"
)

var (
	addr       = flag.String("addr", ":8080", "HTTP listen address")
	mockData   = flag.Bool("mock", true, "Use mock data")
	configPath = flag.String("config", "configs/circles/default.qlconf", "Path to circle configuration file")
)

// Server handles HTTP requests.
type Server struct {
	engine                 *loop.Engine
	templates              *template.Template
	eventEmitter           *eventLogger
	clk                    clock.Clock
	execRouter             *execrouter.Router
	execExecutor           *execexecutor.Executor
	multiCircleConfig      *config.MultiCircleConfig
	identityRepo           *identity.InMemoryRepository       // Phase 13.1: Identity graph
	interestStore          *interest.Store                    // Phase 18.1: Interest capture
	todayEngine            *todayquietly.Engine               // Phase 18.2: Today, quietly
	preferenceStore        *todayquietly.PreferenceStore      // Phase 18.2: Preference capture
	heldEngine             *held.Engine                       // Phase 18.3: Held, not shown
	heldStore              *held.SummaryStore                 // Phase 18.3: Summary store
	surfaceEngine          *surface.Engine                    // Phase 18.4: Quiet Shift
	surfaceStore           *surface.ActionStore               // Phase 18.4: Action store
	proofEngine            *proof.Engine                      // Phase 18.5: Quiet Proof
	proofAckStore          *proof.AckStore                    // Phase 18.5: Ack store
	connectionStore        *persist.InMemoryConnectionStore   // Phase 18.6: First Connect
	mirrorEngine           *mirror.Engine                     // Phase 18.7: Mirror Proof
	mirrorAckStore         *mirror.AckStore                   // Phase 18.7: Mirror Ack store
	tokenBroker            auth.TokenBroker                   // Phase 18.8: OAuth token broker
	oauthStateManager      *oauth.StateManager                // Phase 18.8: OAuth state management
	gmailHandler           *oauth.GmailHandler                // Phase 18.8: Gmail OAuth handler
	syncReceiptStore       *persist.SyncReceiptStore          // Phase 19.1: Sync receipt store
	shadowEngine           *shadowllm.Engine                  // Phase 19.2: Shadow mode engine
	shadowReceiptStore     *persist.ShadowReceiptStore        // Phase 19.2: Shadow receipt store
	shadowCalibrationStore *persist.ShadowCalibrationStore    // Phase 19.4: Shadow calibration store
	shadowGateStore        *persist.ShadowGateStore           // Phase 19.5: Shadow gating store
	rulepackStore          *persist.RulePackStore             // Phase 19.6: Rule pack store
	trustStore             *persist.TrustStore                // Phase 20: Trust store
	trustEngine            *trustengine.Engine                // Phase 20: Trust engine
	modeEngine             *mode.Engine                       // Phase 21: Mode derivation engine
	shadowviewEngine       *shadowview.Engine                 // Phase 21: Shadow receipt viewer engine
	shadowviewAckStore     *shadowview.AckStore               // Phase 21: Shadow receipt acknowledgement store
	quietMirrorEngine      *internalquietmirror.Engine        // Phase 22: Quiet Inbox Mirror engine
	quietMirrorStore       *persist.QuietMirrorStore          // Phase 22: Quiet Inbox Mirror store
	quietMirrorDismissals  *persist.QuietMirrorDismissalStore // Phase 22: Whisper dismissal store
	invitationEngine       *internalinvitation.Engine         // Phase 23: Gentle Action Invitation engine
	invitationStore        *persist.InvitationStore           // Phase 23: Invitation decision store
	firstActionEngine      *internalfirstaction.Engine        // Phase 24: First Reversible Action engine
	firstActionStore       *persist.FirstActionStore          // Phase 24: First action store
	undoableExecEngine     *undoableexec.Engine               // Phase 25: Undoable execution engine
	undoableExecStore      *persist.UndoableExecStore         // Phase 25: Undoable execution store
	journeyEngine          *journey.Engine                    // Phase 26A: Guided Journey engine
	journeyDismissalStore  *persist.JourneyDismissalStore     // Phase 26A: Journey dismissal store
	firstMinutesEngine     *internalfirstminutes.Engine       // Phase 26B: First Minutes engine
	firstMinutesStore      *persist.FirstMinutesStore         // Phase 26B: First Minutes store
	realityEngine          *internalreality.Engine            // Phase 26C: Reality engine
	realityAckStore        *persist.RealityAckStore           // Phase 26C: Reality ack store
	shadowReceiptAckStore  *persist.ShadowReceiptAckStore     // Phase 27: Shadow Receipt ack/vote store
	trustActionStore       *persist.TrustActionStore          // Phase 28: Trust action store
	trustActionEngine      *trustactionengine.Engine          // Phase 28: Trust action engine
	financeMirrorStore     *persist.FinanceMirrorStore        // Phase 29: Finance mirror store
	financeMirrorEngine    *financemirror.Engine              // Phase 29: Finance mirror engine
	trueLayerHandler       *oauth.TrueLayerHandler            // Phase 29: TrueLayer OAuth handler
	deviceKeyStore         *persist.DeviceKeyStore            // Phase 30A: Device key store
	circleBindingStore     *persist.CircleBindingStore        // Phase 30A: Circle binding store
	deviceIdentityEngine   *internaldeviceidentity.Engine     // Phase 30A: Device identity engine
	replayEngine           *internalreplay.Engine             // Phase 30A: Replay bundle engine
	commerceObserverStore  *persist.CommerceObserverStore     // Phase 31: Commerce observer store
	commerceObserverEngine *internalcommerceobserver.Engine   // Phase 31: Commerce observer engine
	commerceIngestEngine   *commerceingest.Engine             // Phase 31.1: Commerce ingest engine
	// Phase 18 Web Control Center
	runStore       *runlog.InMemoryRunStore // Run snapshot store for /runs
	suppressionSet *suppress.SuppressionSet // Suppression rules for /suppressions
	approvalLedger *persist.ApprovalLedger  // Approval ledger for /approve
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
	PreferenceMessage   string
	// Phase 18.3: Held, not shown
	HeldSummary *held.HeldSummary
	// Phase 18.4: Quiet Shift
	SurfaceCue           *surface.SurfaceCue
	SurfacePage          *surface.SurfacePage
	SurfaceActionDone    bool
	SurfaceActionMessage string
	// Phase 18.5: Quiet Proof
	ProofSummary *proof.ProofSummary
	ProofCue     *proof.ProofCue
	// Phase 18.6: First Connect
	ConnectionState     *connection.ConnectionStateSet
	ConnectionKind      connection.ConnectionKind
	ConnectionKindState *connection.ConnectionState
	MockMode            bool
	// Phase 18.7: Mirror Proof
	MirrorPage *domainmirror.MirrorPage
	// Phase 18.9: Gmail OAuth
	CircleID string
	// Phase 19.1: Quiet check
	QuietCheckStatus *persist.QuietCheckStatus
	// Phase 20: Trust accrual
	TrustSummary  *domaintrust.TrustSummary
	TrustCueShown bool
	// Phase 21: Onboarding + Shadow Receipt Viewer
	ModeIndicator     *mode.ModeIndicator
	ShadowReceiptPage *shadowview.ShadowReceiptPage
	ShadowReceiptCue  *shadowview.ReceiptCue // Whisper cue for proof page link
	// Phase 24: First Reversible Action
	FirstActionPage    *domainfirstaction.ActionPage
	FirstActionPreview *domainfirstaction.PreviewPage
	FirstActionPeriod  string // Period hash for form submission
	// Phase 25: Undoable Execution
	UndoablePage    *domainundoableexec.UndoablePage
	UndoDonePage    *domainundoableexec.DonePage
	UndoPage        *domainundoableexec.UndoPage
	UndoRecordID    string // For undo form submission
	UndoEligibility *domainundoableexec.ActionEligibility
	// Phase 26A: Guided Journey
	JourneyPage *journey.JourneyPage
	// Phase 26B: First Five Minutes Proof
	FirstMinutesSummary *domainfirstminutes.FirstMinutesSummary
	FirstMinutesCue     *domainfirstminutes.FirstMinutesCue
	// Phase 26C: Connected Reality Check
	RealityPage *domainreality.RealityPage
	RealityCue  *domainreality.RealityCue
	// Phase 27: Real Shadow Receipt (Primary Proof)
	ShadowReceiptPrimaryCue *domainshadowview.ShadowReceiptCue
	// Phase 28: Trust Kept
	TrustActionPreview   *trustActionPreviewInfo
	TrustActionReceipt   *trustActionReceiptInfo
	TrustActionCue       *trustActionCueInfo
	TrustActionUndoAvail bool
	// Phase 29: TrueLayer Finance Mirror
	FinanceMirrorPage *domainfinancemirror.FinanceMirrorPage
	FinanceMirrorCue  *domainfinancemirror.FinanceMirrorCue
	// Phase 31: Commerce Observers
	CommerceMirrorPage *domaincommerceobserver.CommerceMirrorPage
	CommerceCue        *domaincommerceobserver.CommerceCue
	// Phase 18 Web Control Center
	RunSnapshots     []*runlog.RunSnapshot      // List of run snapshots for /runs
	RunSnapshot      *runlog.RunSnapshot        // Single run snapshot for /runs/:id
	ReplayResult     *runlog.ReplayResult       // Replay result for /runs/:id
	SuppressionRules []suppress.SuppressionRule // Active suppression rules
	SuppressionStats *suppress.Stats            // Suppression statistics
	ApprovalResult   *approvalResultInfo        // Approval token result for /approve
	PendingApprovals []*pendingApprovalInfo     // Pending approvals for person
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

// approvalResultInfo contains approval token verification result. Phase 18 Web Control Center.
type approvalResultInfo struct {
	Valid        bool
	TokenID      string
	StateID      string
	PersonID     string
	ActionType   string
	ActionClass  string
	Description  string
	ExpiresAt    string
	IsExpired    bool
	IsApproved   bool
	IsRejected   bool
	Message      string
	ErrorMessage string
}

// pendingApprovalInfo contains pending approval for display. Phase 18 Web Control Center.
type pendingApprovalInfo struct {
	StateID      string
	TargetType   string
	TargetID     string
	ActionClass  string
	Description  string
	Threshold    int
	CurrentCount int
	ExpiresAt    string
	IsExpired    bool
	ApproveURL   string
	RejectURL    string
}

// trustActionPreviewInfo contains trust action preview data. Phase 28.
type trustActionPreviewInfo struct {
	ActionKind     string
	AbstractTarget string
	HorizonBucket  string
	Reversible     bool
	DraftID        string
	PeriodKey      string
}

// trustActionReceiptInfo contains trust action receipt data. Phase 28.
type trustActionReceiptInfo struct {
	ReceiptID     string
	ActionKind    string
	State         string
	Period        string
	StatusHash    string
	UndoAvailable bool
}

// trustActionCueInfo contains trust action cue data. Phase 28.
type trustActionCueInfo struct {
	Available bool
	CueText   string
	LinkText  string
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

	// Create connection store (Phase 18.6)
	connectionStore := persist.NewInMemoryConnectionStore()

	// Create mirror engine and store (Phase 18.7)
	mirrorEngine := mirror.NewEngine(clk.Now)
	mirrorAckStore := mirror.NewAckStore(128)

	// Create OAuth components (Phase 18.8)
	// Token broker with persistence (reads from env: GOOGLE_CLIENT_ID, GOOGLE_CLIENT_SECRET, TOKEN_ENC_KEY)
	authConfig := auth.LoadConfigFromEnv()
	tokenBroker, err := impl_inmem.NewBrokerWithPersistence(authConfig, nil, impl_inmem.WithClock(clk.Now))
	if err != nil {
		// Fall back to non-persistent broker if persistence fails
		log.Printf("Warning: OAuth token persistence disabled: %v", err)
		tokenBroker = impl_inmem.NewBroker(authConfig, nil, impl_inmem.WithClock(clk.Now))
	}

	// OAuth state manager with secret from env
	oauthSecret := os.Getenv("OAUTH_STATE_SECRET")
	if oauthSecret == "" {
		oauthSecret = "dev-secret-not-for-production-32b" // 32 bytes for HMAC-SHA256
	}
	oauthStateManager := oauth.NewStateManager([]byte(oauthSecret), clk.Now)

	// Gmail OAuth handler
	gmailRedirectBase := os.Getenv("OAUTH_REDIRECT_BASE")
	if gmailRedirectBase == "" {
		gmailRedirectBase = "http://localhost:8080"
	}
	gmailHandler := oauth.NewGmailHandler(
		oauthStateManager,
		tokenBroker,
		nil, // Use default HTTP client
		gmailRedirectBase,
		clk.Now,
	)

	// Create sync receipt store (Phase 19.1)
	syncReceiptStore := persist.NewSyncReceiptStore(clk.Now)

	// Create shadow mode engine and store (Phase 19.2 + 19.3)
	// CRITICAL: Default is stub provider - real providers require explicit opt-in
	shadowProvider, shadowProviderInfo := createShadowProvider(multiCfg, emitter)
	shadowEngine := shadowllm.NewEngine(clk, shadowProvider)
	shadowReceiptStore := persist.NewShadowReceiptStore(clk.Now)
	shadowCalibrationStore := persist.NewShadowCalibrationStore(clk.Now)
	shadowGateStore := persist.NewShadowGateStore(clk.Now)
	rulepackStore := persist.NewRulePackStore(clk.Now)
	trustStore := persist.NewTrustStore(clk.Now)
	trustEng := trustengine.NewEngine(clk)

	// Populate mock trust summaries if requested
	if *mockData {
		populateMockTrustSummaries(trustStore, now)
	}

	// Phase 30A: Create device identity and replay components
	// Key is stored in user's config directory
	deviceKeyPath := filepath.Join(os.TempDir(), "quantumlife-device-key")
	deviceKeyStore := persist.NewDeviceKeyStore(deviceKeyPath)
	circleBindingStore := persist.NewCircleBindingStore(clk.Now, nil) // No storelog for now
	deviceIdentityEngine := internaldeviceidentity.NewEngine(clk.Now, deviceKeyStore, circleBindingStore)
	replayEngine := internalreplay.NewEngine(clk.Now, nil) // No storelog for now

	// Phase 31: Create commerce observer store and engine
	commerceObserverStore := persist.NewCommerceObserverStore(clk.Now)
	commerceObserverEngine := internalcommerceobserver.NewEngine(clk.Now)

	// Phase 31.1: Create commerce ingest engine
	commerceIngestEngine := commerceingest.NewEngine(clk.Now)

	// Phase 18 Web Control Center: Create stores
	runStore := runlog.NewInMemoryRunStore()
	suppressionSet := suppress.NewSuppressionSet()

	// Create server
	server := &Server{
		engine:                 engine,
		templates:              tmpl,
		eventEmitter:           emitter,
		clk:                    clk,
		execRouter:             execRouter,
		execExecutor:           execExecutor,
		multiCircleConfig:      multiCfg,
		identityRepo:           identityRepo,                                  // Phase 13.1
		interestStore:          interestStore,                                 // Phase 18.1
		todayEngine:            todayEngine,                                   // Phase 18.2
		preferenceStore:        preferenceStore,                               // Phase 18.2
		heldEngine:             heldEngine,                                    // Phase 18.3
		heldStore:              heldStore,                                     // Phase 18.3
		surfaceEngine:          surfaceEngine,                                 // Phase 18.4
		surfaceStore:           surfaceStore,                                  // Phase 18.4
		proofEngine:            proofEngine,                                   // Phase 18.5
		proofAckStore:          proofAckStore,                                 // Phase 18.5
		connectionStore:        connectionStore,                               // Phase 18.6
		mirrorEngine:           mirrorEngine,                                  // Phase 18.7
		mirrorAckStore:         mirrorAckStore,                                // Phase 18.7
		tokenBroker:            tokenBroker,                                   // Phase 18.8
		oauthStateManager:      oauthStateManager,                             // Phase 18.8
		gmailHandler:           gmailHandler,                                  // Phase 18.8
		syncReceiptStore:       syncReceiptStore,                              // Phase 19.1
		shadowEngine:           shadowEngine,                                  // Phase 19.2
		shadowReceiptStore:     shadowReceiptStore,                            // Phase 19.2
		shadowCalibrationStore: shadowCalibrationStore,                        // Phase 19.4
		shadowGateStore:        shadowGateStore,                               // Phase 19.5
		rulepackStore:          rulepackStore,                                 // Phase 19.6
		trustStore:             trustStore,                                    // Phase 20
		trustEngine:            trustEng,                                      // Phase 20
		modeEngine:             mode.NewEngine(clk.Now),                       // Phase 21
		shadowviewEngine:       shadowview.NewEngine(clk.Now),                 // Phase 21
		shadowviewAckStore:     shadowview.NewAckStore(0),                     // Phase 21
		quietMirrorEngine:      internalquietmirror.NewEngine(clk.Now),        // Phase 22
		quietMirrorStore:       persist.NewQuietMirrorStore(clk.Now),          // Phase 22
		quietMirrorDismissals:  persist.NewQuietMirrorDismissalStore(clk.Now), // Phase 22
		invitationEngine:       internalinvitation.NewEngine(clk.Now),         // Phase 23
		invitationStore:        persist.NewInvitationStore(clk.Now),           // Phase 23
		firstActionEngine:      internalfirstaction.NewEngine(clk.Now),        // Phase 24
		firstActionStore:       persist.NewFirstActionStore(clk.Now),          // Phase 24
		undoableExecStore:      persist.NewUndoableExecStore(clk.Now),         // Phase 25
		journeyEngine:          journey.NewEngine(clk.Now),                    // Phase 26A
		journeyDismissalStore:  persist.NewJourneyDismissalStore(clk.Now),     // Phase 26A
		firstMinutesEngine:     internalfirstminutes.NewEngine(clk.Now),       // Phase 26B
		firstMinutesStore:      persist.NewFirstMinutesStore(clk.Now),         // Phase 26B
		realityEngine:          internalreality.NewEngine(clk),                // Phase 26C
		realityAckStore:        persist.NewRealityAckStore(clk.Now),           // Phase 26C
		shadowReceiptAckStore:  persist.NewShadowReceiptAckStore(clk.Now),     // Phase 27
		trustActionStore:       persist.NewTrustActionStore(clk.Now),          // Phase 28
		trustActionEngine:      nil,                                           // Phase 28: Set after full initialization
		financeMirrorStore:     persist.NewFinanceMirrorStore(clk.Now),        // Phase 29
		financeMirrorEngine:    nil,                                           // Phase 29: Set after full initialization
		trueLayerHandler:       nil,                                           // Phase 29: Set after full initialization
		deviceKeyStore:         deviceKeyStore,                                // Phase 30A
		circleBindingStore:     circleBindingStore,                            // Phase 30A
		deviceIdentityEngine:   deviceIdentityEngine,                          // Phase 30A
		replayEngine:           replayEngine,                                  // Phase 30A
		commerceObserverStore:  commerceObserverStore,                         // Phase 31
		commerceObserverEngine: commerceObserverEngine,                        // Phase 31
		commerceIngestEngine:   commerceIngestEngine,                          // Phase 31.1
		// Phase 18 Web Control Center
		runStore:       runStore,
		suppressionSet: suppressionSet,
		// approvalLedger: nil, // Will be set when file-backed storage is needed
	}

	// Set up routes
	mux := http.NewServeMux()

	// Phase 18: Static files
	mux.Handle("/static/", http.StripPrefix("/static/", http.FileServer(http.Dir("cmd/quantumlife-web/static"))))

	// Phase 18: Public routes
	mux.HandleFunc("/", server.handleLanding)
	mux.HandleFunc("/interest", server.handleInterest)                                 // Phase 18.1: Interest capture
	mux.HandleFunc("/today", server.handleToday)                                       // Phase 18.2: Today, quietly
	mux.HandleFunc("/today/preference", server.handlePreference)                       // Phase 18.2: Preference capture
	mux.HandleFunc("/held", server.handleHeld)                                         // Phase 18.3: Held, not shown
	mux.HandleFunc("/surface", server.handleSurface)                                   // Phase 18.4: Quiet Shift
	mux.HandleFunc("/surface/hold", server.handleSurfaceHold)                          // Phase 18.4: Hold action
	mux.HandleFunc("/surface/why", server.handleSurfaceWhy)                            // Phase 18.4: Why action
	mux.HandleFunc("/surface/prefer", server.handleSurfacePrefer)                      // Phase 18.4: Prefer show_all
	mux.HandleFunc("/proof", server.handleProof)                                       // Phase 18.5: Quiet Proof
	mux.HandleFunc("/proof/dismiss", server.handleProofDismiss)                        // Phase 18.5: Dismiss proof
	mux.HandleFunc("/start", server.handleStart)                                       // Phase 18.6: First Connect
	mux.HandleFunc("/connections", server.handleConnections)                           // Phase 18.6: Connections
	mux.HandleFunc("/connect/", server.handleConnect)                                  // Phase 18.6: Connect action
	mux.HandleFunc("/disconnect/", server.handleDisconnect)                            // Phase 18.6: Disconnect action
	mux.HandleFunc("/mirror", server.handleMirror)                                     // Phase 18.7: Mirror Proof
	mux.HandleFunc("/connect/gmail", server.handleGmailConsent)                        // Phase 18.9: Gmail consent page
	mux.HandleFunc("/connect/gmail/start", server.handleGmailOAuthStart)               // Phase 18.8: Gmail OAuth start
	mux.HandleFunc("/connect/gmail/callback", server.handleGmailOAuthCallback)         // Phase 18.8: Gmail OAuth callback
	mux.HandleFunc("/disconnect/gmail", server.handleGmailDisconnect)                  // Phase 18.8: Gmail disconnect
	mux.HandleFunc("/run/gmail-sync", server.handleGmailSync)                          // Phase 18.8: Gmail sync
	mux.HandleFunc("/quiet-check", server.handleQuietCheck)                            // Phase 19.1: Quiet baseline verification
	mux.HandleFunc("/run/shadow", server.handleShadowRun)                              // Phase 19.2: Shadow mode run
	mux.HandleFunc("/run/shadow-diff", server.handleShadowDiff)                        // Phase 19.4: Compute shadow diffs
	mux.HandleFunc("/shadow/report", server.handleShadowReport)                        // Phase 19.4: Shadow calibration report
	mux.HandleFunc("/shadow/vote", server.handleShadowVote)                            // Phase 19.4: Shadow calibration vote
	mux.HandleFunc("/shadow/candidates", server.handleShadowCandidates)                // Phase 19.5: Shadow candidates
	mux.HandleFunc("/shadow/candidates/refresh", server.handleShadowCandidatesRefresh) // Phase 19.5: Refresh candidates
	mux.HandleFunc("/shadow/candidates/propose", server.handleShadowCandidatesPropose) // Phase 19.5: Propose promotion
	mux.HandleFunc("/shadow/packs", server.handleRulePackList)                         // Phase 19.6: List packs
	mux.HandleFunc("/shadow/packs/", server.handleRulePackDetail)                      // Phase 19.6: Pack detail
	mux.HandleFunc("/shadow/packs/build", server.handleRulePackBuild)                  // Phase 19.6: Build pack
	mux.HandleFunc("/shadow/health", server.handleShadowHealth)                        // Phase 19.3b: Shadow health
	mux.HandleFunc("/shadow/health/run", server.handleShadowHealthRun)                 // Phase 19.3b: Shadow health run
	mux.HandleFunc("/trust", server.handleTrust)                                       // Phase 20: Trust accrual
	mux.HandleFunc("/trust/dismiss", server.handleTrustDismiss)                        // Phase 20: Dismiss trust cue
	mux.HandleFunc("/onboarding", server.handleOnboarding)                             // Phase 21: Unified onboarding
	mux.HandleFunc("/shadow/receipt", server.handleShadowReceipt)                      // Phase 21/27: Shadow receipt viewer
	mux.HandleFunc("/shadow/receipt/dismiss", server.handleShadowReceiptDismiss)       // Phase 21/27: Dismiss receipt cue
	mux.HandleFunc("/shadow/receipt/vote", server.handleShadowReceiptVote)             // Phase 27: Vote on restraint
	mux.HandleFunc("/mirror/inbox", server.handleQuietInboxMirror)                     // Phase 22: Quiet Inbox Mirror
	mux.HandleFunc("/mirror/inbox/dismiss", server.handleQuietMirrorDismiss)           // Phase 22: Dismiss whisper cue
	mux.HandleFunc("/invite", server.handleInvitation)                                 // Phase 23: Gentle Action Invitation
	mux.HandleFunc("/invite/accept", server.handleInvitationAccept)                    // Phase 23: Accept invitation
	mux.HandleFunc("/invite/dismiss", server.handleInvitationDismiss)                  // Phase 23: Dismiss invitation
	mux.HandleFunc("/action/once", server.handleFirstAction)                           // Phase 24: First Reversible Action
	mux.HandleFunc("/action/once/run", server.handleFirstActionRun)                    // Phase 24: Execute preview
	mux.HandleFunc("/action/once/dismiss", server.handleFirstActionDismiss)            // Phase 24: Dismiss invitation
	mux.HandleFunc("/action/undoable", server.handleUndoable)                          // Phase 25: Undoable execution
	mux.HandleFunc("/action/undoable/run", server.handleUndoableRun)                   // Phase 25: Run undoable
	mux.HandleFunc("/action/undoable/done", server.handleUndoableDone)                 // Phase 25: Done page
	mux.HandleFunc("/action/undoable/undo", server.handleUndoableUndo)                 // Phase 25: Undo page
	mux.HandleFunc("/action/undoable/undo/run", server.handleUndoableUndoRun)          // Phase 25: Execute undo
	mux.HandleFunc("/action/undoable/dismiss", server.handleUndoableDismiss)           // Phase 25: Dismiss
	mux.HandleFunc("/journey", server.handleJourney)                                   // Phase 26A: Guided Journey
	mux.HandleFunc("/journey/next", server.handleJourneyNext)                          // Phase 26A: Journey next step
	mux.HandleFunc("/journey/dismiss", server.handleJourneyDismiss)                    // Phase 26A: Dismiss journey
	mux.HandleFunc("/first-minutes", server.handleFirstMinutes)                        // Phase 26B: First Minutes receipt
	mux.HandleFunc("/first-minutes/dismiss", server.handleFirstMinutesDismiss)         // Phase 26B: Dismiss receipt
	mux.HandleFunc("/reality", server.handleReality)                                   // Phase 26C: Reality check
	mux.HandleFunc("/reality/ack", server.handleRealityAck)                            // Phase 26C: Acknowledge reality
	mux.HandleFunc("/trust/action", server.handleTrustAction)                          // Phase 28: Trust action preview
	mux.HandleFunc("/trust/action/execute", server.handleTrustActionExecute)           // Phase 28: Execute trust action
	mux.HandleFunc("/trust/action/undo", server.handleTrustActionUndo)                 // Phase 28: Undo trust action
	mux.HandleFunc("/trust/action/receipt", server.handleTrustActionReceipt)           // Phase 28: Trust action receipt
	mux.HandleFunc("/trust/action/dismiss", server.handleTrustActionDismiss)           // Phase 28: Dismiss trust action
	mux.HandleFunc("/connect/truelayer/start", server.handleTrueLayerOAuthStart)       // Phase 29: TrueLayer OAuth start
	mux.HandleFunc("/connect/truelayer/callback", server.handleTrueLayerOAuthCallback) // Phase 29: TrueLayer OAuth callback
	mux.HandleFunc("/disconnect/truelayer", server.handleTrueLayerDisconnect)          // Phase 29: TrueLayer disconnect
	mux.HandleFunc("/run/truelayer-sync", server.handleTrueLayerSync)                  // Phase 29: TrueLayer sync
	mux.HandleFunc("/mirror/finance", server.handleFinanceMirror)                      // Phase 29: Finance mirror page
	mux.HandleFunc("/mirror/finance/ack", server.handleFinanceMirrorAck)               // Phase 29: Finance mirror ack
	mux.HandleFunc("/identity", server.handleIdentity)                                 // Phase 30A: Device identity page
	mux.HandleFunc("/identity/bind", server.handleIdentityBind)                        // Phase 30A: Bind device to circle
	mux.HandleFunc("/replay/export", server.handleReplayExport)                        // Phase 30A: Export replay bundle
	mux.HandleFunc("/replay/import", server.handleReplayImport)                        // Phase 30A: Import replay bundle
	mux.HandleFunc("/mirror/commerce", server.handleCommerceMirror)                    // Phase 31: Commerce mirror page
	mux.HandleFunc("/demo", server.handleDemo)

	// Phase 18 Web Control Center: Core routes
	mux.HandleFunc("/approve", server.handleApprove)           // Approval token verification
	mux.HandleFunc("/runs", server.handleRuns)                 // Run log list
	mux.HandleFunc("/runs/", server.handleRunDetail)           // Run log detail
	mux.HandleFunc("/suppressions", server.handleSuppressions) // Suppression management

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
	log.Printf("Shadow provider: %s", shadowProviderInfo)

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

// populateMockTrustSummaries creates realistic mock trust summaries.
// CRITICAL: Uses abstract magnitude buckets only, never raw counts.
func populateMockTrustSummaries(store *persist.TrustStore, now time.Time) {
	// Create summaries for recent periods showing evidence of restraint

	// Last week: Several obligations were held quietly
	lastWeek := now.AddDate(0, 0, -7)
	summary1 := &domaintrust.TrustSummary{
		Period:          domaintrust.PeriodWeek,
		PeriodKey:       domaintrust.WeekKey(lastWeek),
		SignalKind:      domaintrust.SignalQuietHeld,
		MagnitudeBucket: domainshadow.MagnitudeSeveral,
		CreatedBucket:   domaintrust.FiveMinuteBucket(lastWeek),
		CreatedAt:       lastWeek,
	}
	summary1.SummaryID = summary1.ComputeID()
	summary1.SummaryHash = summary1.ComputeHash()
	_ = store.AppendSummary(summary1)

	// Two weeks ago: A few interruptions were prevented
	twoWeeksAgo := now.AddDate(0, 0, -14)
	summary2 := &domaintrust.TrustSummary{
		Period:          domaintrust.PeriodWeek,
		PeriodKey:       domaintrust.WeekKey(twoWeeksAgo),
		SignalKind:      domaintrust.SignalInterruptionPrevented,
		MagnitudeBucket: domainshadow.MagnitudeAFew,
		CreatedBucket:   domaintrust.FiveMinuteBucket(twoWeeksAgo),
		CreatedAt:       twoWeeksAgo,
	}
	summary2.SummaryID = summary2.ComputeID()
	summary2.SummaryHash = summary2.ComputeHash()
	_ = store.AppendSummary(summary2)

	// Last month: Several items held quietly
	lastMonth := now.AddDate(0, -1, 0)
	summary3 := &domaintrust.TrustSummary{
		Period:          domaintrust.PeriodMonth,
		PeriodKey:       domaintrust.MonthKey(lastMonth),
		SignalKind:      domaintrust.SignalQuietHeld,
		MagnitudeBucket: domainshadow.MagnitudeSeveral,
		CreatedBucket:   domaintrust.FiveMinuteBucket(lastMonth),
		CreatedAt:       lastMonth,
	}
	summary3.SummaryID = summary3.ComputeID()
	summary3.SummaryHash = summary3.ComputeHash()
	_ = store.AppendSummary(summary3)

	log.Printf("Populated %d mock trust summaries", 3)
}

// createShadowProvider creates the appropriate shadow provider based on config and env vars.
//
// Phase 19.3: Azure OpenAI Shadow Provider
//
// CRITICAL: Default is stub provider - real providers require explicit opt-in.
// CRITICAL: Never logs API keys or secrets.
// CRITICAL: Emits fallback event if Azure config is incomplete.
//
// Environment variables (override config file):
//   - QL_SHADOW_REAL_ALLOWED: "true" to enable real providers (default: false)
//   - QL_SHADOW_PROVIDER_KIND: "stub" | "azure_openai" (default: stub)
//   - QL_SHADOW_MODE: "off" | "observe" (default: off)
//   - AZURE_OPENAI_ENDPOINT: Azure OpenAI endpoint URL
//   - AZURE_OPENAI_DEPLOYMENT: Model deployment name
//   - AZURE_OPENAI_API_KEY: API key (never logged)
//   - AZURE_OPENAI_API_VERSION: API version (optional)
func createShadowProvider(cfg *config.MultiCircleConfig, emitter *eventLogger) (domainshadow.ShadowModel, string) {
	// Read env var overrides
	realAllowed := cfg.Shadow.RealAllowed
	if envVal := os.Getenv("QL_SHADOW_REAL_ALLOWED"); envVal == "true" {
		realAllowed = true
	}

	providerKind := cfg.Shadow.ProviderKind
	if envVal := os.Getenv("QL_SHADOW_PROVIDER_KIND"); envVal != "" {
		providerKind = envVal
	}

	// Default to stub if not specified
	if providerKind == "" || providerKind == "none" {
		providerKind = "stub"
	}

	// If real not allowed, force stub
	if !realAllowed {
		emitter.Emit(events.Event{
			Type: events.Phase19_3ProviderSelected,
			Metadata: map[string]string{
				"provider":     "stub",
				"real_allowed": "false",
				"reason":       "real_not_allowed",
			},
		})
		return stub.NewStubModel(), "stub (RealAllowed: false)"
	}

	// If provider kind is stub, use stub
	if providerKind == "stub" {
		emitter.Emit(events.Event{
			Type: events.Phase19_3ProviderSelected,
			Metadata: map[string]string{
				"provider":     "stub",
				"real_allowed": "true",
				"reason":       "provider_kind_stub",
			},
		})
		return stub.NewStubModel(), "stub (RealAllowed: true, kind: stub)"
	}

	// Try to create Azure provider
	if providerKind == "azure_openai" {
		// Check if Azure env vars are configured
		if !azureopenai.IsConfigured() {
			// Fall back to stub with event
			emitter.Emit(events.Event{
				Type: events.Phase19_3ProviderFallback,
				Metadata: map[string]string{
					"requested": "azure_openai",
					"fallback":  "stub",
					"reason":    "missing_env_vars",
				},
			})
			return stub.NewStubModel(), "stub (RealAllowed: true, fallback: missing AZURE_OPENAI_* env vars)"
		}

		// Create Azure provider from env
		provider, err := azureopenai.NewProviderFromEnv()
		if err != nil {
			// Fall back to stub with event
			emitter.Emit(events.Event{
				Type: events.Phase19_3ProviderFallback,
				Metadata: map[string]string{
					"requested": "azure_openai",
					"fallback":  "stub",
					"reason":    "provider_init_failed",
				},
			})
			return stub.NewStubModel(), "stub (RealAllowed: true, fallback: provider init failed)"
		}

		emitter.Emit(events.Event{
			Type: events.Phase19_3ProviderSelected,
			Metadata: map[string]string{
				"provider":     "azure_openai",
				"deployment":   provider.Deployment(),
				"real_allowed": "true",
			},
		})
		// CRITICAL: Never log API key or endpoint details
		return wrapAzureProvider(provider), "azure_openai (RealAllowed: true)"
	}

	// Phase 19.3c: Azure Chat provider with strict JSON output
	if providerKind == "azure_openai_chat" {
		// Check if chat-specific env vars are configured
		if !azureopenai.IsChatConfigured() {
			// Fall back to stub with event
			emitter.Emit(events.Event{
				Type: events.Phase19_3ProviderFallback,
				Metadata: map[string]string{
					"requested": "azure_openai_chat",
					"fallback":  "stub",
					"reason":    "missing_chat_env_vars",
				},
			})
			return stub.NewStubModel(), "stub (RealAllowed: true, fallback: missing AZURE_OPENAI_CHAT_* env vars)"
		}

		// Create Azure chat provider from env
		chatProvider, err := azureopenai.NewChatProviderFromEnv()
		if err != nil {
			// Fall back to stub with event
			emitter.Emit(events.Event{
				Type: events.Phase19_3ProviderFallback,
				Metadata: map[string]string{
					"requested": "azure_openai_chat",
					"fallback":  "stub",
					"reason":    "chat_provider_init_failed",
				},
			})
			return stub.NewStubModel(), "stub (RealAllowed: true, fallback: chat provider init failed)"
		}

		maxSuggestions := cfg.Shadow.GetMaxSuggestions()
		emitter.Emit(events.Event{
			Type: events.Phase19_3ProviderSelected,
			Metadata: map[string]string{
				"provider":        "azure_openai_chat",
				"deployment":      chatProvider.Deployment(),
				"real_allowed":    "true",
				"max_suggestions": fmt.Sprintf("%d", maxSuggestions),
			},
		})
		// CRITICAL: Never log API key or endpoint details
		return wrapAzureChatProvider(chatProvider), "azure_openai_chat (RealAllowed: true)"
	}

	// Unknown provider kind - fall back to stub
	emitter.Emit(events.Event{
		Type: events.Phase19_3ProviderFallback,
		Metadata: map[string]string{
			"requested": providerKind,
			"fallback":  "stub",
			"reason":    "unknown_provider_kind",
		},
	})
	return stub.NewStubModel(), "stub (RealAllowed: true, fallback: unknown kind " + providerKind + ")"
}

// azureProviderWrapper wraps the Azure provider to implement ShadowModel interface.
type azureProviderWrapper struct {
	provider *azureopenai.Provider
}

func wrapAzureProvider(p *azureopenai.Provider) domainshadow.ShadowModel {
	return &azureProviderWrapper{provider: p}
}

func (w *azureProviderWrapper) Name() string {
	return w.provider.Name()
}

func (w *azureProviderWrapper) ProviderKind() domainshadow.ProviderKind {
	return domainshadow.ProviderKindAzureOpenAI
}

func (w *azureProviderWrapper) Observe(ctx domainshadow.ShadowContext) (domainshadow.ShadowRun, error) {
	// The Azure provider uses a different interface (privacy.ShadowInput).
	// For now, we return an empty run with the provider name.
	// Full integration would require converting ShadowContext to ShadowInput.
	return domainshadow.ShadowRun{
		RunID:     "azure-" + ctx.InputsHash[:16],
		CircleID:  ctx.CircleID,
		ModelSpec: w.provider.Name(),
		Signals:   nil, // Azure provider returns suggestions, not legacy signals
	}, nil
}

// Phase 19.3c: azureChatProviderWrapper wraps the Azure Chat provider.
type azureChatProviderWrapper struct {
	provider *azureopenai.ChatProvider
}

func wrapAzureChatProvider(p *azureopenai.ChatProvider) domainshadow.ShadowModel {
	return &azureChatProviderWrapper{provider: p}
}

func (w *azureChatProviderWrapper) Name() string {
	return w.provider.Name()
}

func (w *azureChatProviderWrapper) ProviderKind() domainshadow.ProviderKind {
	return domainshadow.ProviderKindAzureOpenAI
}

func (w *azureChatProviderWrapper) Observe(ctx domainshadow.ShadowContext) (domainshadow.ShadowRun, error) {
	// Phase 19.3c: The chat provider uses privacy.ShadowInput interface.
	// For ShadowModel compatibility, we return a run with the provider name.
	// Full integration is done via the chat-specific endpoint.
	return domainshadow.ShadowRun{
		RunID:     "azure-chat-" + ctx.InputsHash[:16],
		CircleID:  ctx.CircleID,
		ModelSpec: w.provider.Name(),
		Signals:   nil, // Chat provider returns suggestions via Complete(), not Observe()
	}, nil
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
	// Priority: surface cue > proof cue > first-minutes cue > reality cue > shadow receipt primary cue > trust action cue
	// If surface is available, hide proof cue (proof accessible via /surface).
	var displaySurfaceCue *surface.SurfaceCue
	var displayProofCue *proof.ProofCue
	var displayFirstMinutesCue *domainfirstminutes.FirstMinutesCue
	var displayRealityCue *domainreality.RealityCue
	var displayShadowReceiptPrimaryCue *domainshadowview.ShadowReceiptCue
	var displayTrustActionCue *trustActionCueInfo

	circleID := identity.EntityID("default")
	now := s.clk.Now()

	if surfaceCue.Available {
		displaySurfaceCue = &surfaceCue
		// Proof cue hidden - accessible via /surface link
	} else if proofCue.Available {
		displayProofCue = &proofCue
	} else {
		// Phase 26B: First Minutes cue
		// Only show if no other cues are active
		firstMinutesInputs := s.buildFirstMinutesInputs(circleID, now)
		otherCueActive := surfaceCue.Available || proofCue.Available
		if s.firstMinutesEngine.ShouldShowFirstMinutesCue(firstMinutesInputs, otherCueActive) {
			cue := s.firstMinutesEngine.ComputeCue(firstMinutesInputs)
			if cue.Available {
				displayFirstMinutesCue = cue
			}
		}

		// Phase 26C: Reality cue (lowest priority)
		// Only show if no other cues are active (including first-minutes)
		if displayFirstMinutesCue == nil {
			realityInputs := s.buildRealityInputs(circleID, now)
			period := internalreality.PeriodKey(now)

			// Check if already acknowledged
			acked := false
			if s.realityAckStore != nil {
				page := s.realityEngine.BuildPage(realityInputs)
				acked = s.realityAckStore.IsAcked(period, page.StatusHash)
			}

			// Check if reality cue should show
			if s.realityEngine.ShouldShowRealityCue(
				realityInputs,
				acked,
				surfaceCue.Available,
				proofCue.Available,
				false, // journeyCueActive - not tracked separately here
				displayFirstMinutesCue != nil,
			) {
				cue := s.realityEngine.ComputeCue(realityInputs, acked)
				if cue.Available {
					displayRealityCue = cue
				}
			}

			// Phase 27: Shadow Receipt Primary cue (lowest priority)
			// Only show if no other cues are active (including reality)
			if displayRealityCue == nil {
				if s.shadowReceiptStore != nil && s.shadowReceiptAckStore != nil {
					latestReceipt, ok := s.shadowReceiptStore.GetLatestForCircle(circleID)
					if ok && latestReceipt != nil {
						period := latestReceipt.CreatedAt.Format("2006-01-02")
						receiptHash := latestReceipt.Hash()
						// Check if already dismissed
						isDismissed := s.shadowReceiptAckStore.IsDismissed(receiptHash, period)

						// Build input for primary cue
						cueInput := shadowview.BuildPrimaryCueInput{
							Receipt:        latestReceipt,
							IsDismissed:    isDismissed,
							OtherCueActive: false, // All other cues already checked
							ProviderKind:   string(latestReceipt.Provenance.ProviderKind),
						}
						cue := s.shadowviewEngine.BuildPrimaryCue(cueInput)
						if cue.Available {
							displayShadowReceiptPrimaryCue = &cue
						}
					}
				}
			}

			// Phase 28: Trust Action cue (lowest priority)
			// Only show if no other cues are active (including shadow receipt primary)
			if displayShadowReceiptPrimaryCue == nil {
				if s.trustActionEngine != nil && s.trustActionEngine.ShouldShowCue(circleID) {
					displayTrustActionCue = &trustActionCueInfo{
						Available: true,
						CueText:   "One thing could happen — if you let it.",
						LinkText:  "preview",
					}
				}
			}
		}
	}

	data := templateData{
		Title:                   "Today, quietly.",
		CurrentTime:             s.clk.Now().Format("2006-01-02 15:04"),
		TodayPage:               &page,
		SurfaceCue:              displaySurfaceCue,
		ProofCue:                displayProofCue,
		FirstMinutesCue:         displayFirstMinutesCue,
		RealityCue:              displayRealityCue,
		ShadowReceiptPrimaryCue: displayShadowReceiptPrimaryCue,
		TrustActionCue:          displayTrustActionCue,
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

// ═══════════════════════════════════════════════════════════════════════════
// Phase 18.6: First Connect - Consent-first Onboarding
// Reference: docs/ADR/ADR-0038-phase18-6-first-connect.md
// ═══════════════════════════════════════════════════════════════════════════

// handleStart serves the consent page.
// GET /start - Calm consent page with connect options.
func (s *Server) handleStart(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Get current connection state
	state := s.connectionStore.State()

	data := templateData{
		Title:           "First, consent.",
		CurrentTime:     s.clk.Now().Format("2006-01-02 15:04"),
		ConnectionState: state,
		MockMode:        *mockData,
	}

	s.render(w, "start", data)
}

// handleConnections serves the connections list page.
// GET /connections - Shows connected sources.
func (s *Server) handleConnections(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Get current connection state
	state := s.connectionStore.State()

	// Emit state computed event
	s.eventEmitter.Emit(events.Event{
		Type:      events.Phase18_6ConnectionStateComputed,
		Timestamp: s.clk.Now(),
		Metadata: map[string]string{
			"state_hash": state.Hash,
		},
	})

	// Get circle ID from query or use default
	circleID := r.URL.Query().Get("circle_id")
	if circleID == "" {
		// Check if we have a Gmail connection - use that circle
		// This handles the case where OAuth was done with a specific circle
		if s.gmailHandler != nil {
			// Try demo-circle first (common for testing)
			if hasConn, _ := s.gmailHandler.HasConnection(r.Context(), "demo-circle"); hasConn {
				circleID = "demo-circle"
			}
		}
		// Fall back to first configured circle
		if circleID == "" {
			circleIDs := s.multiCircleConfig.CircleIDs()
			if len(circleIDs) > 0 {
				circleID = string(circleIDs[0])
			}
		}
	}

	data := templateData{
		Title:           "Connections",
		CurrentTime:     s.clk.Now().Format("2006-01-02 15:04"),
		ConnectionState: state,
		MockMode:        *mockData,
		CircleID:        circleID,
	}

	s.render(w, "connections", data)
}

// handleConnect handles connect actions.
// POST /connect/:kind - Creates a connect intent.
// GET /connect/:kind - Shows stub connector page (optional).
func (s *Server) handleConnect(w http.ResponseWriter, r *http.Request) {
	// Extract kind from URL path
	path := strings.TrimPrefix(r.URL.Path, "/connect/")
	kind := connection.ConnectionKind(path)

	if !kind.Valid() {
		http.Error(w, "Invalid connection kind", http.StatusBadRequest)
		return
	}

	if r.Method == http.MethodGet {
		// Show stub connector page
		state := s.connectionStore.State()
		kindState := state.Get(kind)

		data := templateData{
			Title:               "Connect " + string(kind),
			CurrentTime:         s.clk.Now().Format("2006-01-02 15:04"),
			ConnectionKind:      kind,
			ConnectionKindState: kindState,
			MockMode:            *mockData,
		}

		s.render(w, "connect-stub", data)
		return
	}

	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Determine mode based on mock flag
	mode := connection.ModeReal
	if *mockData {
		mode = connection.ModeMock
	}

	// Create connect intent
	intent := connection.NewConnectIntent(kind, mode, s.clk.Now(), connection.NoteUserInitiated)

	// Append to store
	if err := s.connectionStore.AppendIntent(intent); err != nil {
		log.Printf("Connection store error: %v", err)
		http.Error(w, "Failed to record intent", http.StatusInternalServerError)
		return
	}

	// Emit events
	s.eventEmitter.Emit(events.Event{
		Type:      events.Phase18_6ConnectionConnectRequested,
		Timestamp: s.clk.Now(),
		Metadata: map[string]string{
			"kind": string(kind),
			"mode": string(mode),
		},
	})

	s.eventEmitter.Emit(events.Event{
		Type:      events.Phase18_6ConnectionIntentRecorded,
		Timestamp: s.clk.Now(),
		Metadata: map[string]string{
			"intent_id": intent.ID,
			"kind":      string(kind),
			"action":    string(intent.Action),
			"mode":      string(mode),
		},
	})

	// Redirect to connections
	http.Redirect(w, r, "/connections", http.StatusFound)
}

// handleDisconnect handles disconnect actions.
// POST /disconnect/:kind - Creates a disconnect intent.
func (s *Server) handleDisconnect(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Extract kind from URL path
	path := strings.TrimPrefix(r.URL.Path, "/disconnect/")
	kind := connection.ConnectionKind(path)

	if !kind.Valid() {
		http.Error(w, "Invalid connection kind", http.StatusBadRequest)
		return
	}

	// Determine mode based on mock flag
	mode := connection.ModeReal
	if *mockData {
		mode = connection.ModeMock
	}

	// Create disconnect intent
	intent := connection.NewDisconnectIntent(kind, mode, s.clk.Now(), connection.NoteUserInitiated)

	// Append to store
	if err := s.connectionStore.AppendIntent(intent); err != nil {
		log.Printf("Connection store error: %v", err)
		http.Error(w, "Failed to record intent", http.StatusInternalServerError)
		return
	}

	// Emit events
	s.eventEmitter.Emit(events.Event{
		Type:      events.Phase18_6ConnectionDisconnectRequested,
		Timestamp: s.clk.Now(),
		Metadata: map[string]string{
			"kind": string(kind),
			"mode": string(mode),
		},
	})

	s.eventEmitter.Emit(events.Event{
		Type:      events.Phase18_6ConnectionIntentRecorded,
		Timestamp: s.clk.Now(),
		Metadata: map[string]string{
			"intent_id": intent.ID,
			"kind":      string(kind),
			"action":    string(intent.Action),
			"mode":      string(mode),
		},
	})

	// Redirect to connections
	http.Redirect(w, r, "/connections", http.StatusFound)
}

// handleMirror serves the mirror proof page.
// Phase 18.7: Mirror Proof - Trust Through Evidence of Reading.
// Shows abstract evidence of what was read, without identifiers.
func (s *Server) handleMirror(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Build mirror input from connection state
	connState := s.connectionStore.State()

	// Build source input states from connection state
	sourceStates := make(map[connection.ConnectionKind]domainmirror.SourceInputState)
	for _, kind := range connection.AllKinds() {
		kindState := connState.Get(kind)
		if kindState.Status == connection.StatusConnectedMock || kindState.Status == connection.StatusConnectedReal {
			mode := connection.ModeMock
			if kindState.Status == connection.StatusConnectedReal {
				mode = connection.ModeReal
			}

			// Build mock observed counts based on kind
			observedCounts := make(map[domainmirror.ObservedCategory]int)
			switch kind {
			case connection.KindEmail:
				observedCounts[domainmirror.ObservedTimeCommitments] = 2
				observedCounts[domainmirror.ObservedReceipts] = 3
			case connection.KindCalendar:
				observedCounts[domainmirror.ObservedTimeCommitments] = 5
			case connection.KindFinance:
				observedCounts[domainmirror.ObservedReceipts] = 4
				observedCounts[domainmirror.ObservedPatterns] = 2
			}

			sourceStates[kind] = domainmirror.SourceInputState{
				Connected:      true,
				Mode:           mode,
				ReadSuccess:    true,
				ObservedCounts: observedCounts,
			}
		}
	}

	mirrorInput := domainmirror.MirrorInput{
		ConnectedSources: sourceStates,
		HeldCount:        3, // Mock held count
		SurfacedCount:    0, // Nothing surfaced
		CircleID:         "demo-circle",
	}

	// Check if there are any connected sources
	if !s.mirrorEngine.HasConnectedSources(mirrorInput) {
		// No mirror shown if no connections
		data := templateData{
			Title:       "Mirror",
			CurrentTime: s.clk.Now().Format("2006-01-02 15:04"),
			MirrorPage:  nil, // Empty mirror
		}
		s.render(w, "mirror", data)
		return
	}

	// Build mirror page
	mirrorPage := s.mirrorEngine.BuildMirrorPage(mirrorInput)

	// Record that mirror was viewed
	if err := s.mirrorAckStore.Record(domainmirror.AckViewed, mirrorPage.Hash, s.clk.Now()); err != nil {
		log.Printf("Mirror ack store error: %v", err)
	}

	// Emit mirror computed event
	s.eventEmitter.Emit(events.Event{
		Type:      events.Phase18_7MirrorComputed,
		Timestamp: s.clk.Now(),
		Metadata: map[string]string{
			"mirror_hash":  mirrorPage.Hash,
			"source_count": fmt.Sprintf("%d", len(mirrorPage.Sources)),
			"held_quietly": fmt.Sprintf("%v", mirrorPage.Outcome.HeldQuietly),
		},
	})

	// Emit mirror viewed event
	s.eventEmitter.Emit(events.Event{
		Type:      events.Phase18_7MirrorViewed,
		Timestamp: s.clk.Now(),
		Metadata: map[string]string{
			"mirror_hash": mirrorPage.Hash,
		},
	})

	data := templateData{
		Title:       "Seen, quietly.",
		CurrentTime: s.clk.Now().Format("2006-01-02 15:04"),
		MirrorPage:  &mirrorPage,
	}

	s.render(w, "mirror", data)
}

// handleGmailConsent shows the Gmail consent page with restraint-first copy.
// Phase 18.9: Real Data Quiet Verification.
// This page explains what we read, store, and refuse to do before OAuth.
func (s *Server) handleGmailConsent(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Get circle ID from query, default to demo circle
	circleID := r.URL.Query().Get("circle_id")
	if circleID == "" {
		circleID = "demo-circle"
	}

	data := templateData{
		Title:       "Connect Gmail",
		CurrentTime: s.clk.Now().Format("2006-01-02 15:04"),
		CircleID:    circleID,
	}

	s.render(w, "gmail-connect", data)
}

// handleGmailOAuthStart starts the Gmail OAuth flow.
// Phase 18.8: Real OAuth (Gmail Read-Only).
func (s *Server) handleGmailOAuthStart(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Get circle ID from query
	circleID := r.URL.Query().Get("circle_id")
	if circleID == "" {
		http.Error(w, "circle_id required", http.StatusBadRequest)
		return
	}

	// Start OAuth flow
	result, err := s.gmailHandler.Start(circleID)
	if err != nil {
		log.Printf("Gmail OAuth start failed: %v", err)
		http.Error(w, "OAuth initialization failed", http.StatusInternalServerError)
		return
	}

	// Emit OAuth started event
	s.eventEmitter.Emit(events.Event{
		Type:      events.Phase18_8OAuthStarted,
		Timestamp: s.clk.Now(),
		Metadata: map[string]string{
			"circle_id":    circleID,
			"provider":     "google",
			"product":      "gmail",
			"receipt_hash": result.Receipt.Hash(),
		},
	})

	// Redirect to Google authorization URL
	http.Redirect(w, r, result.AuthURL, http.StatusFound)
}

// handleGmailOAuthCallback handles the OAuth callback from Google.
// Phase 18.8: Real OAuth (Gmail Read-Only).
func (s *Server) handleGmailOAuthCallback(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Check for OAuth error
	if errParam := r.URL.Query().Get("error"); errParam != "" {
		log.Printf("Gmail OAuth error from Google: %s", errParam)
		http.Redirect(w, r, "/connections?error=oauth_denied", http.StatusFound)
		return
	}

	// Get code and state from query
	code := r.URL.Query().Get("code")
	state := r.URL.Query().Get("state")
	if code == "" || state == "" {
		http.Error(w, "Missing code or state", http.StatusBadRequest)
		return
	}

	// Handle callback
	result, err := s.gmailHandler.Callback(r.Context(), code, state)
	if err != nil {
		log.Printf("Gmail OAuth callback failed: %v", err)

		// Emit callback failure event
		s.eventEmitter.Emit(events.Event{
			Type:      events.Phase18_8OAuthCallback,
			Timestamp: s.clk.Now(),
			Metadata: map[string]string{
				"success":     "false",
				"fail_reason": "callback_failed",
			},
		})

		http.Redirect(w, r, "/connections?error=oauth_failed", http.StatusFound)
		return
	}

	// Emit callback success event
	s.eventEmitter.Emit(events.Event{
		Type:      events.Phase18_8OAuthCallback,
		Timestamp: s.clk.Now(),
		Metadata: map[string]string{
			"circle_id":    result.CircleID,
			"success":      "true",
			"receipt_hash": result.Receipt.Hash(),
		},
	})

	// Update connection store to show real connection via intent
	intent := connection.NewConnectIntent(connection.KindEmail, connection.ModeReal, s.clk.Now(), connection.NoteOAuthCallback)
	if err := s.connectionStore.AppendIntent(intent); err != nil {
		log.Printf("Failed to record connection intent: %v", err)
	}

	// Redirect to connections page with success
	http.Redirect(w, r, "/connections?connected=gmail", http.StatusFound)
}

// handleGmailDisconnect disconnects Gmail.
// Phase 18.8: Real OAuth (Gmail Read-Only).
func (s *Server) handleGmailDisconnect(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Get circle ID from query or form
	circleID := r.URL.Query().Get("circle_id")
	if circleID == "" {
		circleID = r.FormValue("circle_id")
	}
	if circleID == "" {
		http.Error(w, "circle_id required", http.StatusBadRequest)
		return
	}

	// Emit revoke requested event
	s.eventEmitter.Emit(events.Event{
		Type:      events.Phase18_8OAuthRevokeRequested,
		Timestamp: s.clk.Now(),
		Metadata: map[string]string{
			"circle_id": circleID,
			"provider":  "google",
			"product":   "gmail",
		},
	})

	// Revoke connection
	result, err := s.gmailHandler.Revoke(r.Context(), circleID)
	if err != nil {
		log.Printf("Gmail disconnect failed: %v", err)
		// Still successful - revoke is idempotent
	}

	// Emit revoke completed event
	s.eventEmitter.Emit(events.Event{
		Type:      events.Phase18_8OAuthRevokeCompleted,
		Timestamp: s.clk.Now(),
		Metadata: map[string]string{
			"circle_id":        circleID,
			"success":          fmt.Sprintf("%v", result.Receipt.Success),
			"provider_revoked": fmt.Sprintf("%v", result.Receipt.ProviderRevoked),
			"local_removed":    fmt.Sprintf("%v", result.Receipt.LocalRemoved),
			"receipt_hash":     result.Receipt.Hash(),
		},
	})

	// Update connection store via disconnect intent
	disconnectIntent := connection.NewDisconnectIntent(connection.KindEmail, connection.ModeReal, s.clk.Now(), connection.NoteOAuthRevoke)
	if err := s.connectionStore.AppendIntent(disconnectIntent); err != nil {
		log.Printf("Failed to record disconnect intent: %v", err)
	}

	// Redirect to connections page
	http.Redirect(w, r, "/connections?disconnected=gmail", http.StatusFound)
}

// handleGmailSync performs a Gmail sync.
// Phase 19.1: Real Gmail Connection (You-only).
// CRITICAL: Only called explicitly by browsing human. No background polling.
// CRITICAL: Max 25 messages, last 7 days.
// CRITICAL: All Gmail obligations are held by default.
func (s *Server) handleGmailSync(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet && r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Get circle ID from query or form
	circleID := r.URL.Query().Get("circle_id")
	if circleID == "" {
		circleID = r.FormValue("circle_id")
	}
	if circleID == "" {
		http.Error(w, "circle_id required", http.StatusBadRequest)
		return
	}

	// Check if connected
	hasConnection, err := s.gmailHandler.HasConnection(r.Context(), circleID)
	if err != nil || !hasConnection {
		http.Error(w, "Not connected to Gmail", http.StatusPreconditionFailed)
		return
	}

	// Phase 19.1: Emit sync requested event
	s.eventEmitter.Emit(events.Event{
		Type:      events.Phase19_1GmailSyncRequested,
		Timestamp: s.clk.Now(),
		Metadata: map[string]string{
			"circle_id": circleID,
		},
	})

	// Emit sync started event
	s.eventEmitter.Emit(events.Event{
		Type:      events.Phase19_1GmailSyncStarted,
		Timestamp: s.clk.Now(),
		Metadata: map[string]string{
			"circle_id": circleID,
		},
	})

	// Get account email from circle config (for now, use placeholder)
	// In production, this would come from the circle's email configuration
	accountEmail := "me" // Gmail API uses "me" for authenticated party

	// Create a real adapter with the broker for this sync
	// The adapter uses the broker to mint tokens as needed
	broker, ok := s.tokenBroker.(*impl_inmem.Broker)
	if !ok {
		log.Printf("Gmail sync failed: invalid broker type")

		// Create failure receipt
		failReceipt := persist.NewSyncReceipt(
			identity.EntityID(circleID),
			"gmail",
			0, 0, s.clk.Now(),
			false, "invalid_broker",
		)
		s.syncReceiptStore.Store(failReceipt)

		s.eventEmitter.Emit(events.Event{
			Type:      events.Phase19_1GmailSyncFailed,
			Timestamp: s.clk.Now(),
			Metadata: map[string]string{
				"circle_id":    circleID,
				"fail_reason":  "invalid_broker",
				"receipt_hash": failReceipt.Hash,
			},
		})
		http.Error(w, "Internal configuration error", http.StatusInternalServerError)
		return
	}

	adapter := gmailread.NewRealAdapter(broker, s.clk, circleID)

	// Phase 19.1: CRITICAL limits
	// Max 25 messages, last 7 days
	const maxMessages = 25
	const syncDays = 7
	since := s.clk.Now().Add(-time.Duration(syncDays) * 24 * time.Hour)

	messages, err := adapter.FetchMessages(accountEmail, since, maxMessages)
	if err != nil {
		log.Printf("Gmail sync failed: %v", err)

		// Create failure receipt
		failReceipt := persist.NewSyncReceipt(
			identity.EntityID(circleID),
			"gmail",
			0, 0, s.clk.Now(),
			false, "sync_failed",
		)
		s.syncReceiptStore.Store(failReceipt)

		s.eventEmitter.Emit(events.Event{
			Type:      events.Phase19_1GmailSyncFailed,
			Timestamp: s.clk.Now(),
			Metadata: map[string]string{
				"circle_id":    circleID,
				"fail_reason":  "sync_failed",
				"receipt_hash": failReceipt.Hash,
			},
		})

		http.Error(w, "Sync failed", http.StatusInternalServerError)
		return
	}

	messageCount := len(messages)
	eventsStored := 0

	// Store events in event store (no raw content - events already abstracted)
	for _, msg := range messages {
		existingEvent, _ := s.engine.EventStore.GetByID(msg.EventID())
		if existingEvent != nil {
			// Deduplicate
			s.eventEmitter.Emit(events.Event{
				Type:      events.Phase19_1EventDeduplicate,
				Timestamp: s.clk.Now(),
				Metadata: map[string]string{
					"circle_id": circleID,
					"event_id":  msg.EventID(),
				},
			})
			continue
		}

		s.engine.EventStore.Store(msg)
		eventsStored++

		s.eventEmitter.Emit(events.Event{
			Type:      events.Phase19_1EventStored,
			Timestamp: s.clk.Now(),
			Metadata: map[string]string{
				"circle_id": circleID,
				"event_id":  msg.EventID(),
			},
		})
	}

	// Create success receipt with magnitude buckets only
	receipt := persist.NewSyncReceipt(
		identity.EntityID(circleID),
		"gmail",
		messageCount,
		eventsStored,
		s.clk.Now(),
		true, "",
	)
	s.syncReceiptStore.Store(receipt)

	// Emit Phase 19.1 sync receipt created
	s.eventEmitter.Emit(events.Event{
		Type:      events.Phase19_1SyncReceiptCreated,
		Timestamp: s.clk.Now(),
		Metadata: map[string]string{
			"circle_id":            circleID,
			"receipt_id":           receipt.ReceiptID,
			"receipt_hash":         receipt.Hash,
			"magnitude_bucket":     string(receipt.MagnitudeBucket),
			"events_stored_bucket": string(receipt.EventsStoredBucket),
		},
	})

	// Emit sync completed event with magnitude buckets only (no raw counts in metadata)
	s.eventEmitter.Emit(events.Event{
		Type:      events.Phase19_1GmailSyncCompleted,
		Timestamp: s.clk.Now(),
		Metadata: map[string]string{
			"circle_id":            circleID,
			"magnitude_bucket":     string(receipt.MagnitudeBucket),
			"events_stored_bucket": string(receipt.EventsStoredBucket),
			"receipt_hash":         receipt.Hash,
		},
	})

	// Phase 31.1: Gmail Receipt Observers
	// Extract message metadata for receipt classification
	// CRITICAL: Raw data is used for classification ONLY and is NOT stored
	if len(messages) > 0 && s.commerceIngestEngine != nil {
		// Emit receipt scan started event
		s.eventEmitter.Emit(events.Event{
			Type:      events.Phase31_1ReceiptScanStarted,
			Timestamp: s.clk.Now(),
			Metadata: map[string]string{
				"circle_id":        circleID,
				"magnitude_bucket": string(receipt.MagnitudeBucket),
			},
		})

		// Extract message data for classification
		// CRITICAL: This data is used for classification only and is immediately discarded
		messageData := make([]commerceingest.MessageData, 0, len(messages))
		for _, msg := range messages {
			messageData = append(messageData, commerceingest.ExtractMessageData(
				msg.SourceID(),   // MessageID (will be hashed, never stored raw)
				msg.SenderDomain, // Domain only (used for classification, not stored)
				msg.Subject,      // Subject (used for classification, not stored)
				msg.BodyPreview,  // Snippet (used for classification, not stored)
			))
		}

		// Build commerce observations from Gmail receipts
		period := commerceingest.PeriodFromTime(s.clk.Now())
		ingestResult := s.commerceIngestEngine.BuildFromGmailMessages(
			circleID,
			period,
			receipt.Hash,
			messageData,
		)

		// Persist observations to commerce observer store
		for _, obs := range ingestResult.Observations {
			if err := s.commerceObserverStore.PersistObservation(circleID, &obs); err != nil {
				log.Printf("Phase 31.1: Failed to persist observation: %v", err)
			}
		}

		// Emit receipt scan completed event with abstract buckets only
		s.eventEmitter.Emit(events.Event{
			Type:      events.Phase31_1ReceiptScanCompleted,
			Timestamp: s.clk.Now(),
			Metadata: map[string]string{
				"circle_id":         circleID,
				"period":            period,
				"magnitude_bucket":  string(ingestResult.OverallMagnitude),
				"observation_count": strconv.Itoa(len(ingestResult.Observations)),
				"status_hash":       ingestResult.StatusHash,
				"sync_receipt_hash": receipt.Hash,
			},
		})

		// Emit observations persisted event if any observations were created
		if len(ingestResult.Observations) > 0 {
			s.eventEmitter.Emit(events.Event{
				Type:      events.Phase31_1CommerceObservationsPersisted,
				Timestamp: s.clk.Now(),
				Metadata: map[string]string{
					"circle_id":         circleID,
					"period":            period,
					"observation_count": strconv.Itoa(len(ingestResult.Observations)),
					"status_hash":       ingestResult.StatusHash,
				},
			})
		}
	}

	// Return success page or redirect
	http.Redirect(w, r, "/connections?synced=gmail", http.StatusFound)
}

// handleQuietCheck serves the quiet baseline verification page.
// Phase 19.1: Shows a calm checklist verifying quiet principles.
func (s *Server) handleQuietCheck(w http.ResponseWriter, r *http.Request) {
	// Emit quiet check requested event
	s.eventEmitter.Emit(events.Event{
		Type:      events.Phase19_1QuietCheckRequested,
		Timestamp: s.clk.Now(),
	})

	// Get circle ID (use first circle if not specified)
	circleID := r.URL.Query().Get("circle_id")
	if circleID == "" {
		// Use default circle from multi-circle config
		circleIDs := s.multiCircleConfig.CircleIDs()
		if len(circleIDs) > 0 {
			circleID = string(circleIDs[0])
		}
	}

	// Check Gmail connection
	gmailConnected := false
	if circleID != "" {
		hasConn, err := s.gmailHandler.HasConnection(r.Context(), circleID)
		if err == nil && hasConn {
			gmailConnected = true
		}
	}

	// Get latest sync receipt
	var lastSyncTime time.Time
	var lastSyncMagnitude persist.MagnitudeBucket = persist.MagnitudeNone
	if circleID != "" {
		receipt := s.syncReceiptStore.GetLatestByCircle(identity.EntityID(circleID))
		if receipt != nil {
			lastSyncTime = receipt.TimeBucket
			lastSyncMagnitude = receipt.MagnitudeBucket
		}
	}

	// Obligations are always held by default (DefaultToHold = true)
	obligationsHeld := true

	// Auto-surface is always disabled
	autoSurface := false

	// Create quiet check status
	status := persist.NewQuietCheckStatus(
		gmailConnected,
		lastSyncTime,
		lastSyncMagnitude,
		obligationsHeld,
		autoSurface,
	)

	// Emit quiet check computed event
	s.eventEmitter.Emit(events.Event{
		Type:      events.Phase19_1QuietCheckComputed,
		Timestamp: s.clk.Now(),
		Metadata: map[string]string{
			"gmail_connected":  fmt.Sprintf("%t", status.GmailConnected),
			"obligations_held": fmt.Sprintf("%t", status.ObligationsHeld),
			"auto_surface":     fmt.Sprintf("%t", status.AutoSurface),
			"is_quiet":         fmt.Sprintf("%t", status.IsQuiet()),
			"status_hash":      status.Hash,
		},
	})

	data := templateData{
		Title:            "Quiet Check",
		CurrentTime:      s.clk.Now().Format("2006-01-02 15:04"),
		CircleID:         circleID,
		QuietCheckStatus: status,
	}

	s.render(w, "quiet-check", data)
}

// handleShadowRun runs a shadow-mode analysis.
//
// Phase 19.2: LLM Shadow Mode Contract
//
// CRITICAL: This is POST-only - explicit user action required.
// CRITICAL: This does NOT affect any other state - observation ONLY.
// CRITICAL: Results are stored but do NOT influence behavior.
// CRITICAL: Uses stub provider - no real LLM API calls.
func (s *Server) handleShadowRun(w http.ResponseWriter, r *http.Request) {
	// POST only - explicit user action required
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed - POST required", http.StatusMethodNotAllowed)
		return
	}

	// Emit shadow requested event
	s.eventEmitter.Emit(events.Event{
		Type:      events.Phase19_2ShadowRequested,
		Timestamp: s.clk.Now(),
	})

	// Get circle ID - prefer query param, then first circle sorted by ID (deterministic)
	circleID := r.URL.Query().Get("circle_id")
	if circleID == "" {
		// Sort by ID for deterministic selection (map iteration is non-deterministic)
		entities, err := s.identityRepo.GetByType(identity.EntityTypeCircle)
		if err == nil && len(entities) > 0 {
			sort.Slice(entities, func(i, j int) bool {
				return entities[i].ID() < entities[j].ID()
			})
			if circle, ok := entities[0].(*identity.Circle); ok {
				circleID = string(circle.ID())
			}
		}
	}

	if circleID == "" {
		s.eventEmitter.Emit(events.Event{
			Type:      events.Phase19_2ShadowFailed,
			Timestamp: s.clk.Now(),
			Metadata: map[string]string{
				"fail_reason": "no_circle_id",
			},
		})
		http.Redirect(w, r, "/today", http.StatusFound)
		return
	}

	// Build abstract input digest from current state
	// CRITICAL: All data is already abstracted/bucketed - no raw content
	digest := s.buildShadowInputDigest(circleID)

	// Run shadow analysis
	input := shadowllm.RunInput{
		CircleID: identity.EntityID(circleID),
		Digest:   digest,
	}

	output, err := s.shadowEngine.Run(input)
	if err != nil {
		s.eventEmitter.Emit(events.Event{
			Type:      events.Phase19_2ShadowFailed,
			Timestamp: s.clk.Now(),
			Metadata: map[string]string{
				"circle_id":   circleID,
				"fail_reason": "engine_error",
			},
		})
		http.Redirect(w, r, "/today", http.StatusFound)
		return
	}

	// Emit shadow computed event
	s.eventEmitter.Emit(events.Event{
		Type:      events.Phase19_2ShadowComputed,
		Timestamp: s.clk.Now(),
		Metadata: map[string]string{
			"circle_id":        circleID,
			"receipt_id":       output.Receipt.ReceiptID,
			"receipt_hash":     output.Receipt.Hash(),
			"suggestion_count": fmt.Sprintf("%d", len(output.Receipt.Suggestions)),
			"model_spec":       output.Receipt.ModelSpec,
		},
	})

	// Persist receipt
	if err := s.shadowReceiptStore.Append(&output.Receipt); err != nil {
		log.Printf("Shadow receipt store error: %v", err)
	} else {
		s.eventEmitter.Emit(events.Event{
			Type:      events.Phase19_2ShadowPersisted,
			Timestamp: s.clk.Now(),
			Metadata: map[string]string{
				"circle_id":    circleID,
				"receipt_id":   output.Receipt.ReceiptID,
				"receipt_hash": output.Receipt.Hash(),
			},
		})
	}

	// Redirect back to /today (no new UI page)
	http.Redirect(w, r, "/today", http.StatusFound)
}

// handleShadowDiff computes diffs between canon rules and shadow observations.
//
// Phase 19.4: Shadow Diff + Calibration
// CRITICAL: Shadow does NOT affect any execution path. This is measurement ONLY.
func (s *Server) handleShadowDiff(w http.ResponseWriter, r *http.Request) {
	// POST only - explicit action required
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed - POST required", http.StatusMethodNotAllowed)
		return
	}

	// Get circle ID - prefer identity repo circles for consistency with loop
	circleID := r.URL.Query().Get("circle_id")
	if circleID == "" {
		// Use first circle from identity repo (matches loop engine)
		entities, err := s.identityRepo.GetByType(identity.EntityTypeCircle)
		if err == nil && len(entities) > 0 {
			if circle, ok := entities[0].(*identity.Circle); ok {
				circleID = string(circle.ID())
			}
		}
	}

	if circleID == "" {
		http.Redirect(w, r, "/shadow/report", http.StatusFound)
		return
	}

	// Get latest shadow receipt
	receipts := s.shadowReceiptStore.ListForCircle(identity.EntityID(circleID))
	if len(receipts) == 0 {
		// No shadow receipts yet - redirect to report
		http.Redirect(w, r, "/shadow/report", http.StatusFound)
		return
	}
	latestReceipt := receipts[len(receipts)-1]

	// Build canon signals from current loop state
	// Run the loop to get current state
	ctx := r.Context()
	result := s.engine.Run(ctx, loop.RunOptions{})

	// Find the circle result
	var circleResult *loop.CircleResult
	for i := range result.Circles {
		if string(result.Circles[i].CircleID) == circleID {
			circleResult = &result.Circles[i]
			break
		}
	}

	// Build canon signals from loop obligations (empty if no matching circle)
	var canonSignals []shadowdiff.CanonSignal
	if circleResult != nil {
		canonSignals = buildCanonSignalsFromLoop(circleResult)
	}

	// If no canon signals and no shadow suggestions, nothing to diff
	if len(canonSignals) == 0 && len(latestReceipt.Suggestions) == 0 {
		log.Printf("No canon signals and no shadow suggestions - nothing to diff")
		http.Redirect(w, r, "/shadow/report", http.StatusFound)
		return
	}

	// Create diff input with the correct circle ID from the receipt
	diffEngine := shadowdiffengine.NewEngine(s.clk)
	input := shadowdiffengine.DiffInput{
		Canon: shadowdiffengine.CanonResult{
			CircleID:   latestReceipt.CircleID, // Use receipt's circle ID
			Signals:    canonSignals,
			ComputedAt: s.clk.Now(),
		},
		Shadow: latestReceipt,
	}

	// Compute diffs
	output, err := diffEngine.Compute(input)
	if err != nil {
		log.Printf("Failed to compute diffs: %v", err)
		http.Redirect(w, r, "/shadow/report", http.StatusFound)
		return
	}

	// Log summary for debugging
	log.Printf("Shadow diff computed: total=%d, matches=%d, conflicts=%d, canon_only=%d, shadow_only=%d",
		output.Summary.TotalDiffs, output.Summary.MatchCount, output.Summary.ConflictCount,
		output.Summary.CanonOnlyCount, output.Summary.ShadowOnlyCount)

	// Persist each diff result and emit events
	for _, result := range output.Results {
		s.eventEmitter.Emit(events.Event{
			Type:      events.Phase19_4DiffComputed,
			Timestamp: s.clk.Now(),
			Metadata: map[string]string{
				"circle_id": circleID,
				"diff_id":   result.DiffID,
				"agreement": string(result.Agreement),
				"novelty":   string(result.NoveltyType),
			},
		})

		resultCopy := result // Copy for pointer
		if err := s.shadowCalibrationStore.AppendDiff(&resultCopy); err != nil {
			log.Printf("Failed to persist diff: %v", err)
		} else {
			s.eventEmitter.Emit(events.Event{
				Type:      events.Phase19_4DiffPersisted,
				Timestamp: s.clk.Now(),
				Metadata: map[string]string{
					"diff_id": result.DiffID,
				},
			})
		}
	}

	// Redirect to shadow report
	http.Redirect(w, r, "/shadow/report", http.StatusFound)
}

// buildCanonSignalsFromLoop builds canon signals from loop results.
func buildCanonSignalsFromLoop(result *loop.CircleResult) []shadowdiff.CanonSignal {
	// Group obligations by category
	categoryCount := make(map[domainshadow.AbstractCategory]int)
	categoryKeys := make(map[domainshadow.AbstractCategory][]string)

	for _, obl := range result.Obligations {
		cat := mapObligationToCategory(obl)
		categoryCount[cat]++
		categoryKeys[cat] = append(categoryKeys[cat], obl.ID)
	}

	// Build signals for each category with obligations
	var signals []shadowdiff.CanonSignal
	for cat, count := range categoryCount {
		var magnitude domainshadow.MagnitudeBucket
		switch {
		case count == 0:
			magnitude = domainshadow.MagnitudeNothing
		case count <= 3:
			magnitude = domainshadow.MagnitudeAFew
		default:
			magnitude = domainshadow.MagnitudeSeveral
		}

		// Create a signal for each item key
		for _, key := range categoryKeys[cat] {
			sig := shadowdiff.CanonSignal{
				Key: shadowdiff.ComparisonKey{
					CircleID:    result.CircleID,
					Category:    cat,
					ItemKeyHash: key,
				},
				Horizon:         domainshadow.HorizonSoon,
				Magnitude:       magnitude,
				SurfaceDecision: false, // Conservative default
				HoldDecision:    true,  // Default to hold
			}
			signals = append(signals, sig)
		}
	}

	return signals
}

// mapObligationToCategory maps an obligation to an abstract category.
func mapObligationToCategory(obl *obligation.Obligation) domainshadow.AbstractCategory {
	// Map by source type first
	switch obl.SourceType {
	case "email":
		return domainshadow.CategoryPeople
	case "calendar":
		return domainshadow.CategoryTime
	case "finance":
		return domainshadow.CategoryMoney
	default:
		// Fallback to obligation type
		switch obl.Type {
		case obligation.ObligationReply, obligation.ObligationFollowup:
			return domainshadow.CategoryPeople
		case obligation.ObligationAttend, obligation.ObligationDecide:
			return domainshadow.CategoryTime
		case obligation.ObligationPay:
			return domainshadow.CategoryMoney
		default:
			return domainshadow.CategoryWork
		}
	}
}

// buildShadowInputDigest builds an abstract input digest from current state.
//
// CRITICAL: All data must already be abstract/bucketed.
// NO raw content is allowed.
func (s *Server) buildShadowInputDigest(circleID string) domainshadow.ShadowInputDigest {
	// Initialize with defaults
	digest := domainshadow.ShadowInputDigest{
		CircleID:                  identity.EntityID(circleID),
		ObligationCountByCategory: make(map[domainshadow.AbstractCategory]domainshadow.MagnitudeBucket),
		HeldCountByCategory:       make(map[domainshadow.AbstractCategory]domainshadow.MagnitudeBucket),
		SurfaceCandidateCount:     domainshadow.MagnitudeNothing,
		DraftCandidateCount:       domainshadow.MagnitudeNothing,
		TriggersSeen:              false,
		MirrorBucket:              domainshadow.MagnitudeNothing,
	}

	// Check if we have sync receipts (triggers seen)
	if s.syncReceiptStore != nil {
		receipt := s.syncReceiptStore.GetLatestByCircle(identity.EntityID(circleID))
		if receipt != nil && receipt.Success {
			digest.TriggersSeen = true
			// Convert sync magnitude to shadow magnitude
			switch receipt.MagnitudeBucket {
			case persist.MagnitudeMany:
				digest.MirrorBucket = domainshadow.MagnitudeSeveral
			case persist.MagnitudeSeveral:
				digest.MirrorBucket = domainshadow.MagnitudeSeveral
			case persist.MagnitudeHandful:
				digest.MirrorBucket = domainshadow.MagnitudeAFew
			default:
				digest.MirrorBucket = domainshadow.MagnitudeNothing
			}
		}
	}

	// Set some default obligation estimates (abstract only)
	// In a real implementation, would query the obligation store with abstract queries
	if digest.TriggersSeen {
		digest.ObligationCountByCategory[domainshadow.CategoryWork] = domainshadow.MagnitudeAFew
		digest.ObligationCountByCategory[domainshadow.CategoryMoney] = domainshadow.MagnitudeAFew
		digest.HeldCountByCategory[domainshadow.CategoryWork] = domainshadow.MagnitudeAFew
		digest.HeldCountByCategory[domainshadow.CategoryMoney] = domainshadow.MagnitudeAFew
	}

	return digest
}

// =============================================================================
// Phase 19.4: Shadow Calibration Handlers
// =============================================================================

// handleShadowReport shows the shadow calibration report.
//
// Phase 19.4: Shadow Diff + Calibration (Truth Harness)
//
// CRITICAL: This is observation-only. Does NOT affect behavior.
// CRITICAL: Contains only abstract data - no identifiable content.
func (s *Server) handleShadowReport(w http.ResponseWriter, r *http.Request) {
	// GET only
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Emit report requested event
	s.eventEmitter.Emit(events.Event{
		Type:      events.Phase19_4ReportRequested,
		Timestamp: s.clk.Now(),
	})

	// Get current period
	periodBucket := s.clk.Now().UTC().Format("2006-01-02")

	// Check if shadow has been run (has receipts)
	hasReceipts := false
	if s.shadowReceiptStore != nil {
		hasReceipts = true
	}

	// Compute calibration stats from stored diffs
	summary := "No comparisons yet. Run shadow mode first, then compute diffs."
	agreementPct := "0%"
	noveltyPct := "0%"
	conflictPct := "0%"
	usefulnessPct := "0%"
	hasVotes := false

	if s.shadowCalibrationStore != nil {
		diffs := s.shadowCalibrationStore.ListDiffsByPeriod(periodBucket)
		if len(diffs) > 0 {
			// Build votes map from store
			votes := make(map[string]shadowdiff.CalibrationVote)
			for _, diff := range diffs {
				if vote, ok := s.shadowCalibrationStore.GetVoteForDiff(diff.DiffID); ok {
					votes[diff.DiffID] = vote
				}
			}

			// Compute stats using calibration engine
			stats := shadowcalibration.ComputeStats(periodBucket, diffs, votes)

			// Generate summary
			summary = shadowcalibration.OverallSummary(stats)
			agreementPct = fmt.Sprintf("%.0f%%", stats.AgreementRate*100)
			noveltyPct = fmt.Sprintf("%.0f%%", stats.NoveltyRate*100)
			conflictPct = fmt.Sprintf("%.0f%%", stats.ConflictRate*100)

			if stats.VotedCount > 0 {
				hasVotes = true
				usefulnessPct = fmt.Sprintf("%.0f%%", stats.UsefulnessScore*100)
			}
		}
	}

	// Emit report rendered event
	s.eventEmitter.Emit(events.Event{
		Type:      events.Phase19_4ReportRendered,
		Timestamp: s.clk.Now(),
		Metadata: map[string]string{
			"period":       periodBucket,
			"has_receipts": fmt.Sprintf("%v", hasReceipts),
		},
	})

	// Render simple whisper-style report
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	fmt.Fprintf(w, `<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>Shadow Report</title>
    <style>
        body { font-family: system-ui, sans-serif; max-width: 600px; margin: 40px auto; padding: 20px; color: #333; }
        h1 { font-size: 1.2rem; font-weight: normal; color: #666; }
        .summary { font-size: 0.9rem; color: #888; margin: 20px 0; }
        .stats { font-size: 0.8rem; color: #999; }
        .stat { margin: 8px 0; }
        .back { margin-top: 30px; }
        .back a { color: #999; text-decoration: none; font-size: 0.8rem; }
        .back a:hover { color: #666; }
        .whisper { font-size: 0.75rem; color: #aaa; margin-top: 40px; }
    </style>
</head>
<body>
    <h1>Shadow observations</h1>
    <p class="summary">%s</p>
    <div class="stats">
        <div class="stat">Agreement: %s</div>
        <div class="stat">Novelty: %s</div>
        <div class="stat">Conflict: %s</div>
        %s
    </div>
    <div class="back">
        <a href="/shadow/candidates">View candidates &rarr;</a>
        <span style="margin: 0 10px; color: #ddd;">|</span>
        <a href="/today">&larr; Back to today</a>
    </div>
    <p class="whisper">Period: %s</p>
</body>
</html>`, summary, agreementPct, noveltyPct, conflictPct,
		func() string {
			if hasVotes {
				return fmt.Sprintf(`<div class="stat">Usefulness: %s</div>`, usefulnessPct)
			}
			return ""
		}(),
		periodBucket)
}

// handleShadowVote records a calibration vote for a diff.
//
// Phase 19.4: Shadow Diff + Calibration (Truth Harness)
//
// CRITICAL: This is feedback-only. Does NOT affect behavior.
func (s *Server) handleShadowVote(w http.ResponseWriter, r *http.Request) {
	// POST only
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed - POST required", http.StatusMethodNotAllowed)
		return
	}

	// Parse form
	if err := r.ParseForm(); err != nil {
		http.Error(w, "Invalid form", http.StatusBadRequest)
		return
	}

	diffID := r.FormValue("diff_id")
	voteStr := r.FormValue("vote")

	if diffID == "" || voteStr == "" {
		http.Error(w, "Missing diff_id or vote", http.StatusBadRequest)
		return
	}

	// Validate vote
	var vote shadowdiff.CalibrationVote
	switch voteStr {
	case "useful":
		vote = shadowdiff.VoteUseful
	case "unnecessary":
		vote = shadowdiff.VoteUnnecessary
	default:
		http.Error(w, "Invalid vote - must be 'useful' or 'unnecessary'", http.StatusBadRequest)
		return
	}

	// Emit vote recorded event
	s.eventEmitter.Emit(events.Event{
		Type:      events.Phase19_4VoteRecorded,
		Timestamp: s.clk.Now(),
		Metadata: map[string]string{
			"diff_id": diffID,
			"vote":    string(vote),
		},
	})

	// Get the diff to look up its hash
	now := s.clk.Now()
	periodBucket := now.Format("2006-01-02")
	var diffHash string
	if s.shadowCalibrationStore != nil {
		diffs := s.shadowCalibrationStore.ListDiffsByPeriod(periodBucket)
		for _, diff := range diffs {
			if diff.DiffID == diffID {
				diffHash = diff.Hash()
				break
			}
		}
	}

	// Persist the vote
	if s.shadowCalibrationStore != nil && diffHash != "" {
		record := &shadowdiff.CalibrationRecord{
			RecordID:     fmt.Sprintf("vote-%s-%d", diffID[:8], now.UnixNano()),
			DiffID:       diffID,
			DiffHash:     diffHash,
			Vote:         vote,
			PeriodBucket: periodBucket,
			CreatedAt:    now,
		}
		if err := s.shadowCalibrationStore.AppendCalibration(record); err != nil {
			log.Printf("Failed to persist vote: %v", err)
		}
	}

	// Emit vote persisted event
	s.eventEmitter.Emit(events.Event{
		Type:      events.Phase19_4VotePersisted,
		Timestamp: s.clk.Now(),
		Metadata: map[string]string{
			"diff_id": diffID,
			"vote":    string(vote),
		},
	})

	// Redirect back to report
	http.Redirect(w, r, "/shadow/report", http.StatusFound)
}

// =============================================================================
// Phase 19.5: Shadow Gating + Promotion Candidates
// =============================================================================

// diffSourceAdapter adapts ShadowCalibrationStore to shadowgate.DiffSource.
type diffSourceAdapter struct {
	store *persist.ShadowCalibrationStore
}

func (a *diffSourceAdapter) ListDiffsByPeriod(periodKey string) []*shadowdiff.DiffResult {
	diffs := a.store.ListDiffsByPeriod(periodKey)
	result := make([]*shadowdiff.DiffResult, len(diffs))
	for i := range diffs {
		result[i] = diffs[i]
	}
	return result
}

func (a *diffSourceAdapter) GetVoteForDiff(diffID string) (shadowdiff.CalibrationVote, bool) {
	return a.store.GetVoteForDiff(diffID)
}

// handleShadowCandidates shows the shadow candidates page (whisper-style).
//
// Phase 19.5: Shadow Gating + Promotion Candidates
// CRITICAL: Shadow does NOT affect any behavior. This is measurement ONLY.
func (s *Server) handleShadowCandidates(w http.ResponseWriter, r *http.Request) {
	// GET only
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Emit candidates viewed event
	s.eventEmitter.Emit(events.Event{
		Type:      events.Phase19_5CandidatesViewed,
		Timestamp: s.clk.Now(),
	})

	// Get current period
	periodKey := domainshadowgate.PeriodKeyFromTime(s.clk.Now())

	// Get candidates from store
	candidates := s.shadowGateStore.GetCandidates(periodKey)
	candidateCount := len(candidates)

	// Get promotion intents
	intents := s.shadowGateStore.GetPromotionIntents(periodKey)
	intentCount := len(intents)

	// Render whisper-style page
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	fmt.Fprintf(w, `<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>Shadow Candidates</title>
    <style>
        body { font-family: system-ui, sans-serif; max-width: 600px; margin: 40px auto; padding: 20px; color: #333; }
        h1 { font-size: 1.2rem; font-weight: normal; color: #666; }
        .summary { font-size: 0.9rem; color: #888; margin: 20px 0; }
        .candidates { margin: 30px 0; }
        .candidate { border-left: 2px solid #ddd; padding: 10px 15px; margin: 15px 0; }
        .candidate-header { font-size: 0.85rem; color: #666; }
        .candidate-why { font-size: 0.8rem; color: #888; margin-top: 5px; }
        .candidate-stats { font-size: 0.75rem; color: #aaa; margin-top: 5px; }
        .candidate-origin-shadow { border-left-color: #9b59b6; }
        .candidate-origin-conflict { border-left-color: #e74c3c; }
        .candidate-origin-canon { border-left-color: #3498db; }
        .actions { margin-top: 15px; }
        .action-btn { padding: 4px 10px; font-size: 0.75rem; border: 1px solid #ddd; background: white; color: #666; cursor: pointer; margin-right: 5px; }
        .action-btn:hover { background: #f5f5f5; }
        .refresh-form { margin: 20px 0; }
        .refresh-btn { padding: 6px 12px; font-size: 0.8rem; border: 1px solid #ddd; background: white; color: #666; cursor: pointer; }
        .refresh-btn:hover { background: #f5f5f5; }
        .nav { margin-top: 30px; }
        .nav a { color: #999; text-decoration: none; font-size: 0.8rem; margin-right: 15px; }
        .nav a:hover { color: #666; }
        .whisper { font-size: 0.75rem; color: #aaa; margin-top: 40px; }
        .intent-badge { font-size: 0.7rem; color: #27ae60; margin-left: 10px; }
    </style>
</head>
<body>
    <h1>Shadow candidates</h1>
    <p class="summary">Patterns that shadow detected but canon didn't surface.</p>

    <form class="refresh-form" action="/shadow/candidates/refresh" method="POST">
        <button type="submit" class="refresh-btn">Refresh candidates</button>
    </form>

    <div class="candidates">`)

	if candidateCount == 0 {
		fmt.Fprintf(w, `        <p class="summary">No candidates yet. Run shadow mode and compute diffs first.</p>`)
	} else {
		for _, c := range candidates {
			// Determine origin class
			originClass := "candidate-origin-shadow"
			originLabel := "shadow only"
			switch c.Origin {
			case domainshadowgate.OriginConflict:
				originClass = "candidate-origin-conflict"
				originLabel = "conflict"
			case domainshadowgate.OriginCanonOnly:
				originClass = "candidate-origin-canon"
				originLabel = "canon only"
			}

			// Check if has intent
			hasIntent := s.shadowGateStore.HasIntentForCandidate(c.ID)
			intentBadge := ""
			if hasIntent {
				intentBadge = `<span class="intent-badge">⬆ proposed</span>`
			}

			fmt.Fprintf(w, `
        <div class="candidate %s">
            <div class="candidate-header">%s • %s%s</div>
            <div class="candidate-why">%s</div>
            <div class="candidate-stats">
                Usefulness: %s (%d/%d votes) • Horizon: %s • Magnitude: %s
            </div>`,
				originClass,
				string(c.Category),
				originLabel,
				intentBadge,
				c.WhyGeneric,
				string(c.UsefulnessBucket),
				c.VotesUseful,
				c.VotesUseful+c.VotesUnnecessary,
				string(c.HorizonBucket),
				string(c.MagnitudeBucket),
			)

			// Only show propose button if no intent yet
			if !hasIntent {
				fmt.Fprintf(w, `
            <div class="actions">
                <form action="/shadow/candidates/propose" method="POST" style="display: inline;">
                    <input type="hidden" name="candidate_id" value="%s">
                    <input type="hidden" name="note_code" value="promote_rule">
                    <button type="submit" class="action-btn">Propose promotion</button>
                </form>
                <form action="/shadow/candidates/propose" method="POST" style="display: inline;">
                    <input type="hidden" name="candidate_id" value="%s">
                    <input type="hidden" name="note_code" value="needs_more_votes">
                    <button type="submit" class="action-btn">Needs more votes</button>
                </form>
                <form action="/shadow/candidates/propose" method="POST" style="display: inline;">
                    <input type="hidden" name="candidate_id" value="%s">
                    <input type="hidden" name="note_code" value="ignore_for_now">
                    <button type="submit" class="action-btn">Ignore</button>
                </form>
            </div>`, c.ID, c.ID, c.ID)
			}

			fmt.Fprintf(w, `
        </div>`)
		}
	}

	fmt.Fprintf(w, `
    </div>

    <div class="nav">
        <a href="/shadow/report">&larr; Back to report</a>
        <a href="/today">&larr; Back to today</a>
    </div>
    <p class="whisper">Period: %s • Candidates: %d • Intents: %d</p>
</body>
</html>`, periodKey, candidateCount, intentCount)
}

// handleShadowCandidatesRefresh recomputes candidates from diffs.
//
// Phase 19.5: Shadow Gating + Promotion Candidates
// CRITICAL: Shadow does NOT affect any behavior. This is measurement ONLY.
func (s *Server) handleShadowCandidatesRefresh(w http.ResponseWriter, r *http.Request) {
	// POST only
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed - POST required", http.StatusMethodNotAllowed)
		return
	}

	// Emit refresh requested event
	s.eventEmitter.Emit(events.Event{
		Type:      events.Phase19_5CandidatesRefreshRequested,
		Timestamp: s.clk.Now(),
	})

	// Get current period
	periodKey := domainshadowgate.PeriodKeyFromTime(s.clk.Now())

	// Create candidate engine
	engine := shadowgate.NewEngine(s.clk)

	// Create diff source adapter
	diffSource := &diffSourceAdapter{store: s.shadowCalibrationStore}

	// Compute candidates
	input := shadowgate.ComputeInput{
		PeriodKey:  periodKey,
		DiffSource: diffSource,
	}

	output, err := engine.Compute(input)
	if err != nil {
		log.Printf("Failed to compute candidates: %v", err)
		http.Redirect(w, r, "/shadow/candidates", http.StatusFound)
		return
	}

	// Emit computed event
	s.eventEmitter.Emit(events.Event{
		Type:      events.Phase19_5CandidatesComputed,
		Timestamp: s.clk.Now(),
		Metadata: map[string]string{
			"period_key":      periodKey,
			"candidate_count": fmt.Sprintf("%d", len(output.Candidates)),
			"total_diffs":     fmt.Sprintf("%d", output.TotalDiffs),
			"total_votes":     fmt.Sprintf("%d", output.TotalVotes),
		},
	})

	// Persist candidates (refresh semantics - clears old candidates for period)
	if err := s.shadowGateStore.AppendCandidates(periodKey, output.Candidates); err != nil {
		log.Printf("Failed to persist candidates: %v", err)
	} else {
		// Emit persisted event
		s.eventEmitter.Emit(events.Event{
			Type:      events.Phase19_5CandidatesPersisted,
			Timestamp: s.clk.Now(),
			Metadata: map[string]string{
				"period_key":      periodKey,
				"candidate_count": fmt.Sprintf("%d", len(output.Candidates)),
			},
		})
	}

	// Redirect back to candidates page
	http.Redirect(w, r, "/shadow/candidates", http.StatusFound)
}

// handleShadowCandidatesPropose records a promotion intent for a candidate.
//
// Phase 19.5: Shadow Gating + Promotion Candidates
// CRITICAL: Does NOT change any canon thresholds or rules. Intent only.
func (s *Server) handleShadowCandidatesPropose(w http.ResponseWriter, r *http.Request) {
	// POST only
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed - POST required", http.StatusMethodNotAllowed)
		return
	}

	// Parse form
	if err := r.ParseForm(); err != nil {
		http.Error(w, "Invalid form", http.StatusBadRequest)
		return
	}

	candidateID := r.FormValue("candidate_id")
	noteCodeStr := r.FormValue("note_code")

	if candidateID == "" || noteCodeStr == "" {
		http.Error(w, "Missing candidate_id or note_code", http.StatusBadRequest)
		return
	}

	// Validate note code
	noteCode := domainshadowgate.NoteCode(noteCodeStr)
	if !noteCode.IsValid() {
		http.Error(w, "Invalid note_code", http.StatusBadRequest)
		return
	}

	// Get the candidate
	candidate, ok := s.shadowGateStore.GetCandidate(candidateID)
	if !ok {
		http.Error(w, "Candidate not found", http.StatusNotFound)
		return
	}

	// Emit promotion proposed event
	s.eventEmitter.Emit(events.Event{
		Type:      events.Phase19_5PromotionProposed,
		Timestamp: s.clk.Now(),
		Metadata: map[string]string{
			"candidate_id": candidateID,
			"note_code":    string(noteCode),
		},
	})

	// Create promotion intent
	now := s.clk.Now()
	intent := &domainshadowgate.PromotionIntent{
		CandidateID:   candidate.ID,
		CandidateHash: candidate.Hash,
		PeriodKey:     candidate.PeriodKey,
		NoteCode:      noteCode,
		CreatedBucket: domainshadowgate.PeriodKeyFromTime(now),
		CreatedAt:     now,
	}

	// Persist intent
	if err := s.shadowGateStore.AppendPromotionIntent(intent); err != nil {
		log.Printf("Failed to persist promotion intent: %v", err)
	} else {
		// Emit persisted event
		s.eventEmitter.Emit(events.Event{
			Type:      events.Phase19_5PromotionPersisted,
			Timestamp: s.clk.Now(),
			Metadata: map[string]string{
				"candidate_id": candidateID,
				"intent_id":    intent.IntentID,
				"intent_hash":  intent.IntentHash,
				"note_code":    string(noteCode),
			},
		})
	}

	// Redirect back to candidates page
	http.Redirect(w, r, "/shadow/candidates", http.StatusFound)
}

// =============================================================================
// Phase 19.3b: Shadow Health (Go Real Azure + Embeddings)
// =============================================================================

// handleShadowHealth shows the shadow provider health status page.
//
// Phase 19.3b: Go Real Azure + Embeddings
// CRITICAL: Does NOT expose secrets.
func (s *Server) handleShadowHealth(w http.ResponseWriter, r *http.Request) {
	// GET only
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Build runtime flags
	flags := s.getShadowRuntimeFlags()

	// Get last receipt if available
	var lastReceiptHTML string
	// Get latest receipt for first circle (or "personal" default)
	circleID := identity.EntityID("personal")
	if s.multiCircleConfig != nil {
		ids := s.multiCircleConfig.CircleIDs()
		if len(ids) > 0 {
			circleID = ids[0]
		}
	}
	lastReceipt, hasReceipt := s.shadowReceiptStore.GetLatestForCircle(circleID)
	if hasReceipt {
		r := lastReceipt
		receiptID := r.ReceiptID
		if len(receiptID) > 16 {
			receiptID = receiptID[:16] + "..."
		}
		lastReceiptHTML = fmt.Sprintf(`
            <div class="receipt">
                <div class="receipt-label">Last Receipt</div>
                <div class="receipt-row"><span>ID:</span> <span>%s</span></div>
                <div class="receipt-row"><span>Provider:</span> <span>%s</span></div>
                <div class="receipt-row"><span>Status:</span> <span>%s</span></div>
                <div class="receipt-row"><span>Latency:</span> <span>%s</span></div>
                <div class="receipt-row"><span>Created:</span> <span>%s</span></div>
            </div>`,
			receiptID,
			string(r.Provenance.ProviderKind),
			string(r.Provenance.Status),
			string(r.Provenance.LatencyBucket),
			r.CreatedAt.Format("2006-01-02 15:04"),
		)
	} else {
		lastReceiptHTML = `<p class="no-receipt">No receipts yet</p>`
	}

	// Emit viewed event
	s.eventEmitter.Emit(events.Event{
		Type:      events.Phase19_3bHealthViewed,
		Timestamp: s.clk.Now(),
		Metadata: map[string]string{
			"provider_kind": flags.ProviderKind,
			"real_allowed":  boolToString(flags.RealAllowed),
		},
	})

	// Check for query params
	errorMsg := r.URL.Query().Get("error")
	successMsg := ""
	if r.URL.Query().Get("success") == "true" {
		successMsg = "Shadow run completed successfully"
	}

	// Render inline HTML (whisper-style)
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	fmt.Fprintf(w, `<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>Shadow Health</title>
    <style>
        body { font-family: system-ui, sans-serif; max-width: 600px; margin: 40px auto; padding: 20px; color: #333; background: #fafafa; }
        h1 { font-size: 1.2rem; font-weight: normal; color: #666; }
        .summary { font-size: 0.9rem; color: #888; margin: 20px 0; }
        .status-grid { display: grid; grid-template-columns: 1fr 1fr; gap: 16px; margin: 24px 0; }
        .status-item { background: white; padding: 16px; border-radius: 8px; border: 1px solid #e0e0e0; }
        .status-label { font-size: 0.75rem; color: #888; text-transform: uppercase; letter-spacing: 0.05em; }
        .status-value { font-size: 1rem; margin-top: 4px; }
        .status-value.enabled { color: #2e7d32; }
        .status-value.disabled { color: #757575; }
        .receipt { background: white; padding: 16px; border-radius: 8px; border: 1px solid #e0e0e0; margin: 24px 0; }
        .receipt-label { font-size: 0.75rem; color: #888; text-transform: uppercase; letter-spacing: 0.05em; margin-bottom: 12px; }
        .receipt-row { display: flex; justify-content: space-between; font-size: 0.9rem; padding: 4px 0; }
        .no-receipt { color: #888; font-size: 0.9rem; }
        .reassurance { font-size: 0.8rem; color: #888; margin: 24px 0; padding: 12px; background: #f5f5f5; border-radius: 4px; }
        .run-form { margin: 24px 0; }
        .run-btn { background: #1976d2; color: white; border: none; padding: 12px 24px; border-radius: 6px; cursor: pointer; font-size: 0.9rem; }
        .run-btn:hover { background: #1565c0; }
        .run-btn:disabled { background: #bdbdbd; cursor: not-allowed; }
        .error { color: #c62828; background: #ffebee; padding: 12px; border-radius: 4px; margin: 16px 0; }
        .success { color: #2e7d32; background: #e8f5e9; padding: 12px; border-radius: 4px; margin: 16px 0; }
        .back-link { font-size: 0.85rem; color: #666; text-decoration: none; }
        .back-link:hover { color: #333; }
    </style>
</head>
<body>
    <a href="/shadow/report" class="back-link">← Back to Shadow Report</a>
    <h1>Shadow Health</h1>
    <p class="summary">Provider status and health verification</p>
    `)

	// Show error/success messages
	if errorMsg != "" {
		fmt.Fprintf(w, `<div class="error">Error: %s</div>`, errorMsg)
	}
	if successMsg != "" {
		fmt.Fprintf(w, `<div class="success">%s</div>`, successMsg)
	}

	// Status grid
	enabledClass := "disabled"
	enabledText := "Off"
	if flags.Enabled {
		enabledClass = "enabled"
		enabledText = "Observe"
	}

	realClass := "disabled"
	realText := "No"
	if flags.RealAllowed {
		realClass = "enabled"
		realText = "Yes"
	}

	chatClass := "disabled"
	chatText := "No"
	if flags.ChatConfigured {
		chatClass = "enabled"
		chatText = "Yes"
	}

	embedClass := "disabled"
	embedText := "No"
	if flags.EmbedConfigured {
		embedClass = "enabled"
		embedText = "Yes"
	}

	fmt.Fprintf(w, `
    <div class="status-grid">
        <div class="status-item">
            <div class="status-label">Shadow Mode</div>
            <div class="status-value %s">%s</div>
        </div>
        <div class="status-item">
            <div class="status-label">Provider</div>
            <div class="status-value">%s</div>
        </div>
        <div class="status-item">
            <div class="status-label">Real Allowed</div>
            <div class="status-value %s">%s</div>
        </div>
        <div class="status-item">
            <div class="status-label">Chat Configured</div>
            <div class="status-value %s">%s</div>
        </div>
        <div class="status-item">
            <div class="status-label">Embed Configured</div>
            <div class="status-value %s">%s</div>
        </div>
    </div>
    `, enabledClass, enabledText, flags.ProviderKind, realClass, realText, chatClass, chatText, embedClass, embedText)

	// Last receipt
	fmt.Fprint(w, lastReceiptHTML)

	// Run button
	disabled := ""
	if !flags.Enabled {
		disabled = "disabled"
	}
	fmt.Fprintf(w, `
    <form class="run-form" method="POST" action="/shadow/health/run">
        <button type="submit" class="run-btn" %s>Run Health Check</button>
    </form>
    `, disabled)

	// Reassurance
	fmt.Fprint(w, `
    <div class="reassurance">
        No secrets stored. No identifiers sent. Safe constant input only.
    </div>
</body>
</html>`)
}

// handleShadowHealthRun triggers a shadow health run with safe demo input.
//
// Phase 19.3b: Go Real Azure + Embeddings
// CRITICAL: POST only - explicit action required.
func (s *Server) handleShadowHealthRun(w http.ResponseWriter, r *http.Request) {
	// POST only
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	flags := s.getShadowRuntimeFlags()

	// Check if enabled
	if !flags.Enabled {
		s.eventEmitter.Emit(events.Event{
			Type:      events.Phase19_3bHealthRunBlocked,
			Timestamp: s.clk.Now(),
			Metadata: map[string]string{
				"reason": "shadow_mode_disabled",
			},
		})
		http.Redirect(w, r, "/shadow/health?error=disabled", http.StatusFound)
		return
	}

	// Run shadow with demo circle and safe seed
	demoCircleID := "personal"
	if s.multiCircleConfig != nil {
		ids := s.multiCircleConfig.CircleIDs()
		if len(ids) > 0 {
			demoCircleID = string(ids[0])
		}
	}

	// Build minimal demo digest
	digest := domainshadow.ShadowInputDigest{
		CircleID: identity.EntityID(demoCircleID),
		ObligationCountByCategory: map[domainshadow.AbstractCategory]domainshadow.MagnitudeBucket{
			domainshadow.CategoryMoney: domainshadow.MagnitudeAFew,
		},
		HeldCountByCategory:   map[domainshadow.AbstractCategory]domainshadow.MagnitudeBucket{},
		SurfaceCandidateCount: domainshadow.MagnitudeNothing,
		DraftCandidateCount:   domainshadow.MagnitudeNothing,
		TriggersSeen:          false,
		MirrorBucket:          domainshadow.MagnitudeNothing,
	}

	// Run shadow engine
	input := shadowllm.RunInput{
		CircleID: identity.EntityID(demoCircleID),
		Digest:   digest,
		Seed:     19_3, // Deterministic demo seed for 19.3b
	}

	output, err := s.shadowEngine.Run(input)
	if err != nil {
		s.eventEmitter.Emit(events.Event{
			Type:      events.Phase19_3bHealthRunFailed,
			Timestamp: s.clk.Now(),
			Metadata: map[string]string{
				"error_bucket": "engine_error",
			},
		})
		http.Redirect(w, r, "/shadow/health?error=failed", http.StatusFound)
		return
	}

	// Store receipt
	_ = s.shadowReceiptStore.Append(&output.Receipt)

	s.eventEmitter.Emit(events.Event{
		Type:      events.Phase19_3bHealthRunCompleted,
		Timestamp: s.clk.Now(),
		Metadata: map[string]string{
			"receipt_id":    output.Receipt.ReceiptID[:16],
			"provider_kind": string(output.Receipt.Provenance.ProviderKind),
			"status":        string(output.Receipt.Provenance.Status),
		},
	})

	http.Redirect(w, r, "/shadow/health?success=true", http.StatusFound)
}

// getShadowRuntimeFlags builds the current shadow runtime flags.
func (s *Server) getShadowRuntimeFlags() pkgconfig.ShadowRuntimeFlags {
	cfg := s.multiCircleConfig.Shadow
	azureCfg := cfg.AzureOpenAI

	// Check if chat is configured (env var or config)
	chatConfigured := azureCfg.GetChatDeployment() != "" ||
		os.Getenv("AZURE_OPENAI_DEPLOYMENT") != "" ||
		os.Getenv("AZURE_OPENAI_CHAT_DEPLOYMENT") != ""

	// Check if embeddings configured
	embedConfigured := azureCfg.EmbedDeployment != "" ||
		os.Getenv("AZURE_OPENAI_EMBED_DEPLOYMENT") != ""

	return pkgconfig.ShadowRuntimeFlags{
		Enabled:         cfg.Mode == "observe",
		RealAllowed:     cfg.RealAllowed || os.Getenv("QL_SHADOW_REAL_ALLOWED") == "true",
		ProviderKind:    getEffectiveProviderKind(cfg),
		ChatConfigured:  chatConfigured,
		EmbedConfigured: embedConfigured,
	}
}

// getEffectiveProviderKind returns the effective provider kind from config/env.
func getEffectiveProviderKind(cfg pkgconfig.ShadowConfig) string {
	if envKind := os.Getenv("QL_SHADOW_PROVIDER_KIND"); envKind != "" {
		return envKind
	}
	if cfg.ProviderKind != "" && cfg.ProviderKind != "none" {
		return cfg.ProviderKind
	}
	return "stub"
}

// boolToString converts bool to "true"/"false" string.
func boolToString(b bool) string {
	if b {
		return "true"
	}
	return "false"
}

// mapPersistMagnitudeToMirror converts persist.MagnitudeBucket to quietmirror.MirrorMagnitude.
// This helper avoids importing internal/persist in internal/quietmirror.
func mapPersistMagnitudeToMirror(mag persist.MagnitudeBucket) quietmirror.MirrorMagnitude {
	switch mag {
	case persist.MagnitudeNone:
		return quietmirror.MagnitudeNothing
	case persist.MagnitudeHandful:
		return quietmirror.MagnitudeAFew
	case persist.MagnitudeSeveral, persist.MagnitudeMany:
		return quietmirror.MagnitudeSeveral
	default:
		return quietmirror.MagnitudeNothing
	}
}

// =============================================================================
// Phase 19.6: Rule Pack Export (Promotion Pipeline)
// =============================================================================

// handleRulePackList shows the list of available rule packs (whisper-style).
//
// Phase 19.6: Rule Pack Export
// CRITICAL: Packs do NOT apply themselves. No behavior change.
func (s *Server) handleRulePackList(w http.ResponseWriter, r *http.Request) {
	// GET only
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Get all packs (most recent first)
	packs := s.rulepackStore.ListAllPacks()

	// Emit viewed event
	s.eventEmitter.Emit(events.Event{
		Type:      events.Phase19_6PackViewed,
		Timestamp: s.clk.Now(),
		Metadata: map[string]string{
			"pack_count": fmt.Sprintf("%d", len(packs)),
		},
	})

	// Render whisper-style page
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	fmt.Fprintf(w, `<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>Rule Packs</title>
    <style>
        body { font-family: system-ui, sans-serif; max-width: 600px; margin: 40px auto; padding: 20px; color: #333; }
        h1 { font-size: 1.2rem; font-weight: normal; color: #666; }
        .summary { font-size: 0.9rem; color: #888; margin: 20px 0; }
        .packs { margin: 30px 0; }
        .pack { border-left: 2px solid #ddd; padding: 10px 15px; margin: 15px 0; }
        .pack-header { font-size: 0.85rem; color: #666; }
        .pack-stats { font-size: 0.75rem; color: #aaa; margin-top: 5px; }
        .pack-link { font-size: 0.8rem; color: #666; text-decoration: none; }
        .pack-link:hover { color: #333; }
        .build-form { margin: 20px 0; }
        .build-btn { padding: 6px 12px; font-size: 0.8rem; border: 1px solid #ddd; background: white; color: #666; cursor: pointer; }
        .build-btn:hover { background: #f5f5f5; }
        .nav { margin-top: 30px; }
        .nav a { color: #999; text-decoration: none; font-size: 0.8rem; margin-right: 15px; }
        .nav a:hover { color: #666; }
        .whisper { font-size: 0.75rem; color: #aaa; margin-top: 40px; }
    </style>
</head>
<body>
    <h1>Suggestions you can review later</h1>
    <p class="summary">Rule packs contain patterns that might be worth promoting to rules.</p>

    <form class="build-form" action="/shadow/packs/build" method="POST">
        <button type="submit" class="build-btn">Build new pack</button>
    </form>

    <div class="packs">`)

	if len(packs) == 0 {
		fmt.Fprintf(w, `        <p class="summary">No packs yet. Build one from promotion intents.</p>`)
	} else {
		for _, pack := range packs {
			magnitude := pack.ChangeMagnitude()
			magnitudeText := "none"
			switch magnitude {
			case domainshadow.MagnitudeAFew:
				magnitudeText = "a few"
			case domainshadow.MagnitudeSeveral:
				magnitudeText = "several"
			}

			fmt.Fprintf(w, `
        <div class="pack">
            <div class="pack-header">
                <a href="/shadow/packs/%s" class="pack-link">Pack %s</a>
            </div>
            <div class="pack-stats">
                Period: %s • Changes: %s • Created: %s
            </div>
        </div>`,
				pack.PackID,
				pack.PackID[:8],
				pack.PeriodKey,
				magnitudeText,
				pack.CreatedAtBucket,
			)
		}
	}

	fmt.Fprintf(w, `
    </div>

    <div class="nav">
        <a href="/shadow/candidates">&larr; Back to candidates</a>
        <a href="/today">&larr; Back to today</a>
    </div>
    <p class="whisper">Packs: %d</p>
</body>
</html>`, len(packs))
}

// handleRulePackDetail shows pack details or handles export/dismiss.
//
// Phase 19.6: Rule Pack Export
// CRITICAL: Packs do NOT apply themselves. No behavior change.
func (s *Server) handleRulePackDetail(w http.ResponseWriter, r *http.Request) {
	// Extract pack ID from URL
	path := r.URL.Path
	packID := ""
	if len(path) > len("/shadow/packs/") {
		packID = path[len("/shadow/packs/"):]
	}

	// Handle sub-routes
	if strings.HasSuffix(packID, "/export") {
		packID = strings.TrimSuffix(packID, "/export")
		s.handleRulePackExport(w, r, packID)
		return
	}
	if strings.HasSuffix(packID, "/dismiss") {
		packID = strings.TrimSuffix(packID, "/dismiss")
		s.handleRulePackDismiss(w, r, packID)
		return
	}

	// GET for detail view
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Get the pack
	pack, ok := s.rulepackStore.GetPack(packID)
	if !ok {
		http.Error(w, "Pack not found", http.StatusNotFound)
		return
	}

	// Record viewed ack
	_ = s.rulepackStore.AckPack(packID, domainrulepack.AckViewed)

	// Emit viewed event
	s.eventEmitter.Emit(events.Event{
		Type:      events.Phase19_6PackViewed,
		Timestamp: s.clk.Now(),
		Metadata: map[string]string{
			"pack_id":   packID,
			"pack_hash": pack.PackHash,
		},
	})

	// Render whisper-style detail page
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	fmt.Fprintf(w, `<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>Pack %s</title>
    <style>
        body { font-family: system-ui, sans-serif; max-width: 600px; margin: 40px auto; padding: 20px; color: #333; }
        h1 { font-size: 1.2rem; font-weight: normal; color: #666; }
        .summary { font-size: 0.9rem; color: #888; margin: 20px 0; }
        .changes { margin: 30px 0; }
        .change { border-left: 2px solid #ddd; padding: 10px 15px; margin: 15px 0; }
        .change-header { font-size: 0.85rem; color: #666; }
        .change-detail { font-size: 0.75rem; color: #aaa; margin-top: 5px; }
        .change-kind-bias { border-left-color: #3498db; }
        .change-kind-threshold { border-left-color: #27ae60; }
        .change-kind-suppress { border-left-color: #e74c3c; }
        .actions { margin: 20px 0; }
        .action-btn { padding: 6px 12px; font-size: 0.8rem; border: 1px solid #ddd; background: white; color: #666; cursor: pointer; margin-right: 10px; }
        .action-btn:hover { background: #f5f5f5; }
        .nav { margin-top: 30px; }
        .nav a { color: #999; text-decoration: none; font-size: 0.8rem; margin-right: 15px; }
        .nav a:hover { color: #666; }
        .whisper { font-size: 0.75rem; color: #aaa; margin-top: 40px; }
    </style>
</head>
<body>
    <h1>Pack %s</h1>
    <p class="summary">Period: %s • Format: %s</p>

    <div class="actions">
        <form action="/shadow/packs/%s/export" method="POST" style="display: inline;">
            <button type="submit" class="action-btn">Export as text</button>
        </form>
        <form action="/shadow/packs/%s/dismiss" method="POST" style="display: inline;">
            <button type="submit" class="action-btn">Dismiss</button>
        </form>
    </div>

    <div class="changes">`,
		pack.PackID[:8],
		pack.PackID[:8],
		pack.PeriodKey,
		pack.ExportFormatVersion,
		pack.PackID,
		pack.PackID,
	)

	if len(pack.Changes) == 0 {
		fmt.Fprintf(w, `        <p class="summary">No changes in this pack.</p>`)
	} else {
		for _, c := range pack.Changes {
			kindClass := "change-kind-bias"
			kindText := "bias adjust"
			switch c.ChangeKind {
			case domainrulepack.ChangeThresholdAdjust:
				kindClass = "change-kind-threshold"
				kindText = "threshold adjust"
			case domainrulepack.ChangeSuppressSuggest:
				kindClass = "change-kind-suppress"
				kindText = "suppress suggest"
			}

			fmt.Fprintf(w, `
        <div class="change %s">
            <div class="change-header">%s • %s • %s</div>
            <div class="change-detail">
                Usefulness: %s • Confidence: %s • Delta: %s
            </div>
        </div>`,
				kindClass,
				kindText,
				string(c.Category),
				string(c.TargetScope),
				string(c.UsefulnessBucket),
				string(c.VoteConfidenceBucket),
				string(c.SuggestedDelta),
			)
		}
	}

	fmt.Fprintf(w, `
    </div>

    <div class="nav">
        <a href="/shadow/packs">&larr; Back to packs</a>
        <a href="/today">&larr; Back to today</a>
    </div>
    <p class="whisper">Hash: %s</p>
</body>
</html>`, pack.PackHash[:16])
}

// handleRulePackExport exports a pack as text/plain.
//
// Phase 19.6: Rule Pack Export
// CRITICAL: Export does NOT apply the pack. No behavior change.
func (s *Server) handleRulePackExport(w http.ResponseWriter, r *http.Request, packID string) {
	// POST only
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed - POST required", http.StatusMethodNotAllowed)
		return
	}

	// Get the pack
	pack, ok := s.rulepackStore.GetPack(packID)
	if !ok {
		http.Error(w, "Pack not found", http.StatusNotFound)
		return
	}

	// Record exported ack
	_ = s.rulepackStore.AckPack(packID, domainrulepack.AckExported)

	// Emit exported event
	s.eventEmitter.Emit(events.Event{
		Type:      events.Phase19_6PackExported,
		Timestamp: s.clk.Now(),
		Metadata: map[string]string{
			"pack_id":   packID,
			"pack_hash": pack.PackHash,
		},
	})

	// Export as text
	text := pack.ToText()

	// Validate privacy (should never fail, but check anyway)
	if err := domainrulepack.ValidateExportPrivacy(text); err != nil {
		http.Error(w, "Export privacy validation failed", http.StatusInternalServerError)
		return
	}

	// Return as text/plain download
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=\"rulepack-%s.txt\"", packID[:8]))
	w.Write([]byte(text))
}

// handleRulePackDismiss dismisses a pack.
//
// Phase 19.6: Rule Pack Export
// CRITICAL: Dismiss does NOT affect any behavior. It only records acknowledgment.
func (s *Server) handleRulePackDismiss(w http.ResponseWriter, r *http.Request, packID string) {
	// POST only
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed - POST required", http.StatusMethodNotAllowed)
		return
	}

	// Verify pack exists
	pack, ok := s.rulepackStore.GetPack(packID)
	if !ok {
		http.Error(w, "Pack not found", http.StatusNotFound)
		return
	}

	// Record dismissed ack
	if err := s.rulepackStore.AckPack(packID, domainrulepack.AckDismissed); err != nil {
		log.Printf("Failed to ack pack dismissal: %v", err)
	}

	// Emit dismissed event
	s.eventEmitter.Emit(events.Event{
		Type:      events.Phase19_6PackDismissed,
		Timestamp: s.clk.Now(),
		Metadata: map[string]string{
			"pack_id":   packID,
			"pack_hash": pack.PackHash,
		},
	})

	// Redirect back to list
	http.Redirect(w, r, "/shadow/packs", http.StatusFound)
}

// handleRulePackBuild builds a new pack from promotion intents.
//
// Phase 19.6: Rule Pack Export
// CRITICAL: Building a pack does NOT apply it. No behavior change.
func (s *Server) handleRulePackBuild(w http.ResponseWriter, r *http.Request) {
	// POST only
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed - POST required", http.StatusMethodNotAllowed)
		return
	}

	// Emit build requested event
	s.eventEmitter.Emit(events.Event{
		Type:      events.Phase19_6PackBuildRequested,
		Timestamp: s.clk.Now(),
	})

	// Get current period
	periodKey := domainrulepack.PeriodKeyFromTime(s.clk.Now())

	// Create engine
	engine := rulepackengine.NewEngine(s.clk)

	// Build pack
	input := rulepackengine.BuildInput{
		PeriodKey:    periodKey,
		IntentSource: s.shadowGateStore,
	}

	output, err := engine.Build(input)
	if err != nil {
		log.Printf("Failed to build pack: %v", err)
		http.Redirect(w, r, "/shadow/packs", http.StatusFound)
		return
	}

	// Emit built event
	s.eventEmitter.Emit(events.Event{
		Type:      events.Phase19_6PackBuilt,
		Timestamp: s.clk.Now(),
		Metadata: map[string]string{
			"pack_id":           output.Pack.PackID,
			"pack_hash":         output.Pack.PackHash,
			"total_intents":     fmt.Sprintf("%d", output.TotalIntents),
			"qualified_intents": fmt.Sprintf("%d", output.QualifiedIntents),
			"skipped_intents":   fmt.Sprintf("%d", output.SkippedIntents),
			"change_count":      fmt.Sprintf("%d", len(output.Pack.Changes)),
		},
	})

	// Persist pack
	if err := s.rulepackStore.AppendPack(output.Pack); err != nil {
		log.Printf("Failed to persist pack: %v", err)
	} else {
		// Emit persisted event
		s.eventEmitter.Emit(events.Event{
			Type:      events.Phase19_6PackPersisted,
			Timestamp: s.clk.Now(),
			Metadata: map[string]string{
				"pack_id":   output.Pack.PackID,
				"pack_hash": output.Pack.PackHash,
			},
		})
	}

	// Redirect to the new pack
	http.Redirect(w, r, "/shadow/packs/"+output.Pack.PackID, http.StatusFound)
}

// =============================================================================
// Phase 20: Trust Accrual Layer
// =============================================================================
//
// CRITICAL INVARIANTS:
//   - Trust signals are NEVER pushed
//   - Trust signals are NEVER frequent
//   - Trust signals are NEVER actionable
//   - Once dismissed, must not reappear for that period
//   - No buttons styled as buttons
//   - Whisper-style only

// handleTrust shows the trust accrual page.
// Shows 1-3 recent undismissed, meaningful trust summaries.
// CRITICAL: Fully optional, never pushed.
func (s *Server) handleTrust(w http.ResponseWriter, r *http.Request) {
	// Get undismissed summaries
	summaries := s.trustStore.ListUndismissedSummaries()

	// Emit viewed event if there are any summaries
	if len(summaries) > 0 {
		s.eventEmitter.Emit(events.Event{
			Type:      events.Phase20TrustViewed,
			Timestamp: s.clk.Now(),
			Metadata: map[string]string{
				"summary_count": fmt.Sprintf("%d", len(summaries)),
			},
		})
	}

	// Limit to 3 recent summaries
	if len(summaries) > 3 {
		summaries = summaries[:3]
	}

	// Filter to meaningful only
	var meaningful []domaintrust.TrustSummary
	for _, s := range summaries {
		if s.IsMeaningful() {
			meaningful = append(meaningful, s)
		}
	}

	// Render page
	const trustHTML = `<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>Proof over time</title>
    <style>
        * { box-sizing: border-box; margin: 0; padding: 0; }
        body {
            font-family: -apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, sans-serif;
            background: #fafafa;
            color: #333;
            min-height: 100vh;
            display: flex;
            flex-direction: column;
            align-items: center;
            justify-content: center;
            padding: 2rem;
        }
        .container {
            max-width: 480px;
            width: 100%;
        }
        h1 {
            font-size: 1.1rem;
            font-weight: 400;
            color: #666;
            margin-bottom: 2rem;
            text-align: center;
        }
        .empty {
            text-align: center;
            color: #999;
            font-size: 0.9rem;
            padding: 3rem 0;
        }
        .summary {
            background: white;
            border: 1px solid #e0e0e0;
            border-radius: 8px;
            padding: 1.5rem;
            margin-bottom: 1rem;
        }
        .summary-signal {
            font-size: 0.95rem;
            color: #555;
            margin-bottom: 0.75rem;
        }
        .summary-chips {
            display: flex;
            gap: 0.5rem;
            flex-wrap: wrap;
        }
        .chip {
            display: inline-block;
            padding: 0.25rem 0.75rem;
            background: #f0f0f0;
            border-radius: 12px;
            font-size: 0.75rem;
            color: #666;
        }
        .dismiss {
            display: block;
            text-align: center;
            margin-top: 0.75rem;
            font-size: 0.8rem;
            color: #999;
            text-decoration: none;
            opacity: 0.6;
        }
        .dismiss:hover { opacity: 1; }
        .back {
            display: block;
            text-align: center;
            margin-top: 2rem;
            font-size: 0.85rem;
            color: #999;
            text-decoration: none;
        }
        .back:hover { color: #666; }
    </style>
</head>
<body>
    <div class="container">
        <h1>Proof over time</h1>
        {{if not .Summaries}}
        <div class="empty">
            Nothing to show.<br>
            Silence is the default.
        </div>
        {{else}}
        {{range .Summaries}}
        <div class="summary">
            <div class="summary-signal">{{.SignalKind.HumanReadable}}</div>
            <div class="summary-chips">
                <span class="chip">{{.Period}}</span>
                <span class="chip">{{.MagnitudeBucket}}</span>
            </div>
            <a class="dismiss" href="/trust/dismiss?id={{.SummaryID}}">dismiss</a>
        </div>
        {{end}}
        {{end}}
        <a class="back" href="/today">← back</a>
    </div>
</body>
</html>`

	tmpl, err := template.New("trust").Parse(trustHTML)
	if err != nil {
		http.Error(w, "Template error", http.StatusInternalServerError)
		return
	}

	data := struct {
		Summaries []domaintrust.TrustSummary
	}{
		Summaries: meaningful,
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := tmpl.Execute(w, data); err != nil {
		log.Printf("Template execution error: %v", err)
	}
}

// handleTrustDismiss handles dismissal of a trust summary.
// Once dismissed, must not reappear for that period.
func (s *Server) handleTrustDismiss(w http.ResponseWriter, r *http.Request) {
	summaryID := r.URL.Query().Get("id")
	if summaryID == "" {
		http.Redirect(w, r, "/trust", http.StatusFound)
		return
	}

	// Dismiss the summary
	if err := s.trustStore.DismissSummary(summaryID); err != nil {
		log.Printf("Failed to dismiss trust summary: %v", err)
	} else {
		// Emit dismissed event
		summary, _ := s.trustStore.GetSummary(summaryID)
		hash := ""
		if summary != nil {
			hash = summary.SummaryHash
		}
		s.eventEmitter.Emit(events.Event{
			Type:      events.Phase20TrustDismissed,
			Timestamp: s.clk.Now(),
			Metadata: map[string]string{
				"summary_id":   summaryID,
				"summary_hash": hash,
			},
		})
	}

	// Redirect back to trust page
	http.Redirect(w, r, "/trust", http.StatusFound)
}

// handleOnboarding serves the unified onboarding page.
//
// Phase 21: Unified Onboarding
//
// CRITICAL: Calm, minimal, truthful copy.
// CRITICAL: Shows mode indicator (Demo/Connected/Shadow).
// CRITICAL: No goroutines. Deterministic rendering.
func (s *Server) handleOnboarding(w http.ResponseWriter, r *http.Request) {
	// Check for Gmail connection (use demo-circle as default)
	circleID := "demo-circle"
	hasGmail := false
	if s.gmailHandler != nil {
		hasConn, err := s.gmailHandler.HasConnection(r.Context(), circleID)
		if err == nil && hasConn {
			hasGmail = true
		}
	}

	// Get shadow config
	shadowCfg := s.multiCircleConfig.Shadow

	// Get latest shadow receipt for the circle
	var latestReceipt *domainshadow.ShadowReceipt
	if receipt, ok := s.shadowReceiptStore.GetLatestForCircle(identity.EntityID(circleID)); ok {
		latestReceipt = receipt
	}

	// Derive current mode
	modeInput := mode.DeriveModeInput{
		HasGmailConnection:   hasGmail,
		ShadowProviderIsStub: shadowCfg.ProviderKind == "" || shadowCfg.ProviderKind == "stub",
		ShadowRealAllowed:    shadowCfg.RealAllowed,
		LatestShadowReceipt:  latestReceipt,
	}
	modeIndicator := s.modeEngine.DeriveModeIndicator(modeInput)

	// Emit event
	s.eventEmitter.Emit(events.Event{
		Type:      events.Phase21OnboardingViewed,
		Timestamp: s.clk.Now(),
		Metadata: map[string]string{
			"mode": string(modeIndicator.Mode),
		},
	})

	// Render page with inline template
	const onboardingHTML = `<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>Welcome</title>
    <style>
        * { box-sizing: border-box; margin: 0; padding: 0; }
        body {
            font-family: -apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, sans-serif;
            background: #fafafa;
            color: #333;
            min-height: 100vh;
            display: flex;
            flex-direction: column;
            align-items: center;
            justify-content: center;
            padding: 2rem;
        }
        .container {
            max-width: 480px;
            width: 100%;
            text-align: center;
        }
        h1 {
            font-size: 1.5rem;
            font-weight: 400;
            color: #333;
            margin-bottom: 1.5rem;
        }
        .tagline {
            font-size: 0.95rem;
            color: #666;
            margin-bottom: 2rem;
            line-height: 1.6;
        }
        .mode-indicator {
            display: inline-block;
            padding: 0.5rem 1rem;
            background: #f0f0f0;
            border-radius: 20px;
            font-size: 0.85rem;
            color: #666;
            margin-bottom: 2rem;
        }
        .mode-demo { background: #fff3e0; color: #e65100; }
        .mode-connected { background: #e3f2fd; color: #1565c0; }
        .mode-shadow { background: #e8f5e9; color: #2e7d32; }
        .section {
            background: white;
            border: 1px solid #e0e0e0;
            border-radius: 8px;
            padding: 1.5rem;
            margin-bottom: 1rem;
            text-align: left;
        }
        .section-title {
            font-size: 0.9rem;
            font-weight: 500;
            color: #555;
            margin-bottom: 0.75rem;
        }
        .section-body {
            font-size: 0.85rem;
            color: #666;
            line-height: 1.5;
        }
        .actions {
            margin-top: 2rem;
        }
        .btn {
            display: inline-block;
            padding: 0.75rem 1.5rem;
            background: #333;
            color: white;
            text-decoration: none;
            border-radius: 6px;
            font-size: 0.9rem;
            margin: 0.5rem;
        }
        .btn:hover { background: #555; }
        .btn-secondary {
            background: transparent;
            color: #666;
            border: 1px solid #ddd;
        }
        .btn-secondary:hover { background: #f5f5f5; }
        .footer {
            margin-top: 2rem;
            font-size: 0.75rem;
            color: #999;
        }
    </style>
</head>
<body>
    <div class="container">
        <h1>QuantumLife</h1>
        <p class="tagline">
            We watch your email so you don't have to.<br>
            Nothing happens unless you say so.
        </p>

        {{if .Mode}}
        <div class="mode-indicator mode-{{.Mode.Mode}}">
            {{.Mode.DisplayText}}
        </div>
        {{end}}

        <div class="section">
            <div class="section-title">What we do</div>
            <p class="section-body">
                We read your email headers (never content).
                We notice patterns. We suggest, never act.
                All proofs are recorded.
            </p>
        </div>

        <div class="section">
            <div class="section-title">What we don't do</div>
            <p class="section-body">
                We never send emails. We never make purchases.
                We never share data. We never nag.
            </p>
        </div>

        <div class="actions">
            <a href="/connections" class="btn">Connect accounts</a>
            <a href="/today" class="btn btn-secondary">View today</a>
        </div>

        <div class="footer">
            Deterministic. Synchronous. Quiet.
        </div>
    </div>
</body>
</html>`

	tmpl, err := template.New("onboarding").Parse(onboardingHTML)
	if err != nil {
		http.Error(w, "Template error", http.StatusInternalServerError)
		return
	}

	data := struct {
		Mode *mode.ModeIndicator
	}{
		Mode: &modeIndicator,
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := tmpl.Execute(w, data); err != nil {
		log.Printf("Template execution error: %v", err)
	}
}

// handleShadowReceipt serves the shadow receipt proof page.
//
// Phase 21: Shadow Receipt Viewer
//
// CRITICAL: Shows ONLY abstract buckets and hashes.
// CRITICAL: No raw content. No identifiable information.
// CRITICAL: No goroutines. Deterministic rendering.
func (s *Server) handleShadowReceipt(w http.ResponseWriter, r *http.Request) {
	// Get circle ID - prefer query param, then first circle sorted by ID (deterministic)
	circleID := r.URL.Query().Get("circle_id")
	if circleID == "" {
		entities, err := s.identityRepo.GetByType(identity.EntityTypeCircle)
		if err == nil && len(entities) > 0 {
			// Sort by ID for deterministic selection (map iteration is non-deterministic)
			sort.Slice(entities, func(i, j int) bool {
				return entities[i].ID() < entities[j].ID()
			})
			if circle, ok := entities[0].(*identity.Circle); ok {
				circleID = string(circle.ID())
			}
		}
	}
	if circleID == "" {
		circleID = "demo-circle"
	}

	// Check for Gmail connection
	hasGmail := false
	if s.gmailHandler != nil {
		hasConn, err := s.gmailHandler.HasConnection(r.Context(), circleID)
		if err == nil && hasConn {
			hasGmail = true
		}
	}

	// Get latest shadow receipt for the circle
	var receipt *domainshadow.ShadowReceipt
	if r, ok := s.shadowReceiptStore.GetLatestForCircle(identity.EntityID(circleID)); ok {
		receipt = r
	}

	// Get calibration data (Phase 19.4+)
	// For now, we derive from period-based vote counts
	var agreementBucket, voteBucket string
	periodBucket := s.clk.Now().UTC().Format("2006-01-02")
	useful, unnecessary := s.shadowCalibrationStore.CountVotesByPeriod(periodBucket)
	if useful > 0 || unnecessary > 0 {
		if useful > unnecessary {
			voteBucket = "mostly_useful"
		} else if unnecessary > useful {
			voteBucket = "mostly_unnecessary"
		} else {
			voteBucket = "mixed"
		}
	}
	// Agreement bucket from diff distribution (simplified)
	diffs := s.shadowCalibrationStore.ListDiffsByPeriod(periodBucket)
	if len(diffs) > 0 {
		matchCount := 0
		for _, d := range diffs {
			if d.Agreement == "match" {
				matchCount++
			}
		}
		if matchCount == len(diffs) {
			agreementBucket = "all_match"
		} else if matchCount > len(diffs)/2 {
			agreementBucket = "mostly_match"
		} else {
			agreementBucket = "mixed"
		}
	}

	// Build the page
	pageInput := shadowview.BuildPageInput{
		Receipt:              receipt,
		HasGmailConnection:   hasGmail,
		CalibrationAgreement: agreementBucket,
		CalibrationVote:      voteBucket,
	}
	page := s.shadowviewEngine.BuildPage(pageInput)

	// Mark as viewed in ack store (Phase 21)
	if receipt != nil {
		_ = s.shadowviewAckStore.Record(shadowview.AckViewed, receipt.Hash(), periodBucket, s.clk.Now())
	}

	// Phase 27: Record viewed and check vote eligibility
	hasVoted := false
	if receipt != nil {
		_ = s.shadowReceiptAckStore.RecordViewed(receipt.Hash(), periodBucket)
		hasVoted = s.shadowReceiptAckStore.HasVoted(receipt.Hash())
	}

	// Phase 27: Build primary page
	shadowCfg := s.multiCircleConfig.Shadow
	primaryPageInput := shadowview.BuildPrimaryPageInput{
		Receipt:      receipt,
		ProviderKind: shadowCfg.ProviderKind,
		HasVoted:     hasVoted,
		IsDismissed:  false,
	}
	primaryPage := s.shadowviewEngine.BuildPrimaryPage(primaryPageInput)

	// Derive mode for page (shadowCfg already declared above)
	modeInput := mode.DeriveModeInput{
		HasGmailConnection:   hasGmail,
		ShadowProviderIsStub: shadowCfg.ProviderKind == "" || shadowCfg.ProviderKind == "stub",
		ShadowRealAllowed:    shadowCfg.RealAllowed,
		LatestShadowReceipt:  receipt,
	}
	modeIndicator := s.modeEngine.DeriveModeIndicator(modeInput)

	// Emit Phase 21 event
	s.eventEmitter.Emit(events.Event{
		Type:      events.Phase21ShadowReceiptViewed,
		Timestamp: s.clk.Now(),
		Metadata: map[string]string{
			"has_receipt":  fmt.Sprintf("%v", page.HasReceipt),
			"receipt_hash": page.ReceiptHash,
			"mode":         string(modeIndicator.Mode),
		},
	})

	// Emit Phase 27 event
	s.eventEmitter.Emit(events.Event{
		Type:      events.Phase27ShadowReceiptRendered,
		Timestamp: s.clk.Now(),
		Metadata: map[string]string{
			"has_receipt":     fmt.Sprintf("%v", primaryPage.HasReceipt),
			"status_hash":     primaryPage.StatusHash,
			"provider_kind":   string(primaryPage.Provider.Kind),
			"model_consulted": fmt.Sprintf("%v", primaryPage.Provider.WasConsulted),
			"vote_eligible":   fmt.Sprintf("%v", primaryPage.VoteEligibility.Eligible),
			"has_voted":       fmt.Sprintf("%v", hasVoted),
		},
	})

	// Render page with inline template
	const shadowReceiptHTML = `<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>Shadow Receipt</title>
    <style>
        * { box-sizing: border-box; margin: 0; padding: 0; }
        body {
            font-family: -apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, sans-serif;
            background: #fafafa;
            color: #333;
            min-height: 100vh;
            display: flex;
            flex-direction: column;
            align-items: center;
            justify-content: center;
            padding: 2rem;
        }
        .container {
            max-width: 520px;
            width: 100%;
        }
        h1 {
            font-size: 1.1rem;
            font-weight: 400;
            color: #666;
            margin-bottom: 0.5rem;
            text-align: center;
        }
        .subtitle {
            text-align: center;
            font-size: 0.85rem;
            color: #999;
            margin-bottom: 2rem;
        }
        .mode-indicator {
            display: block;
            text-align: center;
            padding: 0.4rem 0.8rem;
            background: #f0f0f0;
            border-radius: 12px;
            font-size: 0.75rem;
            color: #666;
            margin: 0 auto 2rem auto;
            width: fit-content;
        }
        .mode-demo { background: #fff3e0; color: #e65100; }
        .mode-connected { background: #e3f2fd; color: #1565c0; }
        .mode-shadow { background: #e8f5e9; color: #2e7d32; }
        .section {
            background: white;
            border: 1px solid #e0e0e0;
            border-radius: 8px;
            padding: 1.25rem;
            margin-bottom: 1rem;
        }
        .section-title {
            font-size: 0.8rem;
            font-weight: 500;
            color: #999;
            text-transform: uppercase;
            letter-spacing: 0.5px;
            margin-bottom: 0.75rem;
        }
        .section-body {
            font-size: 0.9rem;
            color: #555;
            line-height: 1.5;
        }
        .chip {
            display: inline-block;
            padding: 0.25rem 0.75rem;
            background: #f0f0f0;
            border-radius: 12px;
            font-size: 0.75rem;
            color: #666;
            margin-right: 0.5rem;
            margin-bottom: 0.5rem;
        }
        .restraint-list {
            list-style: none;
        }
        .restraint-list li {
            padding: 0.25rem 0;
            font-size: 0.85rem;
            color: #2e7d32;
        }
        .restraint-list li::before {
            content: "✓ ";
            color: #4caf50;
        }
        .hash {
            font-family: "SF Mono", Monaco, monospace;
            font-size: 0.7rem;
            color: #999;
            word-break: break-all;
            background: #f5f5f5;
            padding: 0.5rem;
            border-radius: 4px;
            margin-top: 0.5rem;
        }
        .back {
            display: block;
            text-align: center;
            margin-top: 2rem;
            font-size: 0.85rem;
            color: #999;
            text-decoration: none;
        }
        .back:hover { color: #666; }
        .empty {
            text-align: center;
            color: #999;
            font-size: 0.9rem;
            padding: 3rem 0;
        }
    </style>
</head>
<body>
    <div class="container">
        <h1>Shadow Receipt</h1>
        <p class="subtitle">Proof of observation, not action</p>

        {{if .Mode}}
        <div class="mode-indicator mode-{{.Mode.Mode}}">
            {{.Mode.DisplayText}}
        </div>
        {{end}}

        {{if not .Page.HasReceipt}}
        <div class="empty">
            No shadow receipt recorded yet.<br>
            Connect an account and run shadow mode.
        </div>
        {{else}}

        <div class="section">
            <div class="section-title">Source</div>
            <p class="section-body">{{.Page.Source.Statement}}</p>
        </div>

        <div class="section">
            <div class="section-title">Observation</div>
            <p class="section-body">{{.Page.Observation.Statement}}</p>
            <div style="margin-top: 0.75rem;">
                <span class="chip">{{.Page.Observation.Magnitude}}</span>
                <span class="chip">{{.Page.Observation.Horizon}}</span>
                {{range .Page.Observation.Categories}}
                <span class="chip">{{.}}</span>
                {{end}}
            </div>
        </div>

        <div class="section">
            <div class="section-title">Confidence</div>
            <p class="section-body">
                <span class="chip">{{.Page.Confidence.Bucket}}</span>
                {{.Page.Confidence.Statement}}
            </p>
        </div>

        <div class="section">
            <div class="section-title">Restraint</div>
            <ul class="restraint-list">
                {{range .Page.Restraint.Statements}}
                <li>{{.}}</li>
                {{end}}
            </ul>
        </div>

        {{if .Page.Calibration.HasCalibration}}
        <div class="section">
            <div class="section-title">Calibration</div>
            <p class="section-body">{{.Page.Calibration.Statement}}</p>
        </div>
        {{end}}

        <div class="section">
            <div class="section-title">Trust Anchor</div>
            <p class="section-body">
                Period: {{.Page.TrustAnchor.PeriodLabel}}<br>
                {{.Page.TrustAnchor.Statement}}
            </p>
            {{if .Page.TrustAnchor.ReceiptHash}}
            <div class="hash">{{.Page.TrustAnchor.ReceiptHash}}</div>
            {{end}}
        </div>

        {{end}}

        <a class="back" href="/today">← back</a>
    </div>
</body>
</html>`

	tmpl, err := template.New("shadowreceipt").Parse(shadowReceiptHTML)
	if err != nil {
		http.Error(w, "Template error", http.StatusInternalServerError)
		return
	}

	data := struct {
		Mode *mode.ModeIndicator
		Page *shadowview.ShadowReceiptPage
	}{
		Mode: &modeIndicator,
		Page: &page,
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := tmpl.Execute(w, data); err != nil {
		log.Printf("Template execution error: %v", err)
	}
}

// handleShadowReceiptDismiss dismisses the shadow receipt cue for the current period.
//
// Phase 21/27: Whisper rule integration
//
// CRITICAL: Stores ONLY hash - never raw content.
// CRITICAL: Dismissal is per-period (daily bucket).
func (s *Server) handleShadowReceiptDismiss(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	receiptHash := r.FormValue("receipt_hash")
	if receiptHash == "" {
		http.Redirect(w, r, "/today", http.StatusFound)
		return
	}

	// Record dismissal in Phase 21 store
	periodBucket := s.clk.Now().UTC().Format("2006-01-02")
	if err := s.shadowviewAckStore.Record(shadowview.AckDismissed, receiptHash, periodBucket, s.clk.Now()); err != nil {
		log.Printf("Failed to record shadow receipt dismissal: %v", err)
	}

	// Record dismissal in Phase 27 store
	if err := s.shadowReceiptAckStore.RecordDismissed(receiptHash, periodBucket); err != nil {
		log.Printf("Failed to record shadow receipt dismissal (Phase 27): %v", err)
	}

	// Emit Phase 21 event
	s.eventEmitter.Emit(events.Event{
		Type:      events.Phase21ShadowReceiptDismissed,
		Timestamp: s.clk.Now(),
		Metadata: map[string]string{
			"receipt_hash":  receiptHash,
			"period_bucket": periodBucket,
		},
	})

	// Emit Phase 27 event
	s.eventEmitter.Emit(events.Event{
		Type:      events.Phase27ShadowReceiptDismissed,
		Timestamp: s.clk.Now(),
		Metadata: map[string]string{
			"receipt_hash":  receiptHash,
			"period_bucket": periodBucket,
		},
	})

	// Redirect back to today page
	http.Redirect(w, r, "/today", http.StatusFound)
}

// handleShadowReceiptVote records a vote on shadow receipt restraint.
//
// Phase 27: Real Shadow Receipt (Primary Proof of Intelligence)
//
// CRITICAL INVARIANTS:
//   - Vote does NOT change behavior
//   - Vote feeds Phase 19 calibration only
//   - One vote per receipt hash
//   - Vote dismissal removes prompt permanently for that receipt
//
// Reference: docs/ADR/ADR-0058-phase27-real-shadow-receipt-primary-proof.md
func (s *Server) handleShadowReceiptVote(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	receiptHash := r.FormValue("receipt_hash")
	voteChoice := r.FormValue("vote")

	if receiptHash == "" || voteChoice == "" {
		http.Redirect(w, r, "/shadow/receipt", http.StatusFound)
		return
	}

	// Map vote choice
	var choice domainshadowview.VoteChoice
	switch voteChoice {
	case "useful":
		choice = domainshadowview.VoteUseful
	case "unnecessary":
		choice = domainshadowview.VoteUnnecessary
	case "skip":
		choice = domainshadowview.VoteSkip
	default:
		http.Redirect(w, r, "/shadow/receipt", http.StatusFound)
		return
	}

	periodBucket := s.clk.Now().UTC().Format("2006-01-02")

	// Record vote in Phase 27 store
	// CRITICAL: Vote does NOT change behavior.
	// CRITICAL: Vote feeds Phase 19 calibration only (via CountVotesByPeriod).
	vote := &domainshadowview.ShadowReceiptVote{
		ReceiptHash:  receiptHash,
		Choice:       choice,
		PeriodBucket: periodBucket,
	}
	if err := s.shadowReceiptAckStore.RecordVote(vote); err != nil {
		log.Printf("Failed to record shadow receipt vote: %v", err)
	}

	// Emit Phase 27 event
	s.eventEmitter.Emit(events.Event{
		Type:      events.Phase27ShadowReceiptVoted,
		Timestamp: s.clk.Now(),
		Metadata: map[string]string{
			"receipt_hash":  receiptHash,
			"vote_choice":   string(choice),
			"period_bucket": periodBucket,
		},
	})

	// Redirect back to today page
	http.Redirect(w, r, "/today", http.StatusFound)
}

// handleQuietInboxMirror serves the Phase 22 Quiet Inbox Mirror page.
//
// CRITICAL INVARIANTS:
//   - Whisper-level typography
//   - No buttons, no actions, no lists, no counts
//   - Single calm statement
//   - Optional abstract category chips (max 3)
//
// Reference: docs/ADR/ADR-0052-phase22-quiet-inbox-mirror.md
func (s *Server) handleQuietInboxMirror(w http.ResponseWriter, r *http.Request) {
	now := s.clk.Now()
	period := now.Format("2006-01-02")
	circleID := identity.EntityID("default")
	circleIDStr := string(circleID)

	// Check Gmail connection
	hasConnection, _ := s.gmailHandler.HasConnection(r.Context(), circleIDStr)

	// Get latest sync receipt
	var persistReceipt *persist.SyncReceipt
	if hasConnection {
		persistReceipt = s.syncReceiptStore.GetLatestByCircle(circleID)
	}

	// Build category presence from obligations (abstract only)
	categoryPresence := make(map[quietmirror.MirrorCategory]bool)
	if persistReceipt != nil && persistReceipt.Success {
		// Use abstract category detection from receipt magnitude
		if persistReceipt.MagnitudeBucket != persist.MagnitudeNone {
			// Default to work category for email activity
			categoryPresence[quietmirror.CategoryWork] = true
		}
	}

	// Convert persist.SyncReceipt to internalquietmirror.SyncReceiptAbstract
	var receipt *internalquietmirror.SyncReceiptAbstract
	if persistReceipt != nil {
		receipt = &internalquietmirror.SyncReceiptAbstract{
			Success:   persistReceipt.Success,
			Hash:      persistReceipt.Hash,
			Magnitude: mapPersistMagnitudeToMirror(persistReceipt.MagnitudeBucket),
		}
	}

	// Compute the mirror input
	input := s.quietMirrorEngine.ComputeInput(circleID, hasConnection, receipt, categoryPresence)

	// Compute the summary
	summary := s.quietMirrorEngine.Compute(input)

	// Store the summary
	if err := s.quietMirrorStore.Store(summary); err != nil {
		log.Printf("Failed to store quiet mirror summary: %v", err)
	}

	// Build the page
	page := s.quietMirrorEngine.BuildPage(summary)

	// Emit event
	eventType := events.Phase22QuietMirrorViewed
	if !summary.HasMirror {
		eventType = events.Phase22QuietMirrorAbsent
	}
	s.eventEmitter.Emit(events.Event{
		Type:      eventType,
		Timestamp: now,
		CircleID:  string(circleID),
		Metadata: map[string]string{
			"period":       period,
			"magnitude":    string(summary.Magnitude),
			"has_mirror":   fmt.Sprintf("%t", summary.HasMirror),
			"summary_hash": summary.Hash(),
		},
	})

	// Render inline template (whisper-level UI)
	const quietMirrorTemplate = `<!DOCTYPE html>
<html>
<head>
    <title>{{.Title}}</title>
    <style>
        * { margin: 0; padding: 0; box-sizing: border-box; }
        body {
            font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, sans-serif;
            background: #fafafa;
            color: #333;
            min-height: 100vh;
            display: flex;
            flex-direction: column;
            align-items: center;
            justify-content: center;
            padding: 2rem;
        }
        .container {
            max-width: 420px;
            text-align: center;
        }
        .title {
            font-size: 1.5rem;
            font-weight: 300;
            color: #666;
            margin-bottom: 2rem;
            letter-spacing: 0.02em;
        }
        .statement {
            font-size: 1.1rem;
            color: #444;
            line-height: 1.6;
            margin-bottom: 2rem;
            font-weight: 400;
        }
        .categories {
            display: flex;
            gap: 0.5rem;
            justify-content: center;
            flex-wrap: wrap;
            margin-bottom: 2rem;
        }
        .category-chip {
            background: #f0f0f0;
            color: #666;
            padding: 0.25rem 0.75rem;
            border-radius: 1rem;
            font-size: 0.8rem;
            font-weight: 400;
        }
        .footer {
            font-size: 0.85rem;
            color: #999;
            font-style: italic;
        }
        .back-link {
            margin-top: 3rem;
        }
        .back-link a {
            color: #999;
            text-decoration: none;
            font-size: 0.85rem;
        }
        .back-link a:hover {
            color: #666;
        }
    </style>
</head>
<body>
    <div class="container">
        <h1 class="title">{{.Title}}</h1>
        <p class="statement">{{.Statement}}</p>
        {{if .Categories}}
        <div class="categories">
            {{range .Categories}}
            <span class="category-chip">{{.}}</span>
            {{end}}
        </div>
        {{end}}
        <p class="footer">{{.Footer}}</p>
        <div class="back-link">
            <a href="/today">&larr; Back</a>
        </div>
    </div>
</body>
</html>`

	tmpl, err := template.New("quietmirror").Parse(quietMirrorTemplate)
	if err != nil {
		http.Error(w, "Template error", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := tmpl.Execute(w, page); err != nil {
		log.Printf("Failed to render quiet mirror page: %v", err)
	}
}

// handleQuietMirrorDismiss handles dismissal of the whisper cue.
//
// CRITICAL: POST-only, explicit action required.
func (s *Server) handleQuietMirrorDismiss(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	summaryHash := r.FormValue("summary_hash")
	if summaryHash == "" {
		http.Redirect(w, r, "/today", http.StatusFound)
		return
	}

	// Record dismissal
	now := s.clk.Now()
	period := now.Format("2006-01-02")
	circleID := identity.EntityID("default")

	s.quietMirrorDismissals.RecordDismissal(circleID, period, summaryHash)

	// Emit event
	s.eventEmitter.Emit(events.Event{
		Type:      events.Phase22WhisperCueDismissed,
		Timestamp: now,
		CircleID:  string(circleID),
		Metadata: map[string]string{
			"summary_hash": summaryHash,
			"period":       period,
		},
	})

	// Redirect back to today page
	http.Redirect(w, r, "/today", http.StatusFound)
}

// =============================================================================
// Phase 23: Gentle Action Invitation
// Reference: docs/ADR/ADR-0053-phase23-gentle-invitation.md
//
// CRITICAL INVARIANTS:
//   - Max one invitation per period
//   - Not shown unless trust baseline exists
//   - Never auto-execute
//   - Never create urgency
//   - Whisper styling only
// =============================================================================

// handleInvitation serves the Phase 23 Gentle Action Invitation page.
//
// CRITICAL: This page only shows if eligible.
// Eligibility requires:
//   - Gmail connected
//   - At least one real sync
//   - Quiet mirror viewed
//   - Trust baseline exists
//   - Not dismissed this period
func (s *Server) handleInvitation(w http.ResponseWriter, r *http.Request) {
	now := s.clk.Now()
	period := s.invitationEngine.CurrentPeriod()
	circleID := identity.EntityID("default")
	circleIDStr := string(circleID)

	// Check Gmail connection
	hasGmailConnection, _ := s.gmailHandler.HasConnection(r.Context(), circleIDStr)

	// Check sync receipt
	hasSyncReceipt := false
	if hasGmailConnection {
		receipt := s.syncReceiptStore.GetLatestByCircle(circleID)
		hasSyncReceipt = receipt != nil && receipt.Success
	}

	// Build trust inputs
	trustInputs := &internalinvitation.TrustInputs{
		HasQuietMirrorSummary: s.quietMirrorStore.HasSummaryForPeriod(circleID, now.Format("2006-01-02")),
		HasHeldSummary:        false, // TODO: Check held store
		HeldMagnitude:         "",
		HasTrustAccrual:       true,  // Assume trust if connected
		TrustScore:            0.5,   // Default trust score
		HasShadowReceipt:      false, // TODO: Check shadow receipts
	}

	// Check if dismissed or accepted this period
	dismissedThisPeriod := s.invitationStore.IsDismissedForPeriod(circleID, period.PeriodHash)
	acceptedThisPeriod := s.invitationStore.IsAcceptedForPeriod(circleID, period.PeriodHash)

	// Compute eligibility
	eligibility := s.invitationEngine.ComputeEligibility(
		circleIDStr,
		hasGmailConnection,
		hasSyncReceipt,
		trustInputs,
		dismissedThisPeriod,
		acceptedThisPeriod,
	)

	// Compute invitation
	summary := s.invitationEngine.Compute(eligibility)

	// Emit event
	if summary != nil {
		s.eventEmitter.Emit(events.Event{
			Type:      events.Phase23InvitationEligible,
			Timestamp: now,
			CircleID:  circleIDStr,
			Metadata: map[string]string{
				"period_hash":     period.PeriodHash,
				"kind":            string(summary.Kind),
				"invitation_hash": summary.Hash(),
			},
		})
	} else {
		s.eventEmitter.Emit(events.Event{
			Type:      events.Phase23InvitationSkipped,
			Timestamp: now,
			CircleID:  circleIDStr,
			Metadata: map[string]string{
				"period_hash": period.PeriodHash,
				"reason":      "not_eligible",
			},
		})
	}

	// Build page
	page := s.invitationEngine.BuildPage(summary)

	// Emit render event
	s.eventEmitter.Emit(events.Event{
		Type:      events.Phase23InvitationRendered,
		Timestamp: now,
		CircleID:  circleIDStr,
		Metadata: map[string]string{
			"period_hash":    period.PeriodHash,
			"has_invitation": fmt.Sprintf("%t", page.HasInvitation),
		},
	})

	// Render inline template (whisper-level UI)
	const invitationTemplate = `<!DOCTYPE html>
<html>
<head>
    <meta charset="utf-8">
    <meta name="viewport" content="width=device-width, initial-scale=1">
    <title>{{.Title}}</title>
    <style>
        * { margin: 0; padding: 0; box-sizing: border-box; }
        body {
            font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, sans-serif;
            background: #fafafa;
            color: #333;
            min-height: 100vh;
            display: flex;
            align-items: center;
            justify-content: center;
        }
        .container {
            max-width: 400px;
            padding: 3rem 2rem;
            text-align: center;
        }
        .title {
            font-size: 1.25rem;
            font-weight: 400;
            color: #666;
            margin-bottom: 2rem;
        }
        .statement {
            font-size: 1rem;
            color: #444;
            line-height: 1.6;
            margin-bottom: 2rem;
        }
        .actions {
            display: flex;
            flex-direction: column;
            gap: 0.75rem;
            margin-bottom: 2rem;
        }
        .btn {
            display: inline-block;
            padding: 0.75rem 1.5rem;
            border: 1px solid #ddd;
            border-radius: 4px;
            background: white;
            color: #666;
            font-size: 0.875rem;
            text-decoration: none;
            cursor: pointer;
            transition: background 0.2s;
        }
        .btn:hover {
            background: #f5f5f5;
        }
        .footer {
            font-size: 0.75rem;
            color: #999;
        }
        .back {
            display: block;
            margin-top: 2rem;
            font-size: 0.75rem;
            color: #999;
            text-decoration: none;
        }
        .back:hover {
            color: #666;
        }
    </style>
</head>
<body>
    <div class="container">
        <h1 class="title">{{.Title}}</h1>
        <p class="statement">{{.Statement}}</p>
        {{if .HasInvitation}}
        <div class="actions">
            <form action="/invite/accept" method="post" style="display:inline;">
                <input type="hidden" name="kind" value="{{.Kind}}">
                <input type="hidden" name="invitation_hash" value="{{.InvitationHash}}">
                <button type="submit" class="btn">Yes, that sounds good</button>
            </form>
            <form action="/invite/dismiss" method="post" style="display:inline;">
                <input type="hidden" name="invitation_hash" value="{{.InvitationHash}}">
                <button type="submit" class="btn">Not right now</button>
            </form>
        </div>
        {{end}}
        <p class="footer">{{.Footer}}</p>
        <a href="/today" class="back">← back to today</a>
    </div>
</body>
</html>`

	data := struct {
		Title          string
		Statement      string
		Kind           string
		InvitationHash string
		HasInvitation  bool
		Footer         string
	}{
		Title:         page.Title,
		Statement:     page.Statement,
		Kind:          string(page.Kind),
		HasInvitation: page.HasInvitation,
		Footer:        page.Footer,
	}
	if summary != nil {
		data.InvitationHash = summary.Hash()
	}

	tmpl, err := template.New("invitation").Parse(invitationTemplate)
	if err != nil {
		http.Error(w, "Template error", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := tmpl.Execute(w, data); err != nil {
		log.Printf("Template execution error: %v", err)
	}
}

// handleInvitationAccept handles accepting an invitation.
//
// CRITICAL: Accept does NOT trigger execution.
// It records the decision and redirects quietly.
func (s *Server) handleInvitationAccept(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	invitationHash := r.FormValue("invitation_hash")
	if invitationHash == "" {
		http.Redirect(w, r, "/today", http.StatusFound)
		return
	}

	now := s.clk.Now()
	period := s.invitationEngine.CurrentPeriod()
	circleID := identity.EntityID("default")

	// Record decision
	err := s.invitationStore.RecordDecision(
		circleID,
		invitationHash,
		domaininvitation.DecisionAccepted,
		period.PeriodHash,
	)
	if err != nil {
		log.Printf("Failed to record invitation decision: %v", err)
	}

	// Emit events
	s.eventEmitter.Emit(events.Event{
		Type:      events.Phase23InvitationAccepted,
		Timestamp: now,
		CircleID:  string(circleID),
		Metadata: map[string]string{
			"invitation_hash": invitationHash,
			"period_hash":     period.PeriodHash,
		},
	})

	s.eventEmitter.Emit(events.Event{
		Type:      events.Phase23InvitationPersisted,
		Timestamp: now,
		CircleID:  string(circleID),
		Metadata: map[string]string{
			"invitation_hash": invitationHash,
			"decision":        "accepted",
		},
	})

	// Redirect quietly back to today
	http.Redirect(w, r, "/today", http.StatusFound)
}

// handleInvitationDismiss handles dismissing an invitation.
//
// CRITICAL: Dismiss suppresses for the current period only.
func (s *Server) handleInvitationDismiss(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	invitationHash := r.FormValue("invitation_hash")
	if invitationHash == "" {
		http.Redirect(w, r, "/today", http.StatusFound)
		return
	}

	now := s.clk.Now()
	period := s.invitationEngine.CurrentPeriod()
	circleID := identity.EntityID("default")

	// Record decision
	err := s.invitationStore.RecordDecision(
		circleID,
		invitationHash,
		domaininvitation.DecisionDismissed,
		period.PeriodHash,
	)
	if err != nil {
		log.Printf("Failed to record invitation dismissal: %v", err)
	}

	// Emit events
	s.eventEmitter.Emit(events.Event{
		Type:      events.Phase23InvitationDismissed,
		Timestamp: now,
		CircleID:  string(circleID),
		Metadata: map[string]string{
			"invitation_hash": invitationHash,
			"period_hash":     period.PeriodHash,
		},
	})

	s.eventEmitter.Emit(events.Event{
		Type:      events.Phase23InvitationPersisted,
		Timestamp: now,
		CircleID:  string(circleID),
		Metadata: map[string]string{
			"invitation_hash": invitationHash,
			"decision":        "dismissed",
		},
	})

	// Redirect quietly back to today
	http.Redirect(w, r, "/today", http.StatusFound)
}

// handleFirstAction serves the first action page.
// Phase 24: First Reversible Real Action (Trust-Preserving).
// CRITICAL: Preview only, never execution. One per period.
func (s *Server) handleFirstAction(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	now := s.clk.Now()
	circleID := identity.EntityID("default")

	// Gather abstract inputs for eligibility check
	hasGmailConnection := false
	if s.gmailHandler != nil {
		hasConn, err := s.gmailHandler.HasConnection(r.Context(), string(circleID))
		if err == nil && hasConn {
			hasGmailConnection = true
		}
	}

	// Check trust inputs - derive from available stores
	trustInputs := &internalfirstaction.TrustInputs{
		HasQuietBaseline: s.syncReceiptStore != nil && s.syncReceiptStore.Count() > 0,
		HasMirrorViewed:  s.mirrorAckStore != nil && s.mirrorAckStore.Len() > 0,
		HasTrustAccrual:  false,
		TrustScore:       0.0,
	}

	// Check for trust accrual from trust store
	if s.trustStore != nil {
		summary := s.trustStore.GetRecentMeaningfulSummary()
		if summary != nil {
			trustInputs.HasTrustAccrual = true
			trustInputs.TrustScore = 0.5 // Abstract score, not raw value
		}
	}

	// Check for prior action this period
	period := s.firstActionEngine.CurrentPeriod()
	hasPriorAction := s.firstActionStore != nil && s.firstActionStore.HasActionThisPeriod(circleID, period.PeriodHash)

	// Check for held items (abstract check only)
	hasHeldItems := false
	if s.heldStore != nil {
		hasHeldItems = s.heldStore.Count() > 0
	}

	// Compute eligibility
	eligibility := s.firstActionEngine.ComputeEligibility(
		string(circleID),
		hasGmailConnection,
		trustInputs,
		hasPriorAction,
		hasHeldItems,
	)

	// Emit event
	s.eventEmitter.Emit(events.Event{
		Type:      events.Phase24InvitationOffered,
		Timestamp: now,
		CircleID:  string(circleID),
		Metadata: map[string]string{
			"eligible":    fmt.Sprintf("%t", eligibility.IsEligible()),
			"period_hash": period.PeriodHash,
		},
	})

	// Build action page
	category := domainfirstaction.CategoryWork // Default category
	page := s.firstActionEngine.BuildActionPage(eligibility, category)

	data := templateData{
		Title:             "Once, together",
		CurrentTime:       now.Format("2006-01-02 15:04"),
		FirstActionPage:   page,
		FirstActionPeriod: period.PeriodHash,
	}

	s.render(w, "first-action", data)
}

// handleFirstActionRun executes the preview (not the action).
// Phase 24: Preview only, never execution.
// CRITICAL: This shows a preview, it does NOT execute anything.
func (s *Server) handleFirstActionRun(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	now := s.clk.Now()
	circleID := identity.EntityID("default")
	period := s.firstActionEngine.CurrentPeriod()

	// Check if already acted this period
	if s.firstActionStore != nil && s.firstActionStore.HasActionThisPeriod(circleID, period.PeriodHash) {
		// Already acted, redirect back
		http.Redirect(w, r, "/action/once", http.StatusFound)
		return
	}

	// Gather abstract held items from held store records
	var heldItems []internalfirstaction.HeldItemAbstract
	if s.heldStore != nil && s.heldStore.Count() > 0 {
		records := s.heldStore.Records()
		for _, rec := range records {
			heldItems = append(heldItems, internalfirstaction.HeldItemAbstract{
				Hash:      rec.Hash,
				Category:  domainfirstaction.CategoryWork,
				Horizon:   domainfirstaction.HorizonLater,
				Magnitude: domainfirstaction.MagnitudeSmall,
			})
		}
	}

	// Select one held item deterministically
	selectedItem := s.firstActionEngine.SelectHeldItem(heldItems)
	if selectedItem == nil {
		// Nothing to show
		http.Redirect(w, r, "/action/once", http.StatusFound)
		return
	}

	// Build preview
	preview := s.firstActionEngine.BuildPreview(string(circleID), selectedItem)

	// Record the view
	if s.firstActionStore != nil {
		_ = s.firstActionStore.RecordState(
			circleID,
			preview.Hash(),
			domainfirstaction.StateViewed,
			period.PeriodHash,
		)
	}

	// Emit events
	s.eventEmitter.Emit(events.Event{
		Type:      events.Phase24ActionViewed,
		Timestamp: now,
		CircleID:  string(circleID),
		Metadata: map[string]string{
			"preview_hash": preview.Hash(),
			"period_hash":  period.PeriodHash,
		},
	})

	s.eventEmitter.Emit(events.Event{
		Type:      events.Phase24PreviewRendered,
		Timestamp: now,
		CircleID:  string(circleID),
		Metadata: map[string]string{
			"preview_hash": preview.Hash(),
			"category":     string(preview.Category),
			"horizon":      string(preview.Horizon),
		},
	})

	// Build preview page
	previewPage := s.firstActionEngine.BuildPreviewPage(preview)

	data := templateData{
		Title:              "Preview",
		CurrentTime:        now.Format("2006-01-02 15:04"),
		FirstActionPreview: previewPage,
		FirstActionPeriod:  period.PeriodHash,
	}

	s.render(w, "first-action-preview", data)
}

// handleFirstActionDismiss handles dismissing the first action.
// Phase 24: Circle dismisses, silence resumes.
func (s *Server) handleFirstActionDismiss(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	now := s.clk.Now()
	circleID := identity.EntityID("default")
	period := s.firstActionEngine.CurrentPeriod()

	// Get the preview hash from form
	previewHash := r.FormValue("preview_hash")

	// Record the dismissal
	if s.firstActionStore != nil && previewHash != "" {
		_ = s.firstActionStore.RecordState(
			circleID,
			previewHash,
			domainfirstaction.StateDismissed,
			period.PeriodHash,
		)
	}

	// Emit events
	s.eventEmitter.Emit(events.Event{
		Type:      events.Phase24ActionDismissed,
		Timestamp: now,
		CircleID:  string(circleID),
		Metadata: map[string]string{
			"preview_hash": previewHash,
			"period_hash":  period.PeriodHash,
		},
	})

	s.eventEmitter.Emit(events.Event{
		Type:      events.Phase24PeriodClosed,
		Timestamp: now,
		CircleID:  string(circleID),
		Metadata: map[string]string{
			"period_hash": period.PeriodHash,
			"reason":      "dismissed",
		},
	})

	// Redirect back to today - silence resumes
	http.Redirect(w, r, "/today", http.StatusFound)
}

// handleUndoable serves the undoable execution page.
// Phase 25: First Undoable Execution (Opt-In, Single-Shot).
// CRITICAL: Only calendar respond is undoable.
func (s *Server) handleUndoable(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	now := s.clk.Now()
	circleID := identity.EntityID("default")

	// Check eligibility
	var eligibility *domainundoableexec.ActionEligibility
	if s.undoableExecEngine != nil {
		eligibility = s.undoableExecEngine.EligibleAction(r.Context(), circleID)
	} else {
		eligibility = &domainundoableexec.ActionEligibility{
			Eligible: false,
			Reason:   "engine not initialized",
			CircleID: string(circleID),
		}
	}

	// Emit event
	s.eventEmitter.Emit(events.Event{
		Type:      events.Phase25UndoableViewed,
		Timestamp: now,
		CircleID:  string(circleID),
	})

	s.eventEmitter.Emit(events.Event{
		Type:      events.Phase25EligibleComputed,
		Timestamp: now,
		CircleID:  string(circleID),
		Metadata: map[string]string{
			"eligible": fmt.Sprintf("%t", eligibility.Eligible),
			"reason":   eligibility.Reason,
		},
	})

	// Build page
	page := domainundoableexec.NewUndoablePage(eligibility.Eligible)

	data := templateData{
		Title:           "Once, quietly",
		CurrentTime:     now.Format("2006-01-02 15:04"),
		UndoablePage:    page,
		UndoEligibility: eligibility,
	}

	s.render(w, "undoable", data)
}

// handleUndoableRun executes the undoable action.
// Phase 25: Single-shot execution via calendar boundary.
// CRITICAL: This performs a real external write.
func (s *Server) handleUndoableRun(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	now := s.clk.Now()
	circleID := identity.EntityID("default")

	// Emit request event
	s.eventEmitter.Emit(events.Event{
		Type:      events.Phase25RunRequested,
		Timestamp: now,
		CircleID:  string(circleID),
	})

	// Check engine availability
	if s.undoableExecEngine == nil {
		http.Redirect(w, r, "/action/undoable", http.StatusFound)
		return
	}

	// Check eligibility
	eligibility := s.undoableExecEngine.EligibleAction(r.Context(), circleID)
	if !eligibility.Eligible {
		http.Redirect(w, r, "/action/undoable", http.StatusFound)
		return
	}

	// Execute
	result := s.undoableExecEngine.RunOnce(r.Context(), circleID, eligibility.DraftID)
	if !result.Success {
		// Failed - redirect back
		http.Redirect(w, r, "/action/undoable", http.StatusFound)
		return
	}

	// Emit success events
	s.eventEmitter.Emit(events.Event{
		Type:      events.Phase25RunExecuted,
		Timestamp: now,
		CircleID:  string(circleID),
		Metadata: map[string]string{
			"record_id": result.UndoRecord.ID,
		},
	})

	s.eventEmitter.Emit(events.Event{
		Type:      events.Phase25RecordPersisted,
		Timestamp: now,
		CircleID:  string(circleID),
		Metadata: map[string]string{
			"record_id":   result.UndoRecord.ID,
			"period_key":  result.UndoRecord.PeriodKey,
			"action_kind": string(result.UndoRecord.ActionKind),
		},
	})

	// Redirect to done page
	http.Redirect(w, r, "/action/undoable/done?id="+result.UndoRecord.ID, http.StatusFound)
}

// handleUndoableDone shows the done confirmation page.
// Phase 25: Shows undo availability.
func (s *Server) handleUndoableDone(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	now := s.clk.Now()
	_ = identity.EntityID("default") // Reserved for future circle-scoped undo
	recordID := r.URL.Query().Get("id")

	// Check if undo is still available
	undoAvailable := false
	if s.undoableExecEngine != nil && recordID != "" {
		record, found := s.undoableExecEngine.GetUndoRecord(recordID)
		if found && record.IsUndoAvailable(now) {
			undoAvailable = true
		}
	}

	// Build page
	page := domainundoableexec.NewDonePage(undoAvailable)

	data := templateData{
		Title:        "Done",
		CurrentTime:  now.Format("2006-01-02 15:04"),
		UndoDonePage: page,
		UndoRecordID: recordID,
	}

	s.render(w, "undoable-done", data)
}

// handleUndoableUndo shows the undo confirmation page.
// Phase 25: Allows reversal within undo window.
func (s *Server) handleUndoableUndo(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	now := s.clk.Now()
	circleID := identity.EntityID("default")
	recordID := r.URL.Query().Get("id")

	// Emit viewed event
	s.eventEmitter.Emit(events.Event{
		Type:      events.Phase25UndoViewed,
		Timestamp: now,
		CircleID:  string(circleID),
		Metadata: map[string]string{
			"record_id": recordID,
		},
	})

	// Check if undo is still available
	canUndo := false
	if s.undoableExecEngine != nil && recordID != "" {
		record, found := s.undoableExecEngine.GetUndoRecord(recordID)
		if found && record.IsUndoAvailable(now) {
			canUndo = true
		}
	}

	// Build page
	page := domainundoableexec.NewUndoPage(canUndo)

	data := templateData{
		Title:        "Undo",
		CurrentTime:  now.Format("2006-01-02 15:04"),
		UndoPage:     page,
		UndoRecordID: recordID,
	}

	s.render(w, "undoable-undo", data)
}

// handleUndoableUndoRun executes the undo.
// Phase 25: Reverses the previous action via calendar boundary.
func (s *Server) handleUndoableUndoRun(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	now := s.clk.Now()
	circleID := identity.EntityID("default")
	recordID := r.FormValue("record_id")

	// Emit request event
	s.eventEmitter.Emit(events.Event{
		Type:      events.Phase25UndoRequested,
		Timestamp: now,
		CircleID:  string(circleID),
		Metadata: map[string]string{
			"record_id": recordID,
		},
	})

	// Check engine availability
	if s.undoableExecEngine == nil || recordID == "" {
		http.Redirect(w, r, "/today", http.StatusFound)
		return
	}

	// Execute undo
	result := s.undoableExecEngine.Undo(r.Context(), recordID)
	if !result.Success {
		// Failed - redirect to undo page
		http.Redirect(w, r, "/action/undoable/undo?id="+recordID, http.StatusFound)
		return
	}

	// Emit success events
	s.eventEmitter.Emit(events.Event{
		Type:      events.Phase25UndoExecuted,
		Timestamp: now,
		CircleID:  string(circleID),
		Metadata: map[string]string{
			"record_id": recordID,
		},
	})

	s.eventEmitter.Emit(events.Event{
		Type:      events.Phase25AckPersisted,
		Timestamp: now,
		CircleID:  string(circleID),
		Metadata: map[string]string{
			"record_id": recordID,
			"new_state": string(result.Ack.NewState),
		},
	})

	// Redirect to today - silence resumes
	http.Redirect(w, r, "/today", http.StatusFound)
}

// handleUndoableDismiss dismisses the undoable action offer.
// Phase 25: Circle declines, silence resumes.
func (s *Server) handleUndoableDismiss(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	now := s.clk.Now()
	circleID := identity.EntityID("default")

	// Emit dismissed event
	s.eventEmitter.Emit(events.Event{
		Type:      events.Phase25Dismissed,
		Timestamp: now,
		CircleID:  string(circleID),
	})

	// Redirect to today - silence resumes
	http.Redirect(w, r, "/today", http.StatusFound)
}

// handleJourney serves the guided journey page.
// Phase 26A: Single calm card showing next step.
func (s *Server) handleJourney(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	now := s.clk.Now()
	circleID := identity.EntityID("default")

	// Build journey inputs by gathering state from various stores
	inputs := s.buildJourneyInputs(circleID, now)

	// Build the journey page
	page := s.journeyEngine.BuildPage(inputs)

	// Emit events
	s.eventEmitter.Emit(events.Event{
		Type:      events.Phase26AJourneyRequested,
		Timestamp: now,
		CircleID:  string(circleID),
	})

	s.eventEmitter.Emit(events.Event{
		Type:      events.Phase26AJourneyComputed,
		Timestamp: now,
		CircleID:  string(circleID),
		Metadata: map[string]string{
			"step":        string(page.CurrentStep),
			"status_hash": page.StatusHash,
			"is_done":     fmt.Sprintf("%t", page.IsDone),
		},
	})

	data := templateData{
		Title:       "Journey",
		CurrentTime: now.Format("2006-01-02 15:04"),
		JourneyPage: page,
		CircleID:    string(circleID),
	}

	s.render(w, "journey", data)
}

// handleJourneyNext redirects to the primary action of the current step.
// Phase 26A: Deterministic redirect based on computed step.
func (s *Server) handleJourneyNext(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	now := s.clk.Now()
	circleID := identity.EntityID("default")

	// Build journey inputs and page
	inputs := s.buildJourneyInputs(circleID, now)
	page := s.journeyEngine.BuildPage(inputs)

	// Emit event
	s.eventEmitter.Emit(events.Event{
		Type:      events.Phase26AJourneyNextRedirected,
		Timestamp: now,
		CircleID:  string(circleID),
		Metadata: map[string]string{
			"step": string(page.CurrentStep),
		},
	})

	// Redirect to primary action path
	http.Redirect(w, r, page.PrimaryAction.Path, http.StatusFound)
}

// handleJourneyDismiss dismisses the journey for the current period.
// Phase 26A: Store hash-only dismissal, redirect to /today.
func (s *Server) handleJourneyDismiss(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	now := s.clk.Now()
	circleID := identity.EntityID("default")

	// Get status_hash from form
	if err := r.ParseForm(); err != nil {
		http.Error(w, "Invalid form data", http.StatusBadRequest)
		return
	}
	statusHash := r.FormValue("status_hash")

	// Verify status hash matches current state
	inputs := s.buildJourneyInputs(circleID, now)
	currentHash := inputs.ComputeStatusHash()

	if statusHash != "" && statusHash != currentHash {
		// Status changed since page was rendered - redirect back to journey
		http.Redirect(w, r, "/journey", http.StatusFound)
		return
	}

	// Record dismissal
	periodKey := inputs.PeriodKey()
	_, _ = s.journeyDismissalStore.RecordDismissal(circleID, periodKey, currentHash)

	// Emit event
	s.eventEmitter.Emit(events.Event{
		Type:      events.Phase26AJourneyDismissed,
		Timestamp: now,
		CircleID:  string(circleID),
		Metadata: map[string]string{
			"status_hash": currentHash,
			"period_key":  periodKey,
		},
	})

	// Redirect to today - silence resumes
	http.Redirect(w, r, "/today", http.StatusFound)
}

// buildJourneyInputs gathers state from various stores to build journey inputs.
func (s *Server) buildJourneyInputs(circleID identity.EntityID, now time.Time) *journey.JourneyInputs {
	inputs := &journey.JourneyInputs{
		CircleID: string(circleID),
		Now:      now,
	}

	// Check Gmail connection status
	if s.connectionStore != nil {
		stateSet := s.connectionStore.State()
		if stateSet != nil {
			emailState := stateSet.Get(connection.KindEmail)
			if emailState != nil && emailState.Status != connection.StatusNotConnected {
				inputs.HasGmail = true
				if emailState.Status == connection.StatusConnectedMock {
					inputs.GmailMode = "mock"
				} else {
					inputs.GmailMode = "real"
				}
			}
		}
	}

	// Check sync receipt
	if s.syncReceiptStore != nil {
		latestReceipt := s.syncReceiptStore.GetLatestByCircle(circleID)
		if latestReceipt != nil {
			inputs.HasSyncReceipt = true
			inputs.LastSyncMagnitude = latestReceipt.MagnitudeBucket
		}
	}

	// Check mirror viewed
	if s.quietMirrorStore != nil {
		periodKey := now.UTC().Format("2006-01-02")
		if s.quietMirrorStore.HasSummaryForPeriod(circleID, periodKey) {
			inputs.MirrorViewed = true
		}
	}

	// Check action eligibility (Phase 25)
	if s.undoableExecEngine != nil {
		eligibility := s.undoableExecEngine.EligibleAction(context.Background(), circleID)
		if eligibility != nil && eligibility.Eligible {
			inputs.ActionEligible = true
		}
	}

	// Check if action was used this period
	if s.undoableExecEngine != nil {
		inputs.ActionUsedThisPeriod = s.undoableExecEngine.HasExecutedThisPeriod(circleID)
	}

	// Check for dismissal
	if s.journeyDismissalStore != nil {
		periodKey := inputs.PeriodKey()
		inputs.DismissedStatusHash = s.journeyDismissalStore.GetDismissedStatusHash(circleID, periodKey)
	}

	return inputs
}

// handleFirstMinutes serves the First Five Minutes receipt page.
// Phase 26B: A single calm receipt showing what happened during first minutes.
// This is NOT analytics. This is narrative proof.
func (s *Server) handleFirstMinutes(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	now := s.clk.Now()
	circleID := identity.EntityID("default")
	period := internalfirstminutes.PeriodFromTime(now)

	// Build inputs by gathering state from various stores
	inputs := s.buildFirstMinutesInputs(circleID, now)

	// Compute the summary
	summary := s.firstMinutesEngine.ComputeSummary(inputs)

	// If no meaningful signals, redirect to /today (silence is success)
	if summary == nil {
		http.Redirect(w, r, "/today", http.StatusFound)
		return
	}

	// Emit events
	s.eventEmitter.Emit(events.Event{
		Type:      events.Phase26BFirstMinutesViewed,
		Timestamp: now,
		CircleID:  string(circleID),
		Metadata: map[string]string{
			"period":       string(period),
			"status_hash":  summary.StatusHash,
			"signal_count": fmt.Sprintf("%d", len(summary.Signals)),
		},
	})

	s.eventEmitter.Emit(events.Event{
		Type:      events.Phase26BFirstMinutesComputed,
		Timestamp: now,
		CircleID:  string(circleID),
		Metadata: map[string]string{
			"period":      string(period),
			"status_hash": summary.StatusHash,
		},
	})

	data := templateData{
		Title:               "First Minutes",
		CurrentTime:         now.Format("2006-01-02 15:04"),
		FirstMinutesSummary: summary,
		CircleID:            string(circleID),
	}

	s.render(w, "first-minutes", data)
}

// handleFirstMinutesDismiss dismisses the First Minutes receipt for the current period.
// Phase 26B: Store hash-only dismissal, redirect to /today.
func (s *Server) handleFirstMinutesDismiss(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	now := s.clk.Now()
	circleID := identity.EntityID("default")
	period := internalfirstminutes.PeriodFromTime(now)

	// Get status_hash from form
	if err := r.ParseForm(); err != nil {
		http.Error(w, "Invalid form data", http.StatusBadRequest)
		return
	}
	statusHash := r.FormValue("status_hash")

	// Verify status hash matches current state
	inputs := s.buildFirstMinutesInputs(circleID, now)
	summary := s.firstMinutesEngine.ComputeSummary(inputs)

	if summary == nil {
		// No summary to dismiss
		http.Redirect(w, r, "/today", http.StatusFound)
		return
	}

	if statusHash != "" && statusHash != summary.StatusHash {
		// Status changed since page was rendered - redirect back to first-minutes
		http.Redirect(w, r, "/first-minutes", http.StatusFound)
		return
	}

	// Record dismissal
	_, _ = s.firstMinutesStore.RecordDismissal(circleID, period, summary.StatusHash)

	// Emit event
	s.eventEmitter.Emit(events.Event{
		Type:      events.Phase26BFirstMinutesDismissed,
		Timestamp: now,
		CircleID:  string(circleID),
		Metadata: map[string]string{
			"period":      string(period),
			"status_hash": summary.StatusHash,
		},
	})

	// Redirect to today - silence resumes
	http.Redirect(w, r, "/today", http.StatusFound)
}

// buildFirstMinutesInputs gathers state from various stores to build first minutes inputs.
// Phase 26B: Reads ONLY from existing stores - no new data collection.
func (s *Server) buildFirstMinutesInputs(circleID identity.EntityID, now time.Time) *domainfirstminutes.FirstMinutesInputs {
	period := internalfirstminutes.PeriodFromTime(now)

	inputs := &domainfirstminutes.FirstMinutesInputs{
		CircleID: string(circleID),
		Period:   period,
	}

	// Check connection status (from connection store)
	if s.connectionStore != nil {
		stateSet := s.connectionStore.State()
		if stateSet != nil {
			emailState := stateSet.Get(connection.KindEmail)
			if emailState != nil && emailState.Status != connection.StatusNotConnected {
				inputs.HasConnection = true
				if emailState.Status == connection.StatusConnectedMock {
					inputs.ConnectionMode = "mock"
				} else {
					inputs.ConnectionMode = "real"
				}
			}
		}
	}

	// Check sync receipt (from sync receipt store)
	if s.syncReceiptStore != nil {
		latestReceipt := s.syncReceiptStore.GetLatestByCircle(circleID)
		if latestReceipt != nil {
			inputs.HasSyncReceipt = true
			inputs.SyncMagnitude = mapMagnitude(latestReceipt.MagnitudeBucket)
		}
	}

	// Check mirror (from quiet mirror store)
	if s.quietMirrorStore != nil {
		periodKey := now.UTC().Format("2006-01-02")
		summary := s.quietMirrorStore.GetLatestForPeriod(circleID, periodKey)
		if summary != nil {
			inputs.HasMirror = true
			inputs.MirrorMagnitude = mapQuietMirrorMagnitude(summary.Magnitude)
		}
	}

	// Check held items (from trust store)
	if s.trustStore != nil {
		trustSummary := s.trustStore.GetRecentMeaningfulSummary()
		if trustSummary != nil {
			inputs.HasHeldItems = true
			inputs.HeldMagnitude = mapTrustMagnitude(trustSummary.MagnitudeBucket)
		}
	}

	// Check action previewed (from first action store)
	if s.firstActionStore != nil {
		periodHash := now.UTC().Format("2006-01-02")
		records := s.firstActionStore.GetForPeriod(circleID, periodHash)
		if len(records) > 0 {
			inputs.ActionPreviewed = true
		}
	}

	// Check action executed (from undo store)
	if s.undoableExecStore != nil {
		periodKey := now.UTC().Format("2006-01-02")
		records := s.undoableExecStore.GetForPeriod(circleID, periodKey)
		for _, rec := range records {
			if rec.State == domainundoableexec.StateExecuted ||
				rec.State == domainundoableexec.StateUndoAvailable ||
				rec.State == domainundoableexec.StateUndone ||
				rec.State == domainundoableexec.StateExpired {
				inputs.ActionExecuted = true
				break
			}
		}
	}

	// Check for dismissal
	if s.firstMinutesStore != nil {
		inputs.DismissedSummaryHash = s.firstMinutesStore.GetDismissedSummaryHash(circleID, period)
	}

	return inputs
}

// mapMagnitude maps persist.MagnitudeBucket to domainfirstminutes.MagnitudeBucket.
func mapMagnitude(m persist.MagnitudeBucket) domainfirstminutes.MagnitudeBucket {
	switch m {
	case persist.MagnitudeNone:
		return domainfirstminutes.MagnitudeNothing
	case persist.MagnitudeHandful:
		return domainfirstminutes.MagnitudeAFew
	case persist.MagnitudeSeveral, persist.MagnitudeMany:
		return domainfirstminutes.MagnitudeSeveral
	default:
		return domainfirstminutes.MagnitudeNothing
	}
}

// mapQuietMirrorMagnitude maps quietmirror.MirrorMagnitude to domainfirstminutes.MagnitudeBucket.
func mapQuietMirrorMagnitude(m quietmirror.MirrorMagnitude) domainfirstminutes.MagnitudeBucket {
	switch m {
	case quietmirror.MagnitudeNothing:
		return domainfirstminutes.MagnitudeNothing
	case quietmirror.MagnitudeAFew:
		return domainfirstminutes.MagnitudeAFew
	case quietmirror.MagnitudeSeveral:
		return domainfirstminutes.MagnitudeSeveral
	default:
		return domainfirstminutes.MagnitudeNothing
	}
}

// mapTrustMagnitude maps domainshadow.MagnitudeBucket to domainfirstminutes.MagnitudeBucket.
func mapTrustMagnitude(m domainshadow.MagnitudeBucket) domainfirstminutes.MagnitudeBucket {
	switch m {
	case domainshadow.MagnitudeNothing:
		return domainfirstminutes.MagnitudeNothing
	case domainshadow.MagnitudeAFew:
		return domainfirstminutes.MagnitudeAFew
	case domainshadow.MagnitudeSeveral:
		return domainfirstminutes.MagnitudeSeveral
	default:
		return domainfirstminutes.MagnitudeNothing
	}
}

// =============================================================================
// Phase 26C: Connected Reality Check Handlers
// =============================================================================
// This is NOT analytics. This is a trust proof page.
// Proves "this is real" without showing content, identifiers, or secrets.
// Reference: docs/ADR/ADR-0057-phase26C-connected-reality-check.md
// =============================================================================

// handleReality serves the reality check page.
// Phase 26C: A single calm page proving connected status without showing content.
func (s *Server) handleReality(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	now := s.clk.Now()
	circleID := identity.EntityID("default")
	period := internalreality.PeriodKey(now)

	// Build inputs by gathering state from various stores
	inputs := s.buildRealityInputs(circleID, now)

	// Emit requested event
	s.eventEmitter.Emit(events.Event{
		Type:      events.Phase26CRealityRequested,
		Timestamp: now,
		CircleID:  string(circleID),
		Metadata: map[string]string{
			"period": period,
		},
	})

	// Build the page
	page := s.realityEngine.BuildPage(inputs)

	// Emit computed event
	s.eventEmitter.Emit(events.Event{
		Type:      events.Phase26CRealityComputed,
		Timestamp: now,
		CircleID:  string(circleID),
		Metadata: map[string]string{
			"period":          period,
			"status_hash":     page.StatusHash,
			"gmail_connected": boolToYesNoString(inputs.GmailConnected),
		},
	})

	// Emit viewed event
	s.eventEmitter.Emit(events.Event{
		Type:      events.Phase26CRealityViewed,
		Timestamp: now,
		CircleID:  string(circleID),
		Metadata: map[string]string{
			"period":      period,
			"status_hash": page.StatusHash,
		},
	})

	data := templateData{
		Title:       "Reality",
		CurrentTime: now.Format("2006-01-02 15:04"),
		RealityPage: page,
		CircleID:    string(circleID),
	}

	s.render(w, "reality", data)
}

// handleRealityAck acknowledges the reality page for the current period.
// Phase 26C: Store hash-only acknowledgement, redirect to /today.
func (s *Server) handleRealityAck(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	now := s.clk.Now()
	circleID := identity.EntityID("default")
	period := internalreality.PeriodKey(now)

	// Get status_hash from form
	if err := r.ParseForm(); err != nil {
		http.Error(w, "Invalid form data", http.StatusBadRequest)
		return
	}
	statusHash := r.FormValue("status_hash")

	// Verify status hash matches current state
	inputs := s.buildRealityInputs(circleID, now)
	page := s.realityEngine.BuildPage(inputs)

	if statusHash != "" && statusHash != page.StatusHash {
		// Status changed since page was rendered - redirect back to reality
		http.Redirect(w, r, "/reality", http.StatusFound)
		return
	}

	// Record acknowledgement
	if s.realityAckStore != nil {
		_ = s.realityAckStore.RecordAck(period, page.StatusHash)
	}

	// Emit event
	s.eventEmitter.Emit(events.Event{
		Type:      events.Phase26CRealityAckRecorded,
		Timestamp: now,
		CircleID:  string(circleID),
		Metadata: map[string]string{
			"period":      period,
			"status_hash": page.StatusHash,
		},
	})

	// Redirect to today - back to quiet
	http.Redirect(w, r, "/today", http.StatusFound)
}

// buildRealityInputs gathers state from various stores to build reality inputs.
// Phase 26C: Reads ONLY from existing stores - no new data collection.
// CRITICAL: Never exposes secrets, identifiers, or raw content.
func (s *Server) buildRealityInputs(circleID identity.EntityID, now time.Time) *domainreality.RealityInputs {
	period := internalreality.PeriodKey(now)

	inputs := &domainreality.RealityInputs{
		CircleID:  string(circleID),
		NowBucket: period,
	}

	// Check Gmail connection status (from connection store)
	if s.connectionStore != nil {
		stateSet := s.connectionStore.State()
		if stateSet != nil {
			emailState := stateSet.Get(connection.KindEmail)
			if emailState != nil && emailState.Status != connection.StatusNotConnected {
				inputs.GmailConnected = true
			}
		}
	}

	// Check sync status (from sync receipt store)
	if s.syncReceiptStore != nil {
		latestReceipt := s.syncReceiptStore.GetLatestByCircle(circleID)
		if latestReceipt != nil {
			inputs.SyncBucket = internalreality.ComputeSyncBucket(latestReceipt.TimeBucket, now)
			inputs.SyncMagnitude = internalreality.MapSyncReceiptMagnitude(string(latestReceipt.MagnitudeBucket))
		} else {
			inputs.SyncBucket = domainreality.SyncBucketNever
			inputs.SyncMagnitude = domainreality.MagnitudeNA
		}
	} else {
		inputs.SyncBucket = domainreality.SyncBucketNever
		inputs.SyncMagnitude = domainreality.MagnitudeNA
	}

	// Obligations held - always true per canon (quiet baseline)
	inputs.ObligationsHeld = true

	// Auto-surface - always false per canon
	inputs.AutoSurface = false

	// Shadow mode configuration (from config)
	if s.multiCircleConfig != nil {
		shadowCfg := s.multiCircleConfig.Shadow

		// Map provider kind
		inputs.ShadowProviderKind = internalreality.MapProviderKind(
			shadowCfg.ProviderKind,
			shadowCfg.Mode,
		)
		inputs.ShadowRealAllowed = shadowCfg.RealAllowed

		// Check chat/embed configuration
		azureCfg := shadowCfg.AzureOpenAI
		inputs.ChatConfigured = azureCfg.GetChatDeployment() != ""
		inputs.EmbedConfigured = azureCfg.HasEmbeddings()
		inputs.EndpointConfigured = azureCfg.Endpoint != ""

		// Region - only show if explicitly in config, never derive from URL
		// This is safe to show as it's non-identifying config
		// For now we don't parse region from config, omit it
		inputs.Region = ""
	} else {
		inputs.ShadowProviderKind = domainreality.ProviderOff
		inputs.ShadowRealAllowed = false
		inputs.ChatConfigured = false
		inputs.EmbedConfigured = false
		inputs.EndpointConfigured = false
	}

	// Shadow receipts magnitude (from shadow receipt store)
	if s.shadowReceiptStore != nil && inputs.ShadowProviderKind != domainreality.ProviderOff {
		receipts := s.shadowReceiptStore.ListForCircle(circleID)
		inputs.ShadowMagnitude = internalreality.MapShadowReceiptCount(len(receipts))
	} else {
		inputs.ShadowMagnitude = domainreality.MagnitudeNA
	}

	return inputs
}

// =============================================================================
// Phase 28: Trust Kept — First Real Act, Then Silence
// =============================================================================
//
// CRITICAL INVARIANTS:
//   - Only calendar_respond action allowed
//   - Single execution per period (day)
//   - 15-minute undo window (bucketed)
//   - After execution: silence forever
//   - No growth mechanics, engagement loops, or escalation paths
//
// Reference: docs/ADR/ADR-0059-phase28-trust-kept.md

// handleTrustAction shows the trust action preview page.
// Phase 28: Users can choose "Let it happen" or "Keep holding".
func (s *Server) handleTrustAction(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	now := s.clk.Now()
	circleID := identity.EntityID("default")

	// Check eligibility
	if s.trustActionEngine == nil {
		// Engine not initialized - redirect to today
		http.Redirect(w, r, "/today", http.StatusFound)
		return
	}

	eligibility := s.trustActionEngine.CheckEligibility(circleID)
	if !eligibility.Eligible {
		// Not eligible - redirect to today
		http.Redirect(w, r, "/today", http.StatusFound)
		return
	}

	// Emit preview viewed event
	s.eventEmitter.Emit(events.Event{
		Type:      events.Phase28TrustActionPreviewViewed,
		Timestamp: now,
		CircleID:  string(circleID),
		Metadata: map[string]string{
			"period":         eligibility.PeriodKey,
			"action_kind":    string(eligibility.Preview.ActionKind),
			"horizon_bucket": string(eligibility.Preview.HorizonBucket),
		},
	})

	// Render preview page
	data := templateData{
		Title:       "Trust Action",
		CurrentTime: now.Format("2006-01-02 15:04"),
		TrustActionPreview: &trustActionPreviewInfo{
			ActionKind:     string(eligibility.Preview.ActionKind),
			AbstractTarget: eligibility.Preview.AbstractTarget,
			HorizonBucket:  string(eligibility.Preview.HorizonBucket),
			Reversible:     eligibility.Preview.Reversible,
			DraftID:        eligibility.DraftID,
			PeriodKey:      eligibility.PeriodKey,
		},
	}

	s.render(w, "trust-action", data)
}

// handleTrustActionExecute executes the trust action.
// Phase 28: Single execution per period, via Phase 5 calendar boundary.
func (s *Server) handleTrustActionExecute(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	now := s.clk.Now()
	circleID := identity.EntityID("default")

	if err := r.ParseForm(); err != nil {
		http.Error(w, "Invalid form data", http.StatusBadRequest)
		return
	}
	draftID := r.FormValue("draft_id")

	if s.trustActionEngine == nil {
		http.Redirect(w, r, "/today", http.StatusFound)
		return
	}

	// Execute via engine (delegates to Phase 5 calendar boundary)
	result := s.trustActionEngine.Execute(r.Context(), circleID, draftID)
	if !result.Success {
		// Execution failed - redirect to today
		log.Printf("Trust action execution failed: %s", result.Error)
		http.Redirect(w, r, "/today", http.StatusFound)
		return
	}

	// Emit executed event
	s.eventEmitter.Emit(events.Event{
		Type:      events.Phase28TrustActionExecuted,
		Timestamp: now,
		CircleID:  string(circleID),
		Metadata: map[string]string{
			"period":       result.Receipt.Period,
			"receipt_hash": result.Receipt.StatusHash,
			"action_kind":  string(result.Receipt.ActionKind),
		},
	})

	// Redirect to receipt page
	http.Redirect(w, r, "/trust/action/receipt", http.StatusFound)
}

// handleTrustActionUndo undoes a previously executed trust action.
// Phase 28: Must be within 15-minute undo window.
func (s *Server) handleTrustActionUndo(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	now := s.clk.Now()
	circleID := identity.EntityID("default")

	if err := r.ParseForm(); err != nil {
		http.Error(w, "Invalid form data", http.StatusBadRequest)
		return
	}
	receiptID := r.FormValue("receipt_id")

	if s.trustActionEngine == nil {
		http.Redirect(w, r, "/today", http.StatusFound)
		return
	}

	// Undo via engine
	result := s.trustActionEngine.Undo(r.Context(), receiptID)
	if !result.Success {
		// Undo failed - redirect to receipt
		log.Printf("Trust action undo failed: %s", result.Error)
		http.Redirect(w, r, "/trust/action/receipt", http.StatusFound)
		return
	}

	// Emit undone event
	s.eventEmitter.Emit(events.Event{
		Type:      events.Phase28TrustActionUndone,
		Timestamp: now,
		CircleID:  string(circleID),
		Metadata: map[string]string{
			"period":       result.Receipt.Period,
			"receipt_hash": result.Receipt.StatusHash,
		},
	})

	// Redirect to receipt page
	http.Redirect(w, r, "/trust/action/receipt", http.StatusFound)
}

// handleTrustActionReceipt shows the trust action receipt page.
// Phase 28: Shows proof of action, undo option if available, then silence.
func (s *Server) handleTrustActionReceipt(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	now := s.clk.Now()
	circleID := identity.EntityID("default")

	// Get latest receipt
	var receiptInfo *trustActionReceiptInfo
	var undoAvailable bool

	if s.trustActionEngine != nil {
		latestReceipt := s.trustActionEngine.GetLatestReceipt(circleID)
		if latestReceipt != nil {
			// Undo available if in executed state and within window
			undoAvailable = latestReceipt.State == "executed" && !latestReceipt.UndoBucket.IsExpired(now)
			receiptInfo = &trustActionReceiptInfo{
				ReceiptID:     latestReceipt.ReceiptID,
				ActionKind:    string(latestReceipt.ActionKind),
				State:         string(latestReceipt.State),
				Period:        latestReceipt.Period,
				StatusHash:    latestReceipt.StatusHash,
				UndoAvailable: undoAvailable,
			}
		}
	}

	if receiptInfo == nil {
		// No receipt - redirect to today
		http.Redirect(w, r, "/today", http.StatusFound)
		return
	}

	// Emit receipt viewed event
	s.eventEmitter.Emit(events.Event{
		Type:      events.Phase28TrustActionReceiptViewed,
		Timestamp: now,
		CircleID:  string(circleID),
		Metadata:  map[string]string{},
	})

	data := templateData{
		Title:                "Trust Kept",
		CurrentTime:          now.Format("2006-01-02 15:04"),
		TrustActionReceipt:   receiptInfo,
		TrustActionUndoAvail: undoAvailable,
	}

	s.render(w, "trust-action-receipt", data)
}

// handleTrustActionDismiss handles dismissing the trust action invitation.
// Phase 28: User chose "Keep holding" - silence resumes.
func (s *Server) handleTrustActionDismiss(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	now := s.clk.Now()
	circleID := identity.EntityID("default")
	period := now.UTC().Format("2006-01-02")

	// Emit dismissed event
	s.eventEmitter.Emit(events.Event{
		Type:      events.Phase28TrustActionDismissed,
		Timestamp: now,
		CircleID:  string(circleID),
		Metadata: map[string]string{
			"period": period,
		},
	})

	// Redirect to today - silence resumes
	http.Redirect(w, r, "/today", http.StatusFound)
}

// =============================================================================
// Phase 29: TrueLayer Read-Only Connect + Finance Mirror Proof
// =============================================================================
//
// Reference: docs/ADR/ADR-0060-phase29-truelayer-readonly-finance-mirror.md

// handleTrueLayerOAuthStart initiates the TrueLayer OAuth flow.
// Phase 29: Read-only scopes only. No payment scopes.
func (s *Server) handleTrueLayerOAuthStart(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	now := s.clk.Now()
	circleID := r.URL.Query().Get("circle_id")
	if circleID == "" {
		circleID = "default"
	}

	if s.trueLayerHandler == nil {
		// TrueLayer not configured - redirect to connections
		http.Redirect(w, r, "/connections", http.StatusFound)
		return
	}

	// Start OAuth flow
	result, err := s.trueLayerHandler.Start(circleID)
	if err != nil {
		log.Printf("[Phase 29] TrueLayer OAuth start error: %v", err)
		http.Redirect(w, r, "/connections", http.StatusFound)
		return
	}

	// Emit event
	s.eventEmitter.Emit(events.Event{
		Type:      events.Phase29TrueLayerOAuthStart,
		Timestamp: now,
		CircleID:  circleID,
		Metadata: map[string]string{
			"state_hash": result.State.Hash(),
		},
	})

	// Redirect to TrueLayer
	http.Redirect(w, r, result.AuthURL, http.StatusFound)
}

// handleTrueLayerOAuthCallback handles the OAuth callback from TrueLayer.
// Phase 29: Validates scopes are read-only.
func (s *Server) handleTrueLayerOAuthCallback(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	now := s.clk.Now()

	if s.trueLayerHandler == nil {
		http.Redirect(w, r, "/connections", http.StatusFound)
		return
	}

	code := r.URL.Query().Get("code")
	state := r.URL.Query().Get("state")
	errorParam := r.URL.Query().Get("error")

	// Check for OAuth error
	if errorParam != "" {
		log.Printf("[Phase 29] TrueLayer OAuth error: %s", errorParam)
		s.eventEmitter.Emit(events.Event{
			Type:      events.Phase29TrueLayerOAuthCallback,
			Timestamp: now,
			Metadata: map[string]string{
				"success":    "false",
				"error_type": errorParam,
			},
		})
		http.Redirect(w, r, "/connections", http.StatusFound)
		return
	}

	// Exchange code for tokens
	result, err := s.trueLayerHandler.Callback(r.Context(), code, state)
	if err != nil {
		log.Printf("[Phase 29] TrueLayer callback error: %v", err)
		s.eventEmitter.Emit(events.Event{
			Type:      events.Phase29TrueLayerOAuthCallback,
			Timestamp: now,
			Metadata: map[string]string{
				"success":     "false",
				"error_type":  "token_exchange",
				"fail_reason": result.Receipt.FailReason,
			},
		})
		http.Redirect(w, r, "/connections", http.StatusFound)
		return
	}

	// Store connection hash (not raw tokens)
	if s.financeMirrorStore != nil {
		connectionHash := computeConnectionHash(result.CircleID)
		s.financeMirrorStore.SetConnectionHash(result.CircleID, connectionHash)
	}

	// Emit success event
	s.eventEmitter.Emit(events.Event{
		Type:      events.Phase29TrueLayerOAuthCallback,
		Timestamp: now,
		CircleID:  result.CircleID,
		Metadata: map[string]string{
			"success":      "true",
			"receipt_hash": result.Receipt.Hash(),
		},
	})

	// Redirect to finance mirror
	http.Redirect(w, r, "/mirror/finance", http.StatusFound)
}

// handleTrueLayerDisconnect revokes the TrueLayer connection.
// Phase 29: Idempotent disconnection.
func (s *Server) handleTrueLayerDisconnect(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	now := s.clk.Now()
	circleID := r.FormValue("circle_id")
	if circleID == "" {
		circleID = "default"
	}

	// Remove connection hash
	if s.financeMirrorStore != nil {
		s.financeMirrorStore.RemoveConnection(circleID)
	}

	// Emit event
	s.eventEmitter.Emit(events.Event{
		Type:      events.Phase29TrueLayerOAuthRevoke,
		Timestamp: now,
		CircleID:  circleID,
		Metadata: map[string]string{
			"local_removed": "true",
		},
	})

	http.Redirect(w, r, "/connections", http.StatusFound)
}

// handleTrueLayerSync performs an explicit sync of TrueLayer data.
// Phase 29: Bounded sync (25 items, 7 days). No retries.
func (s *Server) handleTrueLayerSync(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	now := s.clk.Now()
	circleID := r.FormValue("circle_id")
	if circleID == "" {
		circleID = "default"
	}

	// Check if connected
	if s.financeMirrorStore == nil || !s.financeMirrorStore.HasConnection(circleID) {
		http.Redirect(w, r, "/connections", http.StatusFound)
		return
	}

	// Emit sync requested event
	s.eventEmitter.Emit(events.Event{
		Type:      events.Phase29TrueLayerSyncRequested,
		Timestamp: now,
		CircleID:  circleID,
	})

	// In sandbox mode, we create a mock receipt using the constructor
	// Real sync would go through financeMirrorEngine.Sync()
	// Mock data: a few accounts, several transactions
	receipt := domainfinancemirror.NewFinanceSyncReceipt(
		circleID,
		"truelayer",
		now,
		2,                               // accountsCount (maps to "a_few")
		7,                               // transactionsCount (maps to "several")
		[]string{"category|essentials"}, // evidenceTokens
		true,                            // success
		"",                              // failReason
	)

	// Store receipt
	if s.financeMirrorStore != nil {
		_ = s.financeMirrorStore.StoreSyncReceipt(receipt)
	}

	// Emit sync completed event
	s.eventEmitter.Emit(events.Event{
		Type:      events.Phase29TrueLayerSyncCompleted,
		Timestamp: now,
		CircleID:  circleID,
		Metadata: map[string]string{
			"receipt_hash":           receipt.StatusHash,
			"accounts_magnitude":     string(receipt.AccountsMagnitude),
			"transactions_magnitude": string(receipt.TransactionsMagnitude),
		},
	})

	http.Redirect(w, r, "/mirror/finance", http.StatusFound)
}

// handleFinanceMirror shows the finance mirror proof page.
// Phase 29: Abstract data only. No amounts, merchants, or identifiers.
func (s *Server) handleFinanceMirror(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	now := s.clk.Now()
	circleID := "default"

	// Check if connected
	connected := false
	if s.financeMirrorStore != nil {
		connected = s.financeMirrorStore.HasConnection(circleID)
	}

	// Get last receipt
	var lastReceipt *domainfinancemirror.FinanceSyncReceipt
	if s.financeMirrorStore != nil {
		lastReceipt = s.financeMirrorStore.GetLatestSyncReceipt(circleID)
	}

	// Build page via engine
	var page *domainfinancemirror.FinanceMirrorPage
	if s.financeMirrorEngine != nil {
		page = s.financeMirrorEngine.BuildMirrorPage(circleID, connected, lastReceipt)
	} else {
		// Fallback if engine not initialized - use constructor
		// Arguments: connected, lastSyncTime, overallMagnitude, categories
		page = domainfinancemirror.NewFinanceMirrorPage(
			connected,
			time.Time{},                          // lastSyncTime (zero = never)
			domainfinancemirror.MagnitudeNothing, // overallMagnitude
			nil,                                  // categories
		)
	}

	// Emit viewed event
	s.eventEmitter.Emit(events.Event{
		Type:      events.Phase29FinanceMirrorViewed,
		Timestamp: now,
		CircleID:  circleID,
		Metadata: map[string]string{
			"connected":   boolToYesNoString(connected),
			"status_hash": page.StatusHash,
		},
	})

	data := templateData{
		Title:             "Finance Mirror",
		CurrentTime:       now.Format("2006-01-02 15:04"),
		FinanceMirrorPage: page,
	}

	s.render(w, "finance-mirror", data)
}

// handleFinanceMirrorAck acknowledges the finance mirror page.
// Phase 29: Records acknowledgement with hash only.
func (s *Server) handleFinanceMirrorAck(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	now := s.clk.Now()
	circleID := "default"
	periodBucket := now.UTC().Format("2006-01-02")
	statusHash := r.FormValue("status_hash")

	// Create ack using constructor
	ack := domainfinancemirror.NewFinanceMirrorAck(circleID, periodBucket, statusHash)

	// Store ack
	if s.financeMirrorStore != nil {
		_ = s.financeMirrorStore.StoreAck(ack)
	}

	// Emit event
	s.eventEmitter.Emit(events.Event{
		Type:      events.Phase29FinanceMirrorAcked,
		Timestamp: now,
		CircleID:  circleID,
		Metadata: map[string]string{
			"period_bucket": periodBucket,
			"status_hash":   statusHash,
			"ack_hash":      ack.AckHash,
		},
	})

	http.Redirect(w, r, "/today", http.StatusFound)
}

// handleCommerceMirror shows the commerce mirror proof page.
// Phase 31: Abstract data only. No amounts, merchants, or timestamps.
// Default outcome: NOTHING SHOWN. Commerce is observed. Nothing else.
func (s *Server) handleCommerceMirror(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	now := s.clk.Now()
	circleID := "default"

	// Get observations for the current period
	var observations []domaincommerceobserver.CommerceObservation
	if s.commerceObserverStore != nil {
		observations = s.commerceObserverStore.GetLatestObservations(circleID)
	}

	// Build page via engine - returns nil if no observations (silence is success)
	var page *domaincommerceobserver.CommerceMirrorPage
	if s.commerceObserverEngine != nil {
		page = s.commerceObserverEngine.BuildMirrorPage(observations)
	}

	// If no page, show empty state
	if page == nil {
		page = &domaincommerceobserver.CommerceMirrorPage{
			Title:   domaincommerceobserver.DefaultTitle,
			Lines:   []string{"Nothing observed yet. That's fine."},
			Buckets: nil,
		}
	}

	// Emit viewed event
	s.eventEmitter.Emit(events.Event{
		Type:      events.Phase31CommerceMirrorRendered,
		Timestamp: now,
		CircleID:  circleID,
		Metadata: map[string]string{
			"status_hash":  page.StatusHash,
			"bucket_count": fmt.Sprintf("%d", len(page.Buckets)),
		},
	})

	data := templateData{
		Title:              "Commerce Mirror",
		CurrentTime:        now.Format("2006-01-02 15:04"),
		CommerceMirrorPage: page,
	}

	s.render(w, "commerce-mirror", data)
}

// boolToYesNoString converts a bool to "yes" or "no" string for event metadata.
func boolToYesNoString(b bool) string {
	if b {
		return "yes"
	}
	return "no"
}

// computeConnectionHash computes a deterministic hash for TrueLayer connection.
// Phase 29: Used for hash-only storage of connection state.
func computeConnectionHash(circleID string) string {
	canonical := fmt.Sprintf("TRUELAYER_CONNECTION|v1|%s|connected", circleID)
	h := sha256.Sum256([]byte(canonical))
	return hex.EncodeToString(h[:16]) // 32 hex chars
}

// ============================================================================
// Phase 30A: Identity + Replay Handlers
// ============================================================================

// handleIdentity shows the device identity page.
// Phase 30A: Device identity page with fingerprint and binding status.
func (s *Server) handleIdentity(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	now := s.clk.Now()
	circleID := "default"

	// Ensure device has identity
	pubKey, fingerprint, err := s.deviceIdentityEngine.EnsureDeviceIdentity()
	if err != nil {
		log.Printf("Phase 30A: Failed to ensure device identity: %v", err)
		http.Error(w, "Failed to create device identity", http.StatusInternalServerError)
		return
	}

	// Build identity page
	page, err := s.deviceIdentityEngine.BuildIdentityPage(circleID)
	if err != nil {
		log.Printf("Phase 30A: Failed to build identity page: %v", err)
		http.Error(w, "Failed to build identity page", http.StatusInternalServerError)
		return
	}

	// Emit event
	s.eventEmitter.Emit(events.Event{
		Type:      events.Phase30AIdentityViewed,
		Timestamp: now,
		CircleID:  circleID,
		Metadata: map[string]string{
			"fingerprint":     string(fingerprint),
			"public_key":      string(pubKey)[:16] + "...", // Truncated for logging
			"bound_magnitude": string(page.BoundDevicesMagnitude),
			"is_bound":        boolToYesNoString(page.IsBound),
		},
	})

	// Render simple HTML response
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	fmt.Fprintf(w, `<!DOCTYPE html>
<html>
<head><title>Device Identity</title></head>
<body style="font-family: system-ui; max-width: 600px; margin: 40px auto; padding: 20px;">
<h1>Device Identity</h1>
<p style="color: #666;">Phase 30A: Device-rooted identity for replay</p>

<h2>Your Device</h2>
<div style="background: #f5f5f5; padding: 16px; border-radius: 8px; margin: 16px 0;">
  <p><strong>Fingerprint:</strong></p>
  <code style="word-break: break-all; font-size: 12px;">%s</code>
</div>

<h2>Circle Binding</h2>
<p><strong>Bound to circle:</strong> %s</p>
<p><strong>Devices in circle:</strong> %s (max %d)</p>

`, fingerprint, boolToYesNoString(page.IsBound), page.BoundDevicesMagnitude, domaindeviceidentity.MaxDevicesPerCircle)

	if !page.IsBound {
		fmt.Fprintf(w, `
<form action="/identity/bind" method="POST">
  <input type="hidden" name="circle_id" value="%s">
  <button type="submit" style="background: #4CAF50; color: white; padding: 12px 24px; border: none; border-radius: 4px; cursor: pointer;">
    Bind to Circle
  </button>
</form>
`, circleID)
	}

	fmt.Fprintf(w, `
<p style="margin-top: 24px;"><a href="/today">← Back to Today</a></p>
</body>
</html>`)
}

// handleIdentityBind binds the current device to a circle.
// Phase 30A: POST only, requires explicit action.
func (s *Server) handleIdentityBind(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	now := s.clk.Now()
	circleID := r.FormValue("circle_id")
	if circleID == "" {
		circleID = "default"
	}

	// Bind device to circle
	result, err := s.deviceIdentityEngine.BindToCircle(circleID)
	if err != nil {
		log.Printf("Phase 30A: Failed to bind device: %v", err)
		http.Error(w, "Failed to bind device", http.StatusInternalServerError)
		return
	}

	if !result.Success {
		log.Printf("Phase 30A: Bind rejected: %s", result.Error)
		http.Error(w, result.Error, http.StatusBadRequest)
		return
	}

	// Emit event
	s.eventEmitter.Emit(events.Event{
		Type:      events.Phase30AIdentityBound,
		Timestamp: now,
		CircleID:  circleID,
		Metadata: map[string]string{
			"fingerprint":  string(result.Binding.Fingerprint),
			"binding_hash": result.Binding.BindingHash,
			"bound_count":  fmt.Sprintf("%d", result.BoundCount),
			"at_max_limit": boolToYesNoString(result.AtMaxLimit),
		},
	})

	http.Redirect(w, r, "/identity", http.StatusFound)
}

// handleReplayExport exports a replay bundle.
// Phase 30A: POST only, requires bound device, signed request.
func (s *Server) handleReplayExport(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet && r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	now := s.clk.Now()
	circleID := "default"

	// Check device is bound to circle
	isBound, err := s.deviceIdentityEngine.IsBoundToCircle(circleID)
	if err != nil {
		log.Printf("Phase 30A: Failed to check binding: %v", err)
		http.Error(w, "Failed to check device binding", http.StatusInternalServerError)
		return
	}

	if !isBound {
		http.Error(w, "Device not bound to circle. Go to /identity to bind.", http.StatusForbidden)
		return
	}

	if r.Method == http.MethodGet {
		// Show export page
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		fmt.Fprintf(w, `<!DOCTYPE html>
<html>
<head><title>Export Replay Bundle</title></head>
<body style="font-family: system-ui; max-width: 600px; margin: 40px auto; padding: 20px;">
<h1>Export Replay Bundle</h1>
<p style="color: #666;">Phase 30A: Export deterministic replay bundle</p>

<p>This will create a replay bundle containing hash-only records from the last 30 days.</p>
<p><strong>No raw identifiers</strong> are included in the bundle.</p>

<form action="/replay/export" method="POST">
  <input type="hidden" name="circle_id" value="%s">
  <button type="submit" style="background: #2196F3; color: white; padding: 12px 24px; border: none; border-radius: 4px; cursor: pointer;">
    Export Bundle
  </button>
</form>

<p style="margin-top: 24px;"><a href="/identity">← Back to Identity</a></p>
</body>
</html>`, circleID)
		return
	}

	// POST: Build and return bundle
	result, err := s.replayEngine.BuildBundle(circleID, domaindeviceidentity.DefaultRetentionDays)
	if err != nil {
		log.Printf("Phase 30A: Failed to build bundle: %v", err)
		http.Error(w, "Failed to build replay bundle", http.StatusInternalServerError)
		return
	}

	if !result.Success {
		log.Printf("Phase 30A: Bundle build rejected: %s", result.Error)
		http.Error(w, result.Error, http.StatusBadRequest)
		return
	}

	// Emit event
	s.eventEmitter.Emit(events.Event{
		Type:      events.Phase30AReplayExported,
		Timestamp: now,
		CircleID:  circleID,
		Metadata: map[string]string{
			"bundle_hash":  result.Bundle.Header.BundleHash,
			"record_count": fmt.Sprintf("%d", result.Bundle.Header.RecordCount),
			"period_key":   result.Bundle.Header.PeriodKey,
		},
	})

	// Return bundle as downloadable text
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.Header().Set("Content-Disposition", "attachment; filename=replay-bundle.txt")
	fmt.Fprint(w, result.BundleText)
}

// handleReplayImport imports a replay bundle.
// Phase 30A: POST only, requires bound device.
func (s *Server) handleReplayImport(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet && r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	now := s.clk.Now()
	circleID := "default"

	// Check device is bound to circle
	isBound, err := s.deviceIdentityEngine.IsBoundToCircle(circleID)
	if err != nil {
		log.Printf("Phase 30A: Failed to check binding: %v", err)
		http.Error(w, "Failed to check device binding", http.StatusInternalServerError)
		return
	}

	if !isBound {
		http.Error(w, "Device not bound to circle. Go to /identity to bind.", http.StatusForbidden)
		return
	}

	if r.Method == http.MethodGet {
		// Show import page
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		fmt.Fprintf(w, `<!DOCTYPE html>
<html>
<head><title>Import Replay Bundle</title></head>
<body style="font-family: system-ui; max-width: 600px; margin: 40px auto; padding: 20px;">
<h1>Import Replay Bundle</h1>
<p style="color: #666;">Phase 30A: Import deterministic replay bundle</p>

<p>Paste the replay bundle text below to validate and import records.</p>

<form action="/replay/import" method="POST">
  <input type="hidden" name="circle_id" value="%s">
  <textarea name="bundle" rows="10" style="width: 100%%; font-family: monospace; font-size: 12px;" placeholder="Paste bundle text here..."></textarea>
  <br><br>
  <button type="submit" style="background: #FF9800; color: white; padding: 12px 24px; border: none; border-radius: 4px; cursor: pointer;">
    Import Bundle
  </button>
</form>

<p style="margin-top: 24px;"><a href="/identity">← Back to Identity</a></p>
</body>
</html>`, circleID)
		return
	}

	// POST: Validate and import bundle
	bundleText := r.FormValue("bundle")
	if bundleText == "" {
		http.Error(w, "Bundle text is required", http.StatusBadRequest)
		return
	}

	result, err := s.replayEngine.ImportBundle(bundleText, circleID)
	if err != nil {
		log.Printf("Phase 30A: Failed to import bundle: %v", err)
		http.Error(w, "Failed to import replay bundle", http.StatusInternalServerError)
		return
	}

	if !result.Success {
		log.Printf("Phase 30A: Bundle import rejected: %s", result.Error)
		// Emit rejection event
		s.eventEmitter.Emit(events.Event{
			Type:      events.Phase30AReplayRejected,
			Timestamp: now,
			CircleID:  circleID,
			Metadata: map[string]string{
				"error": result.Error,
			},
		})
		http.Error(w, result.Error, http.StatusBadRequest)
		return
	}

	// Emit success event
	s.eventEmitter.Emit(events.Event{
		Type:      events.Phase30AReplayImported,
		Timestamp: now,
		CircleID:  circleID,
		Metadata: map[string]string{
			"records_added":  fmt.Sprintf("%d", result.RecordsAdded),
			"records_exists": fmt.Sprintf("%d", result.RecordsExists),
		},
	})

	// Show success page
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	fmt.Fprintf(w, `<!DOCTYPE html>
<html>
<head><title>Import Complete</title></head>
<body style="font-family: system-ui; max-width: 600px; margin: 40px auto; padding: 20px;">
<h1>Import Complete</h1>
<p style="color: #4CAF50;">✓ Bundle imported successfully</p>

<div style="background: #f5f5f5; padding: 16px; border-radius: 8px; margin: 16px 0;">
  <p><strong>Records added:</strong> %d</p>
  <p><strong>Records already existed:</strong> %d</p>
</div>

<p><a href="/identity">← Back to Identity</a></p>
</body>
</html>`, result.RecordsAdded, result.RecordsExists)
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

// ============================================================================
// Phase 18 Web Control Center Handlers
// ============================================================================

// handleApprove handles approval token verification. Phase 18 Web Control Center.
func (s *Server) handleApprove(w http.ResponseWriter, r *http.Request) {
	s.eventEmitter.Emit(events.Event{
		Type:     events.Phase18WebApproveViewed,
		Metadata: map[string]string{"path": r.URL.Path},
	})

	tokenParam := r.URL.Query().Get("t")

	data := templateData{
		Title:       "Approval",
		CurrentTime: s.clk.Now().Format("2006-01-02 15:04:05"),
	}

	if tokenParam == "" {
		// No token provided - show error
		data.ApprovalResult = &approvalResultInfo{
			Valid:        false,
			ErrorMessage: "No approval token provided. Use ?t=<token> to verify a token.",
		}
		s.render(w, "approve", data)
		return
	}

	// Decode the token
	token, err := approvaltoken.Decode(tokenParam)
	if err != nil {
		data.ApprovalResult = &approvalResultInfo{
			Valid:        false,
			ErrorMessage: "Invalid token format: " + err.Error(),
		}
		s.render(w, "approve", data)
		return
	}

	// Check if token is expired
	now := s.clk.Now()
	isExpired := token.IsExpired(now)

	// Build result info
	data.ApprovalResult = &approvalResultInfo{
		Valid:      true,
		TokenID:    token.TokenID,
		StateID:    token.StateID,
		PersonID:   string(token.PersonID),
		ActionType: string(token.ActionType),
		ExpiresAt:  token.ExpiresAt.Format("2006-01-02 15:04:05"),
		IsExpired:  isExpired,
		IsApproved: token.ActionType == approvaltoken.ActionTypeApprove,
		IsRejected: token.ActionType == approvaltoken.ActionTypeReject,
	}

	if isExpired {
		data.ApprovalResult.Message = "This approval token has expired."
	} else if token.ActionType == approvaltoken.ActionTypeApprove {
		data.ApprovalResult.Message = "This is a valid approval token."
	} else {
		data.ApprovalResult.Message = "This is a valid rejection token."
	}

	s.render(w, "approve", data)
}

// handleRuns handles run log listing. Phase 18 Web Control Center.
func (s *Server) handleRuns(w http.ResponseWriter, r *http.Request) {
	s.eventEmitter.Emit(events.Event{
		Type:     events.Phase18WebRunsViewed,
		Metadata: map[string]string{"path": r.URL.Path},
	})

	snapshots, err := s.runStore.List()
	if err != nil {
		http.Error(w, "Failed to list runs: "+err.Error(), http.StatusInternalServerError)
		return
	}

	// Reverse to show most recent first
	reversed := make([]*runlog.RunSnapshot, len(snapshots))
	for i, snap := range snapshots {
		reversed[len(snapshots)-1-i] = snap
	}

	data := templateData{
		Title:        "Run History",
		CurrentTime:  s.clk.Now().Format("2006-01-02 15:04:05"),
		RunSnapshots: reversed,
	}

	s.render(w, "runs", data)
}

// handleRunDetail handles run log detail view. Phase 18 Web Control Center.
func (s *Server) handleRunDetail(w http.ResponseWriter, r *http.Request) {
	// Extract run ID from path /runs/{id}
	path := r.URL.Path
	if len(path) <= 6 {
		http.Redirect(w, r, "/runs", http.StatusFound)
		return
	}
	runID := path[6:] // Remove "/runs/"

	s.eventEmitter.Emit(events.Event{
		Type:     events.Phase18WebRunDetailViewed,
		Metadata: map[string]string{"run_id": runID},
	})

	snapshot, err := s.runStore.Get(runID)
	if err != nil {
		http.Error(w, "Run not found: "+runID, http.StatusNotFound)
		return
	}

	data := templateData{
		Title:       "Run: " + runID[:16] + "...",
		CurrentTime: s.clk.Now().Format("2006-01-02 15:04:05"),
		RunSnapshot: snapshot,
	}

	s.render(w, "run_detail", data)
}

// handleSuppressions handles suppression rule management. Phase 18 Web Control Center.
func (s *Server) handleSuppressions(w http.ResponseWriter, r *http.Request) {
	s.eventEmitter.Emit(events.Event{
		Type:     events.Phase18WebSuppressionsViewed,
		Metadata: map[string]string{"path": r.URL.Path},
	})

	now := s.clk.Now()
	activeRules := s.suppressionSet.ListActive(now)
	stats := s.suppressionSet.GetStats(now)

	data := templateData{
		Title:            "Suppressions",
		CurrentTime:      now.Format("2006-01-02 15:04:05"),
		SuppressionRules: activeRules,
		SuppressionStats: &stats,
	}

	s.render(w, "suppressions", data)
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
<p class="landing-subtle-link">
    <a href="/start">Start</a>
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

    {{/* Phase 26B: First Minutes cue */}}
    {{if and .FirstMinutesCue .FirstMinutesCue.Available}}
    <section class="whisper-cue first-minutes-cue">
        <p class="whisper-cue-text">{{.FirstMinutesCue.CueText}}</p>
        <a href="/first-minutes" class="whisper-cue-link">{{.FirstMinutesCue.LinkText}}</a>
    </section>
    {{end}}

    {{/* Phase 26C: Reality cue */}}
    {{if and .RealityCue .RealityCue.Available}}
    <section class="whisper-cue reality-cue">
        <p class="whisper-cue-text">{{.RealityCue.CueText}}</p>
        <a href="/reality" class="whisper-cue-link">{{.RealityCue.LinkText}}</a>
    </section>
    {{end}}

    {{/* Phase 27: Shadow Receipt Primary cue (lowest priority) */}}
    {{if and .ShadowReceiptPrimaryCue .ShadowReceiptPrimaryCue.Available}}
    <section class="whisper-cue shadow-receipt-cue">
        <p class="whisper-cue-text">{{.ShadowReceiptPrimaryCue.CueText}}</p>
        <a href="/shadow/receipt" class="whisper-cue-link">{{.ShadowReceiptPrimaryCue.LinkText}}</a>
    </section>
    {{end}}

    {{/* Phase 19.2: Shadow mode whisper link (very subtle) */}}
    {{/* Only show if no other whisper is active */}}
    {{if and (not .SurfaceCue) (not .ProofCue) (not .FirstMinutesCue) (not .RealityCue) (not .ShadowReceiptPrimaryCue)}}
    <section class="shadow-whisper">
        <form action="/run/shadow" method="POST" class="shadow-whisper-form">
            <button type="submit" class="shadow-whisper-link">If you wanted to, we could sanity-check this day.</button>
        </form>
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
{{else if eq .Title "First, consent."}}
    {{template "start-content" .}}
{{else if eq .Title "Connections"}}
    {{template "connections-content" .}}
{{else if eq .Title "Connect Gmail"}}
    {{template "gmail-connect-content" .}}
{{else if eq .Title "Disconnected"}}
    {{template "gmail-disconnected-content" .}}
{{else if hasPrefix .Title "Connect "}}
    {{template "connect-stub-content" .}}
{{else if eq .Title "Seen, quietly."}}
    {{template "mirror-content" .}}
{{else if eq .Title "Mirror"}}
    {{template "mirror-content" .}}
{{else if eq .Title "Quiet Check"}}
    {{template "quiet-check-content" .}}
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

{{/* ================================================================
     PHASE 18.6: FIRST CONNECT - CONSENT-FIRST ONBOARDING
     ================================================================ */}}
{{define "start"}}
{{template "base18" .}}
{{end}}

{{define "start-content"}}
<div class="start">
    <header class="start-header">
        <h1 class="start-title">First, consent.</h1>
        <p class="start-subtitle">QuantumLife stays quiet by default.</p>
    </header>

    <section class="start-section">
        <h2 class="start-section-title">What we can read</h2>
        <p class="start-section-text">Email headers, calendar events, commerce receipts — the shape of your day, not the details.</p>
    </section>

    <section class="start-section">
        <h2 class="start-section-title">What we can do</h2>
        <p class="start-section-text">Draft replies, draft responses, suggest actions — proposals, not commands.</p>
    </section>

    <section class="start-section">
        <h2 class="start-section-title">What we never do</h2>
        <p class="start-section-text">No auto-send, no auto-pay, no background actions. You approve everything.</p>
    </section>

    <section class="start-connect">
        <h3 class="start-connect-title">Connect one source.</h3>
        <div class="start-connect-options">
            <a href="/connect/gmail" class="start-connect-button start-connect-button-gmail">
                Connect Gmail (read-only)
            </a>
            <p class="start-connect-note">
                We read headers only. We do not store message content.
                You sync manually. We never poll in the background.
            </p>
        </div>
        <div class="start-connect-options start-connect-options-secondary">
            <form action="/connect/calendar" method="POST" class="start-connect-form">
                <button type="submit" class="start-connect-button start-connect-button-secondary">Connect Calendar</button>
            </form>
            <form action="/connect/finance" method="POST" class="start-connect-form">
                <button type="submit" class="start-connect-button start-connect-button-secondary">Connect Finance</button>
            </form>
        </div>
    </section>

    <footer class="start-footer">
        <a href="/connections" class="start-connections-link">See connected sources</a>
        <span class="start-footer-divider">·</span>
        <a href="/quiet-check" class="start-quiet-link">Verify quiet baseline</a>
    </footer>
</div>
{{end}}

{{define "connections"}}
{{template "base18" .}}
{{end}}

{{define "connections-content"}}
<div class="connections">
    <header class="connections-header">
        <h1 class="connections-title">Connections</h1>
        <p class="connections-subtitle">Connections change what QuantumLife can read. Not what it can do without you.</p>
    </header>

    <section class="connections-list">
        {{range .ConnectionState.List}}
        <div class="connection-item">
            <div class="connection-kind">{{.Kind}}</div>
            <div class="connection-status connection-status-{{.Status}}">{{.Status.DisplayText}}</div>
            <div class="connection-actions">
                {{if eq .Status.String "not_connected"}}
                {{if eq .Kind.String "email"}}
                <a href="/connect/gmail" class="connection-action-button connection-action-connect">Connect Gmail</a>
                {{else}}
                <form action="/connect/{{.Kind}}" method="POST" class="connection-action-form">
                    <button type="submit" class="connection-action-button connection-action-connect">Connect</button>
                </form>
                {{end}}
                {{else}}
                {{if eq .Kind.String "email"}}
                <form action="/run/gmail-sync" method="POST" class="connection-action-form">
                    <input type="hidden" name="circle_id" value="{{$.CircleID}}">
                    <button type="submit" class="connection-action-button connection-action-sync">Sync now</button>
                </form>
                {{end}}
                <form action="/disconnect/{{.Kind}}" method="POST" class="connection-action-form">
                    <button type="submit" class="connection-action-button connection-action-disconnect">Disconnect</button>
                </form>
                {{end}}
            </div>
        </div>
        {{end}}
    </section>

    <section class="connections-mode">
        <p class="connections-mode-label">Mode: {{if .MockMode}}mock{{else}}real{{end}}</p>
    </section>

    <section class="connections-links">
        <a href="/mirror" class="connections-mirror-link-text">What we noticed (abstractly)</a>
        <span class="connections-divider">·</span>
        <a href="/quiet-check" class="connections-quiet-link">Verify quiet baseline</a>
    </section>

    <footer class="connections-footer">
        <a href="/start" class="connections-back-link">Back to start</a>
        <span class="connections-divider">·</span>
        <a href="/today" class="connections-today-link">Go to today</a>
    </footer>
</div>
{{end}}

{{define "connect-stub"}}
{{template "base18" .}}
{{end}}

{{define "connect-stub-content"}}
<div class="connect-stub">
    <header class="connect-stub-header">
        <h1 class="connect-stub-title">Connect {{.ConnectionKind}}</h1>
    </header>

    <section class="connect-stub-status">
        {{if .MockMode}}
        <p class="connect-stub-text">Mock mode enabled. Click connect to simulate.</p>
        <form action="/connect/{{.ConnectionKind}}" method="POST" class="connect-stub-form">
            <button type="submit" class="connect-stub-button">Connect (Mock)</button>
        </form>
        {{else}}
        <p class="connect-stub-text">Not live yet. This is the UI contract.</p>
        <p class="connect-stub-note">Real connection requires configuration.</p>
        {{end}}
    </section>

    <footer class="connect-stub-footer">
        <a href="/connections" class="connect-stub-back-link">Back to connections</a>
    </footer>
</div>
{{end}}

{{/* ================================================================
     Phase 18.9: Gmail OAuth Connection - Restraint-first copy
     ================================================================ */}}
{{define "gmail-connect"}}
{{template "base18" .}}
{{end}}

{{define "gmail-connect-content"}}
<div class="gmail-connect">
    <header class="gmail-connect-header">
        <h1 class="gmail-connect-title">Connect Gmail</h1>
        <p class="gmail-connect-subtitle">Read-only. Revocable. Nothing stored.</p>
    </header>

    <section class="gmail-connect-promise">
        <div class="gmail-connect-promise-item">
            <h3 class="gmail-connect-promise-title">What we read</h3>
            <p class="gmail-connect-promise-text">Message headers only. Sender domains, timestamps, labels.</p>
            <p class="gmail-connect-promise-not">Not: email bodies, attachments, contact details.</p>
        </div>

        <div class="gmail-connect-promise-item">
            <h3 class="gmail-connect-promise-title">What we store</h3>
            <p class="gmail-connect-promise-text">Hashes, buckets, derived signals. Abstract shapes.</p>
            <p class="gmail-connect-promise-not">Not: subject lines, sender names, message content.</p>
        </div>

        <div class="gmail-connect-promise-item">
            <h3 class="gmail-connect-promise-title">What we never do</h3>
            <p class="gmail-connect-promise-text">No auto-sync. No background polling. Only when you ask.</p>
            <p class="gmail-connect-promise-not">Revoke anytime. Immediate effect. No trace remains.</p>
        </div>
    </section>

    <section class="gmail-connect-action">
        <p class="gmail-connect-scope-note">Scope requested: <strong>gmail.readonly</strong></p>
        <a href="/connect/gmail/start?circle_id={{.CircleID}}" class="gmail-connect-button">Connect with Google</a>
    </section>

    <footer class="gmail-connect-footer">
        <a href="/connections" class="gmail-connect-back-link">Back to connections</a>
    </footer>
</div>
{{end}}

{{/* ================================================================
     Phase 18.9: Gmail Disconnected Confirmation
     ================================================================ */}}
{{define "gmail-disconnected"}}
{{template "base18" .}}
{{end}}

{{define "gmail-disconnected-content"}}
<div class="gmail-disconnected">
    <header class="gmail-disconnected-header">
        <h1 class="gmail-disconnected-title">Disconnected</h1>
        <p class="gmail-disconnected-subtitle">Gmail is no longer connected.</p>
    </header>

    <section class="gmail-disconnected-reassurance">
        <p class="gmail-disconnected-text">Nothing further is read.</p>
        <p class="gmail-disconnected-text">Access was revoked with Google.</p>
        <p class="gmail-disconnected-text">Local tokens removed.</p>
    </section>

    <footer class="gmail-disconnected-footer">
        <a href="/connections" class="gmail-disconnected-link">Back to connections</a>
    </footer>
</div>
{{end}}

{{define "mirror"}}
{{template "base18" .}}
{{end}}

{{define "mirror-content"}}
<div class="mirror">
    <header class="mirror-header">
        <h1 class="mirror-title">{{if .MirrorPage}}{{.MirrorPage.Title}}{{else}}Nothing to show{{end}}</h1>
        <p class="mirror-subtitle">{{if .MirrorPage}}{{.MirrorPage.Subtitle}}{{else}}Connect sources to see what we noticed.{{end}}</p>
    </header>

    {{if .MirrorPage}}
    {{if .MirrorPage.Sources}}
    <section class="mirror-sources">
        {{range .MirrorPage.Sources}}
        <div class="mirror-source">
            <h2 class="mirror-source-kind">{{.Kind}}</h2>
            {{if .ReadSuccessfully}}
            <p class="mirror-source-status">Read successfully</p>
            {{else}}
            <p class="mirror-source-status mirror-source-status-error">Not read</p>
            {{end}}
            <div class="mirror-source-notstored">
                <p class="mirror-source-notstored-label">Not stored:</p>
                <ul class="mirror-source-notstored-list">
                    {{range .NotStored}}
                    <li class="mirror-source-notstored-item">{{.}}</li>
                    {{end}}
                </ul>
            </div>
            {{if .Observed}}
            <div class="mirror-source-observed">
                <p class="mirror-source-observed-label">Observed:</p>
                <ul class="mirror-source-observed-list">
                    {{range .Observed}}
                    <li class="mirror-source-observed-item">{{.Magnitude.DisplayText}} {{.Category.DisplayText}}</li>
                    {{end}}
                </ul>
            </div>
            {{end}}
        </div>
        {{end}}
    </section>
    {{end}}

    <section class="mirror-outcome">
        <h2 class="mirror-outcome-title">As a result:</h2>
        {{if .MirrorPage.Outcome.HeldQuietly}}
        <p class="mirror-outcome-held">{{if eq .MirrorPage.Outcome.HeldMagnitude.String "a_few"}}One item is{{else}}Some items are{{end}} being held quietly</p>
        {{else}}
        <p class="mirror-outcome-nothing">Nothing held</p>
        {{end}}
        {{if .MirrorPage.Outcome.NothingRequiresAttention}}
        <p class="mirror-outcome-attention">Nothing requires your attention</p>
        {{end}}
    </section>

    <section class="mirror-restraint">
        <p class="mirror-restraint-statement">{{.MirrorPage.RestraintStatement}}</p>
        <p class="mirror-restraint-why">{{.MirrorPage.RestraintWhy}}</p>
    </section>
    {{else}}
    <section class="mirror-empty">
        <p class="mirror-empty-text">No sources connected yet.</p>
        <p class="mirror-empty-hint">Connect sources to see what we noticed.</p>
    </section>
    {{end}}

    <footer class="mirror-footer">
        <a href="/connections" class="mirror-back-link">Back to connections</a>
    </footer>
</div>
{{end}}

{{/* ================================================================
     Phase 19.1: Quiet Check - Quiet Baseline Verification
     ================================================================ */}}
{{define "quiet-check"}}
{{template "base18" .}}
{{end}}

{{define "quiet-check-content"}}
<div class="quiet-check">
    <header class="quiet-check-header">
        <h1 class="quiet-check-title">Quiet, verified.</h1>
        <p class="quiet-check-subtitle">Proof that real data stays quiet.</p>
    </header>

    {{if .QuietCheckStatus}}
    <section class="quiet-check-checklist">
        <ul class="quiet-check-list">
            <li class="quiet-check-item {{if .QuietCheckStatus.GmailConnected}}quiet-check-item-yes{{else}}quiet-check-item-no{{end}}">
                <span class="quiet-check-label">Gmail connected</span>
                <span class="quiet-check-value">{{if .QuietCheckStatus.GmailConnected}}yes{{else}}no{{end}}</span>
            </li>
            <li class="quiet-check-item quiet-check-item-neutral">
                <span class="quiet-check-label">Last sync</span>
                <span class="quiet-check-value">{{.QuietCheckStatus.LastSyncTimeBucket}}</span>
            </li>
            <li class="quiet-check-item quiet-check-item-neutral">
                <span class="quiet-check-label">Messages noticed</span>
                <span class="quiet-check-value">{{.QuietCheckStatus.LastSyncMagnitude.DisplayText}}</span>
            </li>
            <li class="quiet-check-item {{if .QuietCheckStatus.ObligationsHeld}}quiet-check-item-yes{{else}}quiet-check-item-no{{end}}">
                <span class="quiet-check-label">Obligations held</span>
                <span class="quiet-check-value">{{if .QuietCheckStatus.ObligationsHeld}}yes{{else}}no{{end}}</span>
            </li>
            <li class="quiet-check-item {{if not .QuietCheckStatus.AutoSurface}}quiet-check-item-yes{{else}}quiet-check-item-no{{end}}">
                <span class="quiet-check-label">Auto-surface</span>
                <span class="quiet-check-value">{{if .QuietCheckStatus.AutoSurface}}enabled{{else}}no{{end}}</span>
            </li>
        </ul>
    </section>

    <section class="quiet-check-result">
        {{if .QuietCheckStatus.IsQuiet}}
        <div class="quiet-check-result-quiet">
            <p class="quiet-check-result-title">Quiet baseline verified.</p>
            <p class="quiet-check-result-text">
                Real Gmail data is connected, synced explicitly, and held quietly.
                Nothing surfaces automatically. Nothing acts without you.
            </p>
        </div>
        {{else}}
        <div class="quiet-check-result-warning">
            <p class="quiet-check-result-title">Check failed.</p>
            <p class="quiet-check-result-text">
                Something is not quiet. Review the checklist above.
            </p>
        </div>
        {{end}}
    </section>

    <section class="quiet-check-hash">
        <p class="quiet-check-hash-label">Status hash:</p>
        <p class="quiet-check-hash-value">{{slice .QuietCheckStatus.Hash 0 16}}...</p>
    </section>
    {{else}}
    <section class="quiet-check-empty">
        <p class="quiet-check-empty-text">No status computed.</p>
        <p class="quiet-check-empty-hint">Connect Gmail and sync to verify quiet baseline.</p>
    </section>
    {{end}}

    <footer class="quiet-check-footer">
        <a href="/connections" class="quiet-check-back-link">Back to connections</a>
        <span class="quiet-check-divider">·</span>
        <a href="/mirror" class="quiet-check-mirror-link">What we noticed</a>
        <span class="quiet-check-divider">·</span>
        <a href="/today" class="quiet-check-today-link">Today, quietly</a>
    </footer>
</div>
{{end}}

{{/* ================================================================
     Phase 18 Web Control Center: Approval Token Verification
     ================================================================ */}}
{{define "approve"}}
{{template "base18" .}}
{{end}}

{{define "approve-content"}}
<div class="approve-page">
    <header class="approve-header">
        <h1 class="approve-title">Approval Verification</h1>
        <p class="approve-subtitle">Verify and process household approval tokens.</p>
    </header>

    {{if .ApprovalResult}}
    <section class="approve-result">
        {{if .ApprovalResult.Valid}}
        <div class="approve-valid">
            <p class="approve-status {{if .ApprovalResult.IsExpired}}approve-status-expired{{else if .ApprovalResult.IsApproved}}approve-status-approve{{else}}approve-status-reject{{end}}">
                {{if .ApprovalResult.IsExpired}}Expired{{else if .ApprovalResult.IsApproved}}Approve{{else}}Reject{{end}}
            </p>
            <p class="approve-message">{{.ApprovalResult.Message}}</p>

            <dl class="approve-details">
                <dt>Token ID</dt>
                <dd>{{slice .ApprovalResult.TokenID 0 12}}...</dd>

                <dt>State ID</dt>
                <dd>{{slice .ApprovalResult.StateID 0 12}}...</dd>

                <dt>Person ID</dt>
                <dd>{{slice .ApprovalResult.PersonID 0 12}}...</dd>

                <dt>Action Type</dt>
                <dd>{{.ApprovalResult.ActionType}}</dd>

                <dt>Expires At</dt>
                <dd>{{.ApprovalResult.ExpiresAt}}</dd>
            </dl>
        </div>
        {{else}}
        <div class="approve-invalid">
            <p class="approve-status approve-status-error">Invalid Token</p>
            <p class="approve-error">{{.ApprovalResult.ErrorMessage}}</p>
        </div>
        {{end}}
    </section>
    {{end}}

    <footer class="approve-footer">
        <a href="/today" class="approve-back-link">Back to Today</a>
        <span class="approve-divider">|</span>
        <a href="/app" class="approve-app-link">Control Center</a>
    </footer>
</div>
{{end}}

{{/* ================================================================
     Phase 18 Web Control Center: Run History
     ================================================================ */}}
{{define "runs"}}
{{template "base18" .}}
{{end}}

{{define "runs-content"}}
<div class="runs-page">
    <header class="runs-header">
        <h1 class="runs-title">Run History</h1>
        <p class="runs-subtitle">Deterministic quiet loop run snapshots.</p>
    </header>

    {{if .RunSnapshots}}
    <section class="runs-list">
        <table class="runs-table">
            <thead>
                <tr>
                    <th>Run ID</th>
                    <th>Started</th>
                    <th>Duration</th>
                    <th>Events</th>
                    <th>Interruptions</th>
                    <th>Drafts</th>
                    <th>Hash</th>
                </tr>
            </thead>
            <tbody>
                {{range .RunSnapshots}}
                <tr class="runs-row">
                    <td><a href="/runs/{{.RunID}}">{{slice .RunID 0 12}}...</a></td>
                    <td>{{formatTime .StartTime}}</td>
                    <td>{{.Duration}}</td>
                    <td>{{.EventsIngested}}</td>
                    <td>{{.InterruptionsCreated}}</td>
                    <td>{{.DraftsCreated}}</td>
                    <td class="runs-hash">{{slice .ResultHash 0 12}}...</td>
                </tr>
                {{end}}
            </tbody>
        </table>
    </section>
    {{else}}
    <section class="runs-empty">
        <p class="runs-empty-text">No run snapshots recorded.</p>
        <p class="runs-empty-hint">Run the daily loop to create run snapshots.</p>
    </section>
    {{end}}

    <footer class="runs-footer">
        <a href="/today" class="runs-back-link">Back to Today</a>
        <span class="runs-divider">|</span>
        <a href="/app" class="runs-app-link">Control Center</a>
    </footer>
</div>
{{end}}

{{/* ================================================================
     Phase 18 Web Control Center: Run Detail
     ================================================================ */}}
{{define "run_detail"}}
{{template "base18" .}}
{{end}}

{{define "run_detail-content"}}
<div class="run-detail-page">
    {{if .RunSnapshot}}
    <header class="run-detail-header">
        <h1 class="run-detail-title">Run Snapshot</h1>
        <p class="run-detail-id">{{.RunSnapshot.RunID}}</p>
    </header>

    <section class="run-detail-summary">
        <dl class="run-detail-stats">
            <dt>Started</dt>
            <dd>{{formatTime .RunSnapshot.StartTime}}</dd>

            <dt>Ended</dt>
            <dd>{{formatTime .RunSnapshot.EndTime}}</dd>

            <dt>Duration</dt>
            <dd>{{.RunSnapshot.Duration}}</dd>

            <dt>Circle ID</dt>
            <dd>{{if .RunSnapshot.CircleID}}{{.RunSnapshot.CircleID}}{{else}}(all circles){{end}}</dd>
        </dl>
    </section>

    <section class="run-detail-counts">
        <h2>Counts</h2>
        <dl class="run-detail-stats">
            <dt>Events Ingested</dt>
            <dd>{{.RunSnapshot.EventsIngested}}</dd>

            <dt>Interruptions Created</dt>
            <dd>{{.RunSnapshot.InterruptionsCreated}}</dd>

            <dt>Interruptions Deduplicated</dt>
            <dd>{{.RunSnapshot.InterruptionsDeduplicated}}</dd>

            <dt>Drafts Created</dt>
            <dd>{{.RunSnapshot.DraftsCreated}}</dd>

            <dt>NeedsYou Items</dt>
            <dd>{{.RunSnapshot.NeedsYouItems}}</dd>
        </dl>
    </section>

    <section class="run-detail-hashes">
        <h2>Hashes</h2>
        <dl class="run-detail-stats">
            <dt>Result Hash</dt>
            <dd class="run-detail-hash">{{.RunSnapshot.ResultHash}}</dd>

            <dt>Config Hash</dt>
            <dd class="run-detail-hash">{{.RunSnapshot.ConfigHash}}</dd>

            <dt>NeedsYou Hash</dt>
            <dd class="run-detail-hash">{{.RunSnapshot.NeedsYouHash}}</dd>
        </dl>
    </section>

    {{if .ReplayResult}}
    <section class="run-detail-replay">
        <h2>Replay Verification</h2>
        {{if .ReplayResult.Success}}
        <p class="run-detail-replay-success">Replay verification passed. Deterministic.</p>
        {{else}}
        <p class="run-detail-replay-fail">Replay verification failed.</p>
        <p>Original hash: {{.ReplayResult.OriginalHash}}</p>
        <p>Replay hash: {{.ReplayResult.ReplayHash}}</p>
        {{if .ReplayResult.Differences}}
        <ul>
            {{range .ReplayResult.Differences}}
            <li>{{.}}</li>
            {{end}}
        </ul>
        {{end}}
        {{end}}
    </section>
    {{end}}
    {{else}}
    <section class="run-detail-notfound">
        <p>Run snapshot not found.</p>
    </section>
    {{end}}

    <footer class="run-detail-footer">
        <a href="/runs" class="run-detail-back-link">Back to Run History</a>
        <span class="run-detail-divider">|</span>
        <a href="/today" class="run-detail-today-link">Today</a>
    </footer>
</div>
{{end}}

{{/* ================================================================
     Phase 18 Web Control Center: Suppressions
     ================================================================ */}}
{{define "suppressions"}}
{{template "base18" .}}
{{end}}

{{define "suppressions-content"}}
<div class="suppressions-page">
    <header class="suppressions-header">
        <h1 class="suppressions-title">Suppressions</h1>
        <p class="suppressions-subtitle">Items you have asked to hold back.</p>
    </header>

    {{if .SuppressionStats}}
    <section class="suppressions-stats">
        <dl class="suppressions-stat-list">
            <dt>Total Rules</dt>
            <dd>{{.SuppressionStats.TotalRules}}</dd>

            <dt>Active</dt>
            <dd>{{.SuppressionStats.ActiveRules}}</dd>

            <dt>Expired</dt>
            <dd>{{.SuppressionStats.ExpiredRules}}</dd>
        </dl>
    </section>
    {{end}}

    {{if .SuppressionRules}}
    <section class="suppressions-list">
        <table class="suppressions-table">
            <thead>
                <tr>
                    <th>Rule ID</th>
                    <th>Circle</th>
                    <th>Scope</th>
                    <th>Key</th>
                    <th>Reason</th>
                    <th>Expires</th>
                </tr>
            </thead>
            <tbody>
                {{range .SuppressionRules}}
                <tr class="suppressions-row">
                    <td>{{slice .RuleID 0 12}}...</td>
                    <td>{{slice .CircleID 0 8}}...</td>
                    <td>{{.Scope}}</td>
                    <td>{{.Key}}</td>
                    <td>{{.Reason}}</td>
                    <td>{{if .ExpiresAt}}{{formatTime .ExpiresAt}}{{else}}permanent{{end}}</td>
                </tr>
                {{end}}
            </tbody>
        </table>
    </section>
    {{else}}
    <section class="suppressions-empty">
        <p class="suppressions-empty-text">No active suppression rules.</p>
        <p class="suppressions-empty-hint">Suppressions are created when you dismiss or hide items.</p>
    </section>
    {{end}}

    <footer class="suppressions-footer">
        <a href="/today" class="suppressions-back-link">Back to Today</a>
        <span class="suppressions-divider">|</span>
        <a href="/policies" class="suppressions-policies-link">Policies</a>
        <span class="suppressions-divider">|</span>
        <a href="/app" class="suppressions-app-link">Control Center</a>
    </footer>
</div>
{{end}}

{{/* ================================================================
     Phase 26A: Guided Journey
     ================================================================ */}}
{{define "journey"}}
{{template "base18" .}}
{{end}}

{{define "journey-content"}}
<div class="journey-page">
    {{if .JourneyPage}}
    {{if .JourneyPage.IsDone}}
    <section class="journey-done">
        <h1 class="journey-title">{{.JourneyPage.Title}}</h1>
        <p class="journey-subtitle">{{.JourneyPage.Subtitle}}</p>
        <footer class="journey-footer">
            <a href="/today" class="journey-back-link">Back to Today</a>
        </footer>
    </section>
    {{else}}
    <header class="journey-header">
        <span class="journey-step-label">{{.JourneyPage.StepLabel}}</span>
    </header>

    <section class="journey-card">
        <h1 class="journey-title">{{.JourneyPage.Title}}</h1>
        <p class="journey-subtitle">{{.JourneyPage.Subtitle}}</p>

        {{if .JourneyPage.Lines}}
        <div class="journey-lines">
            {{range .JourneyPage.Lines}}
            <p class="journey-line">{{.}}</p>
            {{end}}
        </div>
        {{end}}

        <div class="journey-actions">
            {{if eq .JourneyPage.PrimaryAction.Method "POST"}}
            <form method="POST" action="{{.JourneyPage.PrimaryAction.Path}}">
                {{range $k, $v := .JourneyPage.PrimaryAction.FormFields}}
                <input type="hidden" name="{{$k}}" value="{{$v}}">
                {{end}}
                <button type="submit" class="journey-primary-btn">{{.JourneyPage.PrimaryAction.Label}}</button>
            </form>
            {{else}}
            <a href="{{.JourneyPage.PrimaryAction.Path}}" class="journey-primary-btn">{{.JourneyPage.PrimaryAction.Label}}</a>
            {{end}}

            {{if .JourneyPage.SecondaryAction}}
            <form method="POST" action="{{.JourneyPage.SecondaryAction.Path}}" class="journey-secondary-form">
                {{range $k, $v := .JourneyPage.SecondaryAction.FormFields}}
                <input type="hidden" name="{{$k}}" value="{{$v}}">
                {{end}}
                <button type="submit" class="journey-secondary-btn">{{.JourneyPage.SecondaryAction.Label}}</button>
            </form>
            {{end}}
        </div>
    </section>

    <footer class="journey-footer">
        <span class="journey-hash">Hash: {{slice .JourneyPage.StatusHash 0 12}}...</span>
    </footer>
    {{end}}
    {{else}}
    <section class="journey-empty">
        <p>Journey not available.</p>
        <a href="/today" class="journey-back-link">Back to Today</a>
    </section>
    {{end}}
</div>
{{end}}

{{/* ================================================================
     Phase 26B: First Five Minutes Proof
     ================================================================ */}}
{{define "first-minutes"}}
{{template "base18" .}}
{{end}}

{{define "first-minutes-content"}}
<div class="first-minutes-page">
    {{if .FirstMinutesSummary}}
    <header class="first-minutes-header">
        <h1 class="first-minutes-title">How it began</h1>
        <p class="first-minutes-period">{{.FirstMinutesSummary.Period}}</p>
    </header>

    <section class="first-minutes-card">
        <p class="first-minutes-calm-line">{{.FirstMinutesSummary.CalmLine}}</p>

        {{if .FirstMinutesSummary.Signals}}
        <ul class="first-minutes-signals">
            {{range .FirstMinutesSummary.Signals}}
            <li class="first-minutes-signal">
                <span class="first-minutes-signal-kind">{{.Kind}}</span>
                <span class="first-minutes-signal-magnitude">{{.Magnitude}}</span>
            </li>
            {{end}}
        </ul>
        {{end}}
    </section>

    <footer class="first-minutes-footer">
        <form method="POST" action="/first-minutes/dismiss">
            <input type="hidden" name="status_hash" value="{{.FirstMinutesSummary.StatusHash}}">
            <button type="submit" class="first-minutes-dismiss-btn">Dismiss</button>
        </form>
        <span class="first-minutes-hash">Hash: {{slice .FirstMinutesSummary.StatusHash 0 12}}...</span>
        <a href="/today" class="first-minutes-back-link">Back to Today</a>
    </footer>
    {{else}}
    <section class="first-minutes-empty">
        <p>Nothing to show yet.</p>
        <a href="/today" class="first-minutes-back-link">Back to Today</a>
    </section>
    {{end}}
</div>
{{end}}

{{/* ================================================================
     Phase 26C: Connected Reality Check
     ================================================================ */}}
{{define "reality"}}
{{template "base18" .}}
{{end}}

{{define "reality-content"}}
<div class="reality-page">
    {{if .RealityPage}}
    <header class="reality-header">
        <h1 class="reality-title">{{.RealityPage.Title}}</h1>
        <p class="reality-subtitle">{{.RealityPage.Subtitle}}</p>
    </header>

    <section class="reality-card">
        {{if .RealityPage.Lines}}
        <dl class="reality-lines">
            {{range .RealityPage.Lines}}
            <div class="reality-line reality-line-{{.Kind}}">
                <dt class="reality-label">{{.Label}}</dt>
                <dd class="reality-value">{{.Value}}</dd>
            </div>
            {{end}}
        </dl>
        {{end}}

        {{if .RealityPage.CalmLine}}
        <p class="reality-calm-line">{{.RealityPage.CalmLine}}</p>
        {{end}}
    </section>

    <footer class="reality-footer">
        <form method="POST" action="/reality/ack">
            <input type="hidden" name="status_hash" value="{{.RealityPage.StatusHash}}">
            <button type="submit" class="reality-ack-btn">Acknowledge</button>
        </form>
        <span class="reality-hash">Hash: {{slice .RealityPage.StatusHash 0 12}}...</span>
        <a href="{{.RealityPage.BackPath}}" class="reality-back-link">Back</a>
    </footer>
    {{else}}
    <section class="reality-empty">
        <p>Reality check not available.</p>
        <a href="/today" class="reality-back-link">Back to Today</a>
    </section>
    {{end}}
</div>
{{end}}

{{/* ================================================================
     Phase 29: Finance Mirror Proof
     ================================================================ */}}
{{define "finance-mirror"}}
{{template "base18" .}}
{{end}}

{{define "finance-mirror-content"}}
<div class="finance-mirror-page">
    {{if .FinanceMirrorPage}}
    <header class="finance-mirror-header">
        <h1 class="finance-mirror-title">{{.FinanceMirrorPage.Title}}</h1>
    </header>

    <section class="finance-mirror-card">
        {{if .FinanceMirrorPage.CalmLine}}
        <p class="finance-mirror-calm-line">{{.FinanceMirrorPage.CalmLine}}</p>
        {{end}}

        {{if .FinanceMirrorPage.Categories}}
        <ul class="finance-mirror-categories">
            {{range .FinanceMirrorPage.Categories}}
            <li class="finance-mirror-category">
                <span class="category-name">{{.Category.DisplayText}}</span>
                <span class="category-magnitude">{{.Magnitude.DisplayText}}</span>
            </li>
            {{end}}
        </ul>
        {{end}}

        {{if .FinanceMirrorPage.Reassurance}}
        <p class="finance-mirror-reassurance">{{.FinanceMirrorPage.Reassurance}}</p>
        {{end}}
    </section>

    <footer class="finance-mirror-footer">
        <span class="finance-mirror-sync">Last sync: {{.FinanceMirrorPage.LastSyncBucket}}</span>
        <a href="/today" class="finance-mirror-back-link">Back to Today</a>
    </footer>
    {{else}}
    <section class="finance-mirror-empty">
        <p>Finance mirror not available. Connect your finance provider first.</p>
        <a href="/connections" class="finance-mirror-back-link">View connections</a>
    </section>
    {{end}}
</div>
{{end}}

{{/* ================================================================
     Phase 31: Commerce Mirror (Silent by Default)
     ================================================================ */}}
{{define "commerce-mirror"}}
{{template "base18" .}}
{{end}}

{{define "commerce-mirror-content"}}
<div class="commerce-mirror-page">
    {{if .CommerceMirrorPage}}
    <header class="commerce-mirror-header">
        <h1 class="commerce-mirror-title">{{.CommerceMirrorPage.Title}}</h1>
    </header>

    <section class="commerce-mirror-card">
        {{if .CommerceMirrorPage.Lines}}
        {{range .CommerceMirrorPage.Lines}}
        <p class="commerce-mirror-line">{{.}}</p>
        {{end}}
        {{end}}

        {{if .CommerceMirrorPage.Buckets}}
        <ul class="commerce-mirror-buckets">
            {{range .CommerceMirrorPage.Buckets}}
            <li class="commerce-mirror-bucket">{{.DisplayText}}</li>
            {{end}}
        </ul>
        {{end}}
    </section>

    <footer class="commerce-mirror-footer">
        <a href="/today" class="commerce-mirror-back-link">Back to Today</a>
    </footer>
    {{else}}
    <section class="commerce-mirror-empty">
        <p>Nothing observed yet. That's fine.</p>
        <a href="/today" class="commerce-mirror-back-link">Back to Today</a>
    </section>
    {{end}}
</div>
{{end}}
`
