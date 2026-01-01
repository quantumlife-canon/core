package persist

import (
	"context"
	"encoding/hex"
	"sort"
	"strings"
	"sync"
	"time"

	"quantumlife/internal/approval"
	"quantumlife/pkg/domain/storelog"
	"quantumlife/pkg/primitives"
)

// ApprovalStore implements approval.Store with file-backed persistence.
type ApprovalStore struct {
	mu            sync.RWMutex
	log           storelog.AppendOnlyLog
	approvals     map[string]*primitives.ApprovalArtifact     // approvalID -> artifact
	requestTokens map[string]*primitives.ApprovalRequestToken // tokenID -> token

	// Index by intersection+action for efficient lookup
	byAction map[string][]*primitives.ApprovalArtifact // intersectionID:actionID -> approvals
}

// NewApprovalStore creates a new file-backed approval store.
func NewApprovalStore(log storelog.AppendOnlyLog) (*ApprovalStore, error) {
	store := &ApprovalStore{
		log:           log,
		approvals:     make(map[string]*primitives.ApprovalArtifact),
		requestTokens: make(map[string]*primitives.ApprovalRequestToken),
		byAction:      make(map[string][]*primitives.ApprovalArtifact),
	}

	// Replay existing records
	if err := store.replay(); err != nil {
		return nil, err
	}

	return store, nil
}

// replay loads approvals from the log.
func (s *ApprovalStore) replay() error {
	records, err := s.log.ListByType(storelog.RecordTypeApproval)
	if err != nil {
		return err
	}

	for _, record := range records {
		// Parse based on subtype in payload
		if strings.HasPrefix(record.Payload, "approval|type:artifact") {
			artifact, err := parseApprovalArtifact(record.Payload)
			if err != nil {
				continue
			}
			s.indexApproval(artifact)
		} else if strings.HasPrefix(record.Payload, "approval|type:request_token") {
			token, err := parseRequestToken(record.Payload)
			if err != nil {
				continue
			}
			s.requestTokens[token.TokenID] = token
		}
	}

	return nil
}

// indexApproval adds an approval to all indexes.
func (s *ApprovalStore) indexApproval(artifact *primitives.ApprovalArtifact) {
	s.approvals[artifact.ApprovalID] = artifact

	key := artifact.IntersectionID + ":" + artifact.ActionID
	s.byAction[key] = append(s.byAction[key], artifact)
}

// StoreApproval stores an approval artifact.
func (s *ApprovalStore) StoreApproval(ctx context.Context, artifact *primitives.ApprovalArtifact) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Check for duplicate
	key := artifact.IntersectionID + ":" + artifact.ActionID
	for _, existing := range s.byAction[key] {
		if existing.ApproverCircleID == artifact.ApproverCircleID {
			return approval.ErrDuplicateApproval
		}
	}

	// Create log record
	payload := formatApprovalArtifact(artifact)
	record := storelog.NewRecord(
		storelog.RecordTypeApproval,
		artifact.ApprovedAt,
		"", // No single circle for approvals
		payload,
	)

	// Append to log
	if err := s.log.Append(record); err != nil && err != storelog.ErrRecordExists {
		return err
	}

	s.indexApproval(artifact)
	return nil
}

// GetApprovals retrieves all approvals for an action.
func (s *ApprovalStore) GetApprovals(ctx context.Context, intersectionID, actionID string) ([]*primitives.ApprovalArtifact, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	key := intersectionID + ":" + actionID
	approvals := s.byAction[key]

	// Return copy sorted by approval time
	result := make([]*primitives.ApprovalArtifact, len(approvals))
	copy(result, approvals)
	sort.Slice(result, func(i, j int) bool {
		return result[i].ApprovedAt.Before(result[j].ApprovedAt)
	})

	return result, nil
}

// GetApprovalByID retrieves a specific approval by ID.
func (s *ApprovalStore) GetApprovalByID(ctx context.Context, approvalID string) (*primitives.ApprovalArtifact, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	artifact, exists := s.approvals[approvalID]
	if !exists {
		return nil, approval.ErrApprovalNotFound
	}
	return artifact, nil
}

// StoreRequestToken stores an approval request token.
func (s *ApprovalStore) StoreRequestToken(ctx context.Context, token *primitives.ApprovalRequestToken) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Create log record
	payload := formatRequestToken(token)
	record := storelog.NewRecord(
		storelog.RecordTypeApproval,
		token.CreatedAt,
		"",
		payload,
	)

	// Append to log
	if err := s.log.Append(record); err != nil && err != storelog.ErrRecordExists {
		return err
	}

	s.requestTokens[token.TokenID] = token
	return nil
}

// GetRequestToken retrieves a request token by ID.
func (s *ApprovalStore) GetRequestToken(ctx context.Context, tokenID string) (*primitives.ApprovalRequestToken, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	token, exists := s.requestTokens[tokenID]
	if !exists {
		return nil, approval.ErrRequestTokenNotFound
	}
	return token, nil
}

// DeleteExpiredApprovals removes expired approvals from memory (log is preserved).
func (s *ApprovalStore) DeleteExpiredApprovals(ctx context.Context, before time.Time) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	count := 0
	for id, artifact := range s.approvals {
		if artifact.ExpiresAt.Before(before) {
			delete(s.approvals, id)

			// Remove from byAction index
			key := artifact.IntersectionID + ":" + artifact.ActionID
			approvals := s.byAction[key]
			for i, a := range approvals {
				if a.ApprovalID == id {
					s.byAction[key] = append(approvals[:i], approvals[i+1:]...)
					break
				}
			}

			count++
		}
	}

	return count, nil
}

