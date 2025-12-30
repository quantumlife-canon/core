// Command quantumlife-cli is the QuantumLife command-line interface.
//
// Commands:
//
//	auth <provider>      Start OAuth flow for a provider
//	auth exchange        Exchange authorization code for tokens
//	demo family          Run family calendar demo
//	execute create-event Create a calendar event (v6 Execute mode)
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
	fmt.Println("  execute create-event Create a calendar event (v6 Execute mode)")
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
	fmt.Println("  # Create a real calendar event (REQUIRES --approve)")
	fmt.Println("  quantumlife-cli execute create-event --provider google --circle my-circle \\")
	fmt.Println("    --title 'Team Meeting' --start '2025-01-20T10:00:00Z' --duration 60 --approve")
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
