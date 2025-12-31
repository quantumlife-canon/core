// Command quantumlife-cli is the QuantumLife command-line interface.
//
// Commands:
//
//	auth <provider>      Start OAuth flow for a provider
//	auth exchange        Exchange authorization code for tokens
//	demo family          Run family calendar demo
//	execute create-event Create a calendar event (v6 Execute mode)
//	approval request     Request multi-party approval for an action (v7)
//	approval approve     Submit approval for an action (v7)
//
// Reference: docs/TECHNOLOGY_SELECTION_V1.md
package main

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"flag"
	"fmt"
	"os"
	"strings"
	"time"

	actionImpl "quantumlife/internal/action/impl_inmem"
	"quantumlife/internal/approval"
	approvalImpl "quantumlife/internal/approval/impl_inmem"
	auditImpl "quantumlife/internal/audit/impl_inmem"
	authorityImpl "quantumlife/internal/authority/impl_inmem"
	"quantumlife/internal/circle"
	circleImpl "quantumlife/internal/circle/impl_inmem"
	"quantumlife/internal/cli/state"
	"quantumlife/internal/connectors/auth"
	authImpl "quantumlife/internal/connectors/auth/impl_inmem"
	"quantumlife/internal/connectors/calendar"
	"quantumlife/internal/connectors/calendar/providers/google"
	"quantumlife/internal/connectors/calendar/providers/microsoft"
	demoCalendar "quantumlife/internal/demo_family_calendar"
	demoFinance "quantumlife/internal/demo_finance_read"
	"quantumlife/internal/intersection"
	intersectionImpl "quantumlife/internal/intersection/impl_inmem"
	revocationImpl "quantumlife/internal/revocation/impl_inmem"
	"quantumlife/pkg/primitives"
)

const version = "0.1.0"

func main() {
	if len(os.Args) < 2 {
		printUsage()
		os.Exit(0)
	}

	cmd := os.Args[1]

	switch cmd {
	case "auth":
		handleAuth(os.Args[2:])
	case "demo":
		handleDemo(os.Args[2:])
	case "execute":
		handleExecute(os.Args[2:])
	case "approval":
		handleApproval(os.Args[2:])
	case "version":
		fmt.Printf("quantumlife-cli v%s\n", version)
	case "help", "-h", "--help":
		printUsage()
	default:
		fmt.Fprintf(os.Stderr, "Unknown command: %s\n\n", cmd)
		printUsage()
		os.Exit(1)
	}
}

func printUsage() {
	fmt.Println("QuantumLife CLI v" + version)
	fmt.Println()
	fmt.Println("Usage:")
	fmt.Println("  quantumlife-cli <command> [options]")
	fmt.Println()
	fmt.Println("Commands:")
	fmt.Println("  auth <provider>    Start OAuth flow (prints auth URL)")
	fmt.Println("  auth exchange      Exchange authorization code for tokens")
	fmt.Println("  demo family        Run family calendar demo (read-only)")
	fmt.Println("  demo finance-read  Run financial read demo (v8)")
	fmt.Println("  execute create-event Create a calendar event (v6 Execute mode)")
	fmt.Println("  approval request   Request multi-party approval for an action (v7)")
	fmt.Println("  approval approve   Submit approval for an action (v7)")
	fmt.Println("  version            Print version")
	fmt.Println("  help               Show this help")
	fmt.Println()
	fmt.Println("Examples:")
	fmt.Println("  # Start OAuth for Google Calendar")
	fmt.Println("  quantumlife-cli auth google --circle my-circle --redirect http://localhost:8080/callback")
	fmt.Println()
	fmt.Println("  # Start OAuth for TrueLayer (Open Banking)")
	fmt.Println("  quantumlife-cli auth truelayer --circle my-circle --redirect http://localhost:8080/callback")
	fmt.Println()
	fmt.Println("  # Start Plaid Link flow (Financial Data)")
	fmt.Println("  quantumlife-cli auth plaid --circle my-circle --redirect http://localhost:8080/callback")
	fmt.Println()
	fmt.Println("  # Exchange the authorization code")
	fmt.Println("  quantumlife-cli auth exchange --provider google --circle my-circle \\")
	fmt.Println("    --code AUTH_CODE --redirect http://localhost:8080/callback")
	fmt.Println()
	fmt.Println("  # Run demo with mock provider")
	fmt.Println("  quantumlife-cli demo family --provider mock --circleA parent --circleB child")
	fmt.Println()
	fmt.Println("  # Run financial read demo (v8)")
	fmt.Println("  quantumlife-cli demo finance-read --provider mock --circleA alice --circleB bob")
	fmt.Println()
	fmt.Println("  # Create a real calendar event (REQUIRES --approve)")
	fmt.Println("  quantumlife-cli execute create-event --provider google --circle my-circle \\")
	fmt.Println("    --title 'Team Meeting' --start '2025-01-20T10:00:00Z' --duration 60 --approve")
	fmt.Println()
	fmt.Println("  # Request multi-party approval (v7)")
	fmt.Println("  quantumlife-cli approval request --intersection <id> --action <action-id> \\")
	fmt.Println("    --circle <requesting-circle>")
	fmt.Println()
	fmt.Println("  # Submit approval (v7)")
	fmt.Println("  quantumlife-cli approval approve --token <token> --circle <approving-circle>")
	fmt.Println()
	fmt.Println("Environment Variables:")
	fmt.Println("  GOOGLE_CLIENT_ID, GOOGLE_CLIENT_SECRET     Google OAuth credentials")
	fmt.Println("  MICROSOFT_CLIENT_ID, MICROSOFT_CLIENT_SECRET, MICROSOFT_TENANT_ID")
	fmt.Println("  TRUELAYER_CLIENT_ID, TRUELAYER_CLIENT_SECRET, TRUELAYER_ENV (v8.2)")
	fmt.Println("  PLAID_CLIENT_ID, PLAID_SECRET, PLAID_ENV (v8.3)")
	fmt.Println("  TOKEN_ENC_KEY     Encryption key for token persistence (required for real providers)")
}

// handleAuth handles the auth command and subcommands.
func handleAuth(args []string) {
	if len(args) == 0 {
		printAuthUsage()
		os.Exit(1)
	}

	subCmd := args[0]

	switch subCmd {
	case "exchange":
		handleAuthExchange(args[1:])
	case "status":
		handleAuthStatus(args[1:])
	case "google", "microsoft", "truelayer", "plaid":
		handleAuthStart(subCmd, args[1:])
	case "help", "-h", "--help":
		printAuthUsage()
	default:
		fmt.Fprintf(os.Stderr, "Unknown auth command: %s\n\n", subCmd)
		printAuthUsage()
		os.Exit(1)
	}
}

