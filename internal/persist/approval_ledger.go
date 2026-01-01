package persist

import (
	"fmt"
	"strings"
	"sync"
	"time"

	"quantumlife/pkg/domain/approvalflow"
	"quantumlife/pkg/domain/approvaltoken"
	"quantumlife/pkg/domain/identity"
	"quantumlife/pkg/domain/intersection"
	"quantumlife/pkg/domain/storelog"
)

// ApprovalLedger implements persistent storage for multi-party approvals.
//
// Phase 15: Household Approvals + Intersections
// This ledger stores intersection policies, approval states, and tokens.
// All operations are append-only and deterministic.
type ApprovalLedger struct {
	mu           sync.RWMutex
	log          storelog.AppendOnlyLog
	policies     *intersection.IntersectionPolicySet
	states       *approvalflow.ApprovalStateSet
	tokens       *approvaltoken.TokenSet
	tokensByHash map[string]*approvaltoken.Token // lookup by hash for revocation
}

// NewApprovalLedger creates a new file-backed approval ledger.
func NewApprovalLedger(log storelog.AppendOnlyLog) (*ApprovalLedger, error) {
	ledger := &ApprovalLedger{
		log:          log,
		policies:     intersection.NewIntersectionPolicySet(),
		states:       approvalflow.NewApprovalStateSet(),
		tokens:       approvaltoken.NewTokenSet(),
		tokensByHash: make(map[string]*approvaltoken.Token),
	}

	// Replay existing records
	if err := ledger.replay(); err != nil {
		return nil, err
	}

	return ledger, nil
}

// replay loads approval data from the log.
func (l *ApprovalLedger) replay() error {
	// Load intersection policies
	policyRecords, err := l.log.ListByType(storelog.RecordTypeIntersectionPolicy)
	if err != nil {
		return fmt.Errorf("list intersection policies: %w", err)
	}
	for _, record := range policyRecords {
		policy, err := parseIntersectionPolicyPayload(record.Payload)
		if err != nil {
			continue // Skip corrupted records
		}
		l.policies.Add(policy)
	}

	// Load approval state creates
	stateRecords, err := l.log.ListByType(storelog.RecordTypeApprovalStateCreate)
	if err != nil {
		return fmt.Errorf("list approval states: %w", err)
	}
	for _, record := range stateRecords {
		state, err := parseApprovalStatePayload(record.Payload)
		if err != nil {
			continue // Skip corrupted records
		}
		l.states.Add(state)
	}

	// Load approval records (decision records)
	approvalRecords, err := l.log.ListByType(storelog.RecordTypeApprovalStateRecord)
	if err != nil {
		return fmt.Errorf("list approval records: %w", err)
	}
	for _, record := range approvalRecords {
		stateID, approval, err := parseApprovalRecordPayload(record.Payload)
		if err != nil {
			continue // Skip corrupted records
		}
		if state := l.states.Get(stateID); state != nil {
			state.RecordApproval(approval)
		}
	}

	// Load token creates
	tokenRecords, err := l.log.ListByType(storelog.RecordTypeApprovalTokenCreate)
	if err != nil {
		return fmt.Errorf("list tokens: %w", err)
	}
	for _, record := range tokenRecords {
		token, err := parseTokenPayload(record.Payload)
		if err != nil {
			continue // Skip corrupted records
		}
		l.tokens.Add(token)
		l.tokensByHash[token.Hash] = token
	}

	// Load token revocations
	revokeRecords, err := l.log.ListByType(storelog.RecordTypeApprovalTokenRevoke)
	if err != nil {
		return fmt.Errorf("list token revocations: %w", err)
	}
	for _, record := range revokeRecords {
		tokenID := parseTokenRevokePayload(record.Payload)
		if tokenID != "" {
			l.tokens.Remove(tokenID)
			// Remove from hash lookup
			for hash, tok := range l.tokensByHash {
				if tok.TokenID == tokenID {
					delete(l.tokensByHash, hash)
					break
				}
			}
		}
	}

	return nil
}

// AddIntersectionPolicy adds a new intersection policy.
func (l *ApprovalLedger) AddIntersectionPolicy(policy *intersection.IntersectionPolicy) error {
	l.mu.Lock()
	defer l.mu.Unlock()

	// Create log record
	payload := formatIntersectionPolicyPayload(policy)
	logRecord := storelog.NewRecord(
		storelog.RecordTypeIntersectionPolicy,
		policy.CreatedAt,
		identity.EntityID(policy.IntersectionID),
		payload,
	)

	// Append to log
	if err := l.log.Append(logRecord); err != nil && err != storelog.ErrRecordExists {
		return err
	}

	l.policies.Add(policy)
	return nil
}

