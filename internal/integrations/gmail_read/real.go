// Package gmail_read provides a read-only adapter for Gmail integration.
//
// This file implements the real HTTP-based Gmail adapter.
// CRITICAL: This adapter is READ-ONLY. It NEVER writes to Gmail.
//
// Reference: docs/INTEGRATIONS_MATRIX_V1.md
package gmail_read

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"quantumlife/internal/connectors/auth"
	"quantumlife/internal/connectors/auth/impl_inmem"
	"quantumlife/pkg/clock"
	"quantumlife/pkg/domain/events"
	"quantumlife/pkg/domain/identity"
)

const (
	// Gmail API base URL
	gmailAPIBase = "https://gmail.googleapis.com/gmail/v1"

	// Default max results per page
	defaultMaxResults = 100
)

// TokenMinter mints access tokens for read-only operations.
// This interface allows injection for testing.
type TokenMinter interface {
	MintReadOnlyAccessToken(ctx context.Context, circleID string, provider auth.ProviderID, requiredScopes []string) (auth.AccessToken, error)
}

// RealAdapter implements the Gmail read adapter using real HTTP calls.
type RealAdapter struct {
	broker     TokenMinter
	httpClient *http.Client
	clock      clock.Clock
	circleID   string
}

// NewRealAdapter creates a new real Gmail adapter.
func NewRealAdapter(broker *impl_inmem.Broker, clk clock.Clock, circleID string) *RealAdapter {
	return &RealAdapter{
		broker:     broker,
		httpClient: &http.Client{Timeout: 30 * time.Second},
		clock:      clk,
		circleID:   circleID,
	}
}

// NewRealAdapterWithClient creates a real adapter with custom HTTP client (for testing).
func NewRealAdapterWithClient(broker TokenMinter, httpClient *http.Client, clk clock.Clock, circleID string) *RealAdapter {
	return &RealAdapter{
		broker:     broker,
		httpClient: httpClient,
		clock:      clk,
		circleID:   circleID,
	}
}

func (a *RealAdapter) Name() string {
	return "gmail_real"
}

// FetchMessages retrieves messages from Gmail and returns canonical events.
func (a *RealAdapter) FetchMessages(accountEmail string, since time.Time, limit int) ([]*events.EmailMessageEvent, error) {
	ctx := context.Background()

	// Mint read-only access token
	token, err := a.broker.MintReadOnlyAccessToken(ctx, a.circleID, auth.ProviderGoogle, []string{"email:read"})
	if err != nil {
		return nil, fmt.Errorf("mint token: %w", err)
	}

	// Build query
	query := "in:inbox"
	if !since.IsZero() {
		// Gmail uses 'after:' with epoch seconds
		query += fmt.Sprintf(" after:%d", since.Unix())
	}

	// List message IDs
	messageIDs, err := a.listMessageIDs(ctx, token.Token, accountEmail, query, limit)
	if err != nil {
		return nil, fmt.Errorf("list messages: %w", err)
	}

	// Fetch full message details
	now := a.clock.Now()
	var result []*events.EmailMessageEvent

	for _, msgID := range messageIDs {
		msg, err := a.getMessage(ctx, token.Token, accountEmail, msgID)
		if err != nil {
			// Skip individual message errors, continue with others
			continue
		}

		event := a.messageToEvent(accountEmail, msg, now)
		result = append(result, event)
	}

	return result, nil
}

// FetchUnreadCount returns the count of unread messages.
func (a *RealAdapter) FetchUnreadCount(accountEmail string) (int, error) {
	ctx := context.Background()

	// Mint read-only access token
	token, err := a.broker.MintReadOnlyAccessToken(ctx, a.circleID, auth.ProviderGoogle, []string{"email:read"})
	if err != nil {
		return 0, fmt.Errorf("mint token: %w", err)
	}

	// List unread messages
	messageIDs, err := a.listMessageIDs(ctx, token.Token, accountEmail, "in:inbox is:unread", 0)
	if err != nil {
		return 0, fmt.Errorf("list unread: %w", err)
	}

	return len(messageIDs), nil
}

