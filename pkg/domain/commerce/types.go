// Package commerce defines canonical commerce event models for Phase 8.
//
// Commerce events are derived from email messages (receipts, order confirmations,
// shipping updates, invoices) and transformed into vendor-agnostic canonical format.
//
// CRITICAL: No vendor-specific logic in this package.
// CRITICAL: Deterministic ID generation via canonical strings (NOT JSON).
// CRITICAL: All timestamps from injected clock.
//
// Reference: docs/ADR/ADR-0024-phase8-commerce-mirror-email-derived.md
package commerce

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"sort"
	"strings"
	"time"

	"quantumlife/pkg/domain/identity"
)

// CommerceEventType identifies the kind of commerce event.
type CommerceEventType string

const (
	EventOrderPlaced         CommerceEventType = "order_placed"
	EventOrderUpdated        CommerceEventType = "order_updated"
	EventShipmentUpdate      CommerceEventType = "shipment_update"
	EventInvoiceIssued       CommerceEventType = "invoice_issued"
	EventPaymentReceipt      CommerceEventType = "payment_receipt"
	EventSubscriptionCreated CommerceEventType = "subscription_created"
	EventSubscriptionRenewed CommerceEventType = "subscription_renewed"
	EventRideReceipt         CommerceEventType = "ride_receipt"
	EventRefundIssued        CommerceEventType = "refund_issued"
)

// CommerceCategory categorizes the commerce domain.
type CommerceCategory string

const (
	CategoryFoodDelivery  CommerceCategory = "food_delivery"
	CategoryGrocery       CommerceCategory = "grocery"
	CategoryCourier       CommerceCategory = "courier"
	CategoryRideHailing   CommerceCategory = "ride_hailing"
	CategoryRetail        CommerceCategory = "retail"
	CategoryUtilities     CommerceCategory = "utilities"
	CategorySubscriptions CommerceCategory = "subscriptions"
	CategoryUnknown       CommerceCategory = "unknown"
)

// ShipmentStatus indicates delivery progress.
type ShipmentStatus string

const (
	ShipmentDispatched  ShipmentStatus = "dispatched"
	ShipmentInTransit   ShipmentStatus = "in_transit"
	ShipmentOutDelivery ShipmentStatus = "out_for_delivery"
	ShipmentDelivered   ShipmentStatus = "delivered"
	ShipmentFailed      ShipmentStatus = "delivery_failed"
	ShipmentUnknown     ShipmentStatus = "unknown"
)

// CommerceEvent represents a canonical commerce event derived from email.
type CommerceEvent struct {
	// EventID is deterministic: sha256(canonical_string)[:16]
	EventID string

	// Type classification
	Type     CommerceEventType
	Category CommerceCategory

	// Timing
	OccurredAt  time.Time // When the event occurred
	ExtractedAt time.Time // When we extracted it

	// Source attribution
	SourceProvider  string // Email provider (gmail, outlook)
	SourceMessageID string // Email message ID
	CircleID        identity.EntityID

	// Vendor identification
	Vendor       string // Canonical vendor name (e.g., "Deliveroo", "Amazon")
	VendorDomain string // Domain extracted from email (e.g., "deliveroo.co.uk")

	// Financial data (optional)
	Currency    string // ISO 4217 (GBP, USD, EUR, INR)
	AmountCents int64  // Amount in minor units (0 if unknown)

	// Order/tracking references (optional)
	OrderID    string
	TrackingID string

	// Shipment-specific (optional)
	ShipmentStatus ShipmentStatus

	// Raw signals for debugging/audit (limited, deterministic ordering)
	RawSignals map[string]string

	// Internal: canonical string used for ID generation
	canonicalStr string
}

// Signal keys (standardized)
const (
	SignalSubject     = "subject"
	SignalFromAddress = "from_address"
	SignalFromDomain  = "from_domain"
	SignalBodySnippet = "body_snippet"
	SignalAmountRaw   = "amount_raw"
	SignalOrderIDRaw  = "order_id_raw"
	SignalTrackingRaw = "tracking_raw"
	SignalDateRaw     = "date_raw"
)

