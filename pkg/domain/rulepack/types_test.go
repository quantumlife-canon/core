package rulepack

import (
	"strings"
	"testing"
	"time"

	"quantumlife/pkg/domain/shadowgate"
	"quantumlife/pkg/domain/shadowllm"
)

func TestRuleChangeCanonicalString(t *testing.T) {
	change := RuleChange{
		ChangeID:             "change123",
		CandidateHash:        "abc123",
		IntentHash:           "def456",
		CircleID:             "circle-1",
		ChangeKind:           ChangeBiasAdjust,
		TargetScope:          ScopeCategory,
		TargetHash:           "target789",
		Category:             shadowllm.CategoryMoney,
		SuggestedDelta:       DeltaMedium,
		UsefulnessBucket:     shadowgate.UsefulnessHigh,
		VoteConfidenceBucket: shadowgate.VoteConfidenceMedium,
		NoveltyBucket:        NoveltyShadowOnly,
		AgreementBucket:      AgreementMatch,
	}

	canonical := change.CanonicalString()

	// Verify pipe-delimited format
	if !strings.HasPrefix(canonical, "RULE_CHANGE|v1|") {
		t.Errorf("Expected RULE_CHANGE|v1| prefix, got: %s", canonical)
	}

	// Verify contains expected values
	if !strings.Contains(canonical, "change123") {
		t.Error("Missing change ID in canonical string")
	}
	if !strings.Contains(canonical, "abc123") {
		t.Error("Missing candidate hash in canonical string")
	}
	if !strings.Contains(canonical, "bias_adjust") {
		t.Error("Missing change kind in canonical string")
	}
}

func TestRuleChangeHashDeterminism(t *testing.T) {
	change := RuleChange{
		ChangeID:             "change123",
		CandidateHash:        "abc123",
		IntentHash:           "def456",
		CircleID:             "circle-1",
		ChangeKind:           ChangeBiasAdjust,
		TargetScope:          ScopeCategory,
		TargetHash:           "target789",
		Category:             shadowllm.CategoryMoney,
		SuggestedDelta:       DeltaMedium,
		UsefulnessBucket:     shadowgate.UsefulnessHigh,
		VoteConfidenceBucket: shadowgate.VoteConfidenceMedium,
		NoveltyBucket:        NoveltyShadowOnly,
		AgreementBucket:      AgreementMatch,
	}

	hash1 := change.ComputeHash()
	hash2 := change.ComputeHash()

	if hash1 != hash2 {
		t.Errorf("Hash not deterministic: %s != %s", hash1, hash2)
	}

	// Different input should produce different hash
	change.CandidateHash = "xyz999"
	hash3 := change.ComputeHash()

	if hash1 == hash3 {
		t.Error("Different inputs produced same hash")
	}
}

func TestRulePackCanonicalString(t *testing.T) {
	pack := RulePack{
		PackID:              "pack123",
		PackHash:            "packhash456",
		PeriodKey:           "2024-01-15",
		CircleID:            "circle-1",
		CreatedAtBucket:     "2024-01-15T10:30",
		ExportFormatVersion: "v1",
		Changes:             []RuleChange{},
	}

	canonical := pack.CanonicalString()

	if !strings.HasPrefix(canonical, "RULE_PACK|v1|") {
		t.Errorf("Expected RULE_PACK|v1| prefix, got: %s", canonical)
	}

	if !strings.Contains(canonical, "pack123") {
		t.Error("Missing pack ID in canonical string")
	}
	if !strings.Contains(canonical, "2024-01-15") {
		t.Error("Missing period key in canonical string")
	}
}

func TestRulePackHashDeterminism(t *testing.T) {
	changes := []RuleChange{
		{
			ChangeID:             "c1",
			CandidateHash:        "h1",
			IntentHash:           "i1",
			ChangeKind:           ChangeBiasAdjust,
			TargetScope:          ScopeCategory,
			Category:             shadowllm.CategoryMoney,
			SuggestedDelta:       DeltaSmall,
			UsefulnessBucket:     shadowgate.UsefulnessMedium,
			VoteConfidenceBucket: shadowgate.VoteConfidenceMedium,
			NoveltyBucket:        NoveltyShadowOnly,
			AgreementBucket:      AgreementMatch,
		},
	}

	pack := RulePack{
		PackID:              "pack123",
		PeriodKey:           "2024-01-15",
		CircleID:            "circle-1",
		CreatedAtBucket:     "2024-01-15T10:30",
		ExportFormatVersion: "v1",
		Changes:             changes,
	}
	pack.PackHash = pack.ComputeHash()

	hash1 := pack.ComputeHash()
	hash2 := pack.ComputeHash()

	if hash1 != hash2 {
		t.Errorf("Pack hash not deterministic: %s != %s", hash1, hash2)
	}
}

