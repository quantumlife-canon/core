// Package loop provides multi-circle loop extensions for Phase 11.
//
// This file extends the loop engine to support:
// - Multi-circle configuration loading
// - Sync receipt tracking per circle
// - Last sync time tracking
// - Combined ingestion + loop run
//
// Reference: docs/ADR/ADR-0026-phase11-multicircle-real-loop.md
package loop

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"sort"
	"strings"
	"time"

	"quantumlife/internal/config"
	"quantumlife/internal/identityresolve"
	"quantumlife/internal/ingestion"
	"quantumlife/internal/routing"
	"quantumlife/pkg/clock"
	"quantumlife/pkg/domain/identity"
	"quantumlife/pkg/events"
)

// MultiCircleRunner combines ingestion and loop execution for multi-circle operation.
// Phase 13.1: Integrates identity resolution before obligation extraction.
type MultiCircleRunner struct {
	// Engine is the underlying loop engine.
	Engine *Engine

	// Clock for deterministic time.
	Clock clock.Clock

	// Config is the multi-circle configuration.
	Config *config.MultiCircleConfig

	// Router routes events to circles.
	Router *routing.Router

	// MultiRunner handles multi-account ingestion.
	MultiRunner *ingestion.MultiRunner

	// SyncReceipts stores sync receipts per circle.
	SyncReceipts map[identity.EntityID]*CircleSyncState

	// EventEmitter emits audit events.
	EventEmitter events.Emitter

	// Phase 13.1: Identity graph components
	// IdentityRepo stores identity entities and edges.
	IdentityRepo *identity.InMemoryRepository

	// IdentityResolver resolves identity from events.
	IdentityResolver *identityresolve.Resolver
}

// CircleSyncState tracks sync state for a single circle.
type CircleSyncState struct {
	// CircleID is the circle identifier.
	CircleID identity.EntityID

	// CircleName is the display name.
	CircleName string

	// LastSyncAt is when the last sync was performed.
	LastSyncAt time.Time

	// LastSyncReceipt is the receipt from the last sync.
	LastSyncReceipt *ingestion.CircleSyncReceipt

	// IntegrationStatus contains per-integration status.
	IntegrationStatus []IntegrationStatus
}

// IntegrationStatus tracks status for a single integration.
type IntegrationStatus struct {
	// Type is the integration type (email, calendar, finance).
	Type string

	// Provider is the provider name.
	Provider string

	// AccountID is the account identifier.
	AccountID string

	// HasToken indicates if a valid token exists.
	HasToken bool

	// LastSyncAt is when this integration was last synced.
	LastSyncAt time.Time

	// LastEventCount is the number of events from last sync.
	LastEventCount int

	// LastError is any error from the last sync.
	LastError string
}

// NewMultiCircleRunner creates a new multi-circle runner.
func NewMultiCircleRunner(engine *Engine, clk clock.Clock, cfg *config.MultiCircleConfig) *MultiCircleRunner {
	return &MultiCircleRunner{
		Engine:       engine,
		Clock:        clk,
		Config:       cfg,
		Router:       routing.NewRouter(cfg),
		SyncReceipts: make(map[identity.EntityID]*CircleSyncState),
	}
}

// WithMultiRunner sets the multi-account ingestion runner.
func (r *MultiCircleRunner) WithMultiRunner(runner *ingestion.MultiRunner) *MultiCircleRunner {
	r.MultiRunner = runner
	return r
}

// WithEventEmitter sets the event emitter.
func (r *MultiCircleRunner) WithEventEmitter(emitter events.Emitter) *MultiCircleRunner {
	r.EventEmitter = emitter
	return r
}

// WithIdentity sets the identity repository and resolver.
// Phase 13.1: Enables identity-driven routing and resolution.
func (r *MultiCircleRunner) WithIdentity(repo *identity.InMemoryRepository, resolver *identityresolve.Resolver) *MultiCircleRunner {
	r.IdentityRepo = repo
	r.IdentityResolver = resolver

	// Connect identity repository to router for identity-based routing
	if repo != nil {
		r.Router.SetIdentityRepository(repo)
	}

	return r
}

// MultiCircleRunOptions configures a multi-circle run.
type MultiCircleRunOptions struct {
	// CircleID limits the run to a specific circle (empty = all circles).
	CircleID identity.EntityID

	// RunIngestion runs ingestion before the loop.
	RunIngestion bool

	// IngestionOptions configures ingestion if RunIngestion is true.
	IngestionOptions ingestion.MultiRunOptions

	// ExecuteApprovedDrafts executes approved drafts if true.
	ExecuteApprovedDrafts bool
}

