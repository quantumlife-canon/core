// Package mirror provides domain types for the Mirror Proof system.
//
// Phase 18.7: Mirror Proof - Trust Through Evidence of Reading
//
// CRITICAL: Abstract only - no names, dates, vendors, senders, amounts.
// CRITICAL: No timestamps rendered - use horizon buckets only.
// CRITICAL: No goroutines. No time.Now(). stdlib-only.
// CRITICAL: Canonical strings are pipe-delimited, not JSON.
//
// Reference: docs/ADR/ADR-0039-phase18-7-mirror-proof.md
package mirror

import (
	"crypto/sha256"
	"encoding/hex"
	"sort"
	"strings"
	"time"

	"quantumlife/pkg/domain/connection"
)

// MagnitudeBucket represents abstract magnitude (never raw counts).
type MagnitudeBucket string

const (
	MagnitudeNone    MagnitudeBucket = "none"
	MagnitudeAFew    MagnitudeBucket = "a_few"
	MagnitudeSeveral MagnitudeBucket = "several"
)

// String returns string representation.
func (m MagnitudeBucket) String() string {
	return string(m)
}

// DisplayText returns human-readable text for magnitude.
func (m MagnitudeBucket) DisplayText() string {
	switch m {
	case MagnitudeNone:
		return "nothing"
	case MagnitudeAFew:
		return "a few"
	case MagnitudeSeveral:
		return "several"
	default:
		return ""
	}
}

// HorizonBucket represents abstract time horizon (never timestamps).
type HorizonBucket string

const (
	HorizonRecent  HorizonBucket = "recent"
	HorizonOngoing HorizonBucket = "ongoing"
	HorizonEarlier HorizonBucket = "earlier"
)

// String returns string representation.
func (h HorizonBucket) String() string {
	return string(h)
}

// DisplayText returns human-readable text for horizon.
func (h HorizonBucket) DisplayText() string {
	switch h {
	case HorizonRecent:
		return "recently"
	case HorizonOngoing:
		return "ongoing"
	case HorizonEarlier:
		return "earlier"
	default:
		return ""
	}
}

// ObservedCategory represents what kind of data was observed.
// These are abstract categories - never vendor-specific.
type ObservedCategory string

const (
	ObservedTimeCommitments ObservedCategory = "time_commitments"
	ObservedReceipts        ObservedCategory = "receipts"
	ObservedMessages        ObservedCategory = "messages"
	ObservedPatterns        ObservedCategory = "patterns"
)

// String returns string representation.
func (o ObservedCategory) String() string {
	return string(o)
}

// DisplayText returns human-readable text for observed category.
func (o ObservedCategory) DisplayText() string {
	switch o {
	case ObservedTimeCommitments:
		return "time commitments"
	case ObservedReceipts:
		return "receipts"
	case ObservedMessages:
		return "messages"
	case ObservedPatterns:
		return "patterns"
	default:
		return ""
	}
}

// AllObservedCategories returns all categories in deterministic order.
func AllObservedCategories() []ObservedCategory {
	return []ObservedCategory{
		ObservedMessages,
		ObservedPatterns,
		ObservedReceipts,
		ObservedTimeCommitments,
	}
}

// MirrorSourceSummary represents the abstract summary of what was seen
// from a single connected source.
type MirrorSourceSummary struct {
	// Kind is the connection kind (email, calendar, finance).
	Kind connection.ConnectionKind

	// ReadSuccessfully indicates if reading succeeded.
	ReadSuccessfully bool

	// NotStored lists what was explicitly not stored.
	// These are calm, reassuring statements.
	NotStored []string

	// Observed lists abstract observations (never identifiers).
	Observed []ObservedItem
}

// ObservedItem represents a single abstract observation.
type ObservedItem struct {
	// Category is what kind of thing was observed.
	Category ObservedCategory

	// Magnitude is the bucketed amount (a_few, several).
	Magnitude MagnitudeBucket

	// Horizon is when it was observed (recent, ongoing, earlier).
	Horizon HorizonBucket
}

// CanonicalString returns the pipe-delimited canonical representation.
func (o *ObservedItem) CanonicalString() string {
	var b strings.Builder
	b.WriteString("OBS_ITEM|v1|")
	b.WriteString(string(o.Category))
	b.WriteString("|")
	b.WriteString(string(o.Magnitude))
	b.WriteString("|")
	b.WriteString(string(o.Horizon))
	return b.String()
}