func TestRuleChangeSorting(t *testing.T) {
	changes := []RuleChange{
		{CircleID: "circle-2", CandidateHash: "a1", ChangeKind: ChangeBiasAdjust, TargetScope: ScopeCategory, TargetHash: "t1"},
		{CircleID: "circle-1", CandidateHash: "a2", ChangeKind: ChangeBiasAdjust, TargetScope: ScopeCategory, TargetHash: "t2"},
		{CircleID: "circle-1", CandidateHash: "a1", ChangeKind: ChangeThresholdAdjust, TargetScope: ScopeCategory, TargetHash: "t3"},
		{CircleID: "circle-1", CandidateHash: "a1", ChangeKind: ChangeBiasAdjust, TargetScope: ScopeTrigger, TargetHash: "t4"},
		{CircleID: "circle-1", CandidateHash: "a1", ChangeKind: ChangeBiasAdjust, TargetScope: ScopeCategory, TargetHash: "t5"},
	}

	SortRuleChanges(changes)

	// Verify sorting: CircleID first
	if changes[0].CircleID != "circle-1" {
		t.Errorf("Expected circle-1 first, got %s", changes[0].CircleID)
	}

	// Last should be circle-2
	if changes[len(changes)-1].CircleID != "circle-2" {
		t.Errorf("Expected circle-2 last, got %s", changes[len(changes)-1].CircleID)
	}
}

func TestRulePackToText(t *testing.T) {
	changes := []RuleChange{
		{
			ChangeID:             "c1",
			CandidateHash:        "hash123",
			IntentHash:           "intent456",
			CircleID:             "circle-1",
			ChangeKind:           ChangeBiasAdjust,
			TargetScope:          ScopeCategory,
			TargetHash:           "target789",
			Category:             shadowllm.CategoryMoney,
			SuggestedDelta:       DeltaMedium,
			UsefulnessBucket:     shadowgate.UsefulnessHigh,
			VoteConfidenceBucket: shadowgate.VoteConfidenceMedium,
			NoveltyBucket:        NoveltyShadowOnly,
			AgreementBucket:      AgreementMatch,
		},
	}

	pack := RulePack{
		PackID:              "pack123",
		PackHash:            "packhash",
		PeriodKey:           "2024-01-15",
		CircleID:            "circle-1",
		CreatedAtBucket:     "2024-01-15T10:30",
		ExportFormatVersion: "v1",
		Changes:             changes,
	}

	text := pack.ToText()

	// Verify header
	if !strings.HasPrefix(text, "# RULEPACK EXPORT FORMAT v1") {
		t.Error("Missing export header")
	}

	// Verify PACK line
	if !strings.Contains(text, "PACK|pack123|") {
		t.Error("Missing PACK line")
	}

	// Verify CHANGE line
	if !strings.Contains(text, "CHANGE|c1|") {
		t.Error("Missing CHANGE line")
	}

	// Verify footer
	if !strings.HasSuffix(text, "# END\n") {
		t.Error("Missing END footer")
	}

	// Verify no JSON
	if strings.Contains(text, "{") || strings.Contains(text, "}") {
		t.Error("Export contains JSON characters")
	}
}

func TestValidateExportPrivacy(t *testing.T) {
	// Clean text should pass
	cleanText := "PACK|abc123|2024-01-15|circle-1|10:30|1|hash456"
	if err := ValidateExportPrivacy(cleanText); err != nil {
		t.Errorf("Clean text failed validation: %v", err)
	}

	// Email should fail
	emailText := "PACK|user@example.com|2024-01-15"
	if err := ValidateExportPrivacy(emailText); err == nil {
		t.Error("Email pattern should fail validation")
	}

	// URL should fail
	urlText := "PACK|https://example.com|2024-01-15"
	if err := ValidateExportPrivacy(urlText); err == nil {
		t.Error("URL pattern should fail validation")
	}

	// Currency should fail
	currencyText := "PACK|$100|2024-01-15"
	if err := ValidateExportPrivacy(currencyText); err == nil {
		t.Error("Currency pattern should fail validation")
	}

	// Vendor name should fail
	vendorText := "PACK|amazon order|2024-01-15"
	if err := ValidateExportPrivacy(vendorText); err == nil {
		t.Error("Vendor name should fail validation")
	}
}

func TestChangeKindValidation(t *testing.T) {
	valid := []ChangeKind{ChangeBiasAdjust, ChangeThresholdAdjust, ChangeSuppressSuggest}
	for _, k := range valid {
		if !k.Validate() {
			t.Errorf("%s should be valid", k)
		}
	}

	invalid := ChangeKind("invalid")
	if invalid.Validate() {
		t.Error("invalid should not validate")
	}
}