// MultiCircleRunResult contains results from a multi-circle run.
type MultiCircleRunResult struct {
	// RunID is the deterministic run ID.
	RunID string

	// StartedAt is when the run started.
	StartedAt time.Time

	// CompletedAt is when the run completed.
	CompletedAt time.Time

	// IngestionResult contains ingestion results (if ingestion was run).
	IngestionResult *ingestion.MultiRunResult

	// LoopResult contains loop results.
	LoopResult RunResult

	// CircleSyncStates contains per-circle sync states.
	CircleSyncStates map[identity.EntityID]*CircleSyncState

	// Hash is the deterministic hash of the run result.
	Hash string

	// Errors contains any errors.
	Errors []string

	// Phase 13.1: Identity graph statistics
	// IdentityGraphHash is the deterministic hash of the identity graph state.
	IdentityGraphHash string

	// IdentityStats contains counts of identity entities and edges.
	IdentityStats IdentityStats
}

// IdentityStats provides identity graph statistics.
type IdentityStats struct {
	PersonCount       int
	OrganizationCount int
	HouseholdCount    int
	EmailAccountCount int
	PhoneNumberCount  int
	EdgeCount         int
}

// Run executes a multi-circle ingestion + loop run.
// CRITICAL: This method is synchronous. No goroutines are spawned.
// Phase 13.1: Identity resolution runs before obligation extraction.
func (r *MultiCircleRunner) Run(opts MultiCircleRunOptions) MultiCircleRunResult {
	now := r.Clock.Now()
	result := MultiCircleRunResult{
		StartedAt:        now,
		CircleSyncStates: make(map[identity.EntityID]*CircleSyncState),
	}

	// Compute run ID
	result.RunID = r.computeRunID(now, opts)

	// Emit start event
	r.emitEvent(events.EventType("phase11.multicircle.run.started"), map[string]string{
		"run_id":    result.RunID,
		"circle_id": string(opts.CircleID),
	})

	// Run ingestion if requested
	if opts.RunIngestion && r.MultiRunner != nil {
		ingestionResult, err := r.MultiRunner.Run(r.Config, opts.IngestionOptions)
		if err != nil {
			result.Errors = append(result.Errors, fmt.Sprintf("ingestion error: %v", err))
		} else {
			result.IngestionResult = ingestionResult

			// Update sync states from ingestion receipts
			for _, receipt := range ingestionResult.CircleReceipts {
				state := r.updateSyncState(receipt)
				result.CircleSyncStates[receipt.CircleID] = state
			}
		}
	}

	// Phase 13.1: Compute identity graph statistics before loop
	if r.IdentityRepo != nil {
		result.IdentityStats = r.computeIdentityStats()
		result.IdentityGraphHash = r.computeIdentityGraphHash()

		// Emit identity resolution event
		r.emitEvent(events.EventType("phase13.identity.graph.computed"), map[string]string{
			"run_id":             result.RunID,
			"identity_hash":      result.IdentityGraphHash,
			"person_count":       fmt.Sprintf("%d", result.IdentityStats.PersonCount),
			"organization_count": fmt.Sprintf("%d", result.IdentityStats.OrganizationCount),
			"household_count":    fmt.Sprintf("%d", result.IdentityStats.HouseholdCount),
			"edge_count":         fmt.Sprintf("%d", result.IdentityStats.EdgeCount),
		})
	}

	// Run the loop
	loopOpts := RunOptions{
		CircleID:              opts.CircleID,
		ExecuteApprovedDrafts: opts.ExecuteApprovedDrafts,
	}
	result.LoopResult = r.Engine.Run(nil, loopOpts)

	// Copy any existing sync states not updated by ingestion
	for circleID, state := range r.SyncReceipts {
		if _, exists := result.CircleSyncStates[circleID]; !exists {
			result.CircleSyncStates[circleID] = state
		}
	}

	result.CompletedAt = r.Clock.Now()
	result.Hash = r.computeResultHash(&result)

	// Emit completion event
	r.emitEvent(events.EventType("phase11.multicircle.run.completed"), map[string]string{
		"run_id":              result.RunID,
		"duration_ms":         fmt.Sprintf("%d", result.CompletedAt.Sub(result.StartedAt).Milliseconds()),
		"hash":                result.Hash,
		"identity_graph_hash": result.IdentityGraphHash,
	})

	return result
}

