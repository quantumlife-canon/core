package approvaltoken

import (
	"testing"
	"time"

	"quantumlife/pkg/domain/identity"
)

func TestTokenDeterminism(t *testing.T) {
	now := time.Date(2025, 1, 15, 10, 0, 0, 0, time.UTC)
	expires := now.Add(60 * time.Minute)

	// Create two identical tokens
	t1 := NewToken("state-123", "person-satish", ActionTypeApprove, now, expires)
	t2 := NewToken("state-123", "person-satish", ActionTypeApprove, now, expires)

	if t1.TokenID != t2.TokenID {
		t.Errorf("token IDs should match: %s != %s", t1.TokenID, t2.TokenID)
	}

	if t1.Hash != t2.Hash {
		t.Errorf("hashes should match: %s != %s", t1.Hash, t2.Hash)
	}

	if t1.SignableString() != t2.SignableString() {
		t.Error("signable strings should match")
	}

	t.Logf("Token determinism verified: id=%s", t1.TokenID)
}

func TestTokenDifferentInputsDifferentIDs(t *testing.T) {
	now := time.Date(2025, 1, 15, 10, 0, 0, 0, time.UTC)
	expires := now.Add(60 * time.Minute)

	t1 := NewToken("state-1", "person-satish", ActionTypeApprove, now, expires)
	t2 := NewToken("state-2", "person-satish", ActionTypeApprove, now, expires)
	t3 := NewToken("state-1", "person-wife", ActionTypeApprove, now, expires)
	t4 := NewToken("state-1", "person-satish", ActionTypeReject, now, expires)

	ids := map[string]bool{
		t1.TokenID: true,
		t2.TokenID: true,
		t3.TokenID: true,
		t4.TokenID: true,
	}

	if len(ids) != 4 {
		t.Errorf("expected 4 unique IDs, got %d", len(ids))
	}
}

func TestTokenExpiry(t *testing.T) {
	now := time.Date(2025, 1, 15, 10, 0, 0, 0, time.UTC)
	expires := now.Add(30 * time.Minute)

	token := NewToken("state-1", "person-satish", ActionTypeApprove, now, expires)

	// Not expired yet
	if token.IsExpired(now.Add(20 * time.Minute)) {
		t.Error("token should not be expired at 20 min")
	}

	// Expired
	if !token.IsExpired(now.Add(31 * time.Minute)) {
		t.Error("token should be expired at 31 min")
	}
}

func TestTokenValidation(t *testing.T) {
	now := time.Date(2025, 1, 15, 10, 0, 0, 0, time.UTC)
	expires := now.Add(60 * time.Minute)

	// Valid token
	token := NewToken("state-1", "person-satish", ActionTypeApprove, now, expires)
	if err := token.IsValid(); err != nil {
		t.Errorf("valid token should not error: %v", err)
	}

	// Missing state ID
	badToken := &Token{
		TokenID:    "tok-1",
		PersonID:   "person-1",
		ActionType: ActionTypeApprove,
		CreatedAt:  now,
		ExpiresAt:  expires,
	}
	if err := badToken.IsValid(); err == nil {
		t.Error("expected error for missing state ID")
	}

	// Invalid action type
	badToken2 := &Token{
		TokenID:    "tok-1",
		StateID:    "state-1",
		PersonID:   "person-1",
		ActionType: "invalid",
		CreatedAt:  now,
		ExpiresAt:  expires,
	}
	if err := badToken2.IsValid(); err == nil {
		t.Error("expected error for invalid action type")
	}
}

func TestTokenSignature(t *testing.T) {
	now := time.Date(2025, 1, 15, 10, 0, 0, 0, time.UTC)
	expires := now.Add(60 * time.Minute)

	token := NewToken("state-1", "person-satish", ActionTypeApprove, now, expires)

	// Not signed initially
	if token.IsSigned() {
		t.Error("token should not be signed initially")
	}

	// Set signature
	token.SetSignature("Ed25519", "key-1", []byte("fakesig"))

	if !token.IsSigned() {
		t.Error("token should be signed after SetSignature")
	}

	if token.SignatureAlgorithm != "Ed25519" {
		t.Errorf("expected algorithm Ed25519, got %s", token.SignatureAlgorithm)
	}

	if token.KeyID != "key-1" {
		t.Errorf("expected key ID key-1, got %s", token.KeyID)
	}
}

