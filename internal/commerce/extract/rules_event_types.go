package extract

import (
	"regexp"
	"strings"

	"quantumlife/pkg/domain/commerce"
)

// EventTypeClassifier determines commerce event types from email signals.
type EventTypeClassifier struct {
	orderPatterns        []*regexp.Regexp
	shipmentPatterns     []*regexp.Regexp
	deliveredPatterns    []*regexp.Regexp
	invoicePatterns      []*regexp.Regexp
	receiptPatterns      []*regexp.Regexp
	subscriptionPatterns []*regexp.Regexp
	renewalPatterns      []*regexp.Regexp
	ridePatterns         []*regexp.Regexp
	refundPatterns       []*regexp.Regexp
}

// NewEventTypeClassifier creates a classifier with compiled patterns.
func NewEventTypeClassifier() *EventTypeClassifier {
	return &EventTypeClassifier{
		orderPatterns: compilePatterns([]string{
			`order\s*(confirm|placed|received)`,
			`your\s+order\s+(has\s+been\s+)?(?:placed|confirmed|received)`,
			`order\s+#?\d+`,
			`order\s+#?[A-Za-z0-9\-]+\s+confirm`,
			`thank\s+you\s+for\s+(your\s+)?order`,
			`we('ve|\s+have)\s+received\s+your\s+order`,
			`order\s+accepted`,
			`order\s+confirmed`,
		}),
		shipmentPatterns: compilePatterns([]string{
			`ship(ped|ment|ping)`,
			`dispatch(ed)?`,
			`on\s+(its|the)\s+way`,
			`out\s+for\s+delivery`,
			`in\s+transit`,
			`track(ing)?\s+(number|id|code)`,
			`parcel\s+(is|has)`,
			`package\s+(is|has|shipped)`,
			`delivery\s+update`,
			`your\s+delivery`,
			`we('re|\s+are)\s+delivering`,
		}),
		deliveredPatterns: compilePatterns([]string{
			`deliver(ed|y\s+complete)`,
			`has\s+been\s+delivered`,
			`successfully\s+delivered`,
			`left\s+with\s+neighbour`,
			`left\s+in\s+safe\s+place`,
			`signed\s+for\s+by`,
			`delivery\s+confirmed`,
			`your\s+(order|package|parcel)\s+(was\s+)?delivered`,
		}),
		invoicePatterns: compilePatterns([]string{
			`invoice`,
			`bill\s+(for|from|attached)`,
			`payment\s+due`,
			`amount\s+due`,
			`please\s+pay`,
			`outstanding\s+balance`,
		}),
		receiptPatterns: compilePatterns([]string{
			`receipt`,
			`payment\s+(confirm|received|successful)`,
			`thank\s+you\s+for\s+(your\s+)?payment`,
			`payment\s+of\s+[\£\$\€₹]`,
			`payment\s+of\s+(GBP|USD|EUR|INR)\s+[\d,]+`,
			`we('ve|\s+have)\s+received\s+your\s+payment`,
			`transaction\s+(confirm|complete|successful)`,
			`charged\s+[\£\$\€₹]`,
		}),
		subscriptionPatterns: compilePatterns([]string{
			`subscription\s+(confirm|start|activ)`,
			`welcome\s+to\s+(your\s+)?subscription`,
			`you('ve|\s+have)\s+subscribed`,
			`membership\s+(confirm|start|activ)`,
		}),
		renewalPatterns: compilePatterns([]string{
			`subscription\s+renew`,
			`auto[\-\s]?renew`,
			`membership\s+renew`,
			`your\s+subscription\s+has\s+been\s+renewed`,
			`renewal\s+(confirm|notice|reminder)`,
			`billing\s+cycle`,
			`next\s+billing\s+date`,
		}),
		ridePatterns: compilePatterns([]string{
			`trip\s+(receipt|summary|detail)`,
			`ride\s+(receipt|summary|detail)`,
			`your\s+(uber|lyft|ola|bolt)\s+(trip|ride)`,
			`thanks\s+for\s+riding`,
			`your\s+ride\s+with`,
		}),
		refundPatterns: compilePatterns([]string{
			`refund`,
			`money\s+back`,
			`return\s+processed`,
			`credit\s+issued`,
			`we('ve|\s+have)\s+refunded`,
			`your\s+refund`,
			`returning\s+[\£\$\€]`,
		}),
	}
}

