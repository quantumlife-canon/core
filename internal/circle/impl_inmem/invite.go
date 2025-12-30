// Package impl_inmem provides an in-memory implementation of the circle interfaces.
package impl_inmem

import (
	"context"
	"fmt"
	"time"

	"quantumlife/internal/audit"
	"quantumlife/internal/circle"
	"quantumlife/internal/intersection"
	"quantumlife/pkg/crypto"
	"quantumlife/pkg/events"
	"quantumlife/pkg/primitives"
)

// InviteService implements circle.InviteIssuer and circle.InviteAcceptor.
type InviteService struct {
	circleRuntime *Runtime
	intRuntime    intersection.Runtime
	keyManager    crypto.KeyManager
	auditLogger   audit.Logger
	tokenCounter  int
}

// InviteServiceConfig contains configuration for the invite service.
type InviteServiceConfig struct {
	CircleRuntime *Runtime
	IntRuntime    intersection.Runtime
	KeyManager    crypto.KeyManager
	AuditLogger   audit.Logger
}

// NewInviteService creates a new invite service.
func NewInviteService(cfg InviteServiceConfig) *InviteService {
	return &InviteService{
		circleRuntime: cfg.CircleRuntime,
		intRuntime:    cfg.IntRuntime,
		keyManager:    cfg.KeyManager,
		auditLogger:   cfg.AuditLogger,
	}
}

// IssueInviteToken creates a signed invite token for intersection creation.
func (s *InviteService) IssueInviteToken(ctx context.Context, req circle.IssueInviteRequest) (*primitives.InviteToken, error) {
	// Verify issuer circle exists
	issuer, err := s.circleRuntime.Get(ctx, req.IssuerCircleID)
	if err != nil {
		return nil, fmt.Errorf("issuer circle not found: %w", err)
	}
	if issuer.State != circle.StateActive {
		return nil, fmt.Errorf("issuer circle is not active")
	}

	// Generate token ID
	s.tokenCounter++
	tokenID := fmt.Sprintf("invite-%s-%d", req.IssuerCircleID, s.tokenCounter)

	// Get signer for the issuer circle
	keyID := fmt.Sprintf("key-%s", req.IssuerCircleID)
	signer, err := s.keyManager.GetSigner(ctx, keyID)
	if err != nil {
		return nil, fmt.Errorf("failed to get signer: %w", err)
	}

	now := time.Now()
	expiresAt := now.Add(req.ValidFor)
	if req.ValidFor == 0 {
		expiresAt = now.Add(24 * time.Hour) // Default 24 hour expiry
	}

	token := &primitives.InviteToken{
		TokenID:            tokenID,
		IssuerCircleID:     req.IssuerCircleID,
		TargetCircleID:     req.TargetCircleID,
		ProposedName:       req.ProposedName,
		Template:           req.Template,
		IssuedAt:           now,
		ExpiresAt:          expiresAt,
		SignatureKeyID:     signer.KeyID(),
		SignatureAlgorithm: signer.Algorithm(),
	}

	// Sign the token
	payload := token.SigningPayload()
	sig, err := signer.Sign(ctx, payload)
	if err != nil {
		return nil, fmt.Errorf("failed to sign token: %w", err)
	}
	token.Signature = sig

	// Log audit event
	if s.auditLogger != nil {
		s.auditLogger.Log(ctx, audit.Entry{
			CircleID:  req.IssuerCircleID,
			EventType: string(events.EventInviteTokenIssued),
			SubjectID: tokenID,
			Action:    "issue_invite_token",
			Outcome:   "success",
			Metadata: map[string]string{
				"target_circle": req.TargetCircleID,
				"proposed_name": req.ProposedName,
				"expires_at":    expiresAt.Format(time.RFC3339),
				"scope_count":   fmt.Sprintf("%d", len(req.Template.Scopes)),
				"ceiling_count": fmt.Sprintf("%d", len(req.Template.Ceilings)),
			},
		})
	}

	return token, nil
}

// ValidateInviteToken validates a token without accepting it.
func (s *InviteService) ValidateInviteToken(ctx context.Context, token *primitives.InviteToken) error {
	// Check basic validation
	if err := token.Validate(); err != nil {
		return err
	}

	// Check expiry
	if token.IsExpired() {
		return primitives.ErrTokenExpired
	}

	// Verify signature
	verifier, err := s.keyManager.GetVerifier(ctx, token.SignatureKeyID)
	if err != nil {
		return fmt.Errorf("failed to get verifier: %w", err)
	}

	payload := token.SigningPayload()
	if err := verifier.Verify(ctx, payload, token.Signature); err != nil {
		return primitives.ErrInvalidSignature
	}

	return nil
}