// GetCircleSyncState returns the sync state for a circle.
func (r *MultiCircleRunner) GetCircleSyncState(circleID identity.EntityID) *CircleSyncState {
	return r.SyncReceipts[circleID]
}

// GetAllSyncStates returns all sync states in deterministic order.
func (r *MultiCircleRunner) GetAllSyncStates() []*CircleSyncState {
	var states []*CircleSyncState
	for _, id := range r.Config.CircleIDs() {
		if state, ok := r.SyncReceipts[id]; ok {
			states = append(states, state)
		}
	}
	return states
}

// updateSyncState updates the sync state for a circle from an ingestion receipt.
func (r *MultiCircleRunner) updateSyncState(receipt ingestion.CircleSyncReceipt) *CircleSyncState {
	state := &CircleSyncState{
		CircleID:        receipt.CircleID,
		CircleName:      receipt.CircleName,
		LastSyncAt:      receipt.FetchedAt,
		LastSyncReceipt: &receipt,
	}

	// Build integration status from receipt
	for _, ir := range receipt.IntegrationResults {
		status := IntegrationStatus{
			Type:           ir.Type,
			Provider:       ir.Provider,
			AccountID:      ir.AccountID,
			HasToken:       ir.Success,
			LastSyncAt:     receipt.FetchedAt,
			LastEventCount: ir.EventsFetched,
		}
		if !ir.Success {
			status.LastError = ir.Error
		}
		state.IntegrationStatus = append(state.IntegrationStatus, status)
	}

	r.SyncReceipts[receipt.CircleID] = state
	return state
}

// computeRunID computes a deterministic run ID.
func (r *MultiCircleRunner) computeRunID(now time.Time, opts MultiCircleRunOptions) string {
	var b strings.Builder
	b.WriteString("multicircle_run|")
	b.WriteString(now.UTC().Format(time.RFC3339Nano))
	b.WriteString("|circle:")
	b.WriteString(string(opts.CircleID))
	b.WriteString("|ingest:")
	if opts.RunIngestion {
		b.WriteString("true")
	} else {
		b.WriteString("false")
	}

	h := sha256.Sum256([]byte(b.String()))
	return hex.EncodeToString(h[:])[:16]
}

// computeIdentityStats computes identity graph statistics.
// Phase 13.1: Returns counts of all entity types and edges.
func (r *MultiCircleRunner) computeIdentityStats() IdentityStats {
	if r.IdentityRepo == nil {
		return IdentityStats{}
	}

	return IdentityStats{
		PersonCount:       r.IdentityRepo.CountByType(identity.EntityTypePerson),
		OrganizationCount: r.IdentityRepo.CountByType(identity.EntityTypeOrganization),
		HouseholdCount:    r.IdentityRepo.CountByType(identity.EntityTypeHousehold),
		EmailAccountCount: r.IdentityRepo.CountByType(identity.EntityTypeEmailAccount),
		PhoneNumberCount:  r.IdentityRepo.CountByType(identity.EntityTypePhoneNumber),
		EdgeCount:         r.IdentityRepo.EdgeCount(),
	}
}

// computeIdentityGraphHash computes a deterministic hash of the identity graph state.
// Phase 13.1: Hash is computed from sorted entities + sorted edges.
func (r *MultiCircleRunner) computeIdentityGraphHash() string {
	if r.IdentityRepo == nil {
		return ""
	}

	var b strings.Builder
	b.WriteString("identity_graph|")

	// Add persons in sorted order
	persons := r.IdentityRepo.ListPersons()
	for _, p := range persons {
		b.WriteString("person:")
		b.WriteString(string(p.ID()))
		b.WriteString(":")
		b.WriteString(p.PrimaryEmail)
		b.WriteString("|")
	}

	// Add organizations in sorted order
	orgs := r.IdentityRepo.ListOrganizations()
	for _, o := range orgs {
		b.WriteString("org:")
		b.WriteString(string(o.ID()))
		b.WriteString(":")
		b.WriteString(o.Domain)
		b.WriteString("|")
	}

	// Add households in sorted order
	households := r.IdentityRepo.ListHouseholds()
	for _, h := range households {
		b.WriteString("hh:")
		b.WriteString(string(h.ID()))
		b.WriteString(":")
		b.WriteString(h.Name)
		b.WriteString("|")
	}

	// Add edges in sorted order
	edges := r.IdentityRepo.GetAllEdgesSorted()
	for _, e := range edges {
		b.WriteString("edge:")
		b.WriteString(string(e.EdgeType))
		b.WriteString(":")
		b.WriteString(string(e.FromID))
		b.WriteString("->")
		b.WriteString(string(e.ToID))
		b.WriteString("|")
	}

	h := sha256.Sum256([]byte(b.String()))
	return hex.EncodeToString(h[:])
}

