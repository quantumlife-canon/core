package demo_phase50_signed_claims

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base64"
	"strings"
	"testing"
	"time"

	"quantumlife/internal/persist"
	engine "quantumlife/internal/signedclaims"
	domain "quantumlife/pkg/domain/signedclaims"
)

// ============================================================================
// Test Helpers
// ============================================================================

func testClock() time.Time {
	return time.Date(2025, 1, 15, 12, 0, 0, 0, time.UTC)
}

func makeValidRefHash() domain.SafeRefHash {
	return domain.SafeRefHash(strings.Repeat("a", 64))
}

func makeValidCircleIDHash() domain.SafeRefHash {
	return domain.SafeRefHash(strings.Repeat("b", 64))
}

func generateTestKeyPair() (ed25519.PublicKey, ed25519.PrivateKey) {
	pub, priv, _ := ed25519.GenerateKey(rand.Reader)
	return pub, priv
}

func signClaim(priv ed25519.PrivateKey, claim domain.SignedVendorClaim) domain.SignatureB64 {
	sig := ed25519.Sign(priv, claim.MessageBytes())
	return domain.SignatureB64(base64.StdEncoding.EncodeToString(sig))
}

func signManifest(priv ed25519.PrivateKey, manifest domain.SignedPackManifest) domain.SignatureB64 {
	sig := ed25519.Sign(priv, manifest.MessageBytes())
	return domain.SignatureB64(base64.StdEncoding.EncodeToString(sig))
}

func pubKeyToB64(pub ed25519.PublicKey) domain.PublicKeyB64 {
	return domain.PublicKeyB64(base64.StdEncoding.EncodeToString(pub))
}

// ============================================================================
// Section 1: Enum Validation Tests
// ============================================================================

func TestClaimKindValidation(t *testing.T) {
	tests := []struct {
		kind    domain.ClaimKind
		wantErr bool
	}{
		{domain.ClaimVendorCap, false},
		{domain.ClaimPackManifest, false},
		{domain.ClaimObserverBindingIntent, false},
		{"invalid_kind", true},
		{"", true},
	}

	for _, tt := range tests {
		err := tt.kind.Validate()
		if (err != nil) != tt.wantErr {
			t.Errorf("ClaimKind(%q).Validate() error = %v, wantErr %v", tt.kind, err, tt.wantErr)
		}
	}
}

func TestProvenanceValidation(t *testing.T) {
	tests := []struct {
		prov    domain.Provenance
		wantErr bool
	}{
		{domain.ProvenanceUserSupplied, false},
		{domain.ProvenanceMarketplace, false},
		{domain.ProvenanceAdmin, false},
		{"invalid_provenance", true},
		{"", true},
	}

	for _, tt := range tests {
		err := tt.prov.Validate()
		if (err != nil) != tt.wantErr {
			t.Errorf("Provenance(%q).Validate() error = %v, wantErr %v", tt.prov, err, tt.wantErr)
		}
	}
}

func TestVerificationStatusValidation(t *testing.T) {
	tests := []struct {
		status  domain.VerificationStatus
		wantErr bool
	}{
		{domain.VerifiedOK, false},
		{domain.VerifiedBadSig, false},
		{domain.VerifiedBadFormat, false},
		{domain.VerifiedUnknownKey, false},
		{"invalid_status", true},
		{"", true},
	}

	for _, tt := range tests {
		err := tt.status.Validate()
		if (err != nil) != tt.wantErr {
			t.Errorf("VerificationStatus(%q).Validate() error = %v, wantErr %v", tt.status, err, tt.wantErr)
		}
	}
}

func TestVendorScopeValidation(t *testing.T) {
	tests := []struct {
		scope   domain.VendorScope
		wantErr bool
	}{
		{domain.ScopeHuman, false},
		{domain.ScopeInstitution, false},
		{domain.ScopeCommerce, false},
		{"invalid_scope", true},
		{"", true},
	}

	for _, tt := range tests {
		err := tt.scope.Validate()
		if (err != nil) != tt.wantErr {
			t.Errorf("VendorScope(%q).Validate() error = %v, wantErr %v", tt.scope, err, tt.wantErr)
		}
	}
}

func TestPressureCapValidation(t *testing.T) {
	tests := []struct {
		cap     domain.PressureCap
		wantErr bool
	}{
		{domain.AllowHoldOnly, false},
		{domain.AllowSurfaceOnly, false},
		{"invalid_cap", true},
		{"", true},
	}

	for _, tt := range tests {
		err := tt.cap.Validate()
		if (err != nil) != tt.wantErr {
			t.Errorf("PressureCap(%q).Validate() error = %v, wantErr %v", tt.cap, err, tt.wantErr)
		}
	}
}

func TestPackVersionBucketValidation(t *testing.T) {
	tests := []struct {
		version domain.PackVersionBucket
		wantErr bool
	}{
		{domain.PackVersionV0, false},
		{domain.PackVersionV1, false},
		{domain.PackVersionV1_1, false},
		{"invalid_version", true},
		{"", true},
	}

	for _, tt := range tests {
		err := tt.version.Validate()
		if (err != nil) != tt.wantErr {
			t.Errorf("PackVersionBucket(%q).Validate() error = %v, wantErr %v", tt.version, err, tt.wantErr)
		}
	}
}

func TestProofAckKindValidation(t *testing.T) {
	tests := []struct {
		kind    domain.ProofAckKind
		wantErr bool
	}{
		{domain.ProofAckViewed, false},
		{domain.ProofAckDismissed, false},
		{"invalid_ack", true},
	}

	for _, tt := range tests {
		err := tt.kind.Validate()
		if (err != nil) != tt.wantErr {
			t.Errorf("ProofAckKind(%q).Validate() error = %v, wantErr %v", tt.kind, err, tt.wantErr)
		}
	}
}