func TestTargetScopeValidation(t *testing.T) {
	valid := []TargetScope{ScopeCircle, ScopeTrigger, ScopeCategory, ScopeItemKey, ScopeUnknown}
	for _, s := range valid {
		if !s.Validate() {
			t.Errorf("%s should be valid", s)
		}
	}

	invalid := TargetScope("invalid")
	if invalid.Validate() {
		t.Error("invalid should not validate")
	}
}

func TestSuggestedDeltaValidation(t *testing.T) {
	valid := []SuggestedDelta{DeltaNone, DeltaSmall, DeltaMedium, DeltaLarge}
	for _, d := range valid {
		if !d.Validate() {
			t.Errorf("%s should be valid", d)
		}
	}

	invalid := SuggestedDelta("invalid")
	if invalid.Validate() {
		t.Error("invalid should not validate")
	}
}

func TestDeltaFromUsefulness(t *testing.T) {
	tests := []struct {
		input    shadowgate.UsefulnessBucket
		expected SuggestedDelta
	}{
		{shadowgate.UsefulnessHigh, DeltaLarge},
		{shadowgate.UsefulnessMedium, DeltaMedium},
		{shadowgate.UsefulnessLow, DeltaSmall},
		{shadowgate.UsefulnessUnknown, DeltaNone},
	}

	for _, tt := range tests {
		got := DeltaFromUsefulness(tt.input)
		if got != tt.expected {
			t.Errorf("DeltaFromUsefulness(%s) = %s, want %s", tt.input, got, tt.expected)
		}
	}
}

func TestAckKindValidation(t *testing.T) {
	valid := []AckKind{AckViewed, AckExported, AckDismissed}
	for _, a := range valid {
		if !a.Validate() {
			t.Errorf("%s should be valid", a)
		}
	}

	invalid := AckKind("invalid")
	if invalid.Validate() {
		t.Error("invalid should not validate")
	}
}

func TestPackAckCanonicalString(t *testing.T) {
	ack := PackAck{
		AckID:         "ack123",
		PackID:        "pack456",
		PackHash:      "hash789",
		AckKind:       AckExported,
		CreatedBucket: "2024-01-15T10:30",
	}

	canonical := ack.CanonicalString()

	if !strings.HasPrefix(canonical, "PACK_ACK|v1|") {
		t.Errorf("Expected PACK_ACK|v1| prefix, got: %s", canonical)
	}

	if !strings.Contains(canonical, "pack456") {
		t.Error("Missing pack ID in canonical string")
	}
}

func TestRulePackChangeMagnitude(t *testing.T) {
	tests := []struct {
		count    int
		expected shadowllm.MagnitudeBucket
	}{
		{0, shadowllm.MagnitudeNothing},
		{1, shadowllm.MagnitudeAFew},
		{3, shadowllm.MagnitudeAFew},
		{5, shadowllm.MagnitudeSeveral},
		{10, shadowllm.MagnitudeSeveral},
		{15, shadowllm.MagnitudeSeveral},
	}

	for _, tt := range tests {
		pack := RulePack{Changes: make([]RuleChange, tt.count)}
		got := pack.ChangeMagnitude()
		if got != tt.expected {
			t.Errorf("ChangeMagnitude(%d) = %s, want %s", tt.count, got, tt.expected)
		}
	}
}

func TestPeriodKeyFromTime(t *testing.T) {
	// Use a fixed time
	tm := time.Date(2024, 1, 15, 14, 30, 0, 0, time.UTC)
	key := PeriodKeyFromTime(tm)

	if key != "2024-01-15" {
		t.Errorf("PeriodKeyFromTime = %s, want 2024-01-15", key)
	}
}

func TestFiveMinuteBucket(t *testing.T) {
	tests := []struct {
		minute   int
		expected string
	}{
		{0, "2024-01-15T10:0"},
		{3, "2024-01-15T10:0"},
		{5, "2024-01-15T10:5"},
		{7, "2024-01-15T10:5"},
		{15, "2024-01-15T10:15"},
		{59, "2024-01-15T10:55"},
	}

	for _, tt := range tests {
		tm := time.Date(2024, 1, 15, 10, tt.minute, 0, 0, time.UTC)
		got := FiveMinuteBucket(tm)
		if got != tt.expected {
			t.Errorf("FiveMinuteBucket(minute=%d) = %s, want %s", tt.minute, got, tt.expected)
		}
	}
}