// GetIntersectionPolicy returns a policy by ID.
func (l *ApprovalLedger) GetIntersectionPolicy(id string) *intersection.IntersectionPolicy {
	l.mu.RLock()
	defer l.mu.RUnlock()
	return l.policies.Get(id)
}

// ListIntersectionPolicies returns all policies.
func (l *ApprovalLedger) ListIntersectionPolicies() []*intersection.IntersectionPolicy {
	l.mu.RLock()
	defer l.mu.RUnlock()
	return l.policies.List()
}

// FindPoliciesForPerson returns all policies where person is a member.
func (l *ApprovalLedger) FindPoliciesForPerson(personID string) []*intersection.IntersectionPolicy {
	l.mu.RLock()
	defer l.mu.RUnlock()
	return l.policies.FindPoliciesForPerson(personID)
}

// CreateApprovalState creates a new approval state.
func (l *ApprovalLedger) CreateApprovalState(state *approvalflow.ApprovalState) error {
	l.mu.Lock()
	defer l.mu.Unlock()

	// Create log record
	payload := formatApprovalStatePayload(state)
	logRecord := storelog.NewRecord(
		storelog.RecordTypeApprovalStateCreate,
		state.CreatedAt,
		identity.EntityID(state.IntersectionID),
		payload,
	)

	// Append to log
	if err := l.log.Append(logRecord); err != nil && err != storelog.ErrRecordExists {
		return err
	}

	l.states.Add(state)
	return nil
}

// RecordApproval records an approval decision.
func (l *ApprovalLedger) RecordApproval(stateID string, record approvalflow.ApprovalRecord) error {
	l.mu.Lock()
	defer l.mu.Unlock()

	state := l.states.Get(stateID)
	if state == nil {
		return fmt.Errorf("state not found: %s", stateID)
	}

	// Create log record
	payload := formatApprovalRecordPayload(stateID, record)
	logRecord := storelog.NewRecord(
		storelog.RecordTypeApprovalStateRecord,
		record.Timestamp,
		identity.EntityID(state.IntersectionID),
		payload,
	)

	// Append to log
	if err := l.log.Append(logRecord); err != nil && err != storelog.ErrRecordExists {
		return err
	}

	state.RecordApproval(record)
	return nil
}

// GetApprovalState returns a state by ID.
func (l *ApprovalLedger) GetApprovalState(stateID string) *approvalflow.ApprovalState {
	l.mu.RLock()
	defer l.mu.RUnlock()
	return l.states.Get(stateID)
}

// GetApprovalStateByTarget returns a state by target.
func (l *ApprovalLedger) GetApprovalStateByTarget(targetType approvalflow.TargetType, targetID string) *approvalflow.ApprovalState {
	l.mu.RLock()
	defer l.mu.RUnlock()
	return l.states.GetByTarget(targetType, targetID)
}

// ListApprovalStates returns all approval states.
func (l *ApprovalLedger) ListApprovalStates() []*approvalflow.ApprovalState {
	l.mu.RLock()
	defer l.mu.RUnlock()
	return l.states.List()
}

// ListPendingApprovals returns pending approvals.
func (l *ApprovalLedger) ListPendingApprovals(now time.Time) []*approvalflow.ApprovalState {
	l.mu.RLock()
	defer l.mu.RUnlock()
	return l.states.ListPending(now)
}

// ListPendingForPerson returns pending approvals for a person.
func (l *ApprovalLedger) ListPendingForPerson(personID identity.EntityID, now time.Time) []*approvalflow.ApprovalState {
	l.mu.RLock()
	defer l.mu.RUnlock()
	return l.states.ListPendingForPerson(personID, now)
}

// CreateToken creates and stores a new approval token.
func (l *ApprovalLedger) CreateToken(token *approvaltoken.Token) error {
	l.mu.Lock()
	defer l.mu.Unlock()

	// Create log record
	payload := formatTokenPayload(token)
	logRecord := storelog.NewRecord(
		storelog.RecordTypeApprovalTokenCreate,
		token.CreatedAt,
		token.PersonID,
		payload,
	)

	// Append to log
	if err := l.log.Append(logRecord); err != nil && err != storelog.ErrRecordExists {
		return err
	}

	l.tokens.Add(token)
	l.tokensByHash[token.Hash] = token
	return nil
}