func printAuthUsage() {
	fmt.Println("Usage:")
	fmt.Println("  quantumlife-cli auth <provider> [options]")
	fmt.Println("  quantumlife-cli auth exchange [options]")
	fmt.Println("  quantumlife-cli auth status [options]")
	fmt.Println()
	fmt.Println("Start OAuth/Link flow:")
	fmt.Println("  quantumlife-cli auth <google|microsoft|truelayer|plaid> --circle <circle-id> --redirect <uri>")
	fmt.Println()
	fmt.Println("Options:")
	fmt.Println("  --circle     Circle ID that will own the credentials")
	fmt.Println("  --redirect   OAuth redirect URI")
	fmt.Println("  --scopes     Scopes to request")
	fmt.Println("               Calendar providers: calendar:read (default)")
	fmt.Println("               TrueLayer (finance): finance:read (default)")
	fmt.Println("               Plaid (finance): finance:read (default)")
	fmt.Println()
	fmt.Println("Exchange authorization code:")
	fmt.Println("  quantumlife-cli auth exchange --provider <google|microsoft|truelayer|plaid> --circle <id> \\")
	fmt.Println("    --code <auth-code> --redirect <uri>")
	fmt.Println()
	fmt.Println("TrueLayer (Open Banking, v8.2):")
	fmt.Println("  CRITICAL: TrueLayer is READ-ONLY. No payment scopes are allowed.")
	fmt.Println("  Environment: TRUELAYER_CLIENT_ID, TRUELAYER_CLIENT_SECRET, TRUELAYER_ENV")
	fmt.Println()
	fmt.Println("Plaid (Financial Data, v8.3):")
	fmt.Println("  CRITICAL: Plaid is READ-ONLY. No payment/transfer products are allowed.")
	fmt.Println("  Environment: PLAID_CLIENT_ID, PLAID_SECRET, PLAID_ENV")
}

// handleAuthStart handles starting the OAuth flow.
func handleAuthStart(provider string, args []string) {
	// Determine default scopes based on provider
	defaultScopes := "calendar:read"
	if provider == "truelayer" || provider == "plaid" {
		defaultScopes = "finance:read"
	}

	fs := flag.NewFlagSet("auth "+provider, flag.ExitOnError)
	circleID := fs.String("circle", "", "Circle ID")
	redirectURI := fs.String("redirect", "", "OAuth redirect URI")
	scopesStr := fs.String("scopes", defaultScopes, "Comma-separated scopes")

	if err := fs.Parse(args); err != nil {
		os.Exit(1)
	}

	if *circleID == "" {
		fmt.Fprintln(os.Stderr, "Error: --circle is required")
		os.Exit(1)
	}
	if *redirectURI == "" {
		fmt.Fprintln(os.Stderr, "Error: --redirect is required")
		os.Exit(1)
	}

	// Parse scopes
	scopes := strings.Split(*scopesStr, ",")
	for i := range scopes {
		scopes[i] = strings.TrimSpace(scopes[i])
	}

	// Load config
	config := auth.LoadConfigFromEnv()

	// Check provider configuration
	providerID := auth.ProviderID(provider)
	if !config.IsProviderConfigured(providerID) {
		fmt.Fprintf(os.Stderr, "Error: %s is not configured.\n", provider)
		fmt.Fprintln(os.Stderr)
		switch provider {
		case "google":
			fmt.Fprintln(os.Stderr, "Set GOOGLE_CLIENT_ID and GOOGLE_CLIENT_SECRET environment variables.")
		case "microsoft":
			fmt.Fprintln(os.Stderr, "Set MICROSOFT_CLIENT_ID, MICROSOFT_CLIENT_SECRET, and MICROSOFT_TENANT_ID.")
		case "truelayer":
			fmt.Fprintln(os.Stderr, "Set TRUELAYER_CLIENT_ID, TRUELAYER_CLIENT_SECRET, and optionally TRUELAYER_ENV.")
		case "plaid":
			fmt.Fprintln(os.Stderr, "Set PLAID_CLIENT_ID, PLAID_SECRET, and optionally PLAID_ENV.")
		}
		os.Exit(1)
	}

	// Create broker (no authority check needed for auth start)
	broker := authImpl.NewBroker(config, nil)

	// Generate state parameter (CSRF protection)
	stateBytes := make([]byte, 16)
	if _, err := rand.Read(stateBytes); err != nil {
		fmt.Fprintf(os.Stderr, "Error generating state: %v\n", err)
		os.Exit(1)
	}
	stateParam := hex.EncodeToString(stateBytes)

	// Generate auth URL
	authURL, err := broker.BeginOAuth(providerID, *redirectURI, stateParam, scopes)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	fmt.Println("OAuth Authorization URL")
	fmt.Println("=======================")
	fmt.Println()
	fmt.Println("Open this URL in your browser to authorize access:")
	fmt.Println()
	fmt.Println(authURL)
	fmt.Println()
	fmt.Println("After authorization, you will be redirected to:", *redirectURI)
	fmt.Println()
	fmt.Println("Copy the 'code' parameter from the redirect URL and run:")
	fmt.Println()
	fmt.Printf("  quantumlife-cli auth exchange --provider %s --circle %s \\\n", provider, *circleID)
	fmt.Printf("    --code <AUTH_CODE> --redirect %s\n", *redirectURI)
	fmt.Println()
	fmt.Println("State parameter (for verification):", stateParam)
}

