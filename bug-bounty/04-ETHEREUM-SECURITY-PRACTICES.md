# Ethereum & Blockchain Security Best Practices

## Overview
This document compiles security best practices specific to Ethereum client implementations and blockchain systems, drawing from industry knowledge and past incidents.

## 1. Consensus-Critical Code Principles

### 1.1 Deterministic Execution
**Requirement:** All consensus-critical code must produce identical results across all implementations.

**Critical Areas:**
- State transition functions
- Block validation
- Transaction execution
- Gas calculations
- Hash computations
- Signature verification

**Common Pitfalls:**
- ❌ Floating-point arithmetic (non-deterministic)
- ❌ Map iteration order (randomized in Go)
- ❌ Timestamp-dependent logic beyond consensus rules
- ❌ Random number generation without consensus seed
- ❌ Platform-specific behavior differences
- ❌ Concurrent execution with race conditions

**Best Practices:**
- ✅ Use fixed-precision integer arithmetic
- ✅ Sort maps before iteration if order matters
- ✅ Use block timestamp/hash for randomness (where appropriate)
- ✅ Implement cross-client test vectors
- ✅ Run identical tests across different platforms

### 1.2 State Root Consistency
**Critical:** Any difference in state root calculation leads to chain split.

**Verification Points:**
- Account state hashing
- Storage trie construction
- Transaction receipt trie
- Merkle proof generation
- Verkle tree commitments (future)

**Historical Issues:**
- 2021: RETURNDATA corruption causing state root mismatch
- 2016: DAO hack and subsequent fork
- Various precompile bugs affecting state

**Testing Requirements:**
- Extensive state test coverage
- Fuzzing state transition inputs
- Cross-client validation
- Historical block replay testing

## 2. Transaction Validation Security

### 2.1 Signature Verification
**Critical Checks:**
```go
// Verify signature components are in valid range
func validateSignature(v byte, r, s *big.Int, homestead bool) bool {
    // v must be 27 or 28 (or chain-id adjusted)
    // r and s must be in [1, N-1] where N is curve order
    // Prevent signature malleability: s must be in lower half
    // Check for zero values
    // Validate recovery ID
}
```

**Vulnerabilities to Prevent:**
- Signature malleability (Bitcoin-style)
- Invalid curve point attacks
- Zero-value signatures
- Replay attacks (via chain ID)
- Recovery ID manipulation

**Best Practices:**
- ✅ Use chain ID in signature (EIP-155)
- ✅ Reject high s-values (malleability prevention)
- ✅ Validate r,s are in proper range
- ✅ Check for zero addresses after ecrecover
- ✅ Use constant-time comparison for signatures

### 2.2 Nonce Management
**Security Requirements:**
- Prevent nonce reuse (replay attacks)
- Enforce sequential nonce ordering
- Check for nonce wraparound (uint64 max)
- Handle gaps correctly in mempool

**Attack Vectors:**
- Replay attacks on different chains
- Nonce collision attacks
- Front-running via nonce manipulation

### 2.3 Gas Validation
**Critical Checks:**
- Gas limit vs block gas limit
- Gas price validation (base fee + priority fee)
- Gas * price doesn't overflow
- Sender has sufficient balance for max fee
- Blob gas pricing (EIP-4844)

**DoS Prevention:**
- Reject transactions with excessive gas limits
- Prevent integer overflow in gas calculations
- Validate gas refunds don't exceed usage

## 3. EVM Security Patterns

### 3.1 Memory Safety
**Go-specific Considerations:**
```go
// UNSAFE: Overlapping memory operations
copy(memory[start:end], memory[offset:offset+length])

// SAFE: Copy to temporary buffer first
temp := make([]byte, length)
copy(temp, memory[offset:offset+length])
copy(memory[start:end], temp)
```

**Historical Bug (2021):**
- DataCopy precompile with overlapping input/output
- Corrupted RETURNDATA buffer
- Led to consensus failure and chain split

**Best Practices:**
- ✅ Always validate memory bounds
- ✅ Avoid overlapping memory operations
- ✅ Use defensive copying for shared buffers
- ✅ Check for integer overflow in size calculations

### 3.2 Stack Management
**Validation:**
- Stack depth limit (1024)
- Underflow prevention
- Overflow prevention
- Proper cleanup after execution