// RevokeToken revokes a token.
func (l *ApprovalLedger) RevokeToken(tokenID string, revokedAt time.Time) error {
	l.mu.Lock()
	defer l.mu.Unlock()

	token := l.tokens.Get(tokenID)
	if token == nil {
		return fmt.Errorf("token not found: %s", tokenID)
	}

	// Create log record
	payload := formatTokenRevokePayload(tokenID, revokedAt)
	logRecord := storelog.NewRecord(
		storelog.RecordTypeApprovalTokenRevoke,
		revokedAt,
		token.PersonID,
		payload,
	)

	// Append to log
	if err := l.log.Append(logRecord); err != nil && err != storelog.ErrRecordExists {
		return err
	}

	l.tokens.Remove(tokenID)
	delete(l.tokensByHash, token.Hash)
	return nil
}

// GetToken returns a token by ID.
func (l *ApprovalLedger) GetToken(tokenID string) *approvaltoken.Token {
	l.mu.RLock()
	defer l.mu.RUnlock()
	return l.tokens.Get(tokenID)
}

// GetTokenByHash returns a token by its hash.
func (l *ApprovalLedger) GetTokenByHash(hash string) *approvaltoken.Token {
	l.mu.RLock()
	defer l.mu.RUnlock()
	return l.tokensByHash[hash]
}

// ListTokens returns all tokens.
func (l *ApprovalLedger) ListTokens() []*approvaltoken.Token {
	l.mu.RLock()
	defer l.mu.RUnlock()
	return l.tokens.List()
}

// ListActiveTokens returns non-expired tokens.
func (l *ApprovalLedger) ListActiveTokens(now time.Time) []*approvaltoken.Token {
	l.mu.RLock()
	defer l.mu.RUnlock()
	return l.tokens.ListActive(now)
}

// GetTokensForStateAndPerson returns tokens for a specific state and person.
func (l *ApprovalLedger) GetTokensForStateAndPerson(stateID string, personID identity.EntityID) []*approvaltoken.Token {
	l.mu.RLock()
	defer l.mu.RUnlock()
	return l.tokens.GetByStateAndPerson(stateID, personID)
}

// PruneExpired removes expired states and tokens.
func (l *ApprovalLedger) PruneExpired(now time.Time) (states, tokens int) {
	l.mu.Lock()
	defer l.mu.Unlock()
	states = l.states.PruneExpired(now)
	tokens = l.tokens.PruneExpired(now)
	return
}

// Stats returns ledger statistics.
func (l *ApprovalLedger) Stats(now time.Time) LedgerStats {
	l.mu.RLock()
	defer l.mu.RUnlock()

	approvalStats := l.states.GetStats(now)
	tokenStats := l.tokens.GetStats(now)

	return LedgerStats{
		PolicyCount:        len(l.policies.Policies),
		StateCount:         approvalStats.TotalStates,
		PendingStateCount:  approvalStats.PendingCount,
		ApprovedStateCount: approvalStats.ApprovedCount,
		RejectedStateCount: approvalStats.RejectedCount,
		ExpiredStateCount:  approvalStats.ExpiredCount,
		TokenCount:         tokenStats.TotalTokens,
		ActiveTokenCount:   tokenStats.ActiveCount,
	}
}

// LedgerStats holds ledger statistics.
type LedgerStats struct {
	PolicyCount        int
	StateCount         int
	PendingStateCount  int
	ApprovedStateCount int
	RejectedStateCount int
	ExpiredStateCount  int
	TokenCount         int
	ActiveTokenCount   int
}

// Flush ensures all records are persisted.
func (l *ApprovalLedger) Flush() error {
	return l.log.Flush()
}

// PolicySetHash returns the current policy set hash.
func (l *ApprovalLedger) PolicySetHash() string {
	l.mu.RLock()
	defer l.mu.RUnlock()
	return l.policies.Hash
}

// StateSetHash returns the current state set hash.
func (l *ApprovalLedger) StateSetHash() string {
	l.mu.RLock()
	defer l.mu.RUnlock()
	return l.states.Hash
}