// listMessageIDs lists message IDs matching the query.
func (a *RealAdapter) listMessageIDs(ctx context.Context, accessToken, accountEmail, query string, limit int) ([]string, error) {
	endpoint := fmt.Sprintf("%s/users/me/messages", gmailAPIBase)

	params := url.Values{}
	params.Set("q", query)
	if limit > 0 {
		params.Set("maxResults", fmt.Sprintf("%d", limit))
	} else {
		params.Set("maxResults", fmt.Sprintf("%d", defaultMaxResults))
	}

	req, err := http.NewRequestWithContext(ctx, "GET", endpoint+"?"+params.Encode(), nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+accessToken)

	resp, err := a.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("gmail API error: %d - %s", resp.StatusCode, string(body))
	}

	var listResp gmailListResponse
	if err := json.NewDecoder(resp.Body).Decode(&listResp); err != nil {
		return nil, err
	}

	var ids []string
	for _, msg := range listResp.Messages {
		ids = append(ids, msg.ID)
	}

	return ids, nil
}

// getMessage fetches a single message by ID.
func (a *RealAdapter) getMessage(ctx context.Context, accessToken, accountEmail, messageID string) (*gmailMessage, error) {
	endpoint := fmt.Sprintf("%s/users/me/messages/%s", gmailAPIBase, messageID)

	params := url.Values{}
	params.Set("format", "metadata")
	params.Set("metadataHeaders", "From")
	params.Set("metadataHeaders", "To")
	params.Set("metadataHeaders", "Subject")
	params.Set("metadataHeaders", "Date")

	req, err := http.NewRequestWithContext(ctx, "GET", endpoint+"?"+params.Encode(), nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+accessToken)

	resp, err := a.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("gmail API error: %d - %s", resp.StatusCode, string(body))
	}

	var msg gmailMessage
	if err := json.NewDecoder(resp.Body).Decode(&msg); err != nil {
		return nil, err
	}

	return &msg, nil
}

// messageToEvent converts a Gmail message to a canonical event.
func (a *RealAdapter) messageToEvent(accountEmail string, msg *gmailMessage, capturedAt time.Time) *events.EmailMessageEvent {
	// Parse headers
	headers := make(map[string]string)
	for _, h := range msg.Payload.Headers {
		headers[h.Name] = h.Value
	}

	// Parse date
	var occurredAt time.Time
	if dateStr := headers["Date"]; dateStr != "" {
		// Try common date formats
		formats := []string{
			time.RFC1123Z,
			time.RFC1123,
			"Mon, 2 Jan 2006 15:04:05 -0700",
			"2 Jan 2006 15:04:05 -0700",
		}
		for _, format := range formats {
			if t, err := time.Parse(format, dateStr); err == nil {
				occurredAt = t
				break
			}
		}
	}
	if occurredAt.IsZero() {
		// Use internal date as fallback (milliseconds since epoch)
		occurredAt = time.UnixMilli(msg.InternalDate)
	}

	event := events.NewEmailMessageEvent(
		"gmail",
		msg.ID,
		accountEmail,
		capturedAt,
		occurredAt,
	)

	event.ThreadID = msg.ThreadID
	event.Subject = headers["Subject"]

	// Parse From
	if from := headers["From"]; from != "" {
		event.From = parseEmailAddress(from)
		event.SenderDomain = extractDomain(event.From.Address)
	}

	// Parse To
	if to := headers["To"]; to != "" {
		event.To = parseEmailAddresses(to)
	}

	// Parse snippet as body preview
	event.BodyPreview = msg.Snippet

	// Parse labels for flags
	for _, label := range msg.LabelIDs {
		switch label {
		case "UNREAD":
			event.IsRead = false
		case "STARRED":
			event.IsStarred = true
		case "IMPORTANT":
			event.IsImportant = true
		case "INBOX":
			event.Folder = "INBOX"
		case "SENT":
			event.Folder = "SENT"
		case "TRASH":
			event.Folder = "TRASH"
		case "SPAM":
			event.Folder = "SPAM"
		case "CATEGORY_PROMOTIONS":
			event.IsAutomated = true
			event.Labels = append(event.Labels, "promotions")
		case "CATEGORY_UPDATES":
			event.IsTransactional = true
			event.Labels = append(event.Labels, "updates")
		case "CATEGORY_SOCIAL":
			event.Labels = append(event.Labels, "social")
		default:
			if !strings.HasPrefix(label, "CATEGORY_") && !strings.HasPrefix(label, "Label_") {
				event.Labels = append(event.Labels, label)
			}
		}
	}

	// Default folder if not set
	if event.Folder == "" {
		event.Folder = "INBOX"
	}

	// Check for unread - if UNREAD label is NOT present, it's read
	hasUnread := false
	for _, l := range msg.LabelIDs {
		if l == "UNREAD" {
			hasUnread = true
			break
		}
	}
	event.IsRead = !hasUnread

	return event
}

