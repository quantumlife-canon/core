// Package reality provides domain types for Phase 26C: Connected Reality Check.
//
// This is NOT analytics. This is a trust proof page.
// It proves "this is real" (connected + synced + shadow mode status)
// WITHOUT showing any content, identifiers, raw counts, timestamps,
// vendors, people, or secrets.
//
// CRITICAL: All payloads contain only abstract buckets and hashes - never content.
// CRITICAL: No secrets ever rendered (no API keys, no tokens, no env values).
// CRITICAL: No time.Now() - clock injection only.
// CRITICAL: No goroutines.
// CRITICAL: stdlib only.
//
// Reference: docs/ADR/ADR-0057-phase26C-connected-reality-check.md
package reality

import (
	"crypto/sha256"
	"encoding/hex"
	"strings"
)

// RealityLineKind identifies the type of value in a reality line.
type RealityLineKind string

const (
	// LineKindBool represents yes/no values.
	LineKindBool RealityLineKind = "bool"

	// LineKindBucket represents magnitude buckets (nothing, a_few, several).
	LineKindBucket RealityLineKind = "bucket"

	// LineKindEnum represents enum values (provider kinds, sync status).
	LineKindEnum RealityLineKind = "enum"

	// LineKindNote represents informational notes.
	LineKindNote RealityLineKind = "note"
)

// RealityLine represents a single line in the reality proof page.
type RealityLine struct {
	// Label is the human-readable label for this line.
	Label string

	// Value is the abstract value (never raw content).
	Value string

	// Kind identifies the type of value.
	Kind RealityLineKind
}

// CanonicalString returns the pipe-delimited canonical representation.
func (l RealityLine) CanonicalString() string {
	return "LINE|" + l.Label + "|" + l.Value + "|" + string(l.Kind)
}

// RealityPage represents the complete reality proof page.
type RealityPage struct {
	// Title is the page title.
	Title string

	// Subtitle is the page subtitle.
	Subtitle string

	// Lines contains the status lines to display.
	Lines []RealityLine

	// CalmLine is the single calm summary line.
	CalmLine string

	// StatusHash is the deterministic hash of the page (32 hex chars).
	StatusHash string

	// BackPath is the path to return to (e.g., "/today").
	BackPath string
}

// CanonicalString returns the pipe-delimited canonical representation.
func (p *RealityPage) CanonicalString() string {
	var b strings.Builder
	b.WriteString("REALITY_PAGE|v1|")
	b.WriteString(p.Title)
	b.WriteString("|")
	b.WriteString(p.Subtitle)
	b.WriteString("|")

	// Lines in order (order is deterministic from inputs)
	for i, line := range p.Lines {
		if i > 0 {
			b.WriteString(",")
		}
		b.WriteString(line.CanonicalString())
	}

	b.WriteString("|")
	b.WriteString(p.CalmLine)
	b.WriteString("|")
	b.WriteString(p.BackPath)

	return b.String()
}

// ComputeStatusHash computes the deterministic hash of the page.
// Returns 32 hex characters (128 bits).
func (p *RealityPage) ComputeStatusHash() string {
	canonical := p.CanonicalString()
	h := sha256.Sum256([]byte(canonical))
	return hex.EncodeToString(h[:16])
}

// SyncBucket represents the abstract sync recency bucket.
type SyncBucket string

const (
	// SyncBucketNever indicates never synced.
	SyncBucketNever SyncBucket = "never"

	// SyncBucketRecent indicates synced recently (within bucketed time).
	SyncBucketRecent SyncBucket = "recent"

	// SyncBucketStale indicates synced but stale (beyond threshold).
	SyncBucketStale SyncBucket = "stale"

	// SyncBucketUnknown indicates unknown sync status.
	SyncBucketUnknown SyncBucket = "unknown"
)

// MagnitudeBucket represents an abstract quantity - never raw counts.
type MagnitudeBucket string

const (
	// MagnitudeNothing indicates zero items.
	MagnitudeNothing MagnitudeBucket = "nothing"

	// MagnitudeAFew indicates 1-5 items.
	MagnitudeAFew MagnitudeBucket = "a_few"

	// MagnitudeSeveral indicates 6+ items.
	MagnitudeSeveral MagnitudeBucket = "several"

	// MagnitudeNA indicates not applicable.
	MagnitudeNA MagnitudeBucket = "na"
)

// ShadowProviderKind represents the shadow provider type.
type ShadowProviderKind string

const (
	// ProviderOff indicates shadow mode is off.
	ProviderOff ShadowProviderKind = "off"

	// ProviderStub indicates stub provider (no real calls).
	ProviderStub ShadowProviderKind = "stub"

	// ProviderAzureChat indicates Azure OpenAI chat provider.
	ProviderAzureChat ShadowProviderKind = "azure_openai_chat"

	// ProviderAzureEmbed indicates Azure OpenAI embeddings provider.
	ProviderAzureEmbed ShadowProviderKind = "azure_openai_embed"
)