// TokenSetHash returns the current token set hash.
func (l *ApprovalLedger) TokenSetHash() string {
	l.mu.RLock()
	defer l.mu.RUnlock()
	return l.tokens.Hash
}

// Payload formatting functions

func formatIntersectionPolicyPayload(p *intersection.IntersectionPolicy) string {
	return p.CanonicalString()
}

func parseIntersectionPolicyPayload(payload string) (*intersection.IntersectionPolicy, error) {
	// Parse the canonical string format
	// intersection|id:...|name:...|created:...|version:...|members:[...]|requirements:[...]

	policy := &intersection.IntersectionPolicy{
		Members:      []intersection.MemberRef{},
		Requirements: []intersection.ApprovalRequirement{},
	}

	parts := strings.Split(payload, "|")
	for _, part := range parts {
		if strings.HasPrefix(part, "id:") {
			policy.IntersectionID = part[3:]
		} else if strings.HasPrefix(part, "name:") {
			policy.Name = part[5:]
		} else if strings.HasPrefix(part, "created:") {
			t, _ := time.Parse(time.RFC3339, part[8:])
			policy.CreatedAt = t
		} else if strings.HasPrefix(part, "version:") {
			fmt.Sscanf(part[8:], "%d", &policy.Version)
		} else if strings.HasPrefix(part, "members:[") {
			// Parse members
			memberStr := part[9:]
			if strings.HasSuffix(memberStr, "]") {
				memberStr = memberStr[:len(memberStr)-1]
			}
			if memberStr != "" {
				memberParts := strings.Split(memberStr, ",")
				for _, mp := range memberParts {
					member := parseMemberRef(mp)
					if member.PersonID != "" {
						policy.Members = append(policy.Members, member)
					}
				}
			}
		} else if strings.HasPrefix(part, "requirements:[") {
			// Parse requirements
			reqStr := part[14:]
			if strings.HasSuffix(reqStr, "]") {
				reqStr = reqStr[:len(reqStr)-1]
			}
			// Requirements are more complex, skip detailed parsing for now
		}
	}

	policy.Hash = policy.ComputeHash()
	return policy, nil
}

func parseMemberRef(s string) intersection.MemberRef {
	var m intersection.MemberRef
	parts := strings.Split(s, "|")
	for _, part := range parts {
		if strings.HasPrefix(part, "person:") {
			m.PersonID = part[7:]
		} else if strings.HasPrefix(part, "role:") {
			m.Role = intersection.MemberRole(part[5:])
		}
	}
	return m
}

func formatApprovalStatePayload(s *approvalflow.ApprovalState) string {
	return s.CanonicalString()
}

func parseApprovalStatePayload(payload string) (*approvalflow.ApprovalState, error) {
	// Parse the canonical string format
	state := &approvalflow.ApprovalState{
		RequiredApprovers: []approvalflow.ApproverRef{},
		Approvals:         []approvalflow.ApprovalRecord{},
	}

	parts := strings.Split(payload, "|")
	for _, part := range parts {
		if strings.HasPrefix(part, "id:") {
			state.StateID = part[3:]
		} else if strings.HasPrefix(part, "target_type:") {
			state.TargetType = approvalflow.TargetType(part[12:])
		} else if strings.HasPrefix(part, "target_id:") {
			state.TargetID = part[10:]
		} else if strings.HasPrefix(part, "intersection:") {
			state.IntersectionID = part[13:]
		} else if strings.HasPrefix(part, "action:") {
			state.ActionClass = intersection.ActionClass(part[7:])
		} else if strings.HasPrefix(part, "threshold:") {
			fmt.Sscanf(part[10:], "%d", &state.Threshold)
		} else if strings.HasPrefix(part, "max_age:") {
			fmt.Sscanf(part[8:], "%d", &state.MaxAgeMinutes)
		} else if strings.HasPrefix(part, "created:") {
			t, _ := time.Parse(time.RFC3339, part[8:])
			state.CreatedAt = t
		} else if strings.HasPrefix(part, "expires:") {
			t, _ := time.Parse(time.RFC3339, part[8:])
			state.ExpiresAt = t
		} else if strings.HasPrefix(part, "version:") {
			fmt.Sscanf(part[8:], "%d", &state.Version)
		}
	}

	state.Hash = state.ComputeHash()
	return state, nil
}