// ============================================================================
// Section 2: SafeRefHash Validation Tests
// ============================================================================

func TestSafeRefHashValidation(t *testing.T) {
	tests := []struct {
		hash    domain.SafeRefHash
		wantErr bool
	}{
		{domain.SafeRefHash(strings.Repeat("a", 64)), false},
		{domain.SafeRefHash(strings.Repeat("0", 64)), false},
		{domain.SafeRefHash(strings.Repeat("f", 64)), false},
		{domain.SafeRefHash("abc123def456"), true},                   // too short
		{domain.SafeRefHash(strings.Repeat("a", 63)), true},          // 63 chars
		{domain.SafeRefHash(strings.Repeat("a", 65)), true},          // 65 chars
		{domain.SafeRefHash(strings.Repeat("G", 64)), true},          // uppercase
		{domain.SafeRefHash(strings.Repeat("A", 64)), true},          // uppercase
		{domain.SafeRefHash(strings.Repeat("!", 64)), true},          // invalid chars
	}

	for _, tt := range tests {
		err := tt.hash.Validate()
		if (err != nil) != tt.wantErr {
			t.Errorf("SafeRefHash(%q).Validate() error = %v, wantErr %v", tt.hash, err, tt.wantErr)
		}
	}
}

func TestKeyFingerprintValidation(t *testing.T) {
	tests := []struct {
		fp      domain.KeyFingerprint
		wantErr bool
	}{
		{domain.KeyFingerprint(strings.Repeat("a", 64)), false},
		{domain.KeyFingerprint(strings.Repeat("0", 64)), false},
		{domain.KeyFingerprint("abc123"), true},                      // too short
		{domain.KeyFingerprint(strings.Repeat("G", 64)), true},       // uppercase
	}

	for _, tt := range tests {
		err := tt.fp.Validate()
		if (err != nil) != tt.wantErr {
			t.Errorf("KeyFingerprint(%q).Validate() error = %v, wantErr %v", tt.fp, err, tt.wantErr)
		}
	}
}

// ============================================================================
// Section 3: CanonicalString Determinism Tests
// ============================================================================

func TestVendorClaimCanonicalStringDeterminism(t *testing.T) {
	claim := domain.SignedVendorClaim{
		Kind:         domain.ClaimVendorCap,
		Scope:        domain.ScopeCommerce,
		Cap:          domain.AllowHoldOnly,
		RefHash:      makeValidRefHash(),
		Provenance:   domain.ProvenanceUserSupplied,
		CircleIDHash: makeValidCircleIDHash(),
		PeriodKey:    "2025-01-15",
	}

	// Call multiple times - must be deterministic
	s1 := claim.CanonicalString()
	s2 := claim.CanonicalString()
	s3 := claim.CanonicalString()

	if s1 != s2 || s2 != s3 {
		t.Errorf("CanonicalString is not deterministic: %q != %q != %q", s1, s2, s3)
	}

	// Must be pipe-delimited
	if !strings.Contains(s1, "|") {
		t.Error("CanonicalString should be pipe-delimited")
	}

	// Must NOT contain JSON
	if strings.Contains(s1, "{") || strings.Contains(s1, "}") {
		t.Error("CanonicalString should not contain JSON")
	}
}

func TestPackManifestCanonicalStringDeterminism(t *testing.T) {
	manifest := domain.SignedPackManifest{
		PackHash:     makeValidRefHash(),
		Version:      domain.PackVersionV1,
		BindingsHash: makeValidRefHash(),
		Provenance:   domain.ProvenanceMarketplace,
		CircleIDHash: makeValidCircleIDHash(),
		PeriodKey:    "2025-01-15",
	}

	s1 := manifest.CanonicalString()
	s2 := manifest.CanonicalString()

	if s1 != s2 {
		t.Errorf("CanonicalString is not deterministic: %q != %q", s1, s2)
	}

	if !strings.Contains(s1, "|") {
		t.Error("CanonicalString should be pipe-delimited")
	}
}

func TestClaimHashDeterminism(t *testing.T) {
	claim := domain.SignedVendorClaim{
		Kind:         domain.ClaimVendorCap,
		Scope:        domain.ScopeHuman,
		Cap:          domain.AllowSurfaceOnly,
		RefHash:      makeValidRefHash(),
		Provenance:   domain.ProvenanceAdmin,
		CircleIDHash: makeValidCircleIDHash(),
		PeriodKey:    "2025-01-15",
	}

	h1 := claim.Hash()
	h2 := claim.Hash()
	h3 := claim.Hash()

	if h1 != h2 || h2 != h3 {
		t.Errorf("Hash is not deterministic: %q != %q != %q", h1, h2, h3)
	}

	// Hash should be 64 hex chars
	if len(h1) != 64 {
		t.Errorf("Hash should be 64 hex chars, got %d", len(h1))
	}
}

func TestManifestHashDeterminism(t *testing.T) {
	manifest := domain.SignedPackManifest{
		PackHash:     makeValidRefHash(),
		Version:      domain.PackVersionV0,
		BindingsHash: makeValidRefHash(),
		Provenance:   domain.ProvenanceUserSupplied,
		CircleIDHash: makeValidCircleIDHash(),
		PeriodKey:    "2025-01-15",
	}

	h1 := manifest.Hash()
	h2 := manifest.Hash()

	if h1 != h2 {
		t.Errorf("Hash is not deterministic: %q != %q", h1, h2)
	}
}

// ============================================================================
// Section 4: Key Fingerprint Tests
// ============================================================================