func TestTokenEncodeDecodeRoundtrip(t *testing.T) {
	now := time.Date(2025, 1, 15, 10, 0, 0, 0, time.UTC)
	expires := now.Add(60 * time.Minute)

	original := NewToken("state-123", "person-satish", ActionTypeApprove, now, expires)
	original.SetSignature("Ed25519", "key-001", []byte("testsignature"))

	// Encode
	encoded := original.Encode()
	t.Logf("Encoded token: %s", encoded)

	// Decode
	decoded, err := Decode(encoded)
	if err != nil {
		t.Fatalf("decode failed: %v", err)
	}

	// Verify all fields match
	if decoded.TokenID != original.TokenID {
		t.Errorf("token ID mismatch: %s != %s", decoded.TokenID, original.TokenID)
	}
	if decoded.StateID != original.StateID {
		t.Errorf("state ID mismatch: %s != %s", decoded.StateID, original.StateID)
	}
	if decoded.PersonID != original.PersonID {
		t.Errorf("person ID mismatch: %s != %s", decoded.PersonID, original.PersonID)
	}
	if decoded.ActionType != original.ActionType {
		t.Errorf("action type mismatch: %s != %s", decoded.ActionType, original.ActionType)
	}
	if !decoded.CreatedAt.Equal(original.CreatedAt) {
		t.Errorf("created at mismatch: %v != %v", decoded.CreatedAt, original.CreatedAt)
	}
	if !decoded.ExpiresAt.Equal(original.ExpiresAt) {
		t.Errorf("expires at mismatch: %v != %v", decoded.ExpiresAt, original.ExpiresAt)
	}
	if decoded.SignatureAlgorithm != original.SignatureAlgorithm {
		t.Errorf("algorithm mismatch: %s != %s", decoded.SignatureAlgorithm, original.SignatureAlgorithm)
	}
	if decoded.KeyID != original.KeyID {
		t.Errorf("key ID mismatch: %s != %s", decoded.KeyID, original.KeyID)
	}
	if string(decoded.Signature) != string(original.Signature) {
		t.Error("signature mismatch")
	}
}

func TestTokenSetDeterminism(t *testing.T) {
	now := time.Date(2025, 1, 15, 10, 0, 0, 0, time.UTC)
	expires := now.Add(60 * time.Minute)

	// Create two identical sets
	set1 := NewTokenSet()
	set2 := NewTokenSet()

	t1 := NewToken("state-1", "person-1", ActionTypeApprove, now, expires)
	t2 := NewToken("state-1", "person-1", ActionTypeApprove, now, expires)

	set1.Add(t1)
	set2.Add(t2)

	if set1.Hash != set2.Hash {
		t.Errorf("set hashes should match: %s != %s", set1.Hash, set2.Hash)
	}

	t.Logf("TokenSet determinism verified: hash=%s", set1.Hash[:16])
}

func TestTokenSetListOrder(t *testing.T) {
	now := time.Date(2025, 1, 15, 10, 0, 0, 0, time.UTC)
	expires := now.Add(60 * time.Minute)

	set := NewTokenSet()

	// Add tokens that will have different IDs
	set.Add(NewToken("state-zzz", "person-1", ActionTypeApprove, now, expires))
	set.Add(NewToken("state-aaa", "person-1", ActionTypeApprove, now, expires))
	set.Add(NewToken("state-mmm", "person-1", ActionTypeApprove, now, expires))

	tokens := set.List()
	if len(tokens) != 3 {
		t.Fatalf("expected 3 tokens, got %d", len(tokens))
	}

	// Should be sorted by token ID
	for i := 0; i < len(tokens)-1; i++ {
		if tokens[i].TokenID > tokens[i+1].TokenID {
			t.Errorf("tokens not sorted: %s > %s", tokens[i].TokenID, tokens[i+1].TokenID)
		}
	}
}

func TestTokenSetGetByStateAndPerson(t *testing.T) {
	now := time.Date(2025, 1, 15, 10, 0, 0, 0, time.UTC)
	expires := now.Add(60 * time.Minute)

	set := NewTokenSet()

	// Add tokens for different states and people
	set.Add(NewToken("state-1", "person-satish", ActionTypeApprove, now, expires))
	set.Add(NewToken("state-1", "person-satish", ActionTypeReject, now, expires))
	set.Add(NewToken("state-1", "person-wife", ActionTypeApprove, now, expires))
	set.Add(NewToken("state-2", "person-satish", ActionTypeApprove, now, expires))

	// Get tokens for state-1 and person-satish
	tokens := set.GetByStateAndPerson("state-1", "person-satish")
	if len(tokens) != 2 {
		t.Errorf("expected 2 tokens for state-1/person-satish, got %d", len(tokens))
	}

	// Get tokens for state-1 and person-wife
	tokens = set.GetByStateAndPerson("state-1", "person-wife")
	if len(tokens) != 1 {
		t.Errorf("expected 1 token for state-1/person-wife, got %d", len(tokens))
	}
}

