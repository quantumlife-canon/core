// Command quantumlife-cli is the QuantumLife command-line interface.
//
// Commands:
//
//	auth <provider>      Start OAuth flow for a provider
//	auth exchange        Exchange authorization code for tokens
//	demo family          Run family calendar demo
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

	"quantumlife/internal/cli/state"
	"quantumlife/internal/connectors/auth"
	authImpl "quantumlife/internal/connectors/auth/impl_inmem"
	demoCalendar "quantumlife/internal/demo_family_calendar"
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
	fmt.Println("  demo family        Run family calendar demo")
	fmt.Println("  version            Print version")
	fmt.Println("  help               Show this help")
	fmt.Println()
	fmt.Println("Examples:")
	fmt.Println("  # Start OAuth for Google Calendar")
	fmt.Println("  quantumlife-cli auth google --circle my-circle --redirect http://localhost:8080/callback")
	fmt.Println()
	fmt.Println("  # Exchange the authorization code")
	fmt.Println("  quantumlife-cli auth exchange --provider google --circle my-circle \\")
	fmt.Println("    --code AUTH_CODE --redirect http://localhost:8080/callback")
	fmt.Println()
	fmt.Println("  # Run demo with mock provider")
	fmt.Println("  quantumlife-cli demo family --provider mock --circleA parent --circleB child")
	fmt.Println()
	fmt.Println("Environment Variables:")
	fmt.Println("  GOOGLE_CLIENT_ID, GOOGLE_CLIENT_SECRET     Google OAuth credentials")
	fmt.Println("  MICROSOFT_CLIENT_ID, MICROSOFT_CLIENT_SECRET, MICROSOFT_TENANT_ID")
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
	case "google", "microsoft":
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
	fmt.Println()
	fmt.Println("Start OAuth flow:")
	fmt.Println("  quantumlife-cli auth <google|microsoft> --circle <circle-id> --redirect <uri>")
	fmt.Println()
	fmt.Println("Options:")
	fmt.Println("  --circle     Circle ID that will own the credentials")
	fmt.Println("  --redirect   OAuth redirect URI")
	fmt.Println("  --scopes     Scopes to request (default: calendar:read)")
	fmt.Println()
	fmt.Println("Exchange authorization code:")
	fmt.Println("  quantumlife-cli auth exchange --provider <google|microsoft> --circle <id> \\")
	fmt.Println("    --code <auth-code> --redirect <uri>")
}

// handleAuthStart handles starting the OAuth flow.
func handleAuthStart(provider string, args []string) {
	fs := flag.NewFlagSet("auth "+provider, flag.ExitOnError)
	circleID := fs.String("circle", "", "Circle ID")
	redirectURI := fs.String("redirect", "", "OAuth redirect URI")
	scopesStr := fs.String("scopes", "calendar:read", "Comma-separated scopes")

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
		if provider == "google" {
			fmt.Fprintln(os.Stderr, "Set GOOGLE_CLIENT_ID and GOOGLE_CLIENT_SECRET environment variables.")
		} else {
			fmt.Fprintln(os.Stderr, "Set MICROSOFT_CLIENT_ID, MICROSOFT_CLIENT_SECRET, and MICROSOFT_TENANT_ID.")
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
		fmt.Fprintf(os.Stderr, "Error: invalid provider '%s'. Use 'google' or 'microsoft'.\n", *provider)
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
	fmt.Println()
	fmt.Println("Options:")
	fmt.Println("  --provider   Provider to use: google, microsoft, or mock (default: mock)")
	fmt.Println("  --circleA    First circle ID (parent)")
	fmt.Println("  --circleB    Second circle ID (child)")
	fmt.Println("  --mode       Run mode: simulate (default)")
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