// handleAuthExchange handles exchanging an auth code for tokens.
func handleAuthExchange(args []string) {
	fs := flag.NewFlagSet("auth exchange", flag.ExitOnError)
	provider := fs.String("provider", "", "Provider (google or microsoft)")
	circleID := fs.String("circle", "", "Circle ID")
	code := fs.String("code", "", "Authorization code")
	redirectURI := fs.String("redirect", "", "OAuth redirect URI (must match original)")

	if err := fs.Parse(args); err != nil {
		os.Exit(1)
	}

	if *provider == "" {
		fmt.Fprintln(os.Stderr, "Error: --provider is required")
		os.Exit(1)
	}
	if *circleID == "" {
		fmt.Fprintln(os.Stderr, "Error: --circle is required")
		os.Exit(1)
	}
	if *code == "" {
		fmt.Fprintln(os.Stderr, "Error: --code is required")
		os.Exit(1)
	}
	if *redirectURI == "" {
		fmt.Fprintln(os.Stderr, "Error: --redirect is required")
		os.Exit(1)
	}

	// Load config
	config := auth.LoadConfigFromEnv()

	// Check provider configuration
	providerID := auth.ProviderID(*provider)
	if !auth.IsValidProvider(providerID) {
		fmt.Fprintf(os.Stderr, "Error: invalid provider '%s'. Use 'google', 'microsoft', 'truelayer', or 'plaid'.\n", *provider)
		os.Exit(1)
	}

	if !config.IsProviderConfigured(providerID) {
		fmt.Fprintf(os.Stderr, "Error: %s is not configured.\n", *provider)
		os.Exit(1)
	}

	// Check encryption key
	if config.TokenEncryptionKey == "" {
		fmt.Fprintln(os.Stderr, "Warning: TOKEN_ENC_KEY not set. Tokens will not persist across CLI sessions.")
		fmt.Fprintln(os.Stderr)
	}

	// Create broker with persistence
	broker, err := authImpl.NewBrokerWithPersistence(config, nil)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error creating broker: %v\n", err)
		os.Exit(1)
	}

	// Exchange the code
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	handle, err := broker.ExchangeCodeForCircle(ctx, *circleID, providerID, *code, *redirectURI)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error exchanging code: %v\n", err)
		os.Exit(1)
	}

	// Store handle ID in CLI state
	cliState, err := state.Load()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Warning: failed to load CLI state: %v\n", err)
	} else {
		cliState.SetTokenHandle(*circleID, *provider, handle.ID)
		if err := cliState.Save(); err != nil {
			fmt.Fprintf(os.Stderr, "Warning: failed to save CLI state: %v\n", err)
		}
	}

	fmt.Println("Authorization Successful")
	fmt.Println("========================")
	fmt.Println()
	fmt.Println("Token Handle ID:", handle.ID)
	fmt.Println("Circle ID:      ", handle.CircleID)
	fmt.Println("Provider:       ", handle.Provider)
	fmt.Println("Scopes:         ", strings.Join(handle.Scopes, ", "))
	fmt.Println("Created:        ", handle.CreatedAt.Format(time.RFC3339))
	fmt.Println()
	fmt.Println("The token has been stored securely.")
	if broker.IsPersistenceEnabled() {
		fmt.Println("Persistence: ENABLED (will survive CLI restarts)")
	} else {
		fmt.Println("Persistence: DISABLED (token only valid for this session)")
	}
	fmt.Println()
	fmt.Println("You can now run the demo:")
	fmt.Printf("  quantumlife-cli demo family --provider %s --circleA %s --circleB <other-circle>\n", *provider, *circleID)
}

// handleAuthStatus handles the auth status command.
func handleAuthStatus(args []string) {
	fs := flag.NewFlagSet("auth status", flag.ExitOnError)
	circleID := fs.String("circle", "", "Circle ID (optional, shows all if not specified)")
	provider := fs.String("provider", "", "Provider (optional, shows all if not specified)")

	if err := fs.Parse(args); err != nil {
		os.Exit(1)
	}

	// Load config
	config := auth.LoadConfigFromEnv()

	// Check if persistence is enabled
	if config.TokenEncryptionKey == "" {
		fmt.Println("Token Status")
		fmt.Println("============")
		fmt.Println()
		fmt.Println("Warning: TOKEN_ENC_KEY not set. Token persistence is disabled.")
		fmt.Println("Tokens are not persisted across CLI sessions.")
		fmt.Println()
		fmt.Println("To enable token persistence, set TOKEN_ENC_KEY environment variable:")
		fmt.Println("  export TOKEN_ENC_KEY=$(openssl rand -hex 32)")
		return
	}

	// Create broker with persistence to read stored tokens
	broker, err := authImpl.NewBrokerWithPersistence(config, nil)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error loading token store: %v\n", err)
		os.Exit(1)
	}

	// Load CLI state for stored handles
	cliState, err := state.Load()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error loading CLI state: %v\n", err)
		os.Exit(1)
	}

	fmt.Println("Token Status")
	fmt.Println("============")
	fmt.Println()

	if broker.IsPersistenceEnabled() {
		fmt.Println("Persistence: ENABLED")
	} else {
		fmt.Println("Persistence: DISABLED")
	}
	fmt.Println()

	// Show configured providers
	fmt.Println("Configured Providers:")
	providers := []struct {
		id   auth.ProviderID
		name string
	}{
		{auth.ProviderGoogle, "Google"},
		{auth.ProviderMicrosoft, "Microsoft"},
		{auth.ProviderTrueLayer, "TrueLayer"},
		{auth.ProviderPlaid, "Plaid"},
	}
	for _, p := range providers {
		if config.IsProviderConfigured(p.id) {
			fmt.Printf("  ✓ %s\n", p.name)
		} else {
			fmt.Printf("  ✗ %s (not configured)\n", p.name)
		}
	}
	fmt.Println()

	// Get all stored tokens
	allHandles := cliState.GetAllTokenHandles()
	if len(allHandles) == 0 {
		fmt.Println("No stored tokens found.")
		fmt.Println()
		fmt.Println("To authorize a provider, run:")
		fmt.Println("  quantumlife-cli auth google --circle <circle-id> --redirect <uri>")
		return
	}

	fmt.Println("Stored Tokens:")
	fmt.Println()

	for key, handleID := range allHandles {
		// Parse key format: "circleID:provider"
		parts := strings.SplitN(key, ":", 2)
		if len(parts) != 2 {
			continue
		}
		tokenCircleID := parts[0]
		tokenProvider := parts[1]

		// Filter if circle or provider specified
		if *circleID != "" && tokenCircleID != *circleID {
			continue
		}
		if *provider != "" && tokenProvider != *provider {
			continue
		}

		// Try to get handle details from broker
		providerID := auth.ProviderID(tokenProvider)
		handle, found := broker.GetTokenHandle(tokenCircleID, providerID)

		fmt.Printf("  Circle:   %s\n", tokenCircleID)
		fmt.Printf("  Provider: %s\n", tokenProvider)
		fmt.Printf("  Handle:   %s\n", handleID)

		if found {
			fmt.Printf("  Scopes:   %s\n", strings.Join(handle.Scopes, ", "))
			fmt.Printf("  Created:  %s\n", handle.CreatedAt.Format(time.RFC3339))
			if !handle.ExpiresAt.IsZero() {
				fmt.Printf("  Expires:  %s\n", handle.ExpiresAt.Format(time.RFC3339))
			}
			fmt.Printf("  Status:   ✓ Valid\n")
		} else {
			fmt.Printf("  Status:   ⚠ Handle not found in broker store\n")
		}
		fmt.Println()
	}
}

// handleDemo handles the demo command and subcommands.
func handleDemo(args []string) {
	if len(args) == 0 {
		printDemoUsage()
		os.Exit(1)
	}

	subCmd := args[0]

	switch subCmd {
	case "family":
		handleDemoFamily(args[1:])
	case "finance-read":
		handleDemoFinanceRead(args[1:])
	case "help", "-h", "--help":
		printDemoUsage()
	default:
		fmt.Fprintf(os.Stderr, "Unknown demo command: %s\n\n", subCmd)
		printDemoUsage()
		os.Exit(1)
	}
}