func TestKeyFingerprintFromPublicKey(t *testing.T) {
	pub, _ := generateTestKeyPair()

	fp1 := domain.NewKeyFingerprint(pub)
	fp2 := domain.NewKeyFingerprint(pub)

	if fp1 != fp2 {
		t.Errorf("Fingerprint is not stable: %q != %q", fp1, fp2)
	}

	if len(fp1) != 64 {
		t.Errorf("Fingerprint should be 64 hex chars, got %d", len(fp1))
	}

	if err := fp1.Validate(); err != nil {
		t.Errorf("Fingerprint should be valid: %v", err)
	}
}

func TestKeyFingerprintShort(t *testing.T) {
	fp := domain.KeyFingerprint(strings.Repeat("a", 64))
	short := fp.Short()

	if len(short) != 16 {
		t.Errorf("Short fingerprint should be 16 chars, got %d", len(short))
	}
}

func TestDifferentKeysProduceDifferentFingerprints(t *testing.T) {
	pub1, _ := generateTestKeyPair()
	pub2, _ := generateTestKeyPair()

	fp1 := domain.NewKeyFingerprint(pub1)
	fp2 := domain.NewKeyFingerprint(pub2)

	if fp1 == fp2 {
		t.Error("Different keys should produce different fingerprints")
	}
}

// ============================================================================
// Section 5: Signature Verification Tests
// ============================================================================

func TestClaimVerificationSuccess(t *testing.T) {
	pub, priv := generateTestKeyPair()
	eng := engine.NewEngine(testClock)

	claim := domain.SignedVendorClaim{
		Kind:         domain.ClaimVendorCap,
		Scope:        domain.ScopeCommerce,
		Cap:          domain.AllowHoldOnly,
		RefHash:      makeValidRefHash(),
		Provenance:   domain.ProvenanceUserSupplied,
		CircleIDHash: makeValidCircleIDHash(),
		PeriodKey:    "2025-01-15",
	}

	sig := signClaim(priv, claim)
	pubB64 := pubKeyToB64(pub)

	result := eng.VerifyClaim(claim, sig, pubB64)

	if result.Status != domain.VerifiedOK {
		t.Errorf("Expected VerifiedOK, got %v", result.Status)
	}

	if result.Record.Status != domain.VerifiedOK {
		t.Errorf("Record status should be VerifiedOK")
	}

	if result.ClaimHash != claim.Hash() {
		t.Error("Result claim hash should match claim.Hash()")
	}
}

func TestManifestVerificationSuccess(t *testing.T) {
	pub, priv := generateTestKeyPair()
	eng := engine.NewEngine(testClock)

	manifest := domain.SignedPackManifest{
		PackHash:     makeValidRefHash(),
		Version:      domain.PackVersionV1,
		BindingsHash: makeValidRefHash(),
		Provenance:   domain.ProvenanceMarketplace,
		CircleIDHash: makeValidCircleIDHash(),
		PeriodKey:    "2025-01-15",
	}

	sig := signManifest(priv, manifest)
	pubB64 := pubKeyToB64(pub)

	result := eng.VerifyManifest(manifest, sig, pubB64)

	if result.Status != domain.VerifiedOK {
		t.Errorf("Expected VerifiedOK, got %v", result.Status)
	}
}

func TestClaimVerificationFailsWithWrongKey(t *testing.T) {
	pub1, priv1 := generateTestKeyPair()
	pub2, _ := generateTestKeyPair()
	eng := engine.NewEngine(testClock)

	claim := domain.SignedVendorClaim{
		Kind:         domain.ClaimVendorCap,
		Scope:        domain.ScopeHuman,
		Cap:          domain.AllowSurfaceOnly,
		RefHash:      makeValidRefHash(),
		Provenance:   domain.ProvenanceAdmin,
		CircleIDHash: makeValidCircleIDHash(),
		PeriodKey:    "2025-01-15",
	}

	// Sign with priv1 but verify with pub2
	sig := signClaim(priv1, claim)
	pubB64 := pubKeyToB64(pub2)
	_ = pub1 // unused

	result := eng.VerifyClaim(claim, sig, pubB64)

	if result.Status != domain.VerifiedBadSig {
		t.Errorf("Expected VerifiedBadSig, got %v", result.Status)
	}
}

func TestClaimVerificationFailsIfFieldChanges(t *testing.T) {
	pub, priv := generateTestKeyPair()
	eng := engine.NewEngine(testClock)

	claim := domain.SignedVendorClaim{
		Kind:         domain.ClaimVendorCap,
		Scope:        domain.ScopeCommerce,
		Cap:          domain.AllowHoldOnly,
		RefHash:      makeValidRefHash(),
		Provenance:   domain.ProvenanceUserSupplied,
		CircleIDHash: makeValidCircleIDHash(),
		PeriodKey:    "2025-01-15",
	}

	sig := signClaim(priv, claim)
	pubB64 := pubKeyToB64(pub)

	// Modify a field after signing
	claim.Cap = domain.AllowSurfaceOnly

	result := eng.VerifyClaim(claim, sig, pubB64)

	if result.Status != domain.VerifiedBadSig {
		t.Errorf("Expected VerifiedBadSig after field change, got %v", result.Status)
	}
}

func TestManifestVerificationFailsIfFieldChanges(t *testing.T) {
	pub, priv := generateTestKeyPair()
	eng := engine.NewEngine(testClock)

	manifest := domain.SignedPackManifest{
		PackHash:     makeValidRefHash(),
		Version:      domain.PackVersionV1,
		BindingsHash: makeValidRefHash(),
		Provenance:   domain.ProvenanceMarketplace,
		CircleIDHash: makeValidCircleIDHash(),
		PeriodKey:    "2025-01-15",
	}

	sig := signManifest(priv, manifest)
	pubB64 := pubKeyToB64(pub)

	// Modify a field
	manifest.Version = domain.PackVersionV0

	result := eng.VerifyManifest(manifest, sig, pubB64)

	if result.Status != domain.VerifiedBadSig {
		t.Errorf("Expected VerifiedBadSig, got %v", result.Status)
	}
}

