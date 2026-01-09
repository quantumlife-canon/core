// Command qlsign-phase50 is a developer signing helper for Phase 50.
//
// CRITICAL: This tool uses the EXACT same CanonicalString() and MessageBytes()
// as the verification engine, ensuring signature generation matches verification.
// This is the single source of truth for canonical message format.
//
// Usage:
//
//	# Generate new keypair and sign a vendor claim:
//	qlsign-phase50 -type=claim \
//	  -kind=claim_vendor_cap \
//	  -scope=scope_human \
//	  -cap=allow_hold_only \
//	  -ref-hash=<64-hex-chars> \
//	  -provenance=provenance_user_supplied \
//	  -circle-id-hash=<64-hex-chars> \
//	  -period-key=2026-01-09
//
//	# Sign with existing private key (hex-encoded):
//	qlsign-phase50 -type=claim ... -privkey-hex=<128-hex-chars>
//
//	# Sign a pack manifest:
//	qlsign-phase50 -type=manifest \
//	  -pack-hash=<64-hex-chars> \
//	  -version=v1 \
//	  -bindings-hash=<64-hex-chars> \
//	  -provenance=provenance_marketplace \
//	  -circle-id-hash=<64-hex-chars> \
//	  -period-key=2026-01-09
//
// Output (pipe-delimited for easy parsing):
//
//	pubkey_b64|signature_b64|message_hash|canonical_string
//
// Reference: docs/ADR/ADR-0088-phase50-signed-vendor-claims-and-pack-manifests.md
package main

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"flag"
	"fmt"
	"os"

	domain "quantumlife/pkg/domain/signedclaims"
)

func main() {
	// Common flags
	signType := flag.String("type", "", "Type of data to sign: 'claim' or 'manifest' (required)")
	privkeyHex := flag.String("privkey-hex", "", "Ed25519 private key in hex (optional, generates new if not provided)")
	circleIDHash := flag.String("circle-id-hash", "", "Circle ID hash (64 hex chars, required)")
	periodKey := flag.String("period-key", "", "Period key like 2026-01-09 (required)")
	provenanceStr := flag.String("provenance", "", "Provenance: provenance_user_supplied|provenance_marketplace|provenance_admin (required)")

	// Claim-specific flags
	kindStr := flag.String("kind", "", "Claim kind (for claim type)")
	scopeStr := flag.String("scope", "", "Vendor scope (for claim type)")
	capStr := flag.String("cap", "", "Pressure cap (for claim type)")
	refHash := flag.String("ref-hash", "", "Reference hash (for claim type)")

	// Manifest-specific flags
	packHash := flag.String("pack-hash", "", "Pack hash (for manifest type)")
	versionStr := flag.String("version", "", "Pack version bucket (for manifest type)")
	bindingsHash := flag.String("bindings-hash", "", "Bindings hash (for manifest type, optional)")

	flag.Parse()

	if *signType == "" {
		fmt.Fprintln(os.Stderr, "error: -type is required (claim or manifest)")
		os.Exit(1)
	}

	// Get or generate private key
	var privateKey ed25519.PrivateKey
	if *privkeyHex != "" {
		decoded, err := hex.DecodeString(*privkeyHex)
		if err != nil {
			fmt.Fprintf(os.Stderr, "error: invalid privkey-hex: %v\n", err)
			os.Exit(1)
		}
		if len(decoded) != ed25519.PrivateKeySize {
			fmt.Fprintf(os.Stderr, "error: privkey-hex must be %d bytes (%d hex chars)\n",
				ed25519.PrivateKeySize, ed25519.PrivateKeySize*2)
			os.Exit(1)
		}
		privateKey = ed25519.PrivateKey(decoded)
	} else {
		// Generate new keypair
		_, priv, err := ed25519.GenerateKey(rand.Reader)
		if err != nil {
			fmt.Fprintf(os.Stderr, "error: failed to generate keypair: %v\n", err)
			os.Exit(1)
		}
		privateKey = priv
		// Print the private key to stderr so it can be reused
		fmt.Fprintf(os.Stderr, "Generated new keypair. Private key (save this for reuse):\n")
		fmt.Fprintf(os.Stderr, "  -privkey-hex=%s\n", hex.EncodeToString(privateKey))
	}

	publicKey := privateKey.Public().(ed25519.PublicKey)

	var messageBytes []byte
	var canonicalString string

	switch *signType {
	case "claim":
		messageBytes, canonicalString = signClaim(
			*kindStr, *scopeStr, *capStr, *refHash,
			*provenanceStr, *circleIDHash, *periodKey,
		)
	case "manifest":
		messageBytes, canonicalString = signManifest(
			*packHash, *versionStr, *bindingsHash,
			*provenanceStr, *circleIDHash, *periodKey,
		)
	default:
		fmt.Fprintf(os.Stderr, "error: -type must be 'claim' or 'manifest', got %q\n", *signType)
		os.Exit(1)
	}

	// Sign the message
	signature := ed25519.Sign(privateKey, messageBytes)

	// Compute message hash
	msgHash := domain.ComputeSafeRefHash(messageBytes)

	// Encode outputs
	pubkeyB64 := base64.StdEncoding.EncodeToString(publicKey)
	signatureB64 := base64.StdEncoding.EncodeToString(signature)

	// Output: pubkey_b64|signature_b64|message_hash|canonical_string
	fmt.Printf("%s|%s|%s|%s\n", pubkeyB64, signatureB64, msgHash, canonicalString)
}