func printDemoUsage() {
	fmt.Println("Usage:")
	fmt.Println("  quantumlife-cli demo family [options]")
	fmt.Println("  quantumlife-cli demo finance-read [options]")
	fmt.Println()
	fmt.Println("family options:")
	fmt.Println("  --provider   Provider to use: google, microsoft, or mock (default: mock)")
	fmt.Println("  --circleA    First circle ID (parent)")
	fmt.Println("  --circleB    Second circle ID (child)")
	fmt.Println("  --mode       Run mode: simulate (default)")
	fmt.Println()
	fmt.Println("finance-read options (v8):")
	fmt.Println("  --provider      Provider to use: mock, truelayer, or plaid (default: mock)")
	fmt.Println("  --circleA       First circle ID")
	fmt.Println("  --circleB       Second circle ID")
	fmt.Println("  --access-token  Access token for real providers (required for truelayer/plaid)")
	fmt.Println("  --dismiss       Proposal ID to dismiss (optional)")
	fmt.Println()
	fmt.Println("Note: TrueLayer and Plaid providers require valid access tokens.")
	fmt.Println("      CRITICAL: These providers are READ-ONLY. No payment/transfer operations.")
}

// handleDemoFamily runs the family calendar demo.
func handleDemoFamily(args []string) {
	fs := flag.NewFlagSet("demo family", flag.ExitOnError)
	provider := fs.String("provider", "mock", "Provider (google, microsoft, or mock)")
	circleA := fs.String("circleA", "circle-1", "First circle ID")
	circleB := fs.String("circleB", "circle-2", "Second circle ID")
	modeStr := fs.String("mode", "simulate", "Run mode (simulate)")

	if err := fs.Parse(args); err != nil {
		os.Exit(1)
	}

	// Validate mode
	var mode primitives.RunMode
	switch *modeStr {
	case "simulate":
		mode = primitives.ModeSimulate
	case "suggest_only":
		mode = primitives.ModeSuggestOnly
	default:
		fmt.Fprintf(os.Stderr, "Error: invalid mode '%s'. Use 'simulate' or 'suggest_only'.\n", *modeStr)
		os.Exit(1)
	}

	// For non-mock providers, check that we have stored tokens
	if *provider != "mock" {
		cliState, err := state.Load()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error loading CLI state: %v\n", err)
			os.Exit(1)
		}

		handleID, ok := cliState.GetTokenHandle(*circleA, *provider)
		if !ok {
			fmt.Fprintf(os.Stderr, "Error: No stored token for circle '%s' with provider '%s'.\n", *circleA, *provider)
			fmt.Fprintln(os.Stderr)
			fmt.Fprintln(os.Stderr, "To authorize, run:")
			fmt.Fprintf(os.Stderr, "  quantumlife-cli auth %s --circle %s --redirect <your-redirect-uri>\n", *provider, *circleA)
			os.Exit(1)
		}

		fmt.Printf("Using stored token handle: %s\n", handleID)
	}

	fmt.Println()
	fmt.Println("Running Family Calendar Demo")
	fmt.Println("============================")
	fmt.Printf("Provider:  %s\n", *provider)
	fmt.Printf("Circle A:  %s\n", *circleA)
	fmt.Printf("Circle B:  %s\n", *circleB)
	fmt.Printf("Mode:      %s\n", mode)
	fmt.Println()

	// Create and run the demo
	runner := demoCalendar.NewRunnerWithMode(mode)

	// For real providers with CLI state, we need to inject the broker
	// For now, this relies on the daemon demo runner which uses env vars
	// A full implementation would create a custom runner with CLI state
	if *provider != "mock" {
		config := auth.LoadConfigFromEnv()
		if config.TokenEncryptionKey == "" {
			fmt.Fprintln(os.Stderr, "Warning: TOKEN_ENC_KEY not set. Cannot use persistent tokens.")
			fmt.Fprintln(os.Stderr, "Falling back to mock provider.")
			*provider = "mock"
		}
	}

	ctx := context.Background()
	result, err := runner.Run(ctx)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Demo failed: %v\n", err)
		os.Exit(1)
	}

	// Print results
	printDemoResult(result)
}

// printDemoResult prints the demo result in CLI-friendly format.
func printDemoResult(result *demoCalendar.Result) {
	fmt.Println("Demo Results")
	fmt.Println("============")
	fmt.Println()

	if !result.Success {
		fmt.Printf("Status: FAILED - %s\n", result.Error)
		return
	}

	fmt.Println("Status: SUCCESS")
	fmt.Println()

	fmt.Println("Providers Used:", strings.Join(result.ProvidersUsed, ", "))
	if result.UsingMock {
		fmt.Println("  (Using mock data - no real provider configured)")
	}
	fmt.Println()

	fmt.Println("Intersection:", result.IntersectionID)
	fmt.Println("Contract Version:", result.ContractVersion)
	fmt.Println("Authorization Proof:", result.AuthorizationProofID)
	fmt.Println("Trace ID:", result.TraceID)
	fmt.Println()

	fmt.Println("Time Range:")
	fmt.Printf("  From: %s\n", result.TimeRange.Start.Format("Mon 02 Jan 2006 15:04 MST"))
	fmt.Printf("  To:   %s\n", result.TimeRange.End.Format("Mon 02 Jan 2006 15:04 MST"))
	fmt.Println()

	fmt.Printf("Events Fetched: %d\n", result.EventsFetched)
	for provider, count := range result.EventsByProvider {
		fmt.Printf("  - %s: %d events\n", provider, count)
	}
	fmt.Println()

	if len(result.Events) > 0 {
		fmt.Println("Calendar Events:")
		for i, evt := range result.Events {
			fmt.Printf("  [%d] %s\n", i+1, evt.Title)
			fmt.Printf("      %s - %s\n",
				evt.StartTime.Format("15:04"),
				evt.EndTime.Format("15:04"))
			if evt.Location != "" {
				fmt.Printf("      Location: %s\n", evt.Location)
			}
		}
		fmt.Println()
	}

	fmt.Printf("Free Slots Found: %d\n", result.FreeSlotsFound)
	if len(result.FreeSlots) > 0 {
		fmt.Println("Top Free Slots:")
		for i, slot := range result.FreeSlots {
			fmt.Printf("  [%d] %s - %s (%d min)\n",
				i+1,
				slot.Start.Format("15:04"),
				slot.End.Format("15:04"),
				int(slot.Duration.Minutes()))
		}
	}
	fmt.Println()

	fmt.Printf("Audit Entries: %d\n", len(result.AuditEntries))
	for _, entry := range result.AuditEntries {
		fmt.Printf("  - %s: %s (%s)\n", entry.EventType, entry.Action, entry.Outcome)
	}
}