### 3.3 Gas Metering
**Critical Requirements:**
- Accurate gas calculation per opcode
- Memory expansion gas
- Storage access gas (cold/warm)
- Gas refunds (capped)
- Precompile gas calculation

**Attack Prevention:**
- DoS via expensive operations
- Gas griefing attacks
- Out-of-gas edge cases

## 4. P2P Network Security

### 4.1 Eclipse Attack Prevention
**Mitigations:**
- Diverse peer selection (IP subnet diversity)
- Peer reputation scoring
- Automatic peer discovery
- Outbound connection limits
- Connection authentication

**Monitoring:**
- Peer count thresholds
- Geographic peer distribution
- Peer churn rate
- Network partition detection

### 4.2 DoS Protection
**Attack Vectors:**
- Message flooding
- Large message attacks
- Slow peer attacks
- Resource exhaustion

**Mitigations:**
- ✅ Message rate limiting
- ✅ Message size limits
- ✅ Timeout mechanisms
- ✅ Peer banning for misbehavior
- ✅ Connection limits

### 4.3 Message Validation
**RLP Deserialization Security:**
```go
// Validate before allocating
if length > maxAllowedSize {
    return errOversizedPayload
}

// Use limited readers
limitedReader := io.LimitReader(reader, maxSize)

// Prevent recursion DoS
if depth > maxDepth {
    return errExcessiveDepth
}
```

**Critical Checks:**
- Message size limits
- Depth limits (nested structures)
- Type validation
- Checksum verification
- Protocol version compatibility

## 5. State Management Security

### 5.1 State Trie Integrity
**Security Requirements:**
- Atomic state updates
- Consistent state root calculation
- Proof verification
- Snapshot consistency

**Potential Attacks:**
- State root manipulation
- Proof forgery
- Snapshot poisoning
- Database corruption

### 5.2 Verkle Tree Transition (Future)
**Critical Considerations:**
- Gradual migration from Merkle-Patricia to Verkle
- Dual-state maintenance during transition
- Proof format changes
- Backward compatibility

**Security Risks:**
- State desynchronization
- Proof verification bypass
- Transition window attacks

## 6. Precompiled Contract Security

### 6.1 Critical Precompiles
**High-Risk Precompiles:**
1. **ecRecover (0x01)** - Signature verification
2. **bigModExp (0x05)** - Modular exponentiation
3. **bn256Add/Mul/Pairing (0x06-0x08)** - Elliptic curve operations
4. **blake2f (0x09)** - Hash function
5. **Point evaluation (0x0a)** - KZG verification

### 6.2 Input Validation
**Requirements:**
- Validate all input parameters
- Check for edge cases (zero values, max values)
- Prevent integer overflow in size calculations
- Gas calculation must match actual work

**Example Vulnerabilities:**
```go
// UNSAFE: No bounds checking
result := new(big.Int).Exp(base, exp, mod)

// SAFE: Validate inputs first
if exp.BitLen() > maxExpBitLen {
    return errExcessiveExponent
}
if base.Sign() < 0 || mod.Sign() <= 0 {
    return errInvalidInput
}
```

### 6.3 Gas Calculation
**Accuracy Critical:**
- Must match computational complexity
- Prevent DoS via underpriced operations
- Align with EIP specifications
- Test edge cases thoroughly

**Historical Issues:**
- Various precompile gas calculation bugs
- Underpriced operations leading to DoS
- Consensus bugs from gas mismatches

## 7. Fork Management

### 7.1 Hard Fork Activation
**Best Practices:**
- ✅ Use block number OR timestamp-based activation
- ✅ Adequate notice period (months)
- ✅ Testnet activation first
- ✅ Coordinated client releases
- ✅ Clear upgrade documentation

**Risks:**
- Chain split if upgrade not universal
- Activation bugs
- Backward compatibility issues
- Downgrade attacks

### 7.2 Backward Compatibility
**Considerations:**
- Support old transaction types
- Maintain deprecated RPC methods temporarily
- Allow protocol version negotiation
- Clear deprecation timeline

## 8. Access Control & Authentication

### 8.1 RPC Security
**Default Security Posture:**
```
✅ GOOD: Restrictive by default
- Admin APIs bound to localhost only
- Personal namespace disabled
- Debug methods require explicit enable
- CORS disabled by default

❌ BAD: Open by default
- Exposing admin APIs publicly
- No authentication
- Permissive CORS
```