// compilePatterns compiles a list of regex patterns.
func compilePatterns(patterns []string) []*regexp.Regexp {
	compiled := make([]*regexp.Regexp, 0, len(patterns))
	for _, p := range patterns {
		if re, err := regexp.Compile(`(?i)` + p); err == nil {
			compiled = append(compiled, re)
		}
	}
	return compiled
}

// ClassifyResult contains classification results.
type ClassifyResult struct {
	EventType      commerce.CommerceEventType
	Confidence     float64 // 0.0-1.0
	MatchedPattern string
	ShipmentStatus commerce.ShipmentStatus
}

// Classify determines the event type from subject and body.
// Returns the best match with confidence score.
func (c *EventTypeClassifier) Classify(subject, bodyPreview string) ClassifyResult {
	text := strings.ToLower(subject + " " + bodyPreview)

	// Order matters: check more specific patterns first
	// Check for delivered (most specific shipment state)
	if pattern := c.matchFirst(text, c.deliveredPatterns); pattern != "" {
		return ClassifyResult{
			EventType:      commerce.EventShipmentUpdate,
			Confidence:     0.9,
			MatchedPattern: pattern,
			ShipmentStatus: commerce.ShipmentDelivered,
		}
	}

	// Check for refund
	if pattern := c.matchFirst(text, c.refundPatterns); pattern != "" {
		return ClassifyResult{
			EventType:      commerce.EventRefundIssued,
			Confidence:     0.85,
			MatchedPattern: pattern,
		}
	}

	// Check for ride receipt
	if pattern := c.matchFirst(text, c.ridePatterns); pattern != "" {
		return ClassifyResult{
			EventType:      commerce.EventRideReceipt,
			Confidence:     0.9,
			MatchedPattern: pattern,
		}
	}

	// Check for subscription renewal
	if pattern := c.matchFirst(text, c.renewalPatterns); pattern != "" {
		return ClassifyResult{
			EventType:      commerce.EventSubscriptionRenewed,
			Confidence:     0.85,
			MatchedPattern: pattern,
		}
	}

	// Check for new subscription
	if pattern := c.matchFirst(text, c.subscriptionPatterns); pattern != "" {
		return ClassifyResult{
			EventType:      commerce.EventSubscriptionCreated,
			Confidence:     0.85,
			MatchedPattern: pattern,
		}
	}

	// Check for invoice
	if pattern := c.matchFirst(text, c.invoicePatterns); pattern != "" {
		return ClassifyResult{
			EventType:      commerce.EventInvoiceIssued,
			Confidence:     0.8,
			MatchedPattern: pattern,
		}
	}

	// Check for shipment/tracking (general)
	if pattern := c.matchFirst(text, c.shipmentPatterns); pattern != "" {
		status := c.inferShipmentStatus(text)
		return ClassifyResult{
			EventType:      commerce.EventShipmentUpdate,
			Confidence:     0.85,
			MatchedPattern: pattern,
			ShipmentStatus: status,
		}
	}

	// Check for payment receipt
	if pattern := c.matchFirst(text, c.receiptPatterns); pattern != "" {
		return ClassifyResult{
			EventType:      commerce.EventPaymentReceipt,
			Confidence:     0.8,
			MatchedPattern: pattern,
		}
	}

	// Check for order (most generic, check last)
	if pattern := c.matchFirst(text, c.orderPatterns); pattern != "" {
		return ClassifyResult{
			EventType:      commerce.EventOrderPlaced,
			Confidence:     0.75,
			MatchedPattern: pattern,
		}
	}

	// No match
	return ClassifyResult{
		EventType:  "",
		Confidence: 0,
	}
}

// matchFirst returns the first matching pattern or empty string.
func (c *EventTypeClassifier) matchFirst(text string, patterns []*regexp.Regexp) string {
	for _, re := range patterns {
		if match := re.FindString(text); match != "" {
			return match
		}
	}
	return ""
}