// NewCommerceEvent creates a commerce event with deterministic ID.
// The canonical string format is strictly defined for reproducibility.
func NewCommerceEvent(
	eventType CommerceEventType,
	sourceProvider string,
	sourceMessageID string,
	vendor string,
	occurredAt time.Time,
	extractedAt time.Time,
) *CommerceEvent {
	// Canonical string format: commerce:{type}:{provider}:{message_id}:{vendor}:{timestamp}
	canonicalStr := fmt.Sprintf("commerce:%s:%s:%s:%s:%d",
		eventType, sourceProvider, sourceMessageID, normalizeForHash(vendor), occurredAt.Unix())

	hash := sha256.Sum256([]byte(canonicalStr))
	eventID := "commerce_" + hex.EncodeToString(hash[:])[:16]

	return &CommerceEvent{
		EventID:         eventID,
		Type:            eventType,
		Category:        CategoryUnknown,
		OccurredAt:      occurredAt,
		ExtractedAt:     extractedAt,
		SourceProvider:  sourceProvider,
		SourceMessageID: sourceMessageID,
		Vendor:          vendor,
		RawSignals:      make(map[string]string),
		canonicalStr:    canonicalStr,
	}
}

// WithCategory sets the commerce category.
func (e *CommerceEvent) WithCategory(cat CommerceCategory) *CommerceEvent {
	e.Category = cat
	return e
}

// WithCircle sets the circle ID.
func (e *CommerceEvent) WithCircle(circleID identity.EntityID) *CommerceEvent {
	e.CircleID = circleID
	return e
}

// WithVendorDomain sets the vendor domain.
func (e *CommerceEvent) WithVendorDomain(domain string) *CommerceEvent {
	e.VendorDomain = domain
	return e
}

// WithAmount sets the financial amount.
func (e *CommerceEvent) WithAmount(currency string, amountCents int64) *CommerceEvent {
	e.Currency = currency
	e.AmountCents = amountCents
	return e
}

// WithOrderID sets the order reference.
func (e *CommerceEvent) WithOrderID(orderID string) *CommerceEvent {
	e.OrderID = orderID
	return e
}

// WithTrackingID sets the tracking reference.
func (e *CommerceEvent) WithTrackingID(trackingID string) *CommerceEvent {
	e.TrackingID = trackingID
	return e
}

// WithShipmentStatus sets the shipment status.
func (e *CommerceEvent) WithShipmentStatus(status ShipmentStatus) *CommerceEvent {
	e.ShipmentStatus = status
	return e
}

// WithSignal adds a raw signal for audit.
func (e *CommerceEvent) WithSignal(key, value string) *CommerceEvent {
	e.RawSignals[key] = value
	return e
}