// Gmail API response types

type gmailListResponse struct {
	Messages           []gmailMessageRef `json:"messages"`
	NextPageToken      string            `json:"nextPageToken"`
	ResultSizeEstimate int               `json:"resultSizeEstimate"`
}

type gmailMessageRef struct {
	ID       string `json:"id"`
	ThreadID string `json:"threadId"`
}

type gmailMessage struct {
	ID           string       `json:"id"`
	ThreadID     string       `json:"threadId"`
	LabelIDs     []string     `json:"labelIds"`
	Snippet      string       `json:"snippet"`
	InternalDate int64        `json:"internalDate,string"`
	Payload      gmailPayload `json:"payload"`
}

type gmailPayload struct {
	Headers []gmailHeader `json:"headers"`
	Body    gmailBody     `json:"body"`
	Parts   []gmailPart   `json:"parts"`
}

type gmailHeader struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}

type gmailBody struct {
	Size int    `json:"size"`
	Data string `json:"data"` // base64url encoded
}

type gmailPart struct {
	PartID   string      `json:"partId"`
	MimeType string      `json:"mimeType"`
	Body     gmailBody   `json:"body"`
	Parts    []gmailPart `json:"parts"`
}

// Helper functions

// parseEmailAddress parses "Name <email@example.com>" format.
func parseEmailAddress(s string) events.EmailAddress {
	s = strings.TrimSpace(s)

	// Check for "Name <email>" format
	if idx := strings.LastIndex(s, "<"); idx != -1 {
		if endIdx := strings.LastIndex(s, ">"); endIdx > idx {
			return events.EmailAddress{
				Name:    strings.TrimSpace(s[:idx]),
				Address: strings.TrimSpace(s[idx+1 : endIdx]),
			}
		}
	}

	// Plain email
	return events.EmailAddress{Address: s}
}

// parseEmailAddresses parses comma-separated email addresses.
func parseEmailAddresses(s string) []events.EmailAddress {
	var result []events.EmailAddress
	for _, part := range strings.Split(s, ",") {
		part = strings.TrimSpace(part)
		if part != "" {
			result = append(result, parseEmailAddress(part))
		}
	}
	return result
}

// extractDomain extracts the domain from an email address.
func extractDomain(email string) string {
	idx := strings.LastIndex(email, "@")
	if idx == -1 {
		return ""
	}
	return email[idx+1:]
}

// decodeBase64URL decodes base64url-encoded data.
func decodeBase64URL(s string) (string, error) {
	data, err := base64.URLEncoding.DecodeString(s)
	if err != nil {
		// Try standard base64
		data, err = base64.StdEncoding.DecodeString(s)
		if err != nil {
			return "", err
		}
	}
	return string(data), nil
}

// Verify interface compliance.
var _ Adapter = (*RealAdapter)(nil)

// Unused but kept for reference - used in identity normalization
var _ = identity.EntityID("")
