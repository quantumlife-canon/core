# ADR-0017: Crypto Agility and Post-Quantum Cryptography Roadmap

## Status

Accepted

## Context

QuantumLife Canon uses cryptographic signatures for:
- Audit log integrity
- Approval artifact signing
- Action hash computation (v9.6 idempotency)
- View snapshot binding (v9.12/v9.13)

Current cryptographic algorithms (RSA, ECDSA, Ed25519) are vulnerable to quantum computers running Shor's algorithm. NIST has standardized post-quantum cryptographic (PQC) algorithms (ML-DSA, ML-KEM) that resist quantum attacks.

We need a migration path that:
1. Does not break existing signatures
2. Allows gradual adoption of PQC
3. Uses only Go stdlib (no external dependencies)
4. Maintains deterministic behavior for Canon guardrails

## Decision

### 1. Algorithm Agility via Indirection

All cryptographic operations use algorithm identifiers, not hardcoded algorithms:

```go
type AlgorithmID string

const (
    AlgEd25519    AlgorithmID = "Ed25519"
    AlgMLDSA65    AlgorithmID = "ML-DSA-65"
    AlgHybridSig  AlgorithmID = "Ed25519+ML-DSA"
)
```

### 2. SignatureRecord and SealRecord Types

Signature and encryption outputs include metadata for verification:

```go
type SignatureRecord struct {
    Algorithm    AlgorithmID
    KeyID        string
    Signature    []byte
    SignedAt     time.Time
    PQCAlgorithm *AlgorithmID // Optional PQC extension
    PQCSignature []byte       // Optional PQC signature
}

type SealRecord struct {
    Algorithm    AlgorithmID
    KeyID        string
    Nonce        []byte
    Ciphertext   []byte
    Tag          []byte
    PQCAlgorithm *AlgorithmID // Optional PQC KEM
    PQCCiphertext []byte      // Optional PQC-wrapped key
}
```

### 3. Ed25519 as Current Default

Ed25519 from Go's `crypto/ed25519` package is the current signing algorithm:
- Fast verification
- Small signatures (64 bytes)
- No external dependencies
- Well-audited implementation

### 4. Dual-Signature Format for Transition

During PQC transition, signatures can include both classical and PQC:

```go
// Verify accepts either or both signatures
func (v *DualVerifier) Verify(ctx context.Context, data []byte, record SignatureRecord) error {
    classicalOK := v.classicalVerifier.Verify(ctx, data, record.Signature) == nil

    pqcOK := true
    if record.PQCSignature != nil {
        pqcOK = v.pqcVerifier.Verify(ctx, data, record.PQCSignature) == nil
    }

    // Accept if at least one is valid
    if classicalOK || pqcOK {
        return nil
    }
    return ErrInvalidSignature
}
```

### 5. Canonical Serialization

Before signing, data is canonicalized:

```go
func CanonicalHash(data []byte) []byte {
    h := sha256.Sum256(data)
    return h[:]
}
```

For structured data, use sorted JSON:

```go
func CanonicalJSON(v interface{}) ([]byte, error) {
    return json.Marshal(v) // Go's json.Marshal sorts map keys
}
```

### 6. No Auto-Generated Keys

Key generation requires explicit action:
- No automatic key creation
- Key IDs are deterministic from configuration
- Key rotation is audited

## Consequences

### Positive

1. **Future-Proof**: Can add PQC without breaking existing data
2. **Stdlib Only**: No external crypto dependencies
3. **Auditable**: All algorithm choices are explicit and logged
4. **Deterministic**: Canonical serialization ensures reproducibility

### Negative

1. **Larger Signatures**: Dual-signature format increases size
2. **Complexity**: Algorithm indirection adds abstraction
3. **Testing Burden**: Must test multiple algorithm paths

### Neutral

1. **Migration Effort**: PQC algorithms will require implementation when added
2. **Performance**: Ed25519 is already fast; PQC will be slower but acceptable

## Implementation

### Phase 1: Foundation (This ADR)

- `pkg/crypto/agility.go`: Types and Ed25519 implementation
- `pkg/crypto/agility_test.go`: Tests for deterministic hashing and signing

### Phase 2: Integration

- Update audit log signing to use `SignatureRecord`
- Update approval artifact signing
- Add algorithm to audit events

### Phase 3: PQC (Future)

- Add ML-DSA-65 implementation (when Go stdlib supports it)
- Enable dual-signature by default
- Deprecate classical-only mode

## Alternatives Considered

### 1. External Crypto Libraries

**Rejected**: Adds dependency management complexity and audit burden.

### 2. Wait for Go Stdlib PQC

**Partially Accepted**: We prepare the architecture now but wait for stdlib implementation.

### 3. Hardware Security Modules

**Deferred**: May add HSM support later for key management, but not for algorithm selection.

## References

- [POST_QUANTUM_CRYPTO_V1.md](../POST_QUANTUM_CRYPTO_V1.md) - Full strategy document
- [NIST FIPS 204 (ML-DSA)](https://csrc.nist.gov/pubs/fips/204/final)
- [NIST FIPS 203 (ML-KEM)](https://csrc.nist.gov/pubs/fips/203/final)
- [Go crypto/ed25519](https://pkg.go.dev/crypto/ed25519)