func TestTokenSetListActive(t *testing.T) {
	now := time.Date(2025, 1, 15, 10, 0, 0, 0, time.UTC)

	set := NewTokenSet()

	// Token with short expiry
	set.Add(NewToken("state-1", "person-1", ActionTypeApprove, now, now.Add(10*time.Minute)))
	// Token with longer expiry
	set.Add(NewToken("state-2", "person-1", ActionTypeApprove, now, now.Add(60*time.Minute)))

	// At 15 minutes, first token should be expired
	active := set.ListActive(now.Add(15 * time.Minute))
	if len(active) != 1 {
		t.Errorf("expected 1 active token at 15 min, got %d", len(active))
	}
}

func TestTokenSetPruneExpired(t *testing.T) {
	now := time.Date(2025, 1, 15, 10, 0, 0, 0, time.UTC)

	set := NewTokenSet()

	// Token with short expiry
	set.Add(NewToken("state-1", "person-1", ActionTypeApprove, now, now.Add(10*time.Minute)))
	// Token with longer expiry
	set.Add(NewToken("state-2", "person-1", ActionTypeApprove, now, now.Add(60*time.Minute)))

	// Prune at 15 minutes
	pruned := set.PruneExpired(now.Add(15 * time.Minute))
	if pruned != 1 {
		t.Errorf("expected 1 pruned, got %d", pruned)
	}

	if len(set.Tokens) != 1 {
		t.Errorf("expected 1 token remaining, got %d", len(set.Tokens))
	}
}

func TestTokenSetStats(t *testing.T) {
	now := time.Date(2025, 1, 15, 10, 0, 0, 0, time.UTC)

	set := NewTokenSet()

	// Add various tokens
	set.Add(NewToken("state-1", "person-1", ActionTypeApprove, now, now.Add(60*time.Minute)))
	set.Add(NewToken("state-2", "person-1", ActionTypeReject, now, now.Add(60*time.Minute)))
	set.Add(NewToken("state-3", "person-1", ActionTypeApprove, now, now.Add(10*time.Minute))) // Will expire

	stats := set.GetStats(now.Add(15 * time.Minute))

	if stats.TotalTokens != 3 {
		t.Errorf("expected 3 total, got %d", stats.TotalTokens)
	}
	if stats.ActiveCount != 2 {
		t.Errorf("expected 2 active, got %d", stats.ActiveCount)
	}
	if stats.ExpiredCount != 1 {
		t.Errorf("expected 1 expired, got %d", stats.ExpiredCount)
	}
	if stats.ApproveCount != 2 {
		t.Errorf("expected 2 approve tokens, got %d", stats.ApproveCount)
	}
	if stats.RejectCount != 1 {
		t.Errorf("expected 1 reject token, got %d", stats.RejectCount)
	}
}

func TestTokenActionTypes(t *testing.T) {
	now := time.Date(2025, 1, 15, 10, 0, 0, 0, time.UTC)
	expires := now.Add(60 * time.Minute)

	approve := NewToken("state-1", "person-1", ActionTypeApprove, now, expires)
	reject := NewToken("state-1", "person-1", ActionTypeReject, now, expires)

	if approve.ActionType != ActionTypeApprove {
		t.Errorf("expected approve, got %s", approve.ActionType)
	}

	if reject.ActionType != ActionTypeReject {
		t.Errorf("expected reject, got %s", reject.ActionType)
	}
}

func TestDecodeInvalidToken(t *testing.T) {
	// Invalid base64
	_, err := Decode("not-valid-base64!!!")
	if err == nil {
		t.Error("expected error for invalid base64")
	}

	// Wrong number of parts
	_, err = Decode("dG9vOmZldzpwYXJ0cw") // base64("too:few:parts")
	if err == nil {
		t.Error("expected error for wrong number of parts")
	}
}

func TestTokenSignableBytes(t *testing.T) {
	now := time.Date(2025, 1, 15, 10, 0, 0, 0, time.UTC)
	expires := now.Add(60 * time.Minute)

	token := NewToken("state-123", identity.EntityID("person-satish"), ActionTypeApprove, now, expires)

	signable := token.SignableBytes()
	if len(signable) == 0 {
		t.Error("signable bytes should not be empty")
	}

	// Should be deterministic
	signable2 := token.SignableBytes()
	if string(signable) != string(signable2) {
		t.Error("signable bytes should be deterministic")
	}

	// Should contain key fields
	str := string(signable)
	if !contains(str, "state-123") {
		t.Error("should contain state ID")
	}
	if !contains(str, "person-satish") {
		t.Error("should contain person ID")
	}
	if !contains(str, "approve") {
		t.Error("should contain action type")
	}
}

func contains(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