// handleDemoFinanceRead runs the financial read demo (v8).
func handleDemoFinanceRead(args []string) {
	fs := flag.NewFlagSet("demo finance-read", flag.ExitOnError)
	provider := fs.String("provider", "mock", "Provider (mock, truelayer, or plaid)")
	circleA := fs.String("circleA", "circle-alice", "First circle ID")
	circleB := fs.String("circleB", "circle-bob", "Second circle ID")
	dismiss := fs.String("dismiss", "", "Proposal ID to dismiss (optional)")
	seed := fs.String("seed", "demo-seed-v8", "Seed for deterministic data generation")
	verbose := fs.Bool("verbose", true, "Enable verbose output")
	accessToken := fs.String("access-token", "", "Access token for real providers (truelayer or plaid)")

	if err := fs.Parse(args); err != nil {
		os.Exit(1)
	}

	// Validate provider
	validProviders := map[string]bool{"mock": true, "truelayer": true, "plaid": true}
	if !validProviders[*provider] {
		fmt.Fprintf(os.Stderr, "Warning: Invalid provider '%s'. Using mock.\n", *provider)
		*provider = "mock"
	}

	// Real providers require access token
	if (*provider == "truelayer" || *provider == "plaid") && *accessToken == "" {
		fmt.Fprintf(os.Stderr, "Error: Provider '%s' requires --access-token flag.\n", *provider)
		fmt.Fprintln(os.Stderr)
		fmt.Fprintln(os.Stderr, "To obtain an access token, first authenticate:")
		fmt.Fprintf(os.Stderr, "  quantumlife-cli auth %s --circle %s --redirect <uri>\n", *provider, *circleA)
		os.Exit(1)
	}

	config := demoFinance.DemoConfig{
		CircleAID:         *circleA,
		CircleBID:         *circleB,
		IntersectionID:    fmt.Sprintf("ix-%s-%s", *circleA, *circleB),
		ProviderID:        *provider,
		Seed:              *seed,
		DismissProposalID: *dismiss,
		Verbose:           *verbose,
	}

	// Set the appropriate access token
	if *provider == "truelayer" {
		config.TrueLayerAccessToken = *accessToken
	} else if *provider == "plaid" {
		config.PlaidAccessToken = *accessToken
	}

	ctx := context.Background()
	result, err := demoFinance.Run(ctx, config)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Demo failed: %v\n", err)
		os.Exit(1)
	}

	// Print summary
	printFinanceReadResult(result)
}

// printFinanceReadResult prints the finance read demo result.
func printFinanceReadResult(result *demoFinance.Result) {
	fmt.Println()
	fmt.Println("Demo Summary")
	fmt.Println("============")
	fmt.Println()

	// Circle A snapshot
	fmt.Println("Circle A Financial Snapshot:")
	fmt.Printf("  Circle ID:     %s\n", result.CircleASnapshot.CircleID)
	fmt.Printf("  Accounts:      %d\n", result.CircleASnapshot.AccountCount)
	fmt.Printf("  Transactions:  %d\n", result.CircleASnapshot.TransactionCount)
	fmt.Println()

	// Circle B snapshot
	fmt.Println("Circle B Financial Snapshot:")
	fmt.Printf("  Circle ID:     %s\n", result.CircleBSnapshot.CircleID)
	fmt.Printf("  Accounts:      %d\n", result.CircleBSnapshot.AccountCount)
	fmt.Printf("  Transactions:  %d\n", result.CircleBSnapshot.TransactionCount)
	fmt.Println()

	// Observations
	fmt.Printf("Observations Generated: %d\n", len(result.Observations))
	for _, obs := range result.Observations {
		fmt.Printf("  [%s] %s\n", obs.Severity, obs.Title)
	}
	fmt.Println()

	// Proposals
	fmt.Printf("Proposals Generated: %d\n", len(result.Proposals))
	for _, prop := range result.Proposals {
		fmt.Printf("  [%s] %s - %s\n", prop.Type, prop.Title, prop.Status)
	}

	// Silence
	if result.SilenceApplied {
		fmt.Println()
		fmt.Printf("Silence Applied: %s\n", result.SilenceReason)
	}

	// Dismissal
	if result.DismissalResult != "" {
		fmt.Println()
		fmt.Printf("Dismissal: %s\n", result.DismissalResult)
	}
	fmt.Println()
}

// ============================================================================
// Execute Command - v6 Execute Mode
// ============================================================================

// handleExecute handles the execute command and subcommands.
func handleExecute(args []string) {
	if len(args) == 0 {
		printExecuteUsage()
		os.Exit(1)
	}

	subCmd := args[0]

	switch subCmd {
	case "create-event":
		handleExecuteCreateEvent(args[1:])
	case "help", "-h", "--help":
		printExecuteUsage()
	default:
		fmt.Fprintf(os.Stderr, "Unknown execute command: %s\n\n", subCmd)
		printExecuteUsage()
		os.Exit(1)
	}
}

func printExecuteUsage() {
	fmt.Println("Execute Mode Commands (v6)")
	fmt.Println("==========================")
	fmt.Println()
	fmt.Println("CRITICAL: Execute mode performs REAL external writes.")
	fmt.Println("All write operations REQUIRE the --approve flag.")
	fmt.Println()
	fmt.Println("Usage:")
	fmt.Println("  quantumlife-cli execute create-event [options]")
	fmt.Println()
	fmt.Println("Create Event Options:")
	fmt.Println("  --approve          REQUIRED: Explicit human approval for write")
	fmt.Println("  --provider         Provider (google or microsoft)")
	fmt.Println("  --circle           Circle ID that owns the credentials")
	fmt.Println("  --title            Event title")
	fmt.Println("  --start            Start time (RFC3339 format)")
	fmt.Println("  --duration         Duration in minutes (default: 60)")
	fmt.Println("  --description      Event description (optional)")
	fmt.Println("  --location         Event location (optional)")
	fmt.Println()
	fmt.Println("Example:")
	fmt.Println("  quantumlife-cli execute create-event \\")
	fmt.Println("    --provider google \\")
	fmt.Println("    --circle my-circle \\")
	fmt.Println("    --title 'Team Meeting' \\")
	fmt.Println("    --start '2025-01-20T10:00:00Z' \\")
	fmt.Println("    --duration 60 \\")
	fmt.Println("    --approve")
}