// RealityInputs captures all inputs needed to compute the reality page.
// These are gathered from existing stores - no new data collection.
type RealityInputs struct {
	// CircleID identifies the circle (required for scoping).
	CircleID string

	// NowBucket is a derived bucket string (e.g., period key) - not a timestamp.
	NowBucket string

	// GmailConnected indicates if Gmail is connected (yes/no).
	GmailConnected bool

	// SyncBucket indicates last sync recency (never/recent/stale/unknown).
	SyncBucket SyncBucket

	// SyncMagnitude indicates sync magnitude (nothing/a_few/several/na).
	SyncMagnitude MagnitudeBucket

	// ObligationsHeld indicates all obligations are held (should always be true).
	ObligationsHeld bool

	// AutoSurface indicates if auto-surface is enabled (should always be false).
	AutoSurface bool

	// ShadowProviderKind indicates the shadow provider type.
	ShadowProviderKind ShadowProviderKind

	// ShadowRealAllowed indicates if real providers are permitted.
	ShadowRealAllowed bool

	// ShadowMagnitude indicates shadow receipt magnitude (nothing/a_few/several/na).
	ShadowMagnitude MagnitudeBucket

	// ChatConfigured indicates if chat deployment is configured.
	ChatConfigured bool

	// EmbedConfigured indicates if embeddings deployment is configured.
	EmbedConfigured bool

	// EndpointConfigured indicates if Azure endpoint is configured.
	EndpointConfigured bool

	// Region is the Azure region if explicitly configured (e.g., "uksouth").
	// Empty if not configured or not derivable from config (never from URL).
	Region string
}

// CanonicalString returns the pipe-delimited canonical representation.
// Stable ordering ensures deterministic hashing.
func (i *RealityInputs) CanonicalString() string {
	var b strings.Builder
	b.WriteString("REALITY_INPUTS|v1|")
	b.WriteString(i.CircleID)
	b.WriteString("|")
	b.WriteString(i.NowBucket)
	b.WriteString("|gmail:")
	b.WriteString(boolToYesNo(i.GmailConnected))
	b.WriteString("|sync:")
	b.WriteString(string(i.SyncBucket))
	b.WriteString("|sync_mag:")
	b.WriteString(string(i.SyncMagnitude))
	b.WriteString("|held:")
	b.WriteString(boolToYesNo(i.ObligationsHeld))
	b.WriteString("|auto_surface:")
	b.WriteString(boolToYesNo(i.AutoSurface))
	b.WriteString("|shadow_provider:")
	b.WriteString(string(i.ShadowProviderKind))
	b.WriteString("|shadow_real:")
	b.WriteString(boolToYesNo(i.ShadowRealAllowed))
	b.WriteString("|shadow_mag:")
	b.WriteString(string(i.ShadowMagnitude))
	b.WriteString("|chat:")
	b.WriteString(boolToYesNo(i.ChatConfigured))
	b.WriteString("|embed:")
	b.WriteString(boolToYesNo(i.EmbedConfigured))
	b.WriteString("|endpoint:")
	b.WriteString(boolToYesNo(i.EndpointConfigured))
	b.WriteString("|region:")
	if i.Region != "" {
		b.WriteString(i.Region)
	} else {
		b.WriteString("none")
	}

	return b.String()
}

// Hash returns the SHA256 hash of the canonical string (32 hex chars).
func (i *RealityInputs) Hash() string {
	canonical := i.CanonicalString()
	h := sha256.Sum256([]byte(canonical))
	return hex.EncodeToString(h[:16])
}

// HasMeaningfulState returns true if there is any state worth showing.
func (i *RealityInputs) HasMeaningfulState() bool {
	return i.GmailConnected || i.SyncBucket != SyncBucketNever ||
		i.ShadowProviderKind != ProviderOff
}

// RealityAck represents an acknowledgement of viewing the reality page.
// Hash-only storage - no identifiers.
type RealityAck struct {
	// Period is the day bucket (YYYY-MM-DD).
	Period string

	// StatusHash is the hash of the acknowledged status.
	StatusHash string
}

// CanonicalString returns the pipe-delimited canonical representation.
func (a *RealityAck) CanonicalString() string {
	return "REALITY_ACK|v1|" + a.Period + "|" + a.StatusHash
}

// RealityCue represents the whisper cue for the reality page.
type RealityCue struct {
	// Available indicates if the cue should be shown.
	Available bool

	// CueText is the subtle cue text.
	CueText string

	// LinkText is the link text for the cue.
	LinkText string

	// StatusHash is the current status hash.
	StatusHash string
}

// DefaultCueText is the subtle cue text for the reality whisper.
const DefaultCueText = "if you ever wondered—connected is real."

// DefaultLinkText is the link text for the reality whisper.
const DefaultLinkText = "proof"

// boolToYesNo converts a bool to "yes" or "no" string.
func boolToYesNo(b bool) string {
	if b {
		return "yes"
	}
	return "no"
}

// YesNoToDisplay converts "yes"/"no" to display text.
func YesNoToDisplay(s string) string {
	switch s {
	case "yes":
		return "yes"
	case "no":
		return "no"
	default:
		return s
	}
}

// MagnitudeToDisplay converts magnitude bucket to display text.
func MagnitudeToDisplay(m MagnitudeBucket) string {
	switch m {
	case MagnitudeNothing:
		return "nothing"
	case MagnitudeAFew:
		return "a few"
	case MagnitudeSeveral:
		return "several"
	case MagnitudeNA:
		return "n/a"
	default:
		return string(m)
	}
}

// SyncBucketToDisplay converts sync bucket to display text.
func SyncBucketToDisplay(s SyncBucket) string {
	switch s {
	case SyncBucketNever:
		return "never"
	case SyncBucketRecent:
		return "recent"
	case SyncBucketStale:
		return "stale"
	case SyncBucketUnknown:
		return "unknown"
	default:
		return string(s)
	}
}

// ProviderKindToDisplay converts provider kind to display text.
func ProviderKindToDisplay(p ShadowProviderKind) string {
	switch p {
	case ProviderOff:
		return "off"
	case ProviderStub:
		return "stub"
	case ProviderAzureChat:
		return "azure (chat)"
	case ProviderAzureEmbed:
		return "azure (embed)"
	default:
		return string(p)
	}
}