// inferShipmentStatus infers detailed shipment status from text.
func (c *EventTypeClassifier) inferShipmentStatus(text string) commerce.ShipmentStatus {
	// Check in order of specificity
	outForDeliveryPatterns := []string{
		"out for delivery",
		"on its way to you",
		"arriving today",
		"driver is on the way",
	}
	for _, p := range outForDeliveryPatterns {
		if strings.Contains(text, p) {
			return commerce.ShipmentOutDelivery
		}
	}

	inTransitPatterns := []string{
		"in transit",
		"on the way",
		"in our network",
		"being processed",
	}
	for _, p := range inTransitPatterns {
		if strings.Contains(text, p) {
			return commerce.ShipmentInTransit
		}
	}

	dispatchedPatterns := []string{
		"dispatched",
		"shipped",
		"left our warehouse",
		"on its way",
		"has been sent",
	}
	for _, p := range dispatchedPatterns {
		if strings.Contains(text, p) {
			return commerce.ShipmentDispatched
		}
	}

	failedPatterns := []string{
		"delivery failed",
		"unable to deliver",
		"delivery unsuccessful",
		"returned to sender",
	}
	for _, p := range failedPatterns {
		if strings.Contains(text, p) {
			return commerce.ShipmentFailed
		}
	}

	return commerce.ShipmentUnknown
}

// IsCommerceEmail checks if an email is likely commerce-related.
// Quick filter before full extraction.
func (c *EventTypeClassifier) IsCommerceEmail(subject, bodyPreview, senderDomain string) bool {
	text := strings.ToLower(subject + " " + bodyPreview)

	// Quick keyword check
	commerceKeywords := []string{
		"order", "receipt", "invoice", "payment", "delivery",
		"shipped", "tracking", "confirm", "purchase", "subscription",
		"renew", "refund", "trip", "ride", "bill",
	}

	for _, kw := range commerceKeywords {
		if strings.Contains(text, kw) {
			return true
		}
	}

	// Check sender domain
	if IsCommerceRelatedDomain(senderDomain) {
		return true
	}

	return false
}

// ExtractOrderID attempts to extract an order ID from text.
func ExtractOrderID(subject, bodyPreview string) string {
	text := subject + " " + bodyPreview

	// Common order ID patterns
	patterns := []*regexp.Regexp{
		regexp.MustCompile(`(?i)order\s*(?:#|number|id)?[:\s]*([A-Z0-9\-]{5,20})`),
		regexp.MustCompile(`(?i)order\s+([0-9]{3,}[\-0-9A-Z]*)`),
		regexp.MustCompile(`(?i)#([A-Z0-9\-]{6,20})\b`),
		regexp.MustCompile(`(?i)reference[:\s]+([A-Z0-9\-]{5,20})`),
	}

	for _, re := range patterns {
		if matches := re.FindStringSubmatch(text); len(matches) > 1 {
			return strings.TrimSpace(matches[1])
		}
	}

	return ""
}

// ExtractTrackingID attempts to extract a tracking ID from text.
func ExtractTrackingID(subject, bodyPreview string) string {
	text := subject + " " + bodyPreview

	// Common tracking number patterns
	patterns := []*regexp.Regexp{
		regexp.MustCompile(`(?i)track(?:ing)?(?:\s*(?:#|number|id|code))?[:\s]*([A-Z0-9]{8,30})`),
		regexp.MustCompile(`(?i)parcel\s*(?:#|id)?[:\s]*([A-Z0-9]{8,20})`),
		regexp.MustCompile(`(?i)consignment[:\s]+([A-Z0-9\-]{8,25})`),
		// DPD format
		regexp.MustCompile(`(?i)\b([0-9]{14,16})\b`),
		// Royal Mail format
		regexp.MustCompile(`(?i)\b([A-Z]{2}[0-9]{9}GB)\b`),
	}

	for _, re := range patterns {
		if matches := re.FindStringSubmatch(text); len(matches) > 1 {
			id := strings.TrimSpace(matches[1])
			// Validate it looks like a tracking number (alphanumeric, reasonable length)
			if len(id) >= 8 && len(id) <= 30 {
				return id
			}
		}
	}

	return ""
}