// CanonicalString returns the pipe-delimited canonical representation.
func (s *MirrorSourceSummary) CanonicalString() string {
	var b strings.Builder
	b.WriteString("MIRROR_SRC|v1|")
	b.WriteString(string(s.Kind))
	b.WriteString("|read:")
	if s.ReadSuccessfully {
		b.WriteString("true")
	} else {
		b.WriteString("false")
	}
	b.WriteString("|notstored:")
	b.WriteString(strings.Join(s.NotStored, ","))
	b.WriteString("|obs:")
	for i, obs := range s.Observed {
		if i > 0 {
			b.WriteString(",")
		}
		b.WriteString(obs.CanonicalString())
	}
	return b.String()
}

// Hash returns the SHA256 hash of the canonical string.
func (s *MirrorSourceSummary) Hash() string {
	h := sha256.Sum256([]byte(s.CanonicalString()))
	return hex.EncodeToString(h[:])
}

// MirrorOutcome represents the abstract outcome of mirror reflection.
type MirrorOutcome struct {
	// HeldQuietly indicates if items are being held.
	HeldQuietly bool

	// HeldMagnitude is the bucketed amount of held items.
	HeldMagnitude MagnitudeBucket

	// NothingRequiresAttention is true when nothing needs the user.
	NothingRequiresAttention bool
}

// CanonicalString returns the pipe-delimited canonical representation.
func (o *MirrorOutcome) CanonicalString() string {
	var b strings.Builder
	b.WriteString("MIRROR_OUT|v1|")
	b.WriteString("held:")
	if o.HeldQuietly {
		b.WriteString("true")
	} else {
		b.WriteString("false")
	}
	b.WriteString("|mag:")
	b.WriteString(string(o.HeldMagnitude))
	b.WriteString("|nothing_req:")
	if o.NothingRequiresAttention {
		b.WriteString("true")
	} else {
		b.WriteString("false")
	}
	return b.String()
}

// Hash returns the SHA256 hash of the canonical string.
func (o *MirrorOutcome) Hash() string {
	h := sha256.Sum256([]byte(o.CanonicalString()))
	return hex.EncodeToString(h[:])
}

// MirrorPage represents the complete mirror proof page.
type MirrorPage struct {
	// Title is the page title ("Seen, quietly.").
	Title string

	// Subtitle is the page subtitle.
	Subtitle string

	// Sources lists summaries for each connected source.
	Sources []MirrorSourceSummary

	// Outcome describes what changed (if anything).
	Outcome MirrorOutcome

	// RestraintStatement is the reassurance about restraint.
	RestraintStatement string

	// RestraintWhy explains why quiet is good.
	RestraintWhy string

	// GeneratedAt is when this page was generated (from injected clock).
	GeneratedAt time.Time

	// Hash is the deterministic hash of the page.
	Hash string
}

// CanonicalString returns the pipe-delimited canonical representation.
func (p *MirrorPage) CanonicalString() string {
	var b strings.Builder
	b.WriteString("MIRROR_PAGE|v1|")
	b.WriteString("title:")
	b.WriteString(p.Title)
	b.WriteString("|sub:")
	b.WriteString(p.Subtitle)
	b.WriteString("|sources:")

	// Sort sources by kind for determinism
	sortedSources := make([]MirrorSourceSummary, len(p.Sources))
	copy(sortedSources, p.Sources)
	sort.Slice(sortedSources, func(i, j int) bool {
		return sortedSources[i].Kind < sortedSources[j].Kind
	})

	for i, src := range sortedSources {
		if i > 0 {
			b.WriteString(";")
		}
		b.WriteString(src.CanonicalString())
	}
	b.WriteString("|outcome:")
	b.WriteString(p.Outcome.CanonicalString())
	b.WriteString("|restraint:")
	b.WriteString(p.RestraintStatement)
	b.WriteString("|why:")
	b.WriteString(p.RestraintWhy)
	b.WriteString("|ts:")
	b.WriteString(p.GeneratedAt.UTC().Format(time.RFC3339))
	return b.String()
}