// AcceptInviteToken validates and accepts an invite token.
func (s *InviteService) AcceptInviteToken(ctx context.Context, token *primitives.InviteToken, acceptorID string) (*circle.IntersectionRef, error) {
	// Validate token
	if err := s.ValidateInviteToken(ctx, token); err != nil {
		if s.auditLogger != nil {
			s.auditLogger.Log(ctx, audit.Entry{
				CircleID:  acceptorID,
				EventType: string(events.EventInviteTokenInvalid),
				SubjectID: token.TokenID,
				Action:    "validate_invite_token",
				Outcome:   "failure",
				Metadata: map[string]string{
					"error": err.Error(),
				},
			})
		}
		return nil, err
	}

	// Check if acceptor is authorized
	if !token.CanBeAcceptedBy(acceptorID) {
		if s.auditLogger != nil {
			s.auditLogger.Log(ctx, audit.Entry{
				CircleID:  acceptorID,
				EventType: string(events.EventInviteTokenRejected),
				SubjectID: token.TokenID,
				Action:    "accept_invite_token",
				Outcome:   "unauthorized",
				Metadata: map[string]string{
					"reason": "not authorized acceptor",
				},
			})
		}
		return nil, primitives.ErrUnauthorizedAcceptor
	}

	// Create or verify acceptor circle exists
	acceptor, err := s.circleRuntime.Get(ctx, acceptorID)
	if err != nil {
		// Create the acceptor circle
		acceptor, err = s.circleRuntime.Create(ctx, circle.CreateRequest{
			TenantID: "demo-tenant",
		})
		if err != nil {
			return nil, fmt.Errorf("failed to create acceptor circle: %w", err)
		}
		// Update the acceptor ID to match
		acceptorID = acceptor.ID
	}

	if acceptor.State != circle.StateActive {
		return nil, fmt.Errorf("acceptor circle is not active")
	}

	// Create the intersection
	now := time.Now()
	intContract := intersection.Contract{
		Version: "1.0.0",
		Parties: []intersection.Party{
			{
				CircleID:      token.IssuerCircleID,
				PartyType:     "initiator",
				JoinedAt:      token.IssuedAt,
				GrantedScopes: extractScopeNames(token.Template.Scopes),
			},
			{
				CircleID:      acceptorID,
				PartyType:     "acceptor",
				JoinedAt:      now,
				GrantedScopes: extractScopeNames(token.Template.Scopes),
			},
		},
		Scopes:     convertScopes(token.Template.Scopes),
		Ceilings:   convertCeilings(token.Template.Ceilings),
		Governance: convertGovernance(token.Template.Governance),
		CreatedAt:  now,
	}

	intReq := intersection.CreateRequest{
		TenantID:    "demo-tenant",
		InitiatorID: token.IssuerCircleID,
		AcceptorID:  acceptorID,
		Contract:    intContract,
		InviteToken: token.TokenID,
	}

	int, err := s.intRuntime.Create(ctx, intReq)
	if err != nil {
		return nil, fmt.Errorf("failed to create intersection: %w", err)
	}

	// Log audit events
	if s.auditLogger != nil {
		// Token accepted
		s.auditLogger.Log(ctx, audit.Entry{
			CircleID:       acceptorID,
			IntersectionID: int.ID,
			EventType:      string(events.EventInviteTokenAccepted),
			SubjectID:      token.TokenID,
			Action:         "accept_invite_token",
			Outcome:        "success",
			Metadata: map[string]string{
				"issuer_circle": token.IssuerCircleID,
				"intersection":  int.ID,
			},
		})

		// Intersection created
		s.auditLogger.Log(ctx, audit.Entry{
			CircleID:       token.IssuerCircleID,
			IntersectionID: int.ID,
			EventType:      string(events.EventIntersectionCreated),
			SubjectID:      int.ID,
			Action:         "create_intersection",
			Outcome:        "success",
			Metadata: map[string]string{
				"version":       intContract.Version,
				"party_count":   fmt.Sprintf("%d", len(intContract.Parties)),
				"scope_count":   fmt.Sprintf("%d", len(intContract.Scopes)),
				"ceiling_count": fmt.Sprintf("%d", len(intContract.Ceilings)),
				"name":          token.ProposedName,
			},
		})
	}

	return &circle.IntersectionRef{
		IntersectionID: int.ID,
		Version:        intContract.Version,
		CreatedAt:      now,
	}, nil
}

// extractScopeNames extracts scope names from a slice of scopes.
func extractScopeNames(scopes []primitives.IntersectionScope) []string {
	names := make([]string, len(scopes))
	for i, s := range scopes {
		names[i] = s.Name
	}
	return names
}

// convertScopes converts primitives scopes to intersection scopes.
func convertScopes(scopes []primitives.IntersectionScope) []intersection.Scope {
	result := make([]intersection.Scope, len(scopes))
	for i, s := range scopes {
		result[i] = intersection.Scope{
			Name:        s.Name,
			Description: s.Description,
			ReadWrite:   s.Permission,
		}
	}
	return result
}

// convertCeilings converts primitives ceilings to intersection ceilings.
func convertCeilings(ceilings []primitives.IntersectionCeiling) []intersection.Ceiling {
	result := make([]intersection.Ceiling, len(ceilings))
	for i, c := range ceilings {
		result[i] = intersection.Ceiling{
			Type:  c.Type,
			Value: c.Value,
			Unit:  c.Unit,
		}
	}
	return result
}

// convertGovernance converts primitives governance to intersection governance.
func convertGovernance(gov primitives.IntersectionGovernance) intersection.Governance {
	return intersection.Governance{
		AmendmentRequires: gov.AmendmentRequires,
		DissolutionPolicy: gov.DissolutionPolicy,
	}
}

// Verify interface compliance at compile time.
var (
	_ circle.InviteIssuer   = (*InviteService)(nil)
	_ circle.InviteAcceptor = (*InviteService)(nil)
)