**Production Recommendations:**
- Use firewall rules for network isolation
- Require authentication for sensitive endpoints
- Use JWT for Engine API (CL-EL communication)
- Implement rate limiting at proxy level
- Monitor API usage for anomalies

### 8.2 Engine API Security
**Critical:** Consensus Layer ↔ Execution Layer communication

**Security Requirements:**
- ✅ JWT authentication mandatory
- ✅ Secure JWT secret generation and storage
- ✅ Token expiration and rotation
- ✅ Transport encryption (localhost exception)
- ✅ Validate all requests from CL

**Attack Prevention:**
- Prevent unauthorized block submission
- Validate payload integrity
- Ensure proper sequencing
- Detect malicious CL behavior

## 9. Cryptographic Best Practices

### 9.1 Random Number Generation
**Consensus-Safe Randomness:**
```go
// WRONG: Non-deterministic
random := rand.Int63()

// RIGHT: Use block data
seed := block.Hash()
rng := NewDeterministicRNG(seed)
```

**For Non-Consensus:**
- Use crypto/rand for keys
- Use secure seed sources
- Avoid predictable patterns

### 9.2 Constant-Time Operations
**Required for:**
- Signature verification
- MAC comparisons
- Key comparisons
- Secret-dependent branches

**Example:**
```go
import "crypto/subtle"

// WRONG: Timing attack vulnerable
if signature == expectedSignature {
    ...
}

// RIGHT: Constant-time comparison
if subtle.ConstantTimeCompare(signature, expectedSignature) == 1 {
    ...
}
```

### 9.3 Key Management
**Best Practices:**
- ✅ Encrypt keys at rest (scrypt/AES)
- ✅ Use external signers (Clef) when possible
- ✅ Implement key derivation (HD wallets)
- ✅ Zero out key material after use
- ✅ Avoid key logging
- ✅ Secure key backup procedures

## 10. Database Security

### 10.1 Data Integrity
**Protection Mechanisms:**
- Atomic writes (all-or-nothing)
- Checksums for critical data
- Snapshot consistency verification
- Ancient data freezing (immutability)

**Corruption Detection:**
- Merkle proof verification
- Block hash validation
- State root checks
- Database checksum validation

### 10.2 Performance DoS Prevention
**Limits:**
- Query result limits
- Iterator limits
- Memory cache bounds
- Disk I/O rate limiting

**Monitoring:**
- Database size
- Query latency
- Disk usage patterns
- Abnormal access patterns

## 11. Incident Response

### 11.1 Vulnerability Disclosure Process
**Timeline (from Geth team):**
1. **Day 0:** Vulnerability reported
2. **Day 0-1:** Initial assessment and fix development
3. **Day 1-3:** Testing and verification
4. **Day 3:** Notify downstream projects (private)
5. **Day 3-5:** Prepare release
6. **Day 5:** Public announcement + patch release
7. **Week 4-8:** Full disclosure with details

**Communication:**
- Advance notice to node operators
- Clear upgrade instructions
- Severity assessment
- Workarounds if available

### 11.2 Emergency Response
**Chain Split Response:**
1. Identify affected versions
2. Release emergency patch
3. Coordinate with miners/validators
4. Monitor network hash rate distribution
5. Provide clear upgrade guidance
6. Consider rollback if early detection

**DoS Attack Response:**
1. Identify attack vector
2. Implement temporary mitigations
3. Deploy rate limiting
4. Release permanent fix
5. Update best practices

## 12. Testing Requirements

### 12.1 Consensus Testing
**Required Test Coverage:**
- ✅ Official Ethereum test suite
- ✅ State tests (10,000+ tests)
- ✅ Block tests
- ✅ Transaction tests
- ✅ Cross-client compatibility tests
- ✅ Fork transition tests

### 12.2 Fuzzing
**Priority Targets:**
- RLP decoder/encoder
- EVM opcodes
- Precompiled contracts
- P2P message parsing
- Transaction deserialization
- ABI encoding/decoding

**Tools:**
- go-fuzz
- OSS-Fuzz integration
- AFL-based fuzzing
- Property-based testing

### 12.3 Integration Testing
**Scenarios:**
- Multi-client testnets
- Load testing (transaction throughput)
- Network partition simulation
- Byzantine behavior testing
- Upgrade/downgrade testing

