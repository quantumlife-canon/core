package demo_family_simulate

import (
	"fmt"
	"strings"
)

// PrintResult prints the demo result in a formatted way.
func PrintResult(result *Result) {
	fmt.Println()
	fmt.Println("============================================================")
	fmt.Println("  QuantumLife Demo: Family Simulate Action (Vertical Slice v4)")
	fmt.Println("============================================================")
	fmt.Println()

	fmt.Printf("Circle A (You):    %s\n", result.CircleA)
	fmt.Printf("Circle B (Spouse): %s\n", result.CircleB)
	fmt.Printf("Intersection:      %s\n", result.IntersectionID)
	fmt.Printf("Mode:              %s\n", result.Mode)
	fmt.Printf("Status:            %s\n", statusText(result.Success, result.Error))
	fmt.Println()

	// CONTRACT VERSION
	printSection("CONTRACT VERSION")
	fmt.Printf("  Intersection ID: %s\n", result.IntersectionID)
	fmt.Printf("  Version:         %s\n", result.ContractVersion)
	fmt.Printf("  Scopes:          %s\n", strings.Join(result.ContractScopes, ", "))
	fmt.Println("  Ceilings:")
	for _, c := range result.ContractCeilings {
		fmt.Printf("    - %s: %s %s\n", c.Type, c.Value, c.Unit)
	}
	fmt.Println()

	// COMMITMENT SUMMARY
	printSection("COMMITMENT SUMMARY")
	fmt.Printf("  Commitment ID:   %s\n", result.CommitmentID)
	fmt.Printf("  Summary:         %s\n", result.CommitmentSummary)
	fmt.Println()

	// ACTION SUMMARY
	printSection("ACTION SUMMARY")
	if result.ActionID != "" {
		fmt.Printf("  Action ID:       %s\n", result.ActionID)
		fmt.Printf("  Action Type:     %s\n", result.ActionType)
		fmt.Printf("  Status:          %s\n", result.ActionSummary)
	} else {
		fmt.Printf("  %s\n", result.ActionSummary)
	}
	fmt.Println()

	// AUTHORIZATION PROOF
	printSection("AUTHORIZATION PROOF")
	if result.AuthorizationProof != nil {
		proof := result.AuthorizationProof
		fmt.Printf("  Proof ID:          %s\n", proof.ID)
		fmt.Printf("  Intersection:      %s\n", proof.IntersectionID)
		fmt.Printf("  Contract Version:  %s\n", proof.ContractVersion)
		fmt.Printf("  Scopes Used:       %s\n", strings.Join(proof.ScopesUsed, ", "))
		fmt.Printf("  Scopes Granted:    %s\n", strings.Join(proof.ScopesGranted, ", "))
		fmt.Printf("  Authorized:        %t\n", proof.Authorized)
		if !proof.Authorized {
			fmt.Printf("  Denial Reason:     %s\n", proof.DenialReason)
		}
		fmt.Println("  Mode Check:")
		fmt.Printf("    - Requested:     %s\n", proof.ModeCheck.RequestedMode)
		fmt.Printf("    - Allowed:       %t\n", proof.ModeCheck.Allowed)
		fmt.Printf("    - Reason:        %s\n", proof.ModeCheck.Reason)
		fmt.Println("  Ceiling Checks:")
		for _, check := range proof.CeilingChecks {
			passedStr := "✓"
			if !check.Passed {
				passedStr = "✗"
			}
			fmt.Printf("    - %s %s: %s (%s)\n", passedStr, check.CeilingType, check.CeilingValue, check.Reason)
		}
	} else {
		fmt.Println("  No authorization proof (suggest-only mode)")
	}
	fmt.Println()

	// SIMULATED OUTCOME
	printSection("SIMULATED OUTCOME (NO EXTERNAL WRITE)")
	if result.ExecutionOutcome != nil {
		outcome := result.ExecutionOutcome
		fmt.Printf("  Success:           %t\n", outcome.Success)
		fmt.Printf("  Simulated:         %t\n", outcome.Simulated)
		fmt.Printf("  Result Code:       %s\n", outcome.ResultCode)
		fmt.Printf("  Connector:         %s\n", outcome.ConnectorID)
		if len(outcome.ProposedPayload) > 0 {
			fmt.Println("  Proposed Payload:")
			for k, v := range outcome.ProposedPayload {
				fmt.Printf("    - %s: %s\n", k, v)
			}
		}
		fmt.Println()
		fmt.Println("  *** IMPORTANT: NO EXTERNAL WRITE PERFORMED ***")
		fmt.Println("  This is a simulation. The calendar event was NOT created.")
	} else {
		fmt.Println("  No simulated execution (suggest-only mode)")
	}
	fmt.Println()

	// SETTLEMENT SUMMARY
	printSection("SETTLEMENT SUMMARY")
	if result.SettlementID != "" {
		fmt.Printf("  Settlement ID:     %s\n", result.SettlementID)
		fmt.Printf("  Status:            %s\n", result.SettlementStatus)
		fmt.Printf("  Summary:           %s\n", result.SettlementSummary)
	} else {
		fmt.Printf("  %s\n", result.SettlementSummary)
	}
	fmt.Println()

	// MEMORY UPDATE SUMMARY
	printSection("MEMORY UPDATE SUMMARY")
	if result.MemoryEntry != nil {
		entry := result.MemoryEntry
		fmt.Printf("  Entry ID:          %s\n", entry.ID)
		fmt.Printf("  Owner Type:        %s\n", entry.OwnerType)
		fmt.Printf("  Owner ID:          %s\n", entry.OwnerID)
		fmt.Printf("  Key:               %s\n", entry.Key)
		fmt.Printf("  Version:           %d\n", entry.Version)
		fmt.Printf("  Summary:           %s\n", result.MemorySummary)
	} else {
		fmt.Printf("  %s\n", result.MemorySummary)
	}
	fmt.Println()

	// AUDIT TRACE SUMMARY
	printSection("AUDIT TRACE SUMMARY")
	if len(result.AuditEntries) > 0 {
		// Filter to show only v4-specific events
		v4Events := []string{
			"action.created",
			"authorization.checked",
			"simulated.execution.completed",
			"settlement.recorded",
			"memory.written",
		}

		fmt.Println("  v4 Simulation Events:")
		for _, entry := range result.AuditEntries {
			for _, v4Event := range v4Events {
				if entry.Type == v4Event {
					proofInfo := ""
					if entry.AuthorizationProofID != "" {
						proofInfo = fmt.Sprintf(" [proof: %s]", entry.AuthorizationProofID)
					}
					fmt.Printf("  [%s] %s%s\n", entry.ID, entry.Type, proofInfo)
					fmt.Printf("           Intersection: %s | Action: %s\n", entry.IntersectionID, entry.SubjectID)
					break
				}
			}
		}
		fmt.Println()
		fmt.Printf("  Total audit entries: %d\n", len(result.AuditEntries))
	} else {
		fmt.Println("  No audit entries")
	}
	fmt.Println()

	// Final summary
	fmt.Println("============================================================")
	if result.Mode == "simulate" {
		fmt.Println("  Demo completed. SIMULATE mode: deterministic execution.")
		fmt.Println("  Pipeline: Commitment -> Action -> Auth -> Simulate -> Settle -> Memory")
		fmt.Println("  NO external writes were performed.")
	} else {
		fmt.Println("  Demo completed. SUGGEST_ONLY mode.")
		fmt.Println("  No action, execution, settlement, or memory writes occurred.")
	}
	fmt.Println("============================================================")
}

func printSection(title string) {
	fmt.Println("------------------------------------------------------------")
	fmt.Printf("  %s\n", title)
	fmt.Println("------------------------------------------------------------")
	fmt.Println()
}

func statusText(success bool, errMsg string) string {
	if success {
		return "SUCCESS"
	}
	if errMsg != "" {
		return fmt.Sprintf("FAILED: %s", errMsg)
	}
	return "FAILED"
}
