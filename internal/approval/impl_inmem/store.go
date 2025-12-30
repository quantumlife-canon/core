// Package impl_inmem provides an in-memory implementation of the approval store.
// This is for demo and testing purposes.
//
// CRITICAL: In production, approvals must be persisted and distributed.
//
// Reference: v7 Multi-party approval governance
package impl_inmem

import (
	"context"
	"fmt"
	"sync"
	"time"

	"quantumlife/internal/approval"
	auditImpl "quantumlife/internal/audit/impl_inmem"
	"quantumlife/pkg/events"
	"quantumlife/pkg/primitives"
)

// Store implements approval.Manager with in-memory storage.
type Store struct {
	mu                sync.RWMutex
	approvals         map[string]*primitives.ApprovalArtifact     // approvalID -> approval
	approvalsByAction map[string][]*primitives.ApprovalArtifact   // intersectionID:actionID -> approvals
	requestTokens     map[string]*primitives.ApprovalRequestToken // tokenID -> token
	clockFunc         func() time.Time
	idCounter         int
	signingSecret     []byte
	auditStore        *auditImpl.Store
	intersectionStore IntersectionStore
}

// IntersectionStore provides access to intersection contracts for approval verification.
// This interface uses approval.ContractForApproval to avoid importing the intersection package.
type IntersectionStore interface {
	// GetContractForApproval returns the contract info needed for approval verification.
	GetContractForApproval(ctx context.Context, intersectionID string) (*approval.ContractForApproval, error)
}

// StoreConfig configures the approval store.
type StoreConfig struct {
	ClockFunc         func() time.Time
	SigningSecret     []byte
	AuditStore        *auditImpl.Store
	IntersectionStore IntersectionStore
}

// NewStore creates a new in-memory approval store.
func NewStore(config StoreConfig) *Store {
	clockFunc := config.ClockFunc
	if clockFunc == nil {
		clockFunc = time.Now
	}

	signingSecret := config.SigningSecret
	if len(signingSecret) == 0 {
		signingSecret = []byte("default-signing-secret-for-demo")
	}

	return &Store{
		approvals:         make(map[string]*primitives.ApprovalArtifact),
		approvalsByAction: make(map[string][]*primitives.ApprovalArtifact),
		requestTokens:     make(map[string]*primitives.ApprovalRequestToken),
		clockFunc:         clockFunc,
		signingSecret:     signingSecret,
		auditStore:        config.AuditStore,
		intersectionStore: config.IntersectionStore,
	}
}

// StoreApproval stores an approval artifact.
func (s *Store) StoreApproval(ctx context.Context, appr *primitives.ApprovalArtifact) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Check for duplicate approval from same circle for same action
	key := fmt.Sprintf("%s:%s", appr.IntersectionID, appr.ActionID)
	for _, existing := range s.approvalsByAction[key] {
		if existing.ApproverCircleID == appr.ApproverCircleID {
			return approval.ErrDuplicateApproval
		}
	}

	s.approvals[appr.ApprovalID] = appr
	s.approvalsByAction[key] = append(s.approvalsByAction[key], appr)

	return nil
}

// GetApprovals retrieves all approvals for an action.
func (s *Store) GetApprovals(ctx context.Context, intersectionID, actionID string) ([]*primitives.ApprovalArtifact, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	key := fmt.Sprintf("%s:%s", intersectionID, actionID)
	approvals := s.approvalsByAction[key]

	// Return a copy to prevent mutation
	result := make([]*primitives.ApprovalArtifact, len(approvals))
	copy(result, approvals)

	return result, nil
}

// GetApprovalByID retrieves a specific approval by ID.
func (s *Store) GetApprovalByID(ctx context.Context, approvalID string) (*primitives.ApprovalArtifact, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	appr, ok := s.approvals[approvalID]
	if !ok {
		return nil, approval.ErrApprovalNotFound
	}

	return appr, nil
}

// StoreRequestToken stores an approval request token.
func (s *Store) StoreRequestToken(ctx context.Context, token *primitives.ApprovalRequestToken) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.requestTokens[token.TokenID] = token
	return nil
}