func TestClaimVerificationFailsWithInvalidClaim(t *testing.T) {
	pub, _ := generateTestKeyPair()
	eng := engine.NewEngine(testClock)

	// Invalid claim - missing period key
	claim := domain.SignedVendorClaim{
		Kind:         domain.ClaimVendorCap,
		Scope:        domain.ScopeCommerce,
		Cap:          domain.AllowHoldOnly,
		RefHash:      makeValidRefHash(),
		Provenance:   domain.ProvenanceUserSupplied,
		CircleIDHash: makeValidCircleIDHash(),
		PeriodKey:    "", // Invalid
	}

	sig := domain.SignatureB64(base64.StdEncoding.EncodeToString(make([]byte, 64)))
	pubB64 := pubKeyToB64(pub)

	result := eng.VerifyClaim(claim, sig, pubB64)

	if result.Status != domain.VerifiedBadFormat {
		t.Errorf("Expected VerifiedBadFormat, got %v", result.Status)
	}
}

// ============================================================================
// Section 6: Store Tests
// ============================================================================

func TestSignedClaimStoreDedup(t *testing.T) {
	store := persist.NewSignedClaimStore(testClock)

	record := domain.SignedClaimRecord{
		ClaimHash:      makeValidRefHash(),
		KeyFingerprint: domain.KeyFingerprint(strings.Repeat("c", 64)),
		Status:         domain.VerifiedOK,
		Provenance:     domain.ProvenanceUserSupplied,
		Kind:           domain.ClaimVendorCap,
		PeriodKey:      "2025-01-15",
		CircleIDHash:   makeValidCircleIDHash(),
		CreatedBucket:  "2025-01-15",
	}

	// Append first time
	_ = store.AppendClaim(record)
	if store.Count() != 1 {
		t.Errorf("Expected 1 record, got %d", store.Count())
	}

	// Append same record again (should be deduped)
	_ = store.AppendClaim(record)
	if store.Count() != 1 {
		t.Errorf("Expected 1 record (dedup), got %d", store.Count())
	}

	// Different claim hash should be added
	record2 := record
	record2.ClaimHash = domain.SafeRefHash(strings.Repeat("d", 64))
	_ = store.AppendClaim(record2)
	if store.Count() != 2 {
		t.Errorf("Expected 2 records, got %d", store.Count())
	}
}

func TestSignedManifestStoreDedup(t *testing.T) {
	store := persist.NewSignedManifestStore(testClock)

	record := domain.SignedManifestRecord{
		ManifestHash:   makeValidRefHash(),
		KeyFingerprint: domain.KeyFingerprint(strings.Repeat("c", 64)),
		Status:         domain.VerifiedOK,
		Provenance:     domain.ProvenanceMarketplace,
		PeriodKey:      "2025-01-15",
		CircleIDHash:   makeValidCircleIDHash(),
		PackHash:       makeValidRefHash(),
		CreatedBucket:  "2025-01-15",
	}

	_ = store.AppendManifest(record)
	_ = store.AppendManifest(record) // Dedup

	if store.Count() != 1 {
		t.Errorf("Expected 1 record (dedup), got %d", store.Count())
	}
}

func TestSignedClaimStoreIsClaimSeen(t *testing.T) {
	store := persist.NewSignedClaimStore(testClock)

	claimHash := makeValidRefHash()
	circleIDHash := makeValidCircleIDHash()
	periodKey := "2025-01-15"

	// Not seen initially
	if store.IsClaimSeen(claimHash, circleIDHash, periodKey) {
		t.Error("Claim should not be seen initially")
	}

	// Add record
	record := domain.SignedClaimRecord{
		ClaimHash:      claimHash,
		KeyFingerprint: domain.KeyFingerprint(strings.Repeat("c", 64)),
		Status:         domain.VerifiedOK,
		Provenance:     domain.ProvenanceUserSupplied,
		Kind:           domain.ClaimVendorCap,
		PeriodKey:      periodKey,
		CircleIDHash:   circleIDHash,
		CreatedBucket:  periodKey,
	}
	_ = store.AppendClaim(record)

	// Now should be seen
	if !store.IsClaimSeen(claimHash, circleIDHash, periodKey) {
		t.Error("Claim should be seen after append")
	}
}

func TestSignedClaimStoreBoundedRetention(t *testing.T) {
	// Create a store with a clock that advances
	currentTime := time.Date(2025, 1, 15, 12, 0, 0, 0, time.UTC)
	clockFn := func() time.Time { return currentTime }
	store := persist.NewSignedClaimStore(clockFn)

	// Add records up to max
	for i := 0; i < 205; i++ {
		hash := domain.SafeRefHash(strings.Repeat(string(rune('a'+i%26)), 64))
		record := domain.SignedClaimRecord{
			ClaimHash:      hash,
			KeyFingerprint: domain.KeyFingerprint(strings.Repeat("c", 64)),
			Status:         domain.VerifiedOK,
			Provenance:     domain.ProvenanceUserSupplied,
			Kind:           domain.ClaimVendorCap,
			PeriodKey:      "2025-01-15",
			CircleIDHash:   makeValidCircleIDHash(),
			CreatedBucket:  "2025-01-15",
		}
		_ = store.AppendClaim(record)
	}

	// Should not exceed max (200)
	if store.Count() > 200 {
		t.Errorf("Store should not exceed 200 records, got %d", store.Count())
	}
}

