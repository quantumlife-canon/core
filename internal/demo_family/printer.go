// Package demo_family provides the Family Intersection demo.
package demo_family

import (
	"fmt"
	"strings"
)

// PrintResult prints the family demo result in a formatted way.
func PrintResult(result *FamilyDemoResult) {
	fmt.Println("============================================================")
	fmt.Println("  QuantumLife Demo: Family Intersection (Vertical Slice v2)")
	fmt.Println("============================================================")
	fmt.Println()

	fmt.Printf("Circle A (You):    %s\n", result.CircleAID)
	fmt.Printf("Circle B (Spouse): %s\n", result.CircleBID)
	fmt.Printf("Status:            %s\n", map[bool]string{true: "SUCCESS", false: "FAILED"}[result.Success])
	fmt.Println()

	fmt.Println("------------------------------------------------------------")
	fmt.Println("  INVITE TOKEN SUMMARY")
	fmt.Println("------------------------------------------------------------")
	fmt.Println()
	fmt.Printf("  Token ID:      %s\n", result.TokenSummary.TokenID)
	fmt.Printf("  Issuer:        %s\n", result.TokenSummary.IssuerCircleID)
	fmt.Printf("  Target:        %s\n", result.TokenSummary.TargetCircleID)
	fmt.Printf("  Proposed Name: %s\n", result.TokenSummary.ProposedName)
	fmt.Printf("  Issued At:     %s\n", result.TokenSummary.IssuedAt.Format("2006-01-02 15:04:05"))
	fmt.Printf("  Expires At:    %s\n", result.TokenSummary.ExpiresAt.Format("2006-01-02 15:04:05"))
	fmt.Printf("  Scopes:        %d\n", result.TokenSummary.ScopeCount)
	fmt.Printf("  Ceilings:      %d\n", result.TokenSummary.CeilingCount)
	fmt.Printf("  Signature:     %s\n", result.TokenSummary.SignatureRedacted)
	fmt.Printf("  Algorithm:     %s\n", result.TokenSummary.Algorithm)
	fmt.Println()

	fmt.Println("------------------------------------------------------------")
	fmt.Println("  INTERSECTION CONTRACT SUMMARY")
	fmt.Println("------------------------------------------------------------")
	fmt.Println()
	fmt.Printf("  Intersection ID: %s\n", result.IntersectionSummary.ID)
	fmt.Printf("  Name:            %s\n", result.IntersectionSummary.Name)
	fmt.Printf("  Version:         %s\n", result.IntersectionSummary.Version)
	fmt.Printf("  Parties:         %s\n", strings.Join(result.IntersectionSummary.PartyIDs, ", "))
	fmt.Printf("  Scopes:          %s\n", strings.Join(result.IntersectionSummary.Scopes, ", "))
	fmt.Println("  Ceilings:")
	for _, c := range result.IntersectionSummary.Ceilings {
		fmt.Printf("    - %s: %s %s\n", c.Type, c.Value, c.Unit)
	}
	fmt.Printf("  Governance:      %s\n", result.IntersectionSummary.Governance)
	fmt.Printf("  Created At:      %s\n", result.IntersectionSummary.CreatedAt.Format("2006-01-02 15:04:05"))
	fmt.Println()

	fmt.Println("------------------------------------------------------------")
	fmt.Println("  INTERSECTION-SCOPED SUGGESTIONS (read-only, no execution)")
	fmt.Println("------------------------------------------------------------")
	fmt.Println()

	for i, sug := range result.Suggestions {
		fmt.Printf("Suggestion %d:\n", i+1)
		fmt.Printf("  Intersection: %s\n", sug.IntersectionID)
		fmt.Printf("  Time Slot:    %s\n", sug.TimeSlot)
		fmt.Printf("  Category:     %s\n", sug.Category)
		fmt.Printf("  Description:  %s\n", sug.Description)
		fmt.Printf("  Scopes Used:  %s\n", strings.Join(sug.ScopesUsed, ", "))
		fmt.Printf("  Ceilings Applied:\n")
		for _, c := range sug.CeilingsApplied {
			fmt.Printf("    - %s\n", c)
		}
		fmt.Printf("  Why: %s\n", sug.Explanation)
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
		fmt.Printf("[%s] %s\n", entry.ID, entry.EventType)
		fmt.Printf("         Circle: %s | Intersection: %s\n", entry.CircleID, intID)
		fmt.Printf("         Action: %s | Outcome: %s\n", entry.Action, entry.Outcome)
		fmt.Println()
	}

	fmt.Println("============================================================")
	fmt.Println("  Demo completed. SUGGEST-ONLY mode: no external actions.")
	fmt.Println("  All suggestions respect intersection scopes and ceilings.")
	fmt.Println("============================================================")
}