// GetRequestToken retrieves a request token by ID.
func (s *Store) GetRequestToken(ctx context.Context, tokenID string) (*primitives.ApprovalRequestToken, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	token, ok := s.requestTokens[tokenID]
	if !ok {
		return nil, approval.ErrRequestTokenNotFound
	}

	return token, nil
}

// DeleteExpiredApprovals removes expired approvals.
func (s *Store) DeleteExpiredApprovals(ctx context.Context, before time.Time) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	count := 0
	for id, appr := range s.approvals {
		if appr.ExpiresAt.Before(before) {
			delete(s.approvals, id)
			count++

			// Emit audit event
			if s.auditStore != nil {
				s.auditStore.Append(ctx, auditImpl.Entry{
					Type:           string(events.EventApprovalExpired),
					IntersectionID: appr.IntersectionID,
					Action:         "approval_expired",
					Outcome:        fmt.Sprintf("approval %s expired", appr.ApprovalID),
				})
			}
		}
	}

	// Clean up approvalsByAction
	for key, approvals := range s.approvalsByAction {
		var valid []*primitives.ApprovalArtifact
		for _, appr := range approvals {
			if !appr.ExpiresAt.Before(before) {
				valid = append(valid, appr)
			}
		}
		if len(valid) == 0 {
			delete(s.approvalsByAction, key)
		} else {
			s.approvalsByAction[key] = valid
		}
	}

	return count, nil
}

// RequestApproval creates an approval request token for an action.
func (s *Store) RequestApproval(ctx context.Context, req approval.ApprovalRequest) (*primitives.ApprovalRequestToken, error) {
	s.mu.Lock()
	s.idCounter++
	tokenID := fmt.Sprintf("reqtoken-%d", s.idCounter)
	s.mu.Unlock()

	now := s.clockFunc()

	// Get expiry from request or use default
	expirySeconds := req.ExpirySeconds
	if expirySeconds == 0 {
		expirySeconds = 3600 // Default 1 hour
	}

	// Compute action hash
	actionHash := primitives.ComputeActionHashFromAction(
		req.Action,
		req.IntersectionID,
		req.ContractVersion,
		req.ScopesRequired,
		primitives.ModeExecute,
	)

	// Build action summary
	summary := fmt.Sprintf("%s: %s", req.Action.Type, req.Action.ID)
	if title, ok := req.Action.Parameters["title"]; ok {
		summary = fmt.Sprintf("%s: %s", req.Action.Type, title)
	}

	token := &primitives.ApprovalRequestToken{
		TokenID:            tokenID,
		IntersectionID:     req.IntersectionID,
		ContractVersion:    req.ContractVersion,
		ActionID:           req.Action.ID,
		ActionHash:         actionHash,
		ActionType:         req.Action.Type,
		ActionSummary:      summary,
		RequestingCircleID: req.RequestingCircleID,
		ScopesRequired:     req.ScopesRequired,
		CreatedAt:          now,
		ExpiresAt:          now.Add(time.Duration(expirySeconds) * time.Second),
	}

	// Sign the token
	token.Signature = primitives.SignRequestToken(token, s.signingSecret)

	// Store the token
	if err := s.StoreRequestToken(ctx, token); err != nil {
		return nil, err
	}

	// Emit audit event
	if s.auditStore != nil {
		s.auditStore.Append(ctx, auditImpl.Entry{
			Type:           string(events.EventApprovalRequested),
			CircleID:       req.RequestingCircleID,
			IntersectionID: req.IntersectionID,
			Action:         "approval_requested",
			Outcome:        fmt.Sprintf("token %s created for action %s", tokenID, req.Action.ID),
			TraceID:        req.TraceID,
		})
	}

	return token, nil
}

