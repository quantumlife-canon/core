// Package demo_v9_guarded demonstrates v9 guarded execution.
//
// CRITICAL: This demo uses GUARDED adapters.
// NO REAL MONEY MOVES. NO PROVIDER WRITE CALLS.
//
// This slice proves that:
// - Execution pipeline reaches the adapter
// - Adapter blocks execution
// - Audit trail is complete
// - Settlement is blocked (not succeeded)
//
// Settlement outcome is ALWAYS: blocked
// NEVER: settled_successfully
//
// Subordinate to:
// - docs/QUANTUMLIFE_CANON_V1.md
// - docs/CANON_ADDENDUM_V9_EXECUTION.md
// - docs/TECHNICAL_SPLIT_V9_EXECUTION.md
package demo_v9_guarded

import (
	"fmt"
	"time"

	"quantumlife/internal/finance/execution"
	"quantumlife/pkg/events"
)

// Scenario represents a v9 guarded execution scenario.
type Scenario struct {
	// Name is the scenario name.
	Name string

	// Description describes what this scenario demonstrates.
	Description string

	// Intent is the execution intent.
	Intent execution.ExecutionIntent

	// AdapterProvider is the adapter provider to use.
	AdapterProvider string

	// ShouldRevoke indicates if revocation should occur.
	ShouldRevoke bool

	// RevocationReason is the reason for revocation (if applicable).
	RevocationReason string

	// ShouldExpireApproval indicates if approval should expire.
	ShouldExpireApproval bool

	// ExpectedStatus is the expected settlement status.
	ExpectedStatus execution.SettlementStatus
}

// ScenarioResult contains the result of running a scenario.
type ScenarioResult struct {
	// Scenario is the scenario that was run.
	Scenario *Scenario

	// Intent is the execution intent.
	Intent execution.ExecutionIntent

	// Envelope is the sealed execution envelope.
	Envelope *execution.ExecutionEnvelope

	// ApprovalRequest is the approval request.
	ApprovalRequest *execution.ApprovalRequest

	// Approval is the approval artifact.
	Approval *execution.ApprovalArtifact

	// PrepareResult is the adapter prepare result.
	PrepareResult *execution.PrepareResult

	// ExecutionAttempt is the adapter execution attempt.
	ExecutionAttempt *execution.ExecutionAttempt

	// ExecutionResult is the final execution result.
	ExecutionResult *execution.ExecutionResult

	// AuditEvents is the list of audit events emitted.
	AuditEvents []events.Event

	// Success indicates if the scenario completed as expected.
	Success bool

	// FailureReason explains why the scenario failed (if applicable).
	FailureReason string
}

// DefaultScenario creates the default guarded execution scenario.
//
// This demonstrates:
// 1. Intent creation
// 2. Envelope building
// 3. Approval request + submission
// 4. Revocation window expires WITHOUT revocation
// 5. Adapter prepare
// 6. Adapter execute - BLOCKED
// 7. Settlement = blocked
func DefaultScenario() *Scenario {
	return &Scenario{
		Name:        "v9-guarded-blocked",
		Description: "Demonstrates guarded execution: adapter blocks despite valid approval",
		Intent: execution.ExecutionIntent{
			IntentID:       "", // Will be set by runner
			CircleID:       "circle_alice",
			IntersectionID: "intersection_family_finance",
			Description:    "Payment of GBP 75.00 to household utilities",
			ActionType:     execution.ActionTypePayment,
			AmountCents:    7500, // GBP 75.00
			Currency:       "GBP",
			PayeeID:        "Utility Provider",
			ViewHash:       "", // Will be set from v8 view
			CreatedAt:      time.Time{},
		},
		AdapterProvider:      "mock-finance",
		ShouldRevoke:         false, // No revocation - window expires naturally
		ShouldExpireApproval: false,
		ExpectedStatus:       execution.SettlementBlocked, // Guarded adapter always blocks
	}
}

// RevocationScenario creates a scenario where revocation occurs.
// This demonstrates that revocation still halts execution.
func RevocationScenario() *Scenario {
	return &Scenario{
		Name:        "v9-guarded-revoked",
		Description: "Demonstrates revocation still blocks even with guarded adapter",
		Intent: execution.ExecutionIntent{
			IntentID:       "",
			CircleID:       "circle_alice",
			IntersectionID: "intersection_family_finance",
			Description:    "Payment of GBP 30.00 to streaming service",
			ActionType:     execution.ActionTypePayment,
			AmountCents:    3000, // GBP 30.00
			Currency:       "GBP",
			PayeeID:        "Streaming Service",
			ViewHash:       "",
			CreatedAt:      time.Time{},
		},
		AdapterProvider:  "mock-finance",
		ShouldRevoke:     true,
		RevocationReason: "Circle-initiated revocation during window",
		ExpectedStatus:   execution.SettlementRevoked,
	}
}

// ExpiredApprovalScenario creates a scenario where approval expires.
// This demonstrates that expired approval blocks execution.
func ExpiredApprovalScenario() *Scenario {
	return &Scenario{
		Name:        "v9-guarded-approval-expired",
		Description: "Demonstrates expired approval blocks execution",
		Intent: execution.ExecutionIntent{
			IntentID:       "",
			CircleID:       "circle_alice",
			IntersectionID: "intersection_family_finance",
			Description:    "Payment of GBP 20.00 to food delivery",
			ActionType:     execution.ActionTypePayment,
			AmountCents:    2000, // GBP 20.00
			Currency:       "GBP",
			PayeeID:        "Food Delivery",
			ViewHash:       "",
			CreatedAt:      time.Time{},
		},
		AdapterProvider:      "mock-finance",
		ShouldRevoke:         false,
		ShouldExpireApproval: true,
		ExpectedStatus:       execution.SettlementBlocked,
	}
}

// PlaidStubScenario creates a scenario using the Plaid stub adapter.
// This demonstrates the same guardrail works across different provider stubs.
func PlaidStubScenario() *Scenario {
	return &Scenario{
		Name:        "v9-guarded-plaid-stub",
		Description: "Demonstrates Plaid stub adapter blocks execution",
		Intent: execution.ExecutionIntent{
			IntentID:       "",
			CircleID:       "circle_alice",
			IntersectionID: "intersection_family_finance",
			Description:    "Transfer of GBP 50.00 to savings",
			ActionType:     execution.ActionTypeTransfer,
			AmountCents:    5000, // GBP 50.00
			Currency:       "GBP",
			PayeeID:        "Savings Pot",
			ViewHash:       "",
			CreatedAt:      time.Time{},
		},
		AdapterProvider:      "plaid-stub",
		ShouldRevoke:         false,
		ShouldExpireApproval: false,
		ExpectedStatus:       execution.SettlementBlocked,
	}
}

// FormatAmount formats an amount in cents as a currency string.
func FormatAmount(cents int64, currency string) string {
	negative := cents < 0
	if negative {
		cents = -cents
	}

	sign := ""
	if negative {
		sign = "-"
	}

	symbol := ""
	switch currency {
	case "GBP":
		symbol = "£"
	case "USD":
		symbol = "$"
	case "EUR":
		symbol = "€"
	default:
		symbol = currency + " "
	}

	return fmt.Sprintf("%s%s%d.%02d", sign, symbol, cents/100, cents%100)
}