func formatApprovalRecordPayload(stateID string, r approvalflow.ApprovalRecord) string {
	var b strings.Builder
	b.WriteString("approval_record")
	b.WriteString("|state_id:")
	b.WriteString(stateID)
	b.WriteString("|person_id:")
	b.WriteString(string(r.PersonID))
	b.WriteString("|decision:")
	b.WriteString(string(r.Decision))
	b.WriteString("|timestamp:")
	b.WriteString(r.Timestamp.UTC().Format(time.RFC3339))
	b.WriteString("|token_id:")
	b.WriteString(r.TokenID)
	b.WriteString("|reason:")
	b.WriteString(escapePayload(r.Reason))
	return b.String()
}

func parseApprovalRecordPayload(payload string) (string, approvalflow.ApprovalRecord, error) {
	var stateID string
	var r approvalflow.ApprovalRecord

	parts := strings.Split(payload, "|")
	for _, part := range parts {
		if strings.HasPrefix(part, "state_id:") {
			stateID = part[9:]
		} else if strings.HasPrefix(part, "person_id:") {
			r.PersonID = identity.EntityID(part[10:])
		} else if strings.HasPrefix(part, "decision:") {
			r.Decision = approvalflow.Decision(part[9:])
		} else if strings.HasPrefix(part, "timestamp:") {
			t, _ := time.Parse(time.RFC3339, part[10:])
			r.Timestamp = t
		} else if strings.HasPrefix(part, "token_id:") {
			r.TokenID = part[9:]
		} else if strings.HasPrefix(part, "reason:") {
			r.Reason = unescapePayload(part[7:])
		}
	}

	return stateID, r, nil
}

func formatTokenPayload(t *approvaltoken.Token) string {
	var b strings.Builder
	b.WriteString("approval_token")
	b.WriteString("|token_id:")
	b.WriteString(t.TokenID)
	b.WriteString("|state_id:")
	b.WriteString(t.StateID)
	b.WriteString("|person_id:")
	b.WriteString(string(t.PersonID))
	b.WriteString("|action:")
	b.WriteString(string(t.ActionType))
	b.WriteString("|created:")
	b.WriteString(t.CreatedAt.UTC().Format(time.RFC3339))
	b.WriteString("|expires:")
	b.WriteString(t.ExpiresAt.UTC().Format(time.RFC3339))
	b.WriteString("|alg:")
	b.WriteString(t.SignatureAlgorithm)
	b.WriteString("|key:")
	b.WriteString(t.KeyID)
	b.WriteString("|hash:")
	b.WriteString(t.Hash)
	return b.String()
}

func parseTokenPayload(payload string) (*approvaltoken.Token, error) {
	t := &approvaltoken.Token{}

	parts := strings.Split(payload, "|")
	for _, part := range parts {
		if strings.HasPrefix(part, "token_id:") {
			t.TokenID = part[9:]
		} else if strings.HasPrefix(part, "state_id:") {
			t.StateID = part[9:]
		} else if strings.HasPrefix(part, "person_id:") {
			t.PersonID = identity.EntityID(part[10:])
		} else if strings.HasPrefix(part, "action:") {
			t.ActionType = approvaltoken.ActionType(part[7:])
		} else if strings.HasPrefix(part, "created:") {
			ts, _ := time.Parse(time.RFC3339, part[8:])
			t.CreatedAt = ts
		} else if strings.HasPrefix(part, "expires:") {
			ts, _ := time.Parse(time.RFC3339, part[8:])
			t.ExpiresAt = ts
		} else if strings.HasPrefix(part, "alg:") {
			t.SignatureAlgorithm = part[4:]
		} else if strings.HasPrefix(part, "key:") {
			t.KeyID = part[4:]
		} else if strings.HasPrefix(part, "hash:") {
			t.Hash = part[5:]
		}
	}

	return t, nil
}

func formatTokenRevokePayload(tokenID string, revokedAt time.Time) string {
	var b strings.Builder
	b.WriteString("token_revoke")
	b.WriteString("|token_id:")
	b.WriteString(tokenID)
	b.WriteString("|revoked:")
	b.WriteString(revokedAt.UTC().Format(time.RFC3339))
	return b.String()
}

func parseTokenRevokePayload(payload string) string {
	parts := strings.Split(payload, "|")
	for _, part := range parts {
		if strings.HasPrefix(part, "token_id:") {
			return part[9:]
		}
	}
	return ""
}
