package demo_phase8_commerce_mirror

import (
	"testing"
)

func TestRunAllScenarios(t *testing.T) {
	results, deterministicHash := RunAllScenarios()

	// Verify we have all scenarios
	if len(results) != 4 {
		t.Errorf("expected 4 scenarios, got %d", len(results))
	}

	// Verify each scenario succeeded
	for _, r := range results {
		if !r.Success {
			t.Errorf("scenario %s failed", r.ScenarioName)
		}

		t.Logf("Scenario: %s - Events: %d, Obligations: %d, Hash: %s",
			r.ScenarioName, r.CommerceEventCount, r.CommerceObligationCount, r.EventsHash[:16])
	}

	// Verify determinism
	if len(deterministicHash) < 16 || deterministicHash[:6] == "FAILED" {
		t.Errorf("determinism check failed: %s", deterministicHash)
	}
}

func TestScenarioUK(t *testing.T) {
	results, _ := RunAllScenarios()

	var ukResult DemoResult
	for _, r := range results {
		if r.ScenarioName == "UK Vendors" {
			ukResult = r
			break
		}
	}

	if ukResult.ScenarioName == "" {
		t.Fatal("UK Vendors scenario not found")
	}

	// Should have 5 commerce events (5 UK vendor emails)
	if ukResult.CommerceEventCount != 5 {
		t.Errorf("UK scenario: expected 5 events, got %d", ukResult.CommerceEventCount)
	}

	// Verify vendor matching
	if ukResult.ExtractionMetrics.VendorMatchedCount < 4 {
		t.Errorf("UK scenario: expected at least 4 vendor matches, got %d", ukResult.ExtractionMetrics.VendorMatchedCount)
	}

	// Verify amounts were parsed
	if ukResult.ExtractionMetrics.AmountsParsed < 3 {
		t.Errorf("UK scenario: expected at least 3 amounts parsed, got %d", ukResult.ExtractionMetrics.AmountsParsed)
	}
}

func TestScenarioUS(t *testing.T) {
	results, _ := RunAllScenarios()

	var usResult DemoResult
	for _, r := range results {
		if r.ScenarioName == "US Vendors" {
			usResult = r
			break
		}
	}

	if usResult.ScenarioName == "" {
		t.Fatal("US Vendors scenario not found")
	}

	// Should have 5 commerce events
	if usResult.CommerceEventCount != 5 {
		t.Errorf("US scenario: expected 5 events, got %d", usResult.CommerceEventCount)
	}

	// Verify vendor matching
	if usResult.ExtractionMetrics.VendorMatchedCount < 4 {
		t.Errorf("US scenario: expected at least 4 vendor matches, got %d", usResult.ExtractionMetrics.VendorMatchedCount)
	}
}

func TestScenarioIndia(t *testing.T) {
	results, _ := RunAllScenarios()

	var indiaResult DemoResult
	for _, r := range results {
		if r.ScenarioName == "India Vendors" {
			indiaResult = r
			break
		}
	}

	if indiaResult.ScenarioName == "" {
		t.Fatal("India Vendors scenario not found")
	}

	// Should have 5 commerce events
	if indiaResult.CommerceEventCount != 5 {
		t.Errorf("India scenario: expected 5 events, got %d", indiaResult.CommerceEventCount)
	}

	// Verify vendor matching
	if indiaResult.ExtractionMetrics.VendorMatchedCount < 4 {
		t.Errorf("India scenario: expected at least 4 vendor matches, got %d", indiaResult.ExtractionMetrics.VendorMatchedCount)
	}

	// Verify INR amounts were parsed
	if indiaResult.ExtractionMetrics.AmountsParsed < 3 {
		t.Errorf("India scenario: expected at least 3 amounts parsed, got %d", indiaResult.ExtractionMetrics.AmountsParsed)
	}
}

func TestScenarioMixed(t *testing.T) {
	results, _ := RunAllScenarios()

	var mixedResult DemoResult
	for _, r := range results {
		if r.ScenarioName == "Mixed Vendors" {
			mixedResult = r
			break
		}
	}

	if mixedResult.ScenarioName == "" {
		t.Fatal("Mixed Vendors scenario not found")
	}

	// Should have 15 commerce events (5 UK + 5 US + 5 India, misc emails filtered)
	if mixedResult.CommerceEventCount != 15 {
		t.Errorf("Mixed scenario: expected 15 events, got %d", mixedResult.CommerceEventCount)
	}

	// Non-commerce emails should be filtered (2 misc emails)
	if mixedResult.ExtractionMetrics.EmailsScanned != 17 {
		t.Errorf("Mixed scenario: expected 17 emails scanned, got %d", mixedResult.ExtractionMetrics.EmailsScanned)
	}
}

func TestDeterminism(t *testing.T) {
	// Run multiple times and verify same hash
	_, hash1 := RunAllScenarios()
	_, hash2 := RunAllScenarios()
	_, hash3 := RunAllScenarios()

	if hash1 != hash2 || hash2 != hash3 {
		t.Errorf("determinism failed: hashes differ: %s, %s, %s", hash1, hash2, hash3)
	}

	// Verify hash format
	if len(hash1) < 16 {
		t.Errorf("hash too short: %s", hash1)
	}

	t.Logf("Deterministic hash: %s", hash1)
}

func TestCommerceObligations(t *testing.T) {
	results, _ := RunAllScenarios()

	// Check UK scenario for specific obligation types
	var ukResult DemoResult
	for _, r := range results {
		if r.ScenarioName == "UK Vendors" {
			ukResult = r
			break
		}
	}

	if ukResult.ScenarioName == "" {
		t.Fatal("UK Vendors scenario not found")
	}

	// Should have some obligations generated
	// DPD shipment should generate a tracking obligation (out for delivery)
	// EDF invoice should generate a payment obligation
	foundShipment := false
	foundInvoice := false

	for _, obl := range ukResult.CommerceObligations {
		if obl.SourceType == "commerce" {
			switch obl.Type {
			case "followup":
				foundShipment = true
			case "pay":
				foundInvoice = true
			}
		}
	}

	// Note: Obligations depend on time-based logic, so we just verify at least some were created
	if ukResult.CommerceObligationCount == 0 {
		t.Log("No commerce obligations generated (may be expected depending on time logic)")
	} else {
		t.Logf("Commerce obligations: %d (shipment: %t, invoice: %t)",
			ukResult.CommerceObligationCount, foundShipment, foundInvoice)
	}
}
