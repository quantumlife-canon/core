// Package demo_v9_dryrun demonstrates v9 financial execution in dry-run mode.
//
// CRITICAL: This demo is DRY-RUN ONLY.
// NO REAL MONEY MOVES. NO PROVIDER WRITE CALLS.
//
// This slice proves that execution can be:
// - Prepared
// - Inspected
// - Revoked
// - Safely stopped
//
// Settlement outcome is ALWAYS one of: blocked, revoked, expired, aborted.
// NEVER settled_successfully.
//
// Subordinate to:
// - docs/QUANTUMLIFE_CANON_V1.md
// - docs/CANON_ADDENDUM_V9_EXECUTION.md
// - docs/TECHNICAL_SPLIT_V9_EXECUTION.md
package demo_v9_dryrun

import (
	"fmt"
	"time"

	"quantumlife/internal/finance/execution"
	"quantumlife/pkg/events"
)

// Scenario represents a v9 dry-run execution scenario.
type Scenario struct {
	// Name is the scenario name.
	Name string

	// Description describes what this scenario demonstrates.
	Description string

	// Intent is the execution intent.
	Intent execution.ExecutionIntent

	// ShouldRevoke indicates if revocation should be exercised.
	ShouldRevoke bool

	// RevocationReason is the reason for revocation (if applicable).
	RevocationReason string

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

	// RevocationSignal is the revocation signal (if revoked).
	RevocationSignal *execution.RevocationSignal

	// ValidityCheck is the validity check result.
	ValidityCheck execution.ValidityCheckResult

	// ExecutionResult is the final execution result.
	ExecutionResult *execution.ExecutionResult

	// AuditEvents is the list of audit events emitted.
	AuditEvents []events.Event

	// Success indicates if the scenario completed as expected.
	Success bool

	// FailureReason explains why the scenario failed (if applicable).
	FailureReason string
}

// DefaultScenario creates the default dry-run scenario.
// This demonstrates:
// 1. Intent creation
// 2. Envelope building
// 3. Approval request with neutral language
// 4. Approval submission
// 5. Revocation window opening
// 6. Revocation triggered
// 7. Execution blocked
// 8. Settlement recorded as revoked
func DefaultScenario() *Scenario {
	return &Scenario{
		Name:        "v9-dryrun-revocation",
		Description: "Demonstrates complete v9 execution flow with revocation",
		Intent: execution.ExecutionIntent{
			IntentID:       "", // Will be set by runner
			CircleID:       "circle_alice",
			IntersectionID: "intersection_family_finance",
			Description:    "Payment of GBP 50.00 to shared household expenses",
			ActionType:     execution.ActionTypePayment,
			AmountCents:    5000, // £50.00
			Currency:       "GBP",
			Recipient:      "Household Expenses",
			ViewHash:       "",          // Will be set from v8 view
			CreatedAt:      time.Time{}, // Will be set by runner
		},
		ShouldRevoke:     true,
		RevocationReason: "Circle-initiated revocation during revocation window",
		ExpectedStatus:   execution.SettlementRevoked,
	}
}

// NoRevocationScenario creates a scenario where no revocation occurs.
// In dry-run mode, this still results in aborted (not settled).
func NoRevocationScenario() *Scenario {
	return &Scenario{
		Name:        "v9-dryrun-aborted",
		Description: "Demonstrates complete v9 execution flow without revocation (still aborted in dry-run)",
		Intent: execution.ExecutionIntent{
			IntentID:       "",
			CircleID:       "circle_alice",
			IntersectionID: "intersection_family_finance",
			Description:    "Transfer of GBP 25.00 to savings",
			ActionType:     execution.ActionTypeTransfer,
			AmountCents:    2500, // £25.00
			Currency:       "GBP",
			Recipient:      "Savings Pot",
			ViewHash:       "",
			CreatedAt:      time.Time{},
		},
		ShouldRevoke:   false,
		ExpectedStatus: execution.SettlementAborted, // Dry-run always aborts
	}
}

// ExpiredApprovalScenario creates a scenario where approval expires.
func ExpiredApprovalScenario() *Scenario {
	return &Scenario{
		Name:        "v9-dryrun-expired",
		Description: "Demonstrates execution blocked due to expired approval",
		Intent: execution.ExecutionIntent{
			IntentID:       "",
			CircleID:       "circle_alice",
			IntersectionID: "",
			Description:    "Payment of GBP 10.00 to utility bill",
			ActionType:     execution.ActionTypePayment,
			AmountCents:    1000, // £10.00
			Currency:       "GBP",
			Recipient:      "Utility Company",
			ViewHash:       "",
			CreatedAt:      time.Time{},
		},
		ShouldRevoke:   false,
		ExpectedStatus: execution.SettlementBlocked,
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
