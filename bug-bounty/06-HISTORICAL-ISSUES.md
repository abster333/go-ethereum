# Historical Security Issues & Vulnerability Patterns

## Overview
This document compiles known historical security issues in Go-Ethereum and similar clients to inform current security reviews. Understanding past failures is critical for preventing recurrence.

## 1. Critical Consensus Bugs

### 1.1 The "Hades" Split (August 2021)
**Type:** Memory Corruption / Consensus Failure
**Component:** EVM / Precompiled Contracts
**CVE:** CVE-2021-39137
**Description:**
A vulnerability in the `dataCopy` (0x04) precompile allowed a malicious transaction to corrupt the `RETURNDATA` buffer. This occurred when the input and output memory ranges overlapped.
**Mechanism:**
- The precompile copied data from input to output.
- If input and output overlapped, the copy operation modified the source data while reading it.
- This corrupted the `RETURNDATA` buffer, leading to incorrect execution results.
- Nodes with the bug calculated a different state root than patched nodes, causing a chain split.
**Fix:**
- Enforce defensive copying or handle overlapping memory ranges correctly.
- **Lesson:** Always validate memory ranges for overlap in copy operations.

### 1.2 The "Berlin" DoS (April 2021)
**Type:** Denial of Service
**Component:** State Trie / Storage
**Description:**
Attackers exploited underpriced storage operations (specifically `SLOAD`) to spam the network with expensive transactions.
**Mechanism:**
- The gas cost for accessing storage was too low relative to the computational/IO cost.
- Attackers created contracts that performed many storage reads.
- Nodes struggled to process blocks within the block time, leading to sync issues and network instability.
**Fix:**
- EIP-2929 increased gas costs for state access.
- EIP-2930 introduced optional access lists.
- **Lesson:** Gas costs must accurately reflect underlying resource usage (IO, CPU, Memory).

### 1.3 The "Shanghai" DoS (September 2016)
**Type:** Denial of Service
**Component:** EVM / IO
**Description:**
Attackers exploited the `EXTCODESIZE` opcode, which was underpriced.
**Mechanism:**
- The opcode required a disk read to fetch code size but cost very little gas.
- Attackers called this opcode thousands of times in a single transaction.
- This forced nodes to perform excessive disk I/O, slowing down processing significantly.
**Fix:**
- EIP-150 repriced IO-heavy opcodes.
- **Lesson:** IO operations are the most common vector for DoS attacks.

## 2. Networking & P2P Vulnerabilities

### 2.1 Eclipse Attacks
**Type:** Network Isolation
**Component:** P2P / Discovery
**Description:**
An attacker monopolizes a victim node's peer connections, isolating it from the honest network.
**Mechanism:**
- Attacker fills the victim's peer table with malicious nodes.
- Victim only receives blocks/txs from the attacker.
- Attacker can feed false chain data (e.g., double spends).
**Mitigation:**
- IP diversity enforcement.
- Peer scoring and reputation.
- Random peer selection.
- **Lesson:** Peer selection logic is critical for network health.

### 2.2 Discovery Protocol Amplification
**Type:** DDoS
**Component:** UDP Discovery Protocol
**Description:**
Attackers spoof source IP addresses to trick nodes into sending large responses to a victim.
**Mechanism:**
- Small request triggers large response (amplification factor).
- UDP allows IP spoofing.
**Mitigation:**
- Protocol design to minimize amplification.
- Rate limiting.
- **Lesson:** UDP protocols must be designed to prevent amplification.

## 3. RPC & API Vulnerabilities

### 3.1 Unprotected Admin APIs
**Type:** Remote Code Execution / Funds Theft
**Component:** JSON-RPC
**Description:**
Users accidentally exposing `admin_` or `personal_` APIs to the public internet.
**Mechanism:**
- Attacker scans for open port 8545.
- Calls `personal_unlockAccount` and sends funds.
- Or calls `admin_addPeer` to eclipse the node.
**Mitigation:**
- Secure defaults (localhost only).
- Removing dangerous methods from default interfaces.
- **Lesson:** Secure defaults are more effective than documentation.

### 3.2 DNS Rebinding
**Type:** Authentication Bypass
**Component:** JSON-RPC
**Description:**
Attacker uses a malicious website to proxy requests to the local Geth node.
**Mechanism:**
- User visits malicious site.
- Site uses DNS rebinding to make browser send requests to `localhost:8545`.
- Bypasses Same-Origin Policy.
**Mitigation:**
- Host header validation.
- **Lesson:** Validate Host headers in HTTP servers.

## 4. Cryptographic Vulnerabilities

### 4.1 Signature Malleability
**Type:** Transaction Replay / ID Manipulation
**Component:** ECDSA
**Description:**
Valid signatures can be modified to create a new valid signature for the same message.
**Mechanism:**
- ECDSA signatures `(r, s)` are valid. `(r, -s mod N)` is also valid.
- Attacker flips `s` value.
- Transaction hash changes, but validity remains.
**Mitigation:**
- Enforce low-s values (EIP-2).
- **Lesson:** Canonicalize cryptographic inputs and outputs.

## 5. Go-Specific Vulnerabilities in Geth

### 5.1 Map Iteration Non-Determinism
**Type:** Consensus Failure
**Description:**
Iterating over a Go map produces random order. If this order affects the block hash, consensus breaks.
**Mechanism:**
- Code iterates over a map of transactions or accounts.
- Order differs between nodes.
- Resulting Merkle root differs.
**Mitigation:**
- Always sort keys before processing map values in consensus code.
- **Lesson:** Never rely on map iteration order in deterministic code.

### 5.2 Goroutine Leaks in P2P
**Type:** Resource Exhaustion
**Description:**
Peers disconnecting without proper cleanup left goroutines running.
**Mechanism:**
- Blocked channel sends/receives without timeouts.
- Missing context cancellation.
**Mitigation:**
- Use `goleak` in tests.
- Ensure all blocking operations have timeouts/cancellation.

## 6. Vulnerability Patterns Checklist

When reviewing code, look for these patterns:

- [ ] **Underpriced Operations:** Is there a way to make the node do lots of work for little gas?
- [ ] **Memory Overlaps:** Are `copy()` operations safe?
- [ ] **Map Iteration:** Is map iteration used in consensus logic without sorting?
- [ ] **Integer Overflow:** Are gas calculations checked for overflow?
- [ ] **Unbounded Loops:** Can a user input trigger a large loop?
- [ ] **Panic Conditions:** Can external input cause a panic (slice bounds, nil dereference)?
- [ ] **Race Conditions:** Is shared state protected?
- [ ] **Resource Leaks:** Are file descriptors/goroutines/memory released on error paths?
- [ ] **Input Validation:** Are all RLP/JSON inputs validated for size and depth?
