// Package execintent defines execution intent types for Phase 10.
//
// An ExecutionIntent represents "what execution should happen" derived from
// an approved draft. It binds the draft to policy and view snapshots and
// routes to the correct execution boundary.
//
// CRITICAL: Deterministic. Same inputs = same IntentID.
// CRITICAL: No external writes. Intent is a plan, not an action.
// CRITICAL: Execution happens ONLY via boundary executors.
//
// Reference: Phase 10 - Approved Draft → Execution Routing
package execintent

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"
	"time"

	"quantumlife/pkg/domain/draft"
)

// ActionClass identifies the type of execution action.
type ActionClass string

const (
	// ActionEmailSend represents sending an email (reply).
	ActionEmailSend ActionClass = "email_send"

	// ActionCalendarRespond represents responding to a calendar event.
	ActionCalendarRespond ActionClass = "calendar_respond"

	// ActionFinancePayment represents executing a financial payment.
	// CRITICAL: All finance actions flow through the Finance Execution Boundary.
	// Phase 17b: Routes to V96Executor.
	ActionFinancePayment ActionClass = "finance_payment"
)

// IntentID is a deterministic identifier for an execution intent.
type IntentID string

// ExecutionIntent represents the plan to execute an approved draft.
type ExecutionIntent struct {
	// IntentID is the deterministic identifier.
	IntentID IntentID

	// DraftID is the source draft that was approved.
	DraftID draft.DraftID

	// CircleID is the owning circle.
	CircleID string

	// Action is the type of execution.
	Action ActionClass

	// Email fields (populated for ActionEmailSend)
	EmailThreadID  string // Thread being replied to
	EmailMessageID string // Message being replied to
	EmailTo        string // Recipient (from draft context, not free-text)
	EmailSubject   string
	EmailBody      string

	// Calendar fields (populated for ActionCalendarRespond)
	CalendarEventID  string
	CalendarResponse string // "accepted", "declined", "tentative"

	// Finance fields (populated for ActionFinancePayment)
	// CRITICAL: PayeeID must be a pre-defined payee, NOT free-text.
	FinancePayeeID     string // Pre-defined payee identifier
	FinanceAmountCents int64  // Amount in minor units (pence/cents)
	FinanceCurrency    string // ISO currency code (e.g., "GBP")
	FinanceDescription string // Payment reference/description
	FinanceEnvelopeID  string // Linked execution envelope ID (set after envelope creation)
	FinanceActionHash  string // Deterministic action hash for approval binding

	// Snapshot hashes for execution safety
	PolicySnapshotHash string
	ViewSnapshotHash   string

	// CreatedAt is when the intent was created (from injected clock).
	CreatedAt time.Time

	// DeterministicHash is the content hash for idempotency.
	DeterministicHash string
}

// CanonicalString returns a deterministic string representation.
// Uses pipe-delimited format, NOT JSON.
func (i *ExecutionIntent) CanonicalString() string {
	var b strings.Builder

	b.WriteString("execintent|")
	b.WriteString(fmt.Sprintf("draft:%s|", i.DraftID))
	b.WriteString(fmt.Sprintf("circle:%s|", i.CircleID))
	b.WriteString(fmt.Sprintf("action:%s|", i.Action))

	// Email fields
	b.WriteString(fmt.Sprintf("email_thread:%s|", normalizeForCanonical(i.EmailThreadID)))
	b.WriteString(fmt.Sprintf("email_message:%s|", normalizeForCanonical(i.EmailMessageID)))
	b.WriteString(fmt.Sprintf("email_to:%s|", normalizeForCanonical(i.EmailTo)))
	b.WriteString(fmt.Sprintf("email_subject:%s|", normalizeForCanonical(i.EmailSubject)))
	b.WriteString(fmt.Sprintf("email_body:%s|", normalizeForCanonical(i.EmailBody)))

	// Calendar fields
	b.WriteString(fmt.Sprintf("calendar_event:%s|", normalizeForCanonical(i.CalendarEventID)))
	b.WriteString(fmt.Sprintf("calendar_response:%s|", normalizeForCanonical(i.CalendarResponse)))

	// Finance fields
	b.WriteString(fmt.Sprintf("finance_payee:%s|", normalizeForCanonical(i.FinancePayeeID)))
	b.WriteString(fmt.Sprintf("finance_amount:%d|", i.FinanceAmountCents))
	b.WriteString(fmt.Sprintf("finance_currency:%s|", normalizeForCanonical(i.FinanceCurrency)))
	b.WriteString(fmt.Sprintf("finance_desc:%s|", normalizeForCanonical(i.FinanceDescription)))
	b.WriteString(fmt.Sprintf("finance_envelope:%s|", normalizeForCanonical(i.FinanceEnvelopeID)))
	b.WriteString(fmt.Sprintf("finance_action_hash:%s|", normalizeForCanonical(i.FinanceActionHash)))

	// Hashes
	b.WriteString(fmt.Sprintf("policy_hash:%s|", i.PolicySnapshotHash))
	b.WriteString(fmt.Sprintf("view_hash:%s", i.ViewSnapshotHash))

	return b.String()
}

