package extract

import (
	"sort"
	"strings"
	"time"

	"quantumlife/pkg/domain/commerce"
	"quantumlife/pkg/domain/events"
	"quantumlife/pkg/domain/identity"
)

// Clock interface for time injection.
type Clock interface {
	Now() time.Time
}

// Engine extracts commerce events from email messages.
// CRITICAL: Deterministic extraction - same inputs produce same outputs.
type Engine struct {
	clock         Clock
	vendorMatcher *VendorMatcher
	classifier    *EventTypeClassifier
	amountParser  *AmountParser
}

// NewEngine creates a commerce extraction engine.
func NewEngine(clock Clock) *Engine {
	return &Engine{
		clock:         clock,
		vendorMatcher: NewVendorMatcher(),
		classifier:    NewEventTypeClassifier(),
		amountParser:  NewAmountParser(),
	}
}

// ExtractFromEmails extracts commerce events from email messages.
// Returns events in deterministic order with extraction metrics.
func (e *Engine) ExtractFromEmails(emails []*events.EmailMessageEvent) ([]*commerce.CommerceEvent, commerce.ExtractionMetrics) {
	var result []*commerce.CommerceEvent
	metrics := commerce.ExtractionMetrics{}

	extractTime := e.clock.Now()

	for _, email := range emails {
		metrics.EmailsScanned++

		// Quick filter: skip non-commerce emails
		if !e.classifier.IsCommerceEmail(email.Subject, email.BodyPreview, email.SenderDomain) {
			continue
		}

		// Extract commerce event
		event := e.extractFromEmail(email, extractTime, &metrics)
		if event != nil {
			result = append(result, event)
			metrics.EventsEmitted++
		}
	}

	// Sort for deterministic output
	commerce.SortByTimeThenID(result)

	return result, metrics
}

// extractFromEmail extracts a single commerce event from an email.
func (e *Engine) extractFromEmail(email *events.EmailMessageEvent, extractTime time.Time, metrics *commerce.ExtractionMetrics) *commerce.CommerceEvent {
	// Step 1: Classify event type
	classification := e.classifier.Classify(email.Subject, email.BodyPreview)
	if classification.EventType == "" {
		metrics.DroppedMissingData++
		return nil
	}

	// Step 2: Identify vendor
	vendorResult := e.vendorMatcher.MatchByDomain(email.SenderDomain)
	if !vendorResult.Matched {
		// Try subject-based matching
		vendorResult = e.vendorMatcher.MatchBySubjectPatterns(email.Subject)
	}

	var vendorName string
	var category commerce.CommerceCategory

	if vendorResult.Matched {
		vendorName = vendorResult.CanonicalName
		category = vendorResult.Category
		metrics.VendorMatchedCount++
	} else {
		// Fallback to domain-derived vendor name
		vendorName = FallbackVendorFromDomain(email.SenderDomain)
		category = commerce.CategoryUnknown
		metrics.UnknownVendorCount++
	}

	// Step 3: Create event
	event := commerce.NewCommerceEvent(
		classification.EventType,
		email.Vendor,
		email.MessageID,
		vendorName,
		email.OccurredAt(),
		extractTime,
	).WithCategory(category).
		WithVendorDomain(email.SenderDomain).
		WithCircle(email.CircleID())

	// Step 4: Extract amount
	amountResult := e.amountParser.Parse(email.Subject + " " + email.BodyPreview)
	if amountResult.Valid {
		event.WithAmount(amountResult.Currency, amountResult.AmountCents)
		event.WithSignal(commerce.SignalAmountRaw, amountResult.RawMatch)
		metrics.AmountsParsed++
	} else {
		metrics.AmountsFailedParse++
	}

	// Step 5: Extract order/tracking IDs
	if orderID := ExtractOrderID(email.Subject, email.BodyPreview); orderID != "" {
		event.WithOrderID(orderID)
		event.WithSignal(commerce.SignalOrderIDRaw, orderID)
	}

	if trackingID := ExtractTrackingID(email.Subject, email.BodyPreview); trackingID != "" {
		event.WithTrackingID(trackingID)
		event.WithSignal(commerce.SignalTrackingRaw, trackingID)
	}

	// Step 6: Set shipment status if applicable
	if classification.ShipmentStatus != "" && classification.ShipmentStatus != commerce.ShipmentUnknown {
		event.WithShipmentStatus(classification.ShipmentStatus)
	}

	// Step 7: Add raw signals for audit
	event.WithSignal(commerce.SignalSubject, truncate(email.Subject, 100))
	event.WithSignal(commerce.SignalFromAddress, email.From.Address)
	event.WithSignal(commerce.SignalFromDomain, email.SenderDomain)
	if email.BodyPreview != "" {
		event.WithSignal(commerce.SignalBodySnippet, truncate(email.BodyPreview, 200))
	}

	return event
}