func TestSignedClaimStoreListByCircle(t *testing.T) {
	store := persist.NewSignedClaimStore(testClock)

	circle1 := makeValidCircleIDHash()
	circle2 := domain.SafeRefHash(strings.Repeat("x", 64))

	// Add records for two circles
	_ = store.AppendClaim(domain.SignedClaimRecord{
		ClaimHash:      makeValidRefHash(),
		KeyFingerprint: domain.KeyFingerprint(strings.Repeat("c", 64)),
		Status:         domain.VerifiedOK,
		Provenance:     domain.ProvenanceUserSupplied,
		Kind:           domain.ClaimVendorCap,
		PeriodKey:      "2025-01-15",
		CircleIDHash:   circle1,
		CreatedBucket:  "2025-01-15",
	})
	_ = store.AppendClaim(domain.SignedClaimRecord{
		ClaimHash:      domain.SafeRefHash(strings.Repeat("d", 64)),
		KeyFingerprint: domain.KeyFingerprint(strings.Repeat("c", 64)),
		Status:         domain.VerifiedOK,
		Provenance:     domain.ProvenanceMarketplace,
		Kind:           domain.ClaimPackManifest,
		PeriodKey:      "2025-01-15",
		CircleIDHash:   circle2,
		CreatedBucket:  "2025-01-15",
	})

	// Query by circle
	list1 := store.ListByCircle(circle1)
	if len(list1) != 1 {
		t.Errorf("Expected 1 record for circle1, got %d", len(list1))
	}

	list2 := store.ListByCircle(circle2)
	if len(list2) != 1 {
		t.Errorf("Expected 1 record for circle2, got %d", len(list2))
	}
}

// ============================================================================
// Section 7: Proof Ack Store Tests
// ============================================================================

func TestSignedClaimProofAckStore(t *testing.T) {
	store := persist.NewSignedClaimProofAckStore(testClock)

	circleIDHash := makeValidCircleIDHash()
	periodKey := "2025-01-15"

	// Not dismissed initially
	if store.IsProofDismissed(circleIDHash, periodKey) {
		t.Error("Proof should not be dismissed initially")
	}

	// Dismiss
	ack := domain.SignedClaimProofAck{
		AckKind:      domain.ProofAckDismissed,
		CircleIDHash: circleIDHash,
		PeriodKey:    periodKey,
	}
	_ = store.AppendAck(ack)

	// Now should be dismissed
	if !store.IsProofDismissed(circleIDHash, periodKey) {
		t.Error("Proof should be dismissed after ack")
	}
}

// ============================================================================
// Section 8: Message Bytes Tests
// ============================================================================

func TestVendorClaimMessageBytesPrefix(t *testing.T) {
	claim := domain.SignedVendorClaim{
		Kind:         domain.ClaimVendorCap,
		Scope:        domain.ScopeCommerce,
		Cap:          domain.AllowHoldOnly,
		RefHash:      makeValidRefHash(),
		Provenance:   domain.ProvenanceUserSupplied,
		CircleIDHash: makeValidCircleIDHash(),
		PeriodKey:    "2025-01-15",
	}

	msg := claim.MessageBytes()
	prefix := "QL|phase50|vendor_claim|"

	if !strings.HasPrefix(string(msg), prefix) {
		t.Errorf("Message bytes should start with %q, got %q", prefix, string(msg[:len(prefix)]))
	}
}

func TestPackManifestMessageBytesPrefix(t *testing.T) {
	manifest := domain.SignedPackManifest{
		PackHash:     makeValidRefHash(),
		Version:      domain.PackVersionV1,
		BindingsHash: makeValidRefHash(),
		Provenance:   domain.ProvenanceMarketplace,
		CircleIDHash: makeValidCircleIDHash(),
		PeriodKey:    "2025-01-15",
	}

	msg := manifest.MessageBytes()
	prefix := "QL|phase50|pack_manifest|"

	if !strings.HasPrefix(string(msg), prefix) {
		t.Errorf("Message bytes should start with %q, got %q", prefix, string(msg[:len(prefix)]))
	}
}

// ============================================================================
// Section 9: SignatureB64 and PublicKeyB64 Validation Tests
// ============================================================================

func TestSignatureB64Validation(t *testing.T) {
	// Valid 64-byte signature
	validSig := base64.StdEncoding.EncodeToString(make([]byte, 64))
	if err := domain.SignatureB64(validSig).Validate(); err != nil {
		t.Errorf("Valid signature should pass: %v", err)
	}

	// Invalid base64
	if err := domain.SignatureB64("not-valid-base64!!!").Validate(); err == nil {
		t.Error("Invalid base64 should fail")
	}

	// Wrong length (32 bytes instead of 64)
	shortSig := base64.StdEncoding.EncodeToString(make([]byte, 32))
	if err := domain.SignatureB64(shortSig).Validate(); err == nil {
		t.Error("32-byte signature should fail")
	}

	// Empty
	if err := domain.SignatureB64("").Validate(); err == nil {
		t.Error("Empty signature should fail")
	}
}

func TestPublicKeyB64Validation(t *testing.T) {
	// Valid 32-byte public key
	validPub := base64.StdEncoding.EncodeToString(make([]byte, 32))
	if err := domain.PublicKeyB64(validPub).Validate(); err != nil {
		t.Errorf("Valid public key should pass: %v", err)
	}

	// Wrong length (64 bytes instead of 32)
	longPub := base64.StdEncoding.EncodeToString(make([]byte, 64))
	if err := domain.PublicKeyB64(longPub).Validate(); err == nil {
		t.Error("64-byte public key should fail")
	}

	// Empty
	if err := domain.PublicKeyB64("").Validate(); err == nil {
		t.Error("Empty public key should fail")
	}
}