// Hash computes the SHA256 hash of the canonical string.
func (i *ExecutionIntent) Hash() string {
	h := sha256.Sum256([]byte(i.CanonicalString()))
	return hex.EncodeToString(h[:])
}

// ComputeIntentID generates a deterministic IntentID from the intent content.
func (i *ExecutionIntent) ComputeIntentID() IntentID {
	hash := i.Hash()
	return IntentID(fmt.Sprintf("intent-%s", hash[:16]))
}

// Finalize sets the IntentID and DeterministicHash based on content.
// Must be called after all fields are populated.
func (i *ExecutionIntent) Finalize() {
	i.DeterministicHash = i.Hash()
	i.IntentID = i.ComputeIntentID()
}

// Validate checks that the intent has all required fields for execution.
func (i *ExecutionIntent) Validate() error {
	if i.DraftID == "" {
		return fmt.Errorf("missing DraftID")
	}
	if i.CircleID == "" {
		return fmt.Errorf("missing CircleID")
	}
	if i.Action == "" {
		return fmt.Errorf("missing Action")
	}
	if i.PolicySnapshotHash == "" {
		return fmt.Errorf("missing PolicySnapshotHash")
	}
	if i.ViewSnapshotHash == "" {
		return fmt.Errorf("missing ViewSnapshotHash")
	}

	switch i.Action {
	case ActionEmailSend:
		if i.EmailThreadID == "" && i.EmailMessageID == "" {
			return fmt.Errorf("email action requires ThreadID or MessageID")
		}
	case ActionCalendarRespond:
		if i.CalendarEventID == "" {
			return fmt.Errorf("calendar action requires EventID")
		}
		if i.CalendarResponse == "" {
			return fmt.Errorf("calendar action requires Response")
		}
	case ActionFinancePayment:
		// CRITICAL: Finance payments require pre-defined payee, not free-text.
		if i.FinancePayeeID == "" {
			return fmt.Errorf("finance action requires PayeeID (pre-defined payee)")
		}
		if i.FinanceAmountCents <= 0 {
			return fmt.Errorf("finance action requires AmountCents > 0")
		}
		if i.FinanceCurrency == "" {
			return fmt.Errorf("finance action requires Currency")
		}
	default:
		return fmt.Errorf("unsupported action: %s", i.Action)
	}

	return nil
}

// normalizeForCanonical normalizes a string for canonical representation.
func normalizeForCanonical(s string) string {
	s = strings.ToLower(s)
	s = strings.TrimSpace(s)
	s = strings.ReplaceAll(s, "|", "_")
	s = strings.ReplaceAll(s, "\r\n", " ")
	s = strings.ReplaceAll(s, "\n", " ")
	s = strings.ReplaceAll(s, "\r", "")
	return s
}

// ExecutionResult contains the outcome of intent execution.
type ExecutionResult struct {
	// IntentID is the executed intent.
	IntentID IntentID

	// Success indicates execution completed without error.
	Success bool

	// Blocked indicates execution was blocked by safety checks.
	Blocked bool

	// BlockReason explains why execution was blocked.
	BlockReason string

	// Error contains any execution error.
	Error error

	// EnvelopeID is the boundary envelope ID (if executed).
	EnvelopeID string

	// ExecutedAt is when execution completed.
	ExecutedAt time.Time
}