func signClaim(kindStr, scopeStr, capStr, refHashStr, provenanceStr, circleIDHashStr, periodKey string) ([]byte, string) {
	// Validate required fields
	if kindStr == "" || scopeStr == "" || capStr == "" || refHashStr == "" ||
		provenanceStr == "" || circleIDHashStr == "" || periodKey == "" {
		fmt.Fprintln(os.Stderr, "error: for claim type, all of -kind, -scope, -cap, -ref-hash, -provenance, -circle-id-hash, -period-key are required")
		os.Exit(1)
	}

	// Parse and validate enums
	kind := domain.ClaimKind(kindStr)
	if err := kind.Validate(); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}

	scope := domain.VendorScope(scopeStr)
	if err := scope.Validate(); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}

	cap := domain.PressureCap(capStr)
	if err := cap.Validate(); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}

	provenance := domain.Provenance(provenanceStr)
	if err := provenance.Validate(); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}

	refHash := domain.SafeRefHash(refHashStr)
	if err := refHash.Validate(); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}

	circleIDHash := domain.SafeRefHash(circleIDHashStr)
	if err := circleIDHash.Validate(); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}

	// Build claim using EXACT same struct as verifier
	claim := domain.SignedVendorClaim{
		Kind:         kind,
		Scope:        scope,
		Cap:          cap,
		RefHash:      refHash,
		Provenance:   provenance,
		CircleIDHash: circleIDHash,
		PeriodKey:    periodKey,
	}

	if err := claim.Validate(); err != nil {
		fmt.Fprintf(os.Stderr, "error: claim validation failed: %v\n", err)
		os.Exit(1)
	}

	// Use EXACT same MessageBytes() as verifier - single source of truth
	return claim.MessageBytes(), claim.CanonicalString()
}

func signManifest(packHashStr, versionStr, bindingsHashStr, provenanceStr, circleIDHashStr, periodKey string) ([]byte, string) {
	// Validate required fields
	if packHashStr == "" || versionStr == "" || provenanceStr == "" ||
		circleIDHashStr == "" || periodKey == "" {
		fmt.Fprintln(os.Stderr, "error: for manifest type, all of -pack-hash, -version, -provenance, -circle-id-hash, -period-key are required")
		os.Exit(1)
	}

	// Parse and validate
	version := domain.PackVersionBucket(versionStr)
	if err := version.Validate(); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}

	provenance := domain.Provenance(provenanceStr)
	if err := provenance.Validate(); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}

	packHash := domain.SafeRefHash(packHashStr)
	if err := packHash.Validate(); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}

	circleIDHash := domain.SafeRefHash(circleIDHashStr)
	if err := circleIDHash.Validate(); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}

	// bindings_hash is optional - use zero hash if not provided
	var bindingsHash domain.SafeRefHash
	if bindingsHashStr != "" {
		bindingsHash = domain.SafeRefHash(bindingsHashStr)
		if err := bindingsHash.Validate(); err != nil {
			fmt.Fprintf(os.Stderr, "error: %v\n", err)
			os.Exit(1)
		}
	} else {
		bindingsHash = domain.SafeRefHash("0000000000000000000000000000000000000000000000000000000000000000")
	}

	// Build manifest using EXACT same struct as verifier
	manifest := domain.SignedPackManifest{
		PackHash:     packHash,
		Version:      version,
		BindingsHash: bindingsHash,
		Provenance:   provenance,
		CircleIDHash: circleIDHash,
		PeriodKey:    periodKey,
	}

	if err := manifest.Validate(); err != nil {
		fmt.Fprintf(os.Stderr, "error: manifest validation failed: %v\n", err)
		os.Exit(1)
	}

	// Use EXACT same MessageBytes() as verifier - single source of truth
	return manifest.MessageBytes(), manifest.CanonicalString()
}