// SubmitApproval submits an approval for an action.
func (s *Store) SubmitApproval(ctx context.Context, req approval.SubmitApprovalRequest) (*primitives.ApprovalArtifact, error) {
	// Decode the token
	token, err := primitives.DecodeApprovalToken(req.Token)
	if err != nil {
		return nil, fmt.Errorf("invalid token: %w", err)
	}

	// Verify token signature
	if !primitives.VerifyRequestTokenSignature(token, s.signingSecret) {
		return nil, approval.ErrInvalidSignature
	}

	// Check if token has expired
	now := s.clockFunc()
	if token.IsExpired(now) {
		return nil, approval.ErrRequestTokenExpired
	}

	// Check if circle is authorized to approve
	if err := s.checkCircleAuthorization(ctx, token.IntersectionID, req.ApproverCircleID); err != nil {
		return nil, err
	}

	// Check for duplicate approval
	existingApprovals, _ := s.GetApprovals(ctx, token.IntersectionID, token.ActionID)
	for _, existing := range existingApprovals {
		if existing.ApproverCircleID == req.ApproverCircleID {
			return nil, approval.ErrDuplicateApproval
		}
	}

	// Generate approval ID
	s.mu.Lock()
	s.idCounter++
	approvalID := fmt.Sprintf("approval-%d", s.idCounter)
	s.mu.Unlock()

	// Get contract for expiry settings
	expirySeconds := 3600 // Default
	if s.intersectionStore != nil {
		contract, err := s.intersectionStore.GetContractForApproval(ctx, token.IntersectionID)
		if err == nil && contract.ApprovalPolicy.ExpirySeconds > 0 {
			expirySeconds = contract.ApprovalPolicy.ExpirySeconds
		}
	}

	// Create approval artifact
	artifact := &primitives.ApprovalArtifact{
		ApprovalID:       approvalID,
		IntersectionID:   token.IntersectionID,
		ContractVersion:  token.ContractVersion,
		ActionID:         token.ActionID,
		ActionHash:       token.ActionHash,
		ApproverCircleID: req.ApproverCircleID,
		ScopesApproved:   token.ScopesRequired,
		ApprovedAt:       now,
		ExpiresAt:        now.Add(time.Duration(expirySeconds) * time.Second),
	}

	// Sign the artifact
	artifact.Signature = primitives.SignApprovalArtifact(artifact, s.signingSecret)

	// Store the approval
	if err := s.StoreApproval(ctx, artifact); err != nil {
		return nil, err
	}

	// Emit audit event
	if s.auditStore != nil {
		s.auditStore.Append(ctx, auditImpl.Entry{
			Type:           string(events.EventApprovalSubmitted),
			CircleID:       req.ApproverCircleID,
			IntersectionID: token.IntersectionID,
			Action:         "approval_submitted",
			Outcome:        fmt.Sprintf("approval %s for action %s", approvalID, token.ActionID),
			TraceID:        req.TraceID,
		})
	}

	return artifact, nil
}

// checkCircleAuthorization verifies a circle can approve for an intersection.
func (s *Store) checkCircleAuthorization(ctx context.Context, intersectionID, circleID string) error {
	if s.intersectionStore == nil {
		// No store configured, allow any circle (for testing)
		return nil
	}

	contract, err := s.intersectionStore.GetContractForApproval(ctx, intersectionID)
	if err != nil {
		return fmt.Errorf("failed to get contract: %w", err)
	}

	// Check if circle is a party to the intersection
	for _, partyID := range contract.Parties {
		if partyID == circleID {
			return nil
		}
	}

	// Check if circle is in RequiredApprovers
	for _, approverID := range contract.ApprovalPolicy.RequiredApprovers {
		if approverID == circleID {
			return nil
		}
	}

	return approval.ErrCircleNotAuthorized
}