// handleExecuteCreateEvent handles creating a real calendar event.
func handleExecuteCreateEvent(args []string) {
	fs := flag.NewFlagSet("execute create-event", flag.ExitOnError)
	approve := fs.Bool("approve", false, "REQUIRED: Explicit human approval")
	provider := fs.String("provider", "", "Provider (google or microsoft)")
	circleID := fs.String("circle", "", "Circle ID")
	title := fs.String("title", "", "Event title")
	startStr := fs.String("start", "", "Start time (RFC3339)")
	duration := fs.Int("duration", 60, "Duration in minutes")
	description := fs.String("description", "", "Event description")
	location := fs.String("location", "", "Event location")

	if err := fs.Parse(args); err != nil {
		os.Exit(1)
	}

	// CRITICAL: Require explicit approval
	if !*approve {
		fmt.Fprintln(os.Stderr, "ERROR: Execute mode requires explicit approval.")
		fmt.Fprintln(os.Stderr)
		fmt.Fprintln(os.Stderr, "This command will create a REAL calendar event on the external provider.")
		fmt.Fprintln(os.Stderr, "Add --approve to confirm you understand and authorize this write operation.")
		os.Exit(1)
	}

	// Validate required parameters
	if *provider == "" {
		fmt.Fprintln(os.Stderr, "Error: --provider is required")
		os.Exit(1)
	}
	if *circleID == "" {
		fmt.Fprintln(os.Stderr, "Error: --circle is required")
		os.Exit(1)
	}
	if *title == "" {
		fmt.Fprintln(os.Stderr, "Error: --title is required")
		os.Exit(1)
	}
	if *startStr == "" {
		fmt.Fprintln(os.Stderr, "Error: --start is required")
		os.Exit(1)
	}

	// Parse start time
	startTime, err := time.Parse(time.RFC3339, *startStr)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: invalid start time format: %v\n", err)
		fmt.Fprintln(os.Stderr, "Use RFC3339 format, e.g., '2025-01-20T10:00:00Z'")
		os.Exit(1)
	}
	endTime := startTime.Add(time.Duration(*duration) * time.Minute)

	// Validate provider
	providerID := auth.ProviderID(*provider)
	if !auth.IsValidProvider(providerID) {
		fmt.Fprintf(os.Stderr, "Error: invalid provider '%s'. Use 'google' or 'microsoft'.\n", *provider)
		os.Exit(1)
	}

	// Check CLI state for stored tokens
	cliState, err := state.Load()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error loading CLI state: %v\n", err)
		os.Exit(1)
	}

	handleID, ok := cliState.GetTokenHandle(*circleID, *provider)
	if !ok {
		fmt.Fprintf(os.Stderr, "Error: No stored token for circle '%s' with provider '%s'.\n", *circleID, *provider)
		fmt.Fprintln(os.Stderr)
		fmt.Fprintln(os.Stderr, "To authorize with write scope, run:")
		fmt.Fprintf(os.Stderr, "  quantumlife-cli auth %s --circle %s --redirect <uri> --scopes calendar:read,calendar:write\n", *provider, *circleID)
		os.Exit(1)
	}

	fmt.Println()
	fmt.Println("╔═══════════════════════════════════════════════════════════════╗")
	fmt.Println("║  EXECUTE MODE - Creating Real Calendar Event                  ║")
	fmt.Println("╠═══════════════════════════════════════════════════════════════╣")
	fmt.Printf("║  Provider:   %-48s ║\n", *provider)
	fmt.Printf("║  Circle:     %-48s ║\n", *circleID)
	fmt.Printf("║  Title:      %-48s ║\n", truncateString(*title, 48))
	fmt.Printf("║  Start:      %-48s ║\n", startTime.Format("Mon 02 Jan 2006 15:04 MST"))
	fmt.Printf("║  Duration:   %-48s ║\n", fmt.Sprintf("%d minutes", *duration))
	fmt.Println("║                                                               ║")
	fmt.Println("║  Approval:   GRANTED (--approve flag provided)                ║")
	fmt.Println("╚═══════════════════════════════════════════════════════════════╝")
	fmt.Println()

	// Run the execution pipeline
	ctx := context.Background()
	result := runExecutePipeline(ctx, *provider, *circleID, handleID, calendar.CreateEventRequest{
		Title:       *title,
		Description: *description,
		StartTime:   startTime,
		EndTime:     endTime,
		Location:    *location,
		CalendarID:  "primary",
	})

	printExecuteResult(result)

	if !result.Success {
		os.Exit(1)
	}
}

// runExecutePipeline runs the execution pipeline.
func runExecutePipeline(ctx context.Context, provider, circleID, handleID string, createReq calendar.CreateEventRequest) *actionImpl.ExecuteResult {
	// Load config and create broker
	config := auth.LoadConfigFromEnv()

	if config.TokenEncryptionKey == "" {
		return &actionImpl.ExecuteResult{
			Error: fmt.Errorf("TOKEN_ENC_KEY not set - cannot use persistent tokens"),
		}
	}

	broker, err := authImpl.NewBrokerWithPersistence(config, nil)
	if err != nil {
		return &actionImpl.ExecuteResult{
			Error: fmt.Errorf("failed to create broker: %w", err),
		}
	}

	// Create write connector
	var connector calendar.WriteConnector
	providerID := auth.ProviderID(provider)
	switch providerID {
	case auth.ProviderGoogle:
		connector = google.NewWriteAdapter(broker, true)
	case auth.ProviderMicrosoft:
		connector = microsoft.NewWriteAdapter(broker, true)
	default:
		return &actionImpl.ExecuteResult{
			Error: fmt.Errorf("unsupported provider for write: %s", provider),
		}
	}

	// Create infrastructure
	circleStore := circleImpl.NewRuntime()
	intersectionStore := intersectionImpl.NewRuntime()
	auditStore := auditImpl.NewStore()
	revocationRegistry := revocationImpl.NewRegistry()

	// Create demo circle (in real system, would look up existing circle)
	circ, err := circleStore.Create(ctx, circle.CreateRequest{TenantID: circleID})
	if err != nil {
		return &actionImpl.ExecuteResult{
			Error: fmt.Errorf("failed to create circle: %w", err),
		}
	}

	// Create intersection with calendar:write scope
	inter, err := intersectionStore.Create(ctx, intersection.CreateRequest{
		TenantID:    "cli-tenant",
		InitiatorID: circ.ID,
		AcceptorID:  circ.ID,
		Contract: intersection.Contract{
			Parties: []intersection.Party{
				{CircleID: circ.ID, PartyType: "initiator", JoinedAt: time.Now()},
			},
			Scopes: []intersection.Scope{
				{Name: "calendar:read", Description: "Read calendar events", ReadWrite: "read"},
				{Name: "calendar:write", Description: "Write calendar events", ReadWrite: "write"},
			},
			Ceilings: []intersection.Ceiling{
				{Type: "time_window", Value: "00:00-23:59", Unit: "daily"},
			},
		},
	})
	if err != nil {
		return &actionImpl.ExecuteResult{
			Error: fmt.Errorf("failed to create intersection: %w", err),
		}
	}

	// Create authority engine
	authorityEngine := authorityImpl.NewEngine(intersectionStore)

	// Create pipeline
	pipeline := actionImpl.NewPipeline(actionImpl.PipelineConfig{
		AuthorityEngine:   authorityEngine,
		RevocationChecker: revocationRegistry,
		AuditStore:        auditStore,
	})

	// Generate trace ID
	traceID := fmt.Sprintf("trace-execute-%d", time.Now().UnixNano())

	// Create action
	action := &primitives.Action{
		ID:             fmt.Sprintf("action-create-event-%d", time.Now().UnixNano()),
		IntersectionID: inter.ID,
		Type:           "calendar.create_event",
		Parameters:     map[string]string{"title": createReq.Title},
	}

	// Execute
	return pipeline.Execute(ctx, actionImpl.ExecuteRequest{
		TraceID:          traceID,
		ActorCircleID:    circ.ID,
		IntersectionID:   inter.ID,
		ContractVersion:  inter.Version,
		Action:           action,
		ApprovalArtifact: "cli:--approve",
		Connector:        connector,
		CreateRequest:    createReq,
	})
}