## 13. Monitoring & Alerting

### 13.1 Security Metrics
**Monitor:**
- Chain split detection (competing tips)
- Consensus participation rate
- Peer count and diversity
- RPC error rates
- Memory/CPU anomalies
- Database corruption indicators

### 13.2 Anomaly Detection
**Indicators:**
- Unexpected peer churn
- Block propagation delays
- State sync failures
- High reorg rate
- Unusual transaction patterns
- Gas price anomalies

## 14. Supply Chain Security

### 14.1 Build Reproducibility
**Requirements:**
- Deterministic builds
- Published checksums
- Signed releases (GPG/minisign)
- Build environment documentation

### 14.2 Dependency Management
**Best Practices:**
- Pin dependency versions
- Regular security audits
- CVE monitoring
- Vendor critical dependencies
- Review dependency updates

## 15. Ethereum-Specific Attack Vectors

### 15.1 MEV (Maximal Extractable Value) Considerations
**Not directly a client bug, but important:**
- Front-running protection (not in core protocol)
- Uncle/reorg attacks
- Time-bandit attacks

**Client Responsibilities:**
- Fair transaction ordering
- Proper uncle handling
- Prevent miner-specific advantages (when applicable)

### 15.2 Long-Range Attacks (PoS)
**Mitigations:**
- Weak subjectivity checkpoints
- Finality verification
- Slashing condition enforcement
- Validator set tracking

### 15.3 Finality Reversion
**Protections:**
- Monitor finalized blocks
- Detect deep reorgs
- Alert on finality violations
- Checkpoint verification

## 16. Audit Checklist for Smart Contract Interactions

### 16.1 Call/Delegatecall Safety
**Client Implementation:**
- Proper gas forwarding
- Return data size limits
- Reentrancy protection in EVM
- Call depth limits

### 16.2 CREATE/CREATE2
**Security Checks:**
- Address collision prevention
- Init code validation
- Constructor gas limits
- Deployment success verification

## 17. Privacy Considerations

### 17.1 Information Leakage
**Prevent:**
- Transaction origin tracking (use mixers externally)
- Account linking
- Timing analysis
- Network topology exposure

### 17.2 Logging Security
**Best Practices:**
- Don't log private keys (obviously)
- Sanitize error messages
- Rate-limit verbose logging
- Secure log file permissions

## 18. Compliance & Legal

### 18.1 Sanctions Compliance
**Considerations:**
- OFAC compliance (tornado cash, etc.)
- Geographic restrictions
- Controversial transaction filtering

**Client Philosophy:**
- Generally permissionless and neutral
- Operators choose their policies
- Transparent about filtering (if any)

## 19. Future Ethereum Upgrades

### 19.1 Statelessness
**Security Implications:**
- Witness format validation
- Proof verification overhead
- Verkle tree security
- State provider trust

### 19.2 Data Availability Sampling (DAS)
**Considerations:**
- Sampling security
- Data withholding attacks
- Erasure code verification

### 19.3 Account Abstraction
**Security Concerns:**
- Validation function exploits
- Gas sponsorship attacks
- Bundler centralization

## 20. Cross-Client Security

### 20.1 Diversity Benefits
**Why Multiple Clients Matter:**
- Reduces impact of single implementation bug
- Prevents network-wide exploits
- Improves overall security
- Decentralization

**Testing Requirements:**
- Cross-client test suites
- Interoperability testing
- Consensus verification
- Compatibility matrices

### 20.2 Responsible Disclosure Coordination
**Multi-Client Coordination:**
- Parallel patching
- Coordinated releases
- Shared security information
- Common vulnerability database

## References & Resources

### Official Documentation
- Ethereum Yellow Paper (consensus rules)
- EIP specifications (ethereum.org/eips)
- Geth documentation (geth.ethereum.org/docs)

### Security Resources
- Ethereum bug bounty program (bounty.ethereum.org)
- Security advisories (github.com/ethereum/go-ethereum/security/advisories)
- Audit reports (docs/audits/)
- Postmortems (docs/postmortems/)

### Testing Resources
- Ethereum test suite (github.com/ethereum/tests)
- Hive testing framework
- Consensus spec tests

### Community
- Ethereum R&D Discord
- AllCoreDevs calls
- Security researchers community