// ExtractFromEmailsWithCircleMapping extracts events with explicit circle mapping.
func (e *Engine) ExtractFromEmailsWithCircleMapping(
	emails []*events.EmailMessageEvent,
	circleMapper func(email *events.EmailMessageEvent) identity.EntityID,
) ([]*commerce.CommerceEvent, commerce.ExtractionMetrics) {
	var result []*commerce.CommerceEvent
	metrics := commerce.ExtractionMetrics{}

	extractTime := e.clock.Now()

	for _, email := range emails {
		metrics.EmailsScanned++

		if !e.classifier.IsCommerceEmail(email.Subject, email.BodyPreview, email.SenderDomain) {
			continue
		}

		event := e.extractFromEmail(email, extractTime, &metrics)
		if event != nil {
			// Apply custom circle mapping
			if circleMapper != nil {
				event.WithCircle(circleMapper(email))
			}
			result = append(result, event)
			metrics.EventsEmitted++
		}
	}

	commerce.SortByTimeThenID(result)

	return result, metrics
}

// GetSupportedVendors returns a sorted list of supported vendors.
func (e *Engine) GetSupportedVendors() []string {
	vendorSet := make(map[string]bool)
	for _, info := range VendorRegistry {
		vendorSet[info.CanonicalName] = true
	}

	vendors := make([]string, 0, len(vendorSet))
	for v := range vendorSet {
		vendors = append(vendors, v)
	}
	sort.Strings(vendors)
	return vendors
}

// GetSupportedCategories returns all commerce categories.
func (e *Engine) GetSupportedCategories() []commerce.CommerceCategory {
	return []commerce.CommerceCategory{
		commerce.CategoryFoodDelivery,
		commerce.CategoryGrocery,
		commerce.CategoryCourier,
		commerce.CategoryRideHailing,
		commerce.CategoryRetail,
		commerce.CategoryUtilities,
		commerce.CategorySubscriptions,
		commerce.CategoryUnknown,
	}
}

// truncate truncates a string to max length.
func truncate(s string, maxLen int) string {
	s = strings.TrimSpace(s)
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen-3] + "..."
}

// FilterTransactionalEmails filters emails that are likely transactional (commerce-related).
// Useful for pre-filtering before extraction.
func FilterTransactionalEmails(emails []*events.EmailMessageEvent) []*events.EmailMessageEvent {
	var result []*events.EmailMessageEvent
	classifier := NewEventTypeClassifier()

	for _, email := range emails {
		// Use IsTransactional flag if available
		if email.IsTransactional {
			result = append(result, email)
			continue
		}

		// Otherwise check patterns
		if classifier.IsCommerceEmail(email.Subject, email.BodyPreview, email.SenderDomain) {
			result = append(result, email)
		}
	}

	return result
}

// GroupEventsByVendor groups events by vendor for reporting.
func GroupEventsByVendor(events []*commerce.CommerceEvent) map[string][]*commerce.CommerceEvent {
	groups := make(map[string][]*commerce.CommerceEvent)
	for _, e := range events {
		groups[e.Vendor] = append(groups[e.Vendor], e)
	}

	// Sort each group
	for _, group := range groups {
		commerce.SortByTimeThenID(group)
	}

	return groups
}

// GroupEventsByCategory groups events by category for reporting.
func GroupEventsByCategory(events []*commerce.CommerceEvent) map[commerce.CommerceCategory][]*commerce.CommerceEvent {
	groups := make(map[commerce.CommerceCategory][]*commerce.CommerceEvent)
	for _, e := range events {
		groups[e.Category] = append(groups[e.Category], e)
	}

	// Sort each group
	for _, group := range groups {
		commerce.SortByTimeThenID(group)
	}

	return groups
}

// SumAmountsByCurrency sums amounts grouped by currency.
func SumAmountsByCurrency(events []*commerce.CommerceEvent) map[string]int64 {
	sums := make(map[string]int64)
	for _, e := range events {
		if e.Currency != "" && e.AmountCents > 0 {
			sums[e.Currency] += e.AmountCents
		}
	}
	return sums
}