// CanonicalString returns the canonical representation for hashing.
// Format is strictly defined and does NOT use JSON.
func (e *CommerceEvent) CanonicalString() string {
	var parts []string

	parts = append(parts, fmt.Sprintf("id:%s", e.EventID))
	parts = append(parts, fmt.Sprintf("type:%s", e.Type))
	parts = append(parts, fmt.Sprintf("category:%s", e.Category))
	parts = append(parts, fmt.Sprintf("occurred:%d", e.OccurredAt.Unix()))
	parts = append(parts, fmt.Sprintf("provider:%s", e.SourceProvider))
	parts = append(parts, fmt.Sprintf("message:%s", e.SourceMessageID))
	parts = append(parts, fmt.Sprintf("vendor:%s", normalizeForHash(e.Vendor)))

	if e.VendorDomain != "" {
		parts = append(parts, fmt.Sprintf("domain:%s", e.VendorDomain))
	}
	if e.CircleID != "" {
		parts = append(parts, fmt.Sprintf("circle:%s", e.CircleID))
	}
	if e.Currency != "" {
		parts = append(parts, fmt.Sprintf("currency:%s", e.Currency))
	}
	if e.AmountCents != 0 {
		parts = append(parts, fmt.Sprintf("amount:%d", e.AmountCents))
	}
	if e.OrderID != "" {
		parts = append(parts, fmt.Sprintf("order:%s", e.OrderID))
	}
	if e.TrackingID != "" {
		parts = append(parts, fmt.Sprintf("tracking:%s", e.TrackingID))
	}
	if e.ShipmentStatus != "" && e.ShipmentStatus != ShipmentUnknown {
		parts = append(parts, fmt.Sprintf("shipment:%s", e.ShipmentStatus))
	}

	// Raw signals sorted for determinism
	if len(e.RawSignals) > 0 {
		keys := make([]string, 0, len(e.RawSignals))
		for k := range e.RawSignals {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, k := range keys {
			parts = append(parts, fmt.Sprintf("sig.%s:%s", k, normalizeForHash(e.RawSignals[k])))
		}
	}

	return strings.Join(parts, "|")
}

// ComputeHash returns a SHA256 hash of the canonical string.
func (e *CommerceEvent) ComputeHash() string {
	hash := sha256.Sum256([]byte(e.CanonicalString()))
	return hex.EncodeToString(hash[:])
}

// normalizeForHash normalizes a string for deterministic hashing.
// Lowercases and removes problematic characters.
func normalizeForHash(s string) string {
	s = strings.ToLower(s)
	s = strings.TrimSpace(s)
	// Replace pipe delimiter in values to avoid parsing issues
	s = strings.ReplaceAll(s, "|", "_")
	s = strings.ReplaceAll(s, "\n", " ")
	s = strings.ReplaceAll(s, "\r", "")
	return s
}

// SortByTimeThenID sorts commerce events deterministically.
// Order: OccurredAt ASC, EventID ASC
func SortByTimeThenID(events []*CommerceEvent) {
	sort.SliceStable(events, func(i, j int) bool {
		a, b := events[i], events[j]

		// 1. OccurredAt (earlier first)
		if !a.OccurredAt.Equal(b.OccurredAt) {
			return a.OccurredAt.Before(b.OccurredAt)
		}

		// 2. EventID for tie-breaking (deterministic)
		return a.EventID < b.EventID
	})
}

// SortByAmountDesc sorts commerce events by amount descending.
func SortByAmountDesc(events []*CommerceEvent) {
	sort.SliceStable(events, func(i, j int) bool {
		a, b := events[i], events[j]

		// 1. Amount (higher first)
		if a.AmountCents != b.AmountCents {
			return a.AmountCents > b.AmountCents
		}

		// 2. EventID for tie-breaking
		return a.EventID < b.EventID
	})
}

// ComputeEventsHash computes a deterministic hash over a sorted slice.
func ComputeEventsHash(events []*CommerceEvent) string {
	if len(events) == 0 {
		return "empty"
	}

	// Events must be sorted before hashing
	sorted := make([]*CommerceEvent, len(events))
	copy(sorted, events)
	SortByTimeThenID(sorted)

	var parts []string
	for _, e := range sorted {
		parts = append(parts, e.CanonicalString())
	}

	canonical := strings.Join(parts, "\n")
	hash := sha256.Sum256([]byte(canonical))
	return hex.EncodeToString(hash[:])
}

// FilterByType returns events of the specified type.
func FilterByType(events []*CommerceEvent, eventType CommerceEventType) []*CommerceEvent {
	var result []*CommerceEvent
	for _, e := range events {
		if e.Type == eventType {
			result = append(result, e)
		}
	}
	return result
}

// FilterByCategory returns events of the specified category.
func FilterByCategory(events []*CommerceEvent, category CommerceCategory) []*CommerceEvent {
	var result []*CommerceEvent
	for _, e := range events {
		if e.Category == category {
			result = append(result, e)
		}
	}
	return result
}

// FilterByVendor returns events from the specified vendor.
func FilterByVendor(events []*CommerceEvent, vendor string) []*CommerceEvent {
	normalizedVendor := strings.ToLower(vendor)
	var result []*CommerceEvent
	for _, e := range events {
		if strings.ToLower(e.Vendor) == normalizedVendor {
			result = append(result, e)
		}
	}
	return result
}

// FilterByCircle returns events for a specific circle.
func FilterByCircle(events []*CommerceEvent, circleID identity.EntityID) []*CommerceEvent {
	var result []*CommerceEvent
	for _, e := range events {
		if e.CircleID == circleID {
			result = append(result, e)
		}
	}
	return result
}

// HasPendingShipment returns true if the event is a shipment that hasn't been delivered.
func (e *CommerceEvent) HasPendingShipment() bool {
	if e.Type != EventShipmentUpdate {
		return false
	}
	switch e.ShipmentStatus {
	case ShipmentDelivered, ShipmentFailed:
		return false
	default:
		return true
	}
}

// ExtractionMetrics holds extraction performance data.
type ExtractionMetrics struct {
	EmailsScanned      int
	EventsEmitted      int
	DroppedMissingData int
	VendorMatchedCount int
	UnknownVendorCount int
	AmountsParsed      int
	AmountsFailedParse int
}

// Add combines two metrics.
func (m *ExtractionMetrics) Add(other ExtractionMetrics) {
	m.EmailsScanned += other.EmailsScanned
	m.EventsEmitted += other.EventsEmitted
	m.DroppedMissingData += other.DroppedMissingData
	m.VendorMatchedCount += other.VendorMatchedCount
	m.UnknownVendorCount += other.UnknownVendorCount
	m.AmountsParsed += other.AmountsParsed
	m.AmountsFailedParse += other.AmountsFailedParse
}