// computeResultHash computes a deterministic hash of the run result.
func (r *MultiCircleRunner) computeResultHash(result *MultiCircleRunResult) string {
	var b strings.Builder
	b.WriteString("multicircle_result|")
	b.WriteString("run_id:")
	b.WriteString(result.RunID)
	b.WriteString("|started:")
	b.WriteString(result.StartedAt.UTC().Format(time.RFC3339))

	// Include loop result hash
	b.WriteString("|loop_needs_you_hash:")
	b.WriteString(result.LoopResult.NeedsYou.Hash)

	// Include ingestion hash if present
	if result.IngestionResult != nil {
		b.WriteString("|ingest_hash:")
		b.WriteString(result.IngestionResult.Hash)
	}

	// Phase 13.1: Include identity graph hash if present
	if result.IdentityGraphHash != "" {
		b.WriteString("|identity_hash:")
		b.WriteString(result.IdentityGraphHash)
	}

	// Include circle states in sorted order
	var circleIDs []identity.EntityID
	for id := range result.CircleSyncStates {
		circleIDs = append(circleIDs, id)
	}
	sort.Slice(circleIDs, func(i, j int) bool {
		return string(circleIDs[i]) < string(circleIDs[j])
	})

	for _, id := range circleIDs {
		state := result.CircleSyncStates[id]
		b.WriteString("|circle:")
		b.WriteString(string(id))
		if state.LastSyncReceipt != nil {
			b.WriteString(":")
			b.WriteString(state.LastSyncReceipt.Hash)
		}
	}

	h := sha256.Sum256([]byte(b.String()))
	return hex.EncodeToString(h[:])
}

// emitEvent emits an event.
func (r *MultiCircleRunner) emitEvent(eventType events.EventType, metadata map[string]string) {
	if r.EventEmitter == nil {
		return
	}
	r.EventEmitter.Emit(events.Event{
		Type:      eventType,
		Timestamp: r.Clock.Now(),
		Metadata:  metadata,
	})
}

// CircleConfigInfo provides info about a configured circle.
type CircleConfigInfo struct {
	ID                 identity.EntityID
	Name               string
	EmailCount         int
	CalendarCount      int
	FinanceCount       int
	HasEmailAdapter    bool
	HasCalendarAdapter bool
	HasFinanceAdapter  bool
	LastSyncAt         *time.Time
	SyncState          *CircleSyncState
}

// GetCircleConfigInfos returns info about all configured circles.
func (r *MultiCircleRunner) GetCircleConfigInfos() []CircleConfigInfo {
	var infos []CircleConfigInfo

	for _, circleID := range r.Config.CircleIDs() {
		circle := r.Config.GetCircle(circleID)
		if circle == nil {
			continue
		}

		info := CircleConfigInfo{
			ID:            circle.ID,
			Name:          circle.Name,
			EmailCount:    len(circle.EmailIntegrations),
			CalendarCount: len(circle.CalendarIntegrations),
			FinanceCount:  len(circle.FinanceIntegrations),
		}

		// Check adapters (based on multi-runner)
		if r.MultiRunner != nil {
			for _, email := range circle.EmailIntegrations {
				if hasAdapter(r.MultiRunner, "email", email.Provider) {
					info.HasEmailAdapter = true
					break
				}
			}
			for _, cal := range circle.CalendarIntegrations {
				if hasAdapter(r.MultiRunner, "calendar", cal.Provider) {
					info.HasCalendarAdapter = true
					break
				}
			}
			for _, fin := range circle.FinanceIntegrations {
				if hasAdapter(r.MultiRunner, "finance", fin.Provider) {
					info.HasFinanceAdapter = true
					break
				}
			}
		}

		// Add sync state if available
		if state, ok := r.SyncReceipts[circleID]; ok {
			info.LastSyncAt = &state.LastSyncAt
			info.SyncState = state
		}

		infos = append(infos, info)
	}

	return infos
}

// hasAdapter checks if a provider adapter is registered.
// This is a helper that would ideally query the multi-runner's adapter map.
func hasAdapter(runner *ingestion.MultiRunner, adapterType, provider string) bool {
	// For now, we can't directly query the runner's adapter maps
	// This would need to be exposed via the MultiRunner interface
	// Return true as a placeholder - actual implementation would check
	_ = runner
	_ = adapterType
	_ = provider
	return false
}