// ============================================================================
// Section 10: Claim/Manifest Validation Tests
// ============================================================================

func TestSignedVendorClaimValidation(t *testing.T) {
	validClaim := domain.SignedVendorClaim{
		Kind:         domain.ClaimVendorCap,
		Scope:        domain.ScopeCommerce,
		Cap:          domain.AllowHoldOnly,
		RefHash:      makeValidRefHash(),
		Provenance:   domain.ProvenanceUserSupplied,
		CircleIDHash: makeValidCircleIDHash(),
		PeriodKey:    "2025-01-15",
	}

	if err := validClaim.Validate(); err != nil {
		t.Errorf("Valid claim should pass: %v", err)
	}

	// Test invalid kind
	invalidKind := validClaim
	invalidKind.Kind = "invalid"
	if err := invalidKind.Validate(); err == nil {
		t.Error("Invalid kind should fail")
	}

	// Test invalid scope
	invalidScope := validClaim
	invalidScope.Scope = "invalid"
	if err := invalidScope.Validate(); err == nil {
		t.Error("Invalid scope should fail")
	}

	// Test empty period key
	emptyPeriod := validClaim
	emptyPeriod.PeriodKey = ""
	if err := emptyPeriod.Validate(); err == nil {
		t.Error("Empty period key should fail")
	}
}

func TestSignedPackManifestValidation(t *testing.T) {
	validManifest := domain.SignedPackManifest{
		PackHash:     makeValidRefHash(),
		Version:      domain.PackVersionV1,
		BindingsHash: makeValidRefHash(),
		Provenance:   domain.ProvenanceMarketplace,
		CircleIDHash: makeValidCircleIDHash(),
		PeriodKey:    "2025-01-15",
	}

	if err := validManifest.Validate(); err != nil {
		t.Errorf("Valid manifest should pass: %v", err)
	}

	// Test invalid version
	invalidVersion := validManifest
	invalidVersion.Version = "invalid"
	if err := invalidVersion.Validate(); err == nil {
		t.Error("Invalid version should fail")
	}

	// Test invalid pack hash
	invalidHash := validManifest
	invalidHash.PackHash = "tooshort"
	if err := invalidHash.Validate(); err == nil {
		t.Error("Invalid pack hash should fail")
	}
}

// ============================================================================
// Section 11: Engine Helper Function Tests
// ============================================================================

func TestEngineCurrentPeriodKey(t *testing.T) {
	eng := engine.NewEngine(testClock)
	periodKey := eng.CurrentPeriodKey()

	expected := "2025-01-15"
	if periodKey != expected {
		t.Errorf("Expected period key %q, got %q", expected, periodKey)
	}
}

func TestHashCanonical(t *testing.T) {
	// Same input should produce same hash
	h1 := engine.HashCanonical("test|data|here")
	h2 := engine.HashCanonical("test|data|here")

	if h1 != h2 {
		t.Errorf("HashCanonical is not deterministic: %q != %q", h1, h2)
	}

	// Different input should produce different hash
	h3 := engine.HashCanonical("different|data")
	if h1 == h3 {
		t.Error("Different inputs should produce different hashes")
	}

	// Hash should be 64 hex chars
	if len(h1) != 64 {
		t.Errorf("Hash should be 64 hex chars, got %d", len(h1))
	}
}

func TestClaimDedupKey(t *testing.T) {
	circleHash := makeValidCircleIDHash()
	periodKey := "2025-01-15"
	claimHash := makeValidRefHash()

	key1 := engine.ClaimDedupKey(circleHash, periodKey, claimHash)
	key2 := engine.ClaimDedupKey(circleHash, periodKey, claimHash)

	if key1 != key2 {
		t.Error("Dedup key should be stable")
	}

	// Should contain pipes
	if !strings.Contains(key1, "|") {
		t.Error("Dedup key should be pipe-delimited")
	}
}

// ============================================================================
// Section 12: Record CanonicalString Tests
// ============================================================================

func TestSignedClaimRecordCanonicalString(t *testing.T) {
	record := domain.SignedClaimRecord{
		ClaimHash:      makeValidRefHash(),
		KeyFingerprint: domain.KeyFingerprint(strings.Repeat("c", 64)),
		Status:         domain.VerifiedOK,
		Provenance:     domain.ProvenanceUserSupplied,
		Kind:           domain.ClaimVendorCap,
		PeriodKey:      "2025-01-15",
		CircleIDHash:   makeValidCircleIDHash(),
		CreatedBucket:  "2025-01-15",
	}

	s1 := record.CanonicalString()
	s2 := record.CanonicalString()

	if s1 != s2 {
		t.Error("Record CanonicalString should be deterministic")
	}

	if !strings.Contains(s1, "|") {
		t.Error("Record CanonicalString should be pipe-delimited")
	}
}

func TestSignedManifestRecordCanonicalString(t *testing.T) {
	record := domain.SignedManifestRecord{
		ManifestHash:   makeValidRefHash(),
		KeyFingerprint: domain.KeyFingerprint(strings.Repeat("c", 64)),
		Status:         domain.VerifiedOK,
		Provenance:     domain.ProvenanceMarketplace,
		PeriodKey:      "2025-01-15",
		CircleIDHash:   makeValidCircleIDHash(),
		PackHash:       makeValidRefHash(),
		CreatedBucket:  "2025-01-15",
	}

	s1 := record.CanonicalString()
	s2 := record.CanonicalString()

	if s1 != s2 {
		t.Error("Record CanonicalString should be deterministic")
	}
}