// printExecuteResult prints the execution result.
func printExecuteResult(result *actionImpl.ExecuteResult) {
	fmt.Println()
	fmt.Println("Execution Result")
	fmt.Println("================")
	fmt.Println()

	if result.Success {
		fmt.Println("Status: ✓ SUCCESS")
		fmt.Printf("Settlement: %s\n", result.SettlementStatus)
		fmt.Println()

		if result.Receipt != nil {
			fmt.Println("Event Created:")
			fmt.Printf("  Provider:     %s\n", result.Receipt.Provider)
			fmt.Printf("  Calendar:     %s\n", result.Receipt.CalendarID)
			fmt.Printf("  External ID:  %s\n", calendar.RedactedExternalID(result.Receipt.ExternalEventID))
			fmt.Printf("  Status:       %s\n", result.Receipt.Status)
			fmt.Printf("  Created At:   %s\n", result.Receipt.CreatedAt.Format(time.RFC3339))
			if result.Receipt.Link != "" {
				fmt.Printf("  Link:         %s\n", result.Receipt.Link)
			}
		}
	} else {
		fmt.Println("Status: ✗ FAILED")
		fmt.Printf("Settlement: %s\n", result.SettlementStatus)
		fmt.Printf("Error: %v\n", result.Error)

		if result.RolledBack {
			fmt.Println()
			fmt.Println("Rollback: Event was deleted (rollback successful)")
		}
		if result.RollbackError != nil {
			fmt.Println()
			fmt.Printf("Rollback Error: %v\n", result.RollbackError)
		}
	}

	fmt.Println()

	if result.AuthorizationProof != nil {
		fmt.Println("Authorization:")
		fmt.Printf("  Proof ID:     %s\n", result.AuthorizationProof.ID)
		fmt.Printf("  Authorized:   %v\n", result.AuthorizationProof.Authorized)
		fmt.Printf("  Approved:     %v (artifact: %s)\n", result.AuthorizationProof.ApprovedByHuman, result.AuthorizationProof.ApprovalArtifact)
	}
}

// truncateString truncates a string to maxLen characters.
func truncateString(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen-3] + "..."
}

// ============================================================================
// Approval Command - v7 Multi-party Approval
// ============================================================================

// handleApproval handles the approval command and subcommands.
func handleApproval(args []string) {
	if len(args) == 0 {
		printApprovalUsage()
		os.Exit(1)
	}

	subCmd := args[0]

	switch subCmd {
	case "request":
		handleApprovalRequest(args[1:])
	case "approve":
		handleApprovalApprove(args[1:])
	case "help", "-h", "--help":
		printApprovalUsage()
	default:
		fmt.Fprintf(os.Stderr, "Unknown approval command: %s\n\n", subCmd)
		printApprovalUsage()
		os.Exit(1)
	}
}

func printApprovalUsage() {
	fmt.Println("Multi-party Approval Commands (v7)")
	fmt.Println("==================================")
	fmt.Println()
	fmt.Println("Multi-party approval allows multiple circles to approve an action")
	fmt.Println("before it can be executed. This is required when an intersection's")
	fmt.Println("ApprovalPolicy has mode='multi'.")
	fmt.Println()
	fmt.Println("Usage:")
	fmt.Println("  quantumlife-cli approval request [options]")
	fmt.Println("  quantumlife-cli approval approve [options]")
	fmt.Println()
	fmt.Println("Request Approval:")
	fmt.Println("  --intersection   Intersection ID")
	fmt.Println("  --action         Action ID")
	fmt.Println("  --action-type    Action type (e.g., calendar.create_event)")
	fmt.Println("  --circle         Requesting circle ID")
	fmt.Println("  --title          Action title (for display)")
	fmt.Println("  --expiry         Token expiry in seconds (default: 3600)")
	fmt.Println()
	fmt.Println("Submit Approval:")
	fmt.Println("  --token          Approval request token")
	fmt.Println("  --circle         Approving circle ID")
	fmt.Println()
	fmt.Println("Examples:")
	fmt.Println("  # Request approval for an action")
	fmt.Println("  quantumlife-cli approval request \\")
	fmt.Println("    --intersection ix-123 \\")
	fmt.Println("    --action act-456 \\")
	fmt.Println("    --action-type calendar.create_event \\")
	fmt.Println("    --circle parent-circle \\")
	fmt.Println("    --title 'Team Meeting'")
	fmt.Println()
	fmt.Println("  # Share the token with approving circles, then they run:")
	fmt.Println("  quantumlife-cli approval approve \\")
	fmt.Println("    --token <token-from-request> \\")
	fmt.Println("    --circle child-circle")
}