// ComputeHash computes and returns the SHA256 hash.
func (p *MirrorPage) ComputeHash() string {
	h := sha256.Sum256([]byte(p.CanonicalString()))
	return hex.EncodeToString(h[:])
}

// MirrorInput represents the input for building a mirror page.
// This is used by the engine to produce deterministic output.
type MirrorInput struct {
	// ConnectedSources lists which sources are connected and their state.
	ConnectedSources map[connection.ConnectionKind]SourceInputState

	// HeldCount is the abstract count of held items (will be bucketed).
	HeldCount int

	// SurfacedCount is the abstract count of surfaced items (will be bucketed).
	SurfacedCount int

	// CircleID is the circle this mirror is for.
	CircleID string
}

// SourceInputState represents the input state of a connected source.
type SourceInputState struct {
	// Connected indicates if the source is connected.
	Connected bool

	// Mode is mock or real.
	Mode connection.IntentMode

	// ReadSuccess indicates if reading succeeded.
	ReadSuccess bool

	// ObservedCounts maps category to raw count (will be bucketed).
	ObservedCounts map[ObservedCategory]int
}

// CanonicalString returns the pipe-delimited canonical representation.
func (i *MirrorInput) CanonicalString() string {
	var b strings.Builder
	b.WriteString("MIRROR_IN|v1|")
	b.WriteString("circle:")
	b.WriteString(i.CircleID)
	b.WriteString("|held:")
	b.WriteString(bucketCountString(i.HeldCount))
	b.WriteString("|surfaced:")
	b.WriteString(bucketCountString(i.SurfacedCount))
	b.WriteString("|sources:")

	// Sort kinds for determinism
	kinds := make([]connection.ConnectionKind, 0, len(i.ConnectedSources))
	for k := range i.ConnectedSources {
		kinds = append(kinds, k)
	}
	sort.Slice(kinds, func(a, b int) bool {
		return kinds[a] < kinds[b]
	})

	for j, kind := range kinds {
		if j > 0 {
			b.WriteString(";")
		}
		state := i.ConnectedSources[kind]
		b.WriteString(string(kind))
		b.WriteString("=")
		if state.Connected {
			b.WriteString("conn")
		} else {
			b.WriteString("disc")
		}
	}
	return b.String()
}

// Hash returns the SHA256 hash of the input.
func (i *MirrorInput) Hash() string {
	h := sha256.Sum256([]byte(i.CanonicalString()))
	return hex.EncodeToString(h[:])
}

// bucketCountString converts count to bucket string for hashing.
func bucketCountString(count int) string {
	switch {
	case count == 0:
		return "none"
	case count <= 3:
		return "a_few"
	default:
		return "several"
	}
}

// BucketCount converts a raw count to a magnitude bucket.
// CRITICAL: Never expose raw counts - always bucket them.
func BucketCount(count int) MagnitudeBucket {
	switch {
	case count == 0:
		return MagnitudeNone
	case count <= 3:
		return MagnitudeAFew
	default:
		return MagnitudeSeveral
	}
}

// MirrorAck represents an acknowledgement of viewing/dismissing the mirror.
type MirrorAck struct {
	// PageHash is the hash of the page that was acknowledged.
	PageHash string

	// Action is what happened (viewed, acknowledged).
	Action AckAction

	// At is when the acknowledgement happened (from injected clock).
	At time.Time
}

// AckAction represents what kind of acknowledgement.
type AckAction string

const (
	AckViewed       AckAction = "viewed"
	AckAcknowledged AckAction = "acknowledged"
)

// CanonicalString returns the pipe-delimited canonical representation.
func (a *MirrorAck) CanonicalString() string {
	var b strings.Builder
	b.WriteString("MIRROR_ACK|v1|")
	b.WriteString("hash:")
	b.WriteString(a.PageHash)
	b.WriteString("|action:")
	b.WriteString(string(a.Action))
	b.WriteString("|at:")
	b.WriteString(a.At.UTC().Format(time.RFC3339))
	return b.String()
}

// Hash returns the SHA256 hash of the ack.
func (a *MirrorAck) Hash() string {
	h := sha256.Sum256([]byte(a.CanonicalString()))
	return hex.EncodeToString(h[:])
}