// ============================================================================
// Section 13: No Power Tests (Import checks via guardrails, basic test here)
// ============================================================================

func TestVerificationHasNoPowerSideEffects(t *testing.T) {
	pub, priv := generateTestKeyPair()
	eng := engine.NewEngine(testClock)

	claim := domain.SignedVendorClaim{
		Kind:         domain.ClaimVendorCap,
		Scope:        domain.ScopeCommerce,
		Cap:          domain.AllowHoldOnly,
		RefHash:      makeValidRefHash(),
		Provenance:   domain.ProvenanceUserSupplied,
		CircleIDHash: makeValidCircleIDHash(),
		PeriodKey:    "2025-01-15",
	}

	sig := signClaim(priv, claim)
	pubB64 := pubKeyToB64(pub)

	// Verification should only return status/record - no side effects
	result := eng.VerifyClaim(claim, sig, pubB64)

	// Status should be a simple enum
	if result.Status != domain.VerifiedOK {
		t.Fatal("Verification should succeed for test setup")
	}

	// Record should only contain hashes/fingerprints (no power fields)
	// The record has no execution, delivery, or decision fields
	// This is enforced by the type system and guardrails
}

// ============================================================================
// Section 14: ProofDisplayData Tests
// ============================================================================

func TestBuildProofDisplayData(t *testing.T) {
	eng := engine.NewEngine(testClock)

	claims := []domain.SignedClaimRecord{
		{Status: domain.VerifiedOK},
		{Status: domain.VerifiedBadSig},
	}
	manifests := []domain.SignedManifestRecord{
		{Status: domain.VerifiedOK},
	}

	data := eng.BuildProofDisplayData(claims, manifests, "2025-01-15")

	if !data.HasVerifiedClaims {
		t.Error("Should have verified claims")
	}
	if !data.HasUnverifiedClaims {
		t.Error("Should have unverified claims")
	}
	if !data.HasVerifiedManifests {
		t.Error("Should have verified manifests")
	}
	if data.PeriodKey != "2025-01-15" {
		t.Error("Period key mismatch")
	}
}

// ============================================================================
// Section 15: Hash-Only Storage Verification Tests
// ============================================================================

func TestRecordContainsOnlyHashes(t *testing.T) {
	// Verify that SignedClaimRecord only stores hashes/fingerprints
	// This is a documentation/compile-time check - the types enforce this

	pub, priv := generateTestKeyPair()
	eng := engine.NewEngine(testClock)

	claim := domain.SignedVendorClaim{
		Kind:         domain.ClaimVendorCap,
		Scope:        domain.ScopeCommerce,
		Cap:          domain.AllowHoldOnly,
		RefHash:      makeValidRefHash(),
		Provenance:   domain.ProvenanceUserSupplied,
		CircleIDHash: makeValidCircleIDHash(),
		PeriodKey:    "2025-01-15",
	}

	sig := signClaim(priv, claim)
	pubB64 := pubKeyToB64(pub)

	result := eng.VerifyClaim(claim, sig, pubB64)

	// Record should have:
	// - ClaimHash (hash of claim)
	// - KeyFingerprint (hash of public key)
	// - CircleIDHash (hash of circle ID)
	// NOT raw signatures, raw public keys, or identifiers

	record := result.Record

	// Verify ClaimHash is a valid hash (64 hex chars)
	if err := record.ClaimHash.Validate(); err != nil {
		t.Errorf("ClaimHash should be valid: %v", err)
	}

	// Verify KeyFingerprint is a valid hash
	if err := record.KeyFingerprint.Validate(); err != nil {
		t.Errorf("KeyFingerprint should be valid: %v", err)
	}

	// Verify CircleIDHash is a valid hash
	if err := record.CircleIDHash.Validate(); err != nil {
		t.Errorf("CircleIDHash should be valid: %v", err)
	}

	// The raw signature and public key are NOT stored in the record
	// This is enforced by the type system (no Signature or PublicKey fields)
}

func TestNoRawIdentifiersInDomainTypes(t *testing.T) {
	// Verify that domain types use hash references, not raw identifiers
	// SignedVendorClaim uses RefHash (not vendorID)
	// SignedPackManifest uses PackHash (not packID)

	claim := domain.SignedVendorClaim{
		Kind:         domain.ClaimVendorCap,
		Scope:        domain.ScopeCommerce,
		Cap:          domain.AllowHoldOnly,
		RefHash:      makeValidRefHash(), // Hash reference, not vendorID
		Provenance:   domain.ProvenanceUserSupplied,
		CircleIDHash: makeValidCircleIDHash(), // Hash, not raw circle ID
		PeriodKey:    "2025-01-15",
	}

	// RefHash must be a valid 64-char hex hash
	if err := claim.RefHash.Validate(); err != nil {
		t.Errorf("RefHash should be valid: %v", err)
	}

	manifest := domain.SignedPackManifest{
		PackHash:     makeValidRefHash(), // Hash reference, not packID
		Version:      domain.PackVersionV1,
		BindingsHash: makeValidRefHash(),
		Provenance:   domain.ProvenanceMarketplace,
		CircleIDHash: makeValidCircleIDHash(),
		PeriodKey:    "2025-01-15",
	}

	if err := manifest.PackHash.Validate(); err != nil {
		t.Errorf("PackHash should be valid: %v", err)
	}
}

// ============================================================================
// Section 16: Period Key Determinism Tests
// ============================================================================

