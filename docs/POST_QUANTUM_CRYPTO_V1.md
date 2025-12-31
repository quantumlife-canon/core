# Post-Quantum Cryptography Strategy

**Version**: 1.0
**Status**: Approved
**Last Updated**: 2024-01-15

## Executive Summary

This document defines QuantumLife Canon's cryptographic posture and post-quantum (PQC) migration strategy. The system uses crypto agility to ensure that cryptographic algorithms can be upgraded without breaking existing signatures or sealed data.

## Threat Model

### Quantum Computing Threats

| Threat | Timeline | Impact | Mitigation |
|--------|----------|--------|------------|
| **Harvest Now, Decrypt Later (HNDL)** | Active now | Encrypted data captured today decrypted by future quantum computers | Use hybrid encryption for long-term secrets |
| **Signature Forgery** | 10-15 years | RSA/ECDSA signatures broken by Shor's algorithm | Dual-signature format with PQC extension |
| **Key Recovery** | 10-15 years | Private keys derived from public keys | PQC key pairs for new keys |

### Current Cryptographic Posture

| Use Case | Current Algorithm | PQC-Ready | Migration Path |
|----------|-------------------|-----------|----------------|
| **Audit Log Signing** | Ed25519 | Partial | Dual-signature (Ed25519 + ML-DSA) |
| **Token Encryption** | AES-256-GCM | Yes | AES-256 is quantum-resistant |
| **Key Derivation** | HKDF-SHA256 | Yes | SHA-256 is quantum-resistant |
| **Content Hashing** | SHA-256 | Yes | SHA-256 is quantum-resistant |
| **Approval Signatures** | Ed25519 | Partial | Dual-signature format |

## Crypto Agility Architecture

### Principle: Algorithm Indirection

All cryptographic operations go through algorithm-agile interfaces. No code directly references specific algorithms.

```go
// Algorithm identifier - allows runtime selection
type AlgorithmID string

const (
    AlgEd25519    AlgorithmID = "Ed25519"
    AlgMLDSA65    AlgorithmID = "ML-DSA-65"      // Future PQC
    AlgKyber768   AlgorithmID = "ML-KEM-768"     // Future PQC
    AlgHybridSig  AlgorithmID = "Ed25519+ML-DSA" // Dual-signature
)
```

### Signature Record Format

Signatures include algorithm metadata for verification:

```go
type SignatureRecord struct {
    Algorithm AlgorithmID // Which algorithm was used
    KeyID     string      // Which key signed
    Signature []byte      // The signature bytes
    SignedAt  time.Time   // When signature was created

    // PQC extension (optional, for dual-signature)
    PQCAlgorithm *AlgorithmID // PQC algorithm if present
    PQCSignature []byte       // PQC signature bytes
}
```

### Sealed Data Format

Encrypted data includes algorithm metadata for decryption:

```go
type SealRecord struct {
    Algorithm AlgorithmID // Encryption algorithm
    KeyID     string      // Which key encrypted
    Nonce     []byte      // Nonce/IV
    Ciphertext []byte     // Encrypted data
    Tag        []byte     // Authentication tag

    // PQC extension (optional, for hybrid encryption)
    PQCAlgorithm *AlgorithmID // PQC KEM if present
    PQCCiphertext []byte      // PQC-encapsulated key
}
```

## Migration Strategy

### Phase 1: Foundation (Current)

**Goal**: Establish crypto agility without changing runtime behavior

**Deliverables**:
- `pkg/crypto/agility.go` - Algorithm IDs, record types
- `SignatureRecord` and `SealRecord` types with PQC extension fields
- Ed25519 implementation using Go stdlib

**Status**: In progress

### Phase 2: Monitoring (Q2 2024)

**Goal**: Track cryptographic usage for migration planning

**Deliverables**:
- Audit events for all cryptographic operations
- Algorithm usage metrics
- Key age and rotation tracking

### Phase 3: Hybrid Readiness (Q4 2024)

**Goal**: Prepare for dual-signature format

**Deliverables**:
- Dual-signature verification (accept either or both)
- Signature format versioning
- Key pair generation with PQC extension

### Phase 4: PQC Integration (2025+)

**Goal**: Add PQC algorithms when NIST standards finalize

**Deliverables**:
- ML-DSA-65 (Dilithium) for signatures
- ML-KEM-768 (Kyber) for key encapsulation
- Hybrid mode by default

## Algorithm Selection Criteria

### Signatures

| Algorithm | Security Level | Signature Size | Verification Speed | Status |
|-----------|---------------|----------------|-------------------|--------|
| Ed25519 | 128-bit classical | 64 bytes | Very fast | Current default |
| ML-DSA-65 | 128-bit quantum | ~3.3 KB | Fast | Future PQC |
| Ed25519+ML-DSA | Hybrid | ~3.4 KB | Fast | Transition |