func TestRulePackValidation(t *testing.T) {
	validPack := RulePack{
		PeriodKey:           "2024-01-15",
		CreatedAtBucket:     "2024-01-15T10:30",
		ExportFormatVersion: "v1",
	}

	if err := validPack.Validate(); err != nil {
		t.Errorf("Valid pack failed validation: %v", err)
	}

	// Missing period key
	invalidPack := RulePack{
		CreatedAtBucket:     "2024-01-15T10:30",
		ExportFormatVersion: "v1",
	}
	if err := invalidPack.Validate(); err != ErrMissingPeriodKey {
		t.Errorf("Expected ErrMissingPeriodKey, got: %v", err)
	}
}

func TestRuleChangeValidation(t *testing.T) {
	validChange := RuleChange{
		CandidateHash:        "hash123",
		IntentHash:           "intent456",
		ChangeKind:           ChangeBiasAdjust,
		TargetScope:          ScopeCategory,
		SuggestedDelta:       DeltaMedium,
		UsefulnessBucket:     shadowgate.UsefulnessHigh,
		VoteConfidenceBucket: shadowgate.VoteConfidenceMedium,
	}

	if err := validChange.Validate(); err != nil {
		t.Errorf("Valid change failed validation: %v", err)
	}

	// Missing candidate hash
	invalidChange := RuleChange{
		IntentHash:     "intent456",
		ChangeKind:     ChangeBiasAdjust,
		TargetScope:    ScopeCategory,
		SuggestedDelta: DeltaMedium,
	}
	if err := invalidChange.Validate(); err != ErrMissingCandidateHash {
		t.Errorf("Expected ErrMissingCandidateHash, got: %v", err)
	}
}

func TestParseText(t *testing.T) {
	// Create a pack and export it
	changes := []RuleChange{
		{
			ChangeID:             "c1",
			CandidateHash:        "hash123",
			IntentHash:           "intent456",
			CircleID:             "circle-1",
			ChangeKind:           ChangeBiasAdjust,
			TargetScope:          ScopeCategory,
			TargetHash:           "target789",
			Category:             shadowllm.CategoryMoney,
			SuggestedDelta:       DeltaMedium,
			UsefulnessBucket:     shadowgate.UsefulnessHigh,
			VoteConfidenceBucket: shadowgate.VoteConfidenceMedium,
			NoveltyBucket:        NoveltyShadowOnly,
			AgreementBucket:      AgreementMatch,
		},
	}

	original := RulePack{
		PackID:              "pack123",
		PackHash:            "packhash",
		PeriodKey:           "2024-01-15",
		CircleID:            "circle-1",
		CreatedAtBucket:     "2024-01-15T10:30",
		ExportFormatVersion: "v1",
		Changes:             changes,
	}

	text := original.ToText()

	// Parse it back
	parsed, err := ParseText(text)
	if err != nil {
		t.Fatalf("ParseText failed: %v", err)
	}

	if parsed.PackID != original.PackID {
		t.Errorf("PackID mismatch: got %s, want %s", parsed.PackID, original.PackID)
	}

	if len(parsed.Changes) != len(original.Changes) {
		t.Errorf("Change count mismatch: got %d, want %d", len(parsed.Changes), len(original.Changes))
	}

	if len(parsed.Changes) > 0 && parsed.Changes[0].CandidateHash != original.Changes[0].CandidateHash {
		t.Errorf("CandidateHash mismatch")
	}
}

func TestAllChangeKinds(t *testing.T) {
	kinds := AllChangeKinds()
	if len(kinds) != 3 {
		t.Errorf("Expected 3 change kinds, got %d", len(kinds))
	}

	for _, k := range kinds {
		if !k.Validate() {
			t.Errorf("AllChangeKinds returned invalid kind: %s", k)
		}
	}
}

func TestEmptyPackHash(t *testing.T) {
	// Empty pack should still have a valid hash
	pack := RulePack{
		PackID:              "empty-pack",
		PeriodKey:           "2024-01-15",
		CircleID:            "",
		CreatedAtBucket:     "2024-01-15T10:30",
		ExportFormatVersion: "v1",
		Changes:             []RuleChange{},
	}

	hash := pack.ComputeHash()
	if hash == "" {
		t.Error("Empty pack should have a non-empty hash")
	}

	// Verify determinism
	hash2 := pack.ComputeHash()
	if hash != hash2 {
		t.Error("Empty pack hash not deterministic")
	}
}

func TestPackWithAllCircle(t *testing.T) {
	pack := RulePack{
		PackID:              "all-pack",
		PeriodKey:           "2024-01-15",
		CircleID:            "", // Empty = "all"
		CreatedAtBucket:     "2024-01-15T10:30",
		ExportFormatVersion: "v1",
		Changes:             []RuleChange{},
	}

	text := pack.ToText()

	// Should show "all" for empty circle
	if !strings.Contains(text, "|all|") {
		t.Error("Empty circle should export as 'all'")
	}
}