// handleApprovalRequest handles creating an approval request.
func handleApprovalRequest(args []string) {
	fs := flag.NewFlagSet("approval request", flag.ExitOnError)
	intersectionID := fs.String("intersection", "", "Intersection ID")
	actionID := fs.String("action", "", "Action ID")
	actionType := fs.String("action-type", "calendar.create_event", "Action type")
	circleID := fs.String("circle", "", "Requesting circle ID")
	title := fs.String("title", "", "Action title")
	expirySeconds := fs.Int("expiry", 3600, "Token expiry in seconds")

	if err := fs.Parse(args); err != nil {
		os.Exit(1)
	}

	// Validate required parameters
	if *intersectionID == "" {
		fmt.Fprintln(os.Stderr, "Error: --intersection is required")
		os.Exit(1)
	}
	if *actionID == "" {
		fmt.Fprintln(os.Stderr, "Error: --action is required")
		os.Exit(1)
	}
	if *circleID == "" {
		fmt.Fprintln(os.Stderr, "Error: --circle is required")
		os.Exit(1)
	}

	// Use action ID as title if not provided
	if *title == "" {
		*title = *actionID
	}

	fmt.Println()
	fmt.Println("╔═══════════════════════════════════════════════════════════════╗")
	fmt.Println("║  APPROVAL REQUEST - Multi-party Approval (v7)                 ║")
	fmt.Println("╠═══════════════════════════════════════════════════════════════╣")
	fmt.Printf("║  Intersection: %-47s ║\n", truncateString(*intersectionID, 47))
	fmt.Printf("║  Action:       %-47s ║\n", truncateString(*actionID, 47))
	fmt.Printf("║  Type:         %-47s ║\n", truncateString(*actionType, 47))
	fmt.Printf("║  Circle:       %-47s ║\n", truncateString(*circleID, 47))
	fmt.Printf("║  Title:        %-47s ║\n", truncateString(*title, 47))
	fmt.Printf("║  Expiry:       %-47s ║\n", fmt.Sprintf("%d seconds", *expirySeconds))
	fmt.Println("╚═══════════════════════════════════════════════════════════════╝")
	fmt.Println()

	// Create approval store (in real system, would connect to shared store)
	auditStore := auditImpl.NewStore()
	approvalStore := approvalImpl.NewStore(approvalImpl.StoreConfig{
		AuditStore: auditStore,
	})

	// Create action
	action := &primitives.Action{
		ID:             *actionID,
		IntersectionID: *intersectionID,
		Type:           *actionType,
		Parameters:     map[string]string{"title": *title},
	}

	// Request approval
	ctx := context.Background()
	token, err := approvalStore.RequestApproval(ctx, approval.ApprovalRequest{
		IntersectionID:     *intersectionID,
		ContractVersion:    "v1",
		Action:             action,
		ScopesRequired:     []string{"calendar:write"},
		RequestingCircleID: *circleID,
		ExpirySeconds:      *expirySeconds,
		TraceID:            fmt.Sprintf("trace-approval-%d", time.Now().UnixNano()),
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error creating approval request: %v\n", err)
		os.Exit(1)
	}

	// Encode token for sharing
	encodedToken := primitives.EncodeApprovalToken(token)

	fmt.Println("Approval Request Created")
	fmt.Println("========================")
	fmt.Println()
	fmt.Println("Token ID:", token.TokenID)
	fmt.Println("Action Hash:", token.ActionHash[:16]+"...")
	fmt.Println("Expires At:", token.ExpiresAt.Format(time.RFC3339))
	fmt.Println()
	fmt.Println("Share this token with approving circles:")
	fmt.Println()
	fmt.Println("  " + encodedToken)
	fmt.Println()
	fmt.Println("Approving circles should run:")
	fmt.Println()
	fmt.Println("  quantumlife-cli approval approve \\")
	fmt.Printf("    --token '%s' \\\n", encodedToken)
	fmt.Println("    --circle <approving-circle-id>")
}

// handleApprovalApprove handles submitting an approval.
func handleApprovalApprove(args []string) {
	fs := flag.NewFlagSet("approval approve", flag.ExitOnError)
	token := fs.String("token", "", "Approval request token")
	circleID := fs.String("circle", "", "Approving circle ID")

	if err := fs.Parse(args); err != nil {
		os.Exit(1)
	}

	// Validate required parameters
	if *token == "" {
		fmt.Fprintln(os.Stderr, "Error: --token is required")
		os.Exit(1)
	}
	if *circleID == "" {
		fmt.Fprintln(os.Stderr, "Error: --circle is required")
		os.Exit(1)
	}

	// Decode the token to display info
	decodedToken, err := primitives.DecodeApprovalToken(*token)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: invalid token format: %v\n", err)
		os.Exit(1)
	}

	fmt.Println()
	fmt.Println("╔═══════════════════════════════════════════════════════════════╗")
	fmt.Println("║  APPROVAL SUBMISSION - Multi-party Approval (v7)              ║")
	fmt.Println("╠═══════════════════════════════════════════════════════════════╣")
	fmt.Printf("║  Token ID:     %-47s ║\n", truncateString(decodedToken.TokenID, 47))
	fmt.Printf("║  Intersection: %-47s ║\n", truncateString(decodedToken.IntersectionID, 47))
	fmt.Printf("║  Action:       %-47s ║\n", truncateString(decodedToken.ActionID, 47))
	fmt.Printf("║  Summary:      %-47s ║\n", truncateString(decodedToken.ActionSummary, 47))
	fmt.Printf("║  Approver:     %-47s ║\n", truncateString(*circleID, 47))
	fmt.Printf("║  Expires:      %-47s ║\n", decodedToken.ExpiresAt.Format(time.RFC3339))
	fmt.Println("╚═══════════════════════════════════════════════════════════════╝")
	fmt.Println()

	// Check if token has expired (local check)
	if decodedToken.IsExpired(time.Now()) {
		fmt.Fprintln(os.Stderr, "Error: approval request token has expired")
		os.Exit(1)
	}

	// Create approval store (in real system, would connect to shared store)
	auditStore := auditImpl.NewStore()
	approvalStore := approvalImpl.NewStore(approvalImpl.StoreConfig{
		AuditStore: auditStore,
	})

	// Submit approval
	ctx := context.Background()
	artifact, err := approvalStore.SubmitApproval(ctx, approval.SubmitApprovalRequest{
		Token:            *token,
		ApproverCircleID: *circleID,
		TraceID:          fmt.Sprintf("trace-approve-%d", time.Now().UnixNano()),
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error submitting approval: %v\n", err)
		os.Exit(1)
	}

	fmt.Println("Approval Submitted Successfully")
	fmt.Println("================================")
	fmt.Println()
	fmt.Println("Approval ID:", artifact.ApprovalID)
	fmt.Println("Circle ID:", artifact.ApproverCircleID)
	fmt.Println("Action ID:", artifact.ActionID)
	fmt.Println("Scopes:", strings.Join(artifact.ScopesApproved, ", "))
	fmt.Println("Approved At:", artifact.ApprovedAt.Format(time.RFC3339))
	fmt.Println("Expires At:", artifact.ExpiresAt.Format(time.RFC3339))
	fmt.Println()
	fmt.Println("This approval has been recorded.")
	fmt.Println("Once all required approvals are collected, the action can be executed.")
}