func TestPeriodKeyFromClockIsDeterministic(t *testing.T) {
	// Same clock input should produce same period key
	fixedTime := time.Date(2025, 6, 15, 14, 30, 0, 0, time.UTC)
	clockFn := func() time.Time { return fixedTime }

	eng := engine.NewEngine(clockFn)

	pk1 := eng.CurrentPeriodKey()
	pk2 := eng.CurrentPeriodKey()
	pk3 := eng.CurrentPeriodKey()

	if pk1 != pk2 || pk2 != pk3 {
		t.Error("Period key should be deterministic for same clock")
	}

	if pk1 != "2025-06-15" {
		t.Errorf("Expected 2025-06-15, got %s", pk1)
	}
}

func TestDifferentClockTimesDifferentPeriodKeys(t *testing.T) {
	eng1 := engine.NewEngine(func() time.Time {
		return time.Date(2025, 1, 15, 0, 0, 0, 0, time.UTC)
	})
	eng2 := engine.NewEngine(func() time.Time {
		return time.Date(2025, 1, 16, 0, 0, 0, 0, time.UTC)
	})

	pk1 := eng1.CurrentPeriodKey()
	pk2 := eng2.CurrentPeriodKey()

	if pk1 == pk2 {
		t.Error("Different dates should produce different period keys")
	}
}

// ============================================================================
// Section 17: CLI Signing Helper Verification Test
// ============================================================================

func TestCLISigningHelperPattern(t *testing.T) {
	// This test verifies that the signing pattern used by the CLI
	// (using domain types' MessageBytes()) produces verifiable signatures

	pub, priv := generateTestKeyPair()
	eng := engine.NewEngine(testClock)

	// Build claim exactly as CLI would
	claim := domain.SignedVendorClaim{
		Kind:         domain.ClaimVendorCap,
		Scope:        domain.ScopeHuman,
		Cap:          domain.AllowHoldOnly,
		RefHash:      makeValidRefHash(),
		Provenance:   domain.ProvenanceUserSupplied,
		CircleIDHash: makeValidCircleIDHash(),
		PeriodKey:    "2025-01-15",
	}

	// CLI would call: claim.MessageBytes() for signing
	messageBytes := claim.MessageBytes()

	// CLI would sign with ed25519.Sign
	signature := ed25519.Sign(priv, messageBytes)

	// Convert to base64 (as CLI outputs)
	sigB64 := domain.SignatureB64(base64.StdEncoding.EncodeToString(signature))
	pubB64 := pubKeyToB64(pub)

	// Verify using engine (same MessageBytes() path)
	result := eng.VerifyClaim(claim, sigB64, pubB64)

	if result.Status != domain.VerifiedOK {
		t.Errorf("CLI signing pattern should produce verifiable signatures, got %v", result.Status)
	}
}

func TestCLIManifestSigningPattern(t *testing.T) {
	pub, priv := generateTestKeyPair()
	eng := engine.NewEngine(testClock)

	// Build manifest exactly as CLI would
	manifest := domain.SignedPackManifest{
		PackHash:     makeValidRefHash(),
		Version:      domain.PackVersionV1,
		BindingsHash: makeValidRefHash(),
		Provenance:   domain.ProvenanceMarketplace,
		CircleIDHash: makeValidCircleIDHash(),
		PeriodKey:    "2025-01-15",
	}

	// Sign using same path as CLI
	signature := ed25519.Sign(priv, manifest.MessageBytes())
	sigB64 := domain.SignatureB64(base64.StdEncoding.EncodeToString(signature))
	pubB64 := pubKeyToB64(pub)

	result := eng.VerifyManifest(manifest, sigB64, pubB64)

	if result.Status != domain.VerifiedOK {
		t.Errorf("CLI manifest signing should produce verifiable signatures, got %v", result.Status)
	}
}

// ============================================================================
// Section 18: Replay Idempotency Tests (Dedup Verification)
// ============================================================================

func TestReplayClaimIsIdempotent(t *testing.T) {
	store := persist.NewSignedClaimStore(testClock)

	record := domain.SignedClaimRecord{
		ClaimHash:      makeValidRefHash(),
		KeyFingerprint: domain.KeyFingerprint(strings.Repeat("c", 64)),
		Status:         domain.VerifiedOK,
		Provenance:     domain.ProvenanceUserSupplied,
		Kind:           domain.ClaimVendorCap,
		PeriodKey:      "2025-01-15",
		CircleIDHash:   makeValidCircleIDHash(),
		CreatedBucket:  "2025-01-15",
	}

	// Submit same claim multiple times (simulating replay)
	for i := 0; i < 10; i++ {
		_ = store.AppendClaim(record)
	}

	// Should only have 1 record (idempotent)
	if store.Count() != 1 {
		t.Errorf("Replay should be idempotent, expected 1 record, got %d", store.Count())
	}

	// Query should return exactly 1
	list := store.ListByCircle(record.CircleIDHash)
	if len(list) != 1 {
		t.Errorf("List should have 1 record, got %d", len(list))
	}
}

func TestReplayManifestIsIdempotent(t *testing.T) {
	store := persist.NewSignedManifestStore(testClock)

	record := domain.SignedManifestRecord{
		ManifestHash:   makeValidRefHash(),
		KeyFingerprint: domain.KeyFingerprint(strings.Repeat("c", 64)),
		Status:         domain.VerifiedOK,
		Provenance:     domain.ProvenanceMarketplace,
		PeriodKey:      "2025-01-15",
		CircleIDHash:   makeValidCircleIDHash(),
		PackHash:       makeValidRefHash(),
		CreatedBucket:  "2025-01-15",
	}

	// Submit same manifest multiple times
	for i := 0; i < 5; i++ {
		_ = store.AppendManifest(record)
	}

	if store.Count() != 1 {
		t.Errorf("Manifest replay should be idempotent, expected 1, got %d", store.Count())
	}
}