// Count returns the total number of approvals.
func (s *ApprovalStore) Count() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.approvals)
}

// RequestTokenCount returns the total number of request tokens.
func (s *ApprovalStore) RequestTokenCount() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.requestTokens)
}

// Flush ensures all records are persisted.
func (s *ApprovalStore) Flush() error {
	return s.log.Flush()
}

// Clear removes all entries from memory (log is preserved).
func (s *ApprovalStore) Clear() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.approvals = make(map[string]*primitives.ApprovalArtifact)
	s.requestTokens = make(map[string]*primitives.ApprovalRequestToken)
	s.byAction = make(map[string][]*primitives.ApprovalArtifact)
}

// formatApprovalArtifact creates a canonical payload for an approval artifact.
func formatApprovalArtifact(a *primitives.ApprovalArtifact) string {
	var b strings.Builder
	b.WriteString("approval|type:artifact")
	b.WriteString("|id:")
	b.WriteString(a.ApprovalID)
	b.WriteString("|intersection:")
	b.WriteString(a.IntersectionID)
	b.WriteString("|action:")
	b.WriteString(a.ActionID)
	b.WriteString("|action_hash:")
	b.WriteString(a.ActionHash)
	b.WriteString("|approver:")
	b.WriteString(a.ApproverCircleID)
	b.WriteString("|approved:")
	b.WriteString(a.ApprovedAt.UTC().Format(time.RFC3339))
	b.WriteString("|expires:")
	b.WriteString(a.ExpiresAt.UTC().Format(time.RFC3339))
	b.WriteString("|sig:")
	b.WriteString(encodeBytes(a.Signature))
	return b.String()
}

// parseApprovalArtifact parses a canonical payload into an approval artifact.
func parseApprovalArtifact(payload string) (*primitives.ApprovalArtifact, error) {
	a := &primitives.ApprovalArtifact{}

	parts := strings.Split(payload, "|")
	for _, part := range parts {
		if strings.HasPrefix(part, "id:") {
			a.ApprovalID = part[3:]
		} else if strings.HasPrefix(part, "intersection:") {
			a.IntersectionID = part[13:]
		} else if strings.HasPrefix(part, "action:") {
			a.ActionID = part[7:]
		} else if strings.HasPrefix(part, "action_hash:") {
			a.ActionHash = part[12:]
		} else if strings.HasPrefix(part, "approver:") {
			a.ApproverCircleID = part[9:]
		} else if strings.HasPrefix(part, "approved:") {
			t, _ := time.Parse(time.RFC3339, part[9:])
			a.ApprovedAt = t
		} else if strings.HasPrefix(part, "expires:") {
			t, _ := time.Parse(time.RFC3339, part[8:])
			a.ExpiresAt = t
		} else if strings.HasPrefix(part, "sig:") {
			a.Signature = decodeBytes(part[4:])
		}
	}

	return a, nil
}

// formatRequestToken creates a canonical payload for a request token.
func formatRequestToken(t *primitives.ApprovalRequestToken) string {
	var b strings.Builder
	b.WriteString("approval|type:request_token")
	b.WriteString("|id:")
	b.WriteString(t.TokenID)
	b.WriteString("|intersection:")
	b.WriteString(t.IntersectionID)
	b.WriteString("|action:")
	b.WriteString(t.ActionID)
	b.WriteString("|action_hash:")
	b.WriteString(t.ActionHash)
	b.WriteString("|requester:")
	b.WriteString(t.RequestingCircleID)
	b.WriteString("|created:")
	b.WriteString(t.CreatedAt.UTC().Format(time.RFC3339))
	b.WriteString("|expires:")
	b.WriteString(t.ExpiresAt.UTC().Format(time.RFC3339))
	return b.String()
}

// parseRequestToken parses a canonical payload into a request token.
func parseRequestToken(payload string) (*primitives.ApprovalRequestToken, error) {
	token := &primitives.ApprovalRequestToken{}

	parts := strings.Split(payload, "|")
	for _, part := range parts {
		if strings.HasPrefix(part, "id:") {
			token.TokenID = part[3:]
		} else if strings.HasPrefix(part, "intersection:") {
			token.IntersectionID = part[13:]
		} else if strings.HasPrefix(part, "action:") {
			token.ActionID = part[7:]
		} else if strings.HasPrefix(part, "action_hash:") {
			token.ActionHash = part[12:]
		} else if strings.HasPrefix(part, "requester:") {
			token.RequestingCircleID = part[10:]
		} else if strings.HasPrefix(part, "created:") {
			ts, _ := time.Parse(time.RFC3339, part[8:])
			token.CreatedAt = ts
		} else if strings.HasPrefix(part, "expires:") {
			ts, _ := time.Parse(time.RFC3339, part[8:])
			token.ExpiresAt = ts
		}
	}

	return token, nil
}

// encodeBytes encodes bytes to hex string.
func encodeBytes(b []byte) string {
	return hex.EncodeToString(b)
}

// decodeBytes decodes hex string to bytes.
func decodeBytes(s string) []byte {
	b, _ := hex.DecodeString(s)
	return b
}

// Verify interface compliance.
var _ approval.Store = (*ApprovalStore)(nil)