### Key Encapsulation

| Algorithm | Security Level | Ciphertext Size | Decapsulation Speed | Status |
|-----------|---------------|-----------------|---------------------|--------|
| X25519 | 128-bit classical | 32 bytes | Very fast | Not used (we use symmetric) |
| ML-KEM-768 | 128-bit quantum | ~1.1 KB | Fast | Future if needed |

### Symmetric Encryption

| Algorithm | Security Level | Overhead | Speed | Status |
|-----------|---------------|----------|-------|--------|
| AES-256-GCM | 128-bit quantum | 16 bytes tag | Very fast | Current, quantum-safe |

## Implementation Guidelines

### 1. Never Hardcode Algorithms

```go
// BAD: Hardcoded algorithm
func Sign(data []byte) []byte {
    return ed25519.Sign(privateKey, data)
}

// GOOD: Algorithm-agile
func Sign(ctx context.Context, alg AlgorithmID, keyID string, data []byte) (SignatureRecord, error) {
    signer, err := keyManager.GetSigner(ctx, keyID)
    if err != nil {
        return SignatureRecord{}, err
    }
    sig, err := signer.Sign(ctx, data)
    if err != nil {
        return SignatureRecord{}, err
    }
    return SignatureRecord{
        Algorithm: alg,
        KeyID:     keyID,
        Signature: sig,
        SignedAt:  clock.Now(),
    }, nil
}
```

### 2. Canonical Serialization for Signing

Always use deterministic serialization before signing:

```go
// Canonicalize before signing (v9.12 style)
func CanonicalizeForSigning(v interface{}) ([]byte, error) {
    // JSON with sorted keys, no whitespace
    return json.Marshal(v) // json.Marshal sorts map keys
}

// Sign canonical form
func SignCanonical(ctx context.Context, v interface{}) (SignatureRecord, error) {
    canonical, err := CanonicalizeForSigning(v)
    if err != nil {
        return SignatureRecord{}, err
    }
    hash := sha256.Sum256(canonical)
    return Sign(ctx, currentAlgorithm, currentKeyID, hash[:])
}
```

### 3. Verify with Algorithm from Record

```go
// Verify using algorithm specified in record
func Verify(ctx context.Context, data []byte, record SignatureRecord) error {
    verifier, err := keyManager.GetVerifier(ctx, record.KeyID)
    if err != nil {
        return err
    }

    // Verify classical signature
    if err := verifier.Verify(ctx, data, record.Signature); err != nil {
        return err
    }

    // If PQC signature present, verify that too
    if record.PQCSignature != nil {
        pqcVerifier, err := keyManager.GetPQCVerifier(ctx, record.KeyID)
        if err != nil {
            return err
        }
        if err := pqcVerifier.Verify(ctx, data, record.PQCSignature); err != nil {
            return err
        }
    }

    return nil
}
```

### 4. Key Rotation Without Downtime

```go
// Rotate key - old key remains valid for verification
func RotateSigningKey(ctx context.Context, keyID string) error {
    // Create new key version
    newMeta, err := keyManager.RotateKey(ctx, keyID)
    if err != nil {
        return err
    }

    // New signatures use new key
    // Old signatures still verifiable with old key version

    // Audit the rotation
    audit.Log(ctx, AuditEvent{
        Type:    "key_rotation",
        KeyID:   keyID,
        NewVersion: newMeta.AlgorithmVersion,
    })

    return nil
}
```

## Audit Trail Requirements

All cryptographic operations MUST be audited:

| Event Type | Required Fields | Purpose |
|------------|-----------------|---------|
| `signature_created` | keyID, algorithm, dataHash | Track algorithm usage |
| `signature_verified` | keyID, algorithm, success | Detect verification failures |
| `key_rotated` | keyID, oldVersion, newVersion | Track key lifecycle |
| `encryption_performed` | keyID, algorithm | Track encryption usage |
| `decryption_performed` | keyID, algorithm, success | Detect decryption failures |

## Testing Requirements

### Unit Tests

1. **Deterministic Payload Hashing**: Same input → same hash
2. **Signature Round-Trip**: Sign → Verify succeeds
3. **Tamper Detection**: Modified data → Verify fails
4. **Algorithm Identification**: Correct algorithm in records

### Integration Tests

1. **Key Rotation**: Old signatures verify after rotation
2. **Dual-Signature**: Verify with either or both
3. **Canonical Serialization**: Cross-platform consistency

## References

- [ADR-0017: Crypto Agility and PQC Roadmap](ADR/ADR-0017-crypto-agility-and-pqc-roadmap.md)
- [NIST Post-Quantum Cryptography Standards](https://csrc.nist.gov/projects/post-quantum-cryptography)
- [pkg/crypto/agility.go](../pkg/crypto/agility.go) - Implementation
