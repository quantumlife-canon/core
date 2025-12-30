// Package demo_family_negotiate provides the Family Negotiation demo.
package demo_family_negotiate

import (
	"fmt"
	"strings"
)

// PrintResult prints the demo result in a formatted way.
func PrintResult(result *DemoResult) {
	fmt.Println("============================================================")
	fmt.Println("  QuantumLife Demo: Family Negotiation (Vertical Slice v3)")
	fmt.Println("============================================================")
	fmt.Println()

	fmt.Printf("Circle A (You):    %s\n", result.CircleAID)
	fmt.Printf("Circle B (Spouse): %s\n", result.CircleBID)
	fmt.Printf("Intersection:      %s\n", result.IntersectionID)
	fmt.Printf("Status:            %s\n", map[bool]string{true: "SUCCESS", false: "FAILED"}[result.Success])
	fmt.Println()

	fmt.Println("------------------------------------------------------------")
	fmt.Println("  INITIAL CONTRACT (v1.0.0)")
	fmt.Println("------------------------------------------------------------")
	printContractSummary(result.InitialContract)

	fmt.Println("------------------------------------------------------------")
	fmt.Println("  NEGOTIATION FLOW")
	fmt.Println("------------------------------------------------------------")
	fmt.Println()

	fmt.Println("Step 1: Circle A submits proposal")
	printProposalSummary("PROPOSAL", result.ProposalSummary)

	fmt.Println("Step 2: Circle B rejects (triggers trust update)")
	fmt.Println("  -> Rejection reason: Time window too wide")
	fmt.Println()

	fmt.Println("Step 3: Circle A submits revised proposal")
	fmt.Println("  -> Reason: Revised terms with narrower window")
	fmt.Println()

	fmt.Println("Step 4: Circle B counterproposals")
	printProposalSummary("COUNTERPROPOSAL", result.CounterSummary)

	fmt.Println("Step 5: Circle A accepts counterproposal")
	fmt.Printf("  -> %s\n", result.AcceptanceResult)
	fmt.Println()

	fmt.Println("Step 6: Finalize -> Amendment Applied")
	fmt.Println()

	fmt.Println("------------------------------------------------------------")
	fmt.Println("  AMENDED CONTRACT (version bumped)")
	fmt.Println("------------------------------------------------------------")
	printContractSummary(result.AmendedContract)

	fmt.Println("------------------------------------------------------------")
	fmt.Println("  COMMITMENT FORMED (handoff to data-plane boundary)")
	fmt.Println("------------------------------------------------------------")
	fmt.Println()
	fmt.Printf("  Commitment ID:   %s\n", result.CommitmentSummary.ID)
	fmt.Printf("  Intersection:    %s\n", result.CommitmentSummary.IntersectionID)
	fmt.Printf("  From Proposal:   %s\n", result.CommitmentSummary.ProposalID)
	fmt.Printf("  Action Type:     %s\n", result.CommitmentSummary.ActionType)
	fmt.Printf("  Action Desc:     %s\n", result.CommitmentSummary.ActionDesc)
	fmt.Printf("  Parties:         %s\n", strings.Join(result.CommitmentSummary.Parties, ", "))
	fmt.Println()
	fmt.Println("  *** IMPORTANT: NOT EXECUTED ***")
	fmt.Println("  This commitment represents the handoff point to the data-plane.")
	fmt.Println("  In production, the execution layer would now process this commitment.")
	fmt.Println("  In this demo, we stop here (suggest-only mode).")
	fmt.Println()

	if len(result.TrustUpdates) > 0 {
		fmt.Println("------------------------------------------------------------")
		fmt.Println("  TRUST UPDATES")
		fmt.Println("------------------------------------------------------------")
		fmt.Println()
		for _, update := range result.TrustUpdates {
			fmt.Printf("  Circle: %s @ %s\n", update.CircleID, update.IntersectionID)
			fmt.Printf("    %s -> %s (reason: %s)\n", update.OldLevel, update.NewLevel, update.Reason)
		}
		fmt.Println()
	}

	fmt.Println("------------------------------------------------------------")
	fmt.Println("  AUDIT LOG (trace of all events)")
	fmt.Println("------------------------------------------------------------")
	fmt.Println()

	for _, entry := range result.AuditLog {
		intID := entry.IntersectionID
		if intID == "" {
			intID = "-"
		}
		circleID := entry.CircleID
		if circleID == "" {
			circleID = "-"
		}
		fmt.Printf("[%s] %s\n", entry.ID, entry.EventType)
		fmt.Printf("         Circle: %s | Intersection: %s\n", circleID, intID)
		fmt.Printf("         Action: %s | Outcome: %s\n", entry.Action, entry.Outcome)
		fmt.Println()
	}

	fmt.Println("============================================================")
	fmt.Println("  Demo completed. SUGGEST-ONLY mode: no external actions.")
	fmt.Println("  Negotiation loop demonstrated:")
	fmt.Println("    Proposal -> Rejection -> Revised -> Counterproposal -> Accept -> Finalize")
	fmt.Println("  Contract version bumped after amendment.")
	fmt.Println("  Commitment formed but NOT executed (data-plane boundary).")
	fmt.Println("============================================================")
}

func printContractSummary(contract ContractSummary) {
	fmt.Println()
	fmt.Printf("  Intersection ID: %s\n", contract.IntersectionID)
	fmt.Printf("  Version:         %s\n", contract.Version)
	fmt.Printf("  Parties:         %s\n", strings.Join(contract.PartyIDs, ", "))
	fmt.Printf("  Scopes:          %s\n", strings.Join(contract.Scopes, ", "))
	fmt.Println("  Ceilings:")
	for _, c := range contract.Ceilings {
		fmt.Printf("    - %s: %s %s\n", c.Type, c.Value, c.Unit)
	}
	fmt.Printf("  Governance:      %s\n", contract.Governance)
	fmt.Println()
}

func printProposalSummary(label string, proposal ProposalSummary) {
	fmt.Println()
	fmt.Printf("  %s:\n", label)
	fmt.Printf("    ID:           %s\n", proposal.ID)
	fmt.Printf("    Issuer:       %s\n", proposal.IssuerID)
	fmt.Printf("    Type:         %s\n", proposal.Type)
	fmt.Printf("    State:        %s\n", proposal.State)
	fmt.Printf("    Reason:       %s\n", proposal.Reason)
	if len(proposal.ScopeAdditions) > 0 {
		fmt.Printf("    Scope adds:   %s\n", strings.Join(proposal.ScopeAdditions, ", "))
	}
	if len(proposal.CeilingChanges) > 0 {
		fmt.Printf("    Ceilings:     %s\n", strings.Join(proposal.CeilingChanges, ", "))
	}
	fmt.Println()
}