// VerifyApprovals checks if approvals satisfy the contract's ApprovalPolicy.
func (s *Store) VerifyApprovals(ctx context.Context, req approval.VerifyApprovalsRequest) (*primitives.ApprovalVerificationResult, error) {
	result := &primitives.ApprovalVerificationResult{
		ValidApprovals:   make([]string, 0),
		InvalidApprovals: make([]primitives.ApprovalFailure, 0),
		MissingApprovers: make([]string, 0),
	}

	policy := req.Contract.ApprovalPolicy
	now := s.clockFunc()

	// Handle single approval mode (v6 compatibility)
	if policy.Mode == "" || policy.Mode == approval.ApprovalModeSingle {
		result.Passed = true
		result.ThresholdRequired = 1
		result.ThresholdMet = 1
		return result, nil
	}

	// Get approvals for this action
	approvals, err := s.GetApprovals(ctx, req.Contract.IntersectionID, req.Action.ID)
	if err != nil {
		return nil, fmt.Errorf("failed to get approvals: %w", err)
	}

	// Validate each approval
	validApproverCircles := make(map[string]bool)
	for _, appr := range approvals {
		// Check expiry
		if appr.IsExpired(now) {
			result.InvalidApprovals = append(result.InvalidApprovals, primitives.ApprovalFailure{
				ApprovalID: appr.ApprovalID,
				CircleID:   appr.ApproverCircleID,
				Reason:     "approval has expired",
			})
			continue
		}

		// Check action hash matches
		if appr.ActionHash != req.ActionHash {
			result.InvalidApprovals = append(result.InvalidApprovals, primitives.ApprovalFailure{
				ApprovalID: appr.ApprovalID,
				CircleID:   appr.ApproverCircleID,
				Reason:     "action hash mismatch (replay protection)",
			})
			continue
		}

		// Verify signature
		if !primitives.VerifyApprovalSignature(appr, s.signingSecret) {
			result.InvalidApprovals = append(result.InvalidApprovals, primitives.ApprovalFailure{
				ApprovalID: appr.ApprovalID,
				CircleID:   appr.ApproverCircleID,
				Reason:     "invalid signature",
			})
			continue
		}

		// Check if from required approver (if specified)
		if len(policy.RequiredApprovers) > 0 {
			found := false
			for _, required := range policy.RequiredApprovers {
				if required == appr.ApproverCircleID {
					found = true
					break
				}
			}
			if !found {
				result.InvalidApprovals = append(result.InvalidApprovals, primitives.ApprovalFailure{
					ApprovalID: appr.ApprovalID,
					CircleID:   appr.ApproverCircleID,
					Reason:     "circle not in required approvers list",
				})
				continue
			}
		}

		// Approval is valid
		result.ValidApprovals = append(result.ValidApprovals, appr.ApprovalID)
		validApproverCircles[appr.ApproverCircleID] = true
	}

	// Calculate threshold
	result.ThresholdRequired = policy.Threshold
	if result.ThresholdRequired == 0 {
		result.ThresholdRequired = 1
	}
	result.ThresholdMet = len(result.ValidApprovals)

	// Check for missing required approvers
	if len(policy.RequiredApprovers) > 0 {
		for _, required := range policy.RequiredApprovers {
			if !validApproverCircles[required] {
				result.MissingApprovers = append(result.MissingApprovers, required)
			}
		}
	}

	// Determine if verification passed
	if result.ThresholdMet >= result.ThresholdRequired {
		// Check if all required approvers have approved (if specified)
		if len(policy.RequiredApprovers) > 0 && len(result.MissingApprovers) > 0 {
			result.Passed = false
			result.Reason = fmt.Sprintf("missing approvals from required approvers: %v", result.MissingApprovers)
		} else {
			result.Passed = true
		}
	} else {
		result.Passed = false
		result.Reason = fmt.Sprintf("insufficient approvals: %d of %d required",
			result.ThresholdMet, result.ThresholdRequired)
	}

	// Emit audit event
	if s.auditStore != nil {
		outcome := "passed"
		if !result.Passed {
			outcome = fmt.Sprintf("failed: %s", result.Reason)
		}
		s.auditStore.Append(ctx, auditImpl.Entry{
			Type:           string(events.EventApprovalVerified),
			IntersectionID: req.Contract.IntersectionID,
			Action:         "approval_verified",
			Outcome:        outcome,
			TraceID:        req.TraceID,
		})
	}

	return result, nil
}

// Verify interface compliance at compile time.
var _ approval.Manager = (*Store)(nil)
