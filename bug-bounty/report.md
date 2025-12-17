# Security Assessment Report: Go-Ethereum (Geth)

## Executive Summary
This assessment focused on the Go-Ethereum (Geth) codebase, specifically targeting high-severity vulnerabilities in the P2P networking, Transaction Pool, and RPC subsystems.

A **High-Severity Denial of Service (DoS)** vulnerability was identified in the handling of Blob Transactions (EIP-4844). Attackers can exploit this to exhaust node CPU resources by spamming invalid blob transactions, which trigger expensive KZG verifications before the node applies any effective mitigation (peer dropping).

## Findings

### [HIGH] CPU Exhaustion via Invalid Blob Transaction Spam

**Severity:** High
**Component:** `eth/fetcher`, `core/txpool`
**Affected Versions:** Geth versions supporting Cancun (EIP-4844)

#### Description
The Geth node allows peers to push transactions via `PooledTransactionsResponse` messages. The `eth` protocol handler accepts these messages even if they were not explicitly requested (unsolicited push). When processing these transactions, the `TxFetcher` enqueues them into the `TxPool`.

The `TxPool.Add` method performs validation, including the computationally expensive KZG proof verification for Blob Transactions (`VerifyBlobProof`). This verification happens *before* the transaction is fully accepted into the pool.

If the verification fails (e.g., due to an invalid proof), the `TxFetcher` counts the rejection but **does not drop the peer**. Instead, it applies a short sleep (200ms) if the rejection rate is high. This throttling is insufficient to prevent CPU exhaustion, as the expensive work (KZG verification) is already completed before the sleep.

An attacker can establish multiple P2P connections and spam `PooledTransactionsResponse` messages containing batches of invalid blob transactions. Each batch forces the node to perform expensive KZG verifications. While the `eth` protocol defines a `softResponseLimit` of 2MB for responses, Geth's `handlePooledTransactions` does not enforce this limit on *incoming* messages (only on outgoing responses). The hard limit for incoming messages is `maxMessageSize` (10MB).

Even with a 2MB soft limit, an attacker can fit ~15 blob transactions (each ~131KB) into a message. Each transaction contains up to 6 blobs, requiring 6 KZG verifications. A single message can thus trigger ~90 KZG pairings. With multiple connections, this can saturate the node's CPU.

#### Technical Details
1. **Unsolicited Acceptance:** `eth/protocols/eth/handlers.go`'s `handlePooledTransactions` calls `backend.Handle`, which calls `h.txFetcher.Enqueue`. There is no check to verify if the transactions were requested.
2. **Sidecar Propagation:** The `eth` protocol allows `BlobTx` to be encoded with sidecars (blobs, commitments, proofs) on the wire. `eth/handler_eth.go` explicitly checks for the presence of these sidecars (`tx.BlobTxSidecar() == nil` check) and validates commitment hashes (cheap) before enqueuing.
3. **Expensive Validation:** `TxFetcher.Enqueue` calls `f.addTxs`, which calls `TxPool.Add`. `TxPool.Add` calls `ValidateTxBasics` -> `ValidateTransaction` -> `validateBlobTx` -> `kzg4844.VerifyBlobProof`. This involves expensive elliptic curve pairing operations.
4. **Ineffective Mitigation:** `TxFetcher.Enqueue` checks for errors. If `VerifyBlobProof` fails, it returns a generic error (e.g., `invalid blob 0: ...`). This error falls into the `default` case (other reject) in `TxFetcher.Enqueue`. It increments `otherreject`. If `otherreject > batch/4`, it sleeps 200ms. It does **not** drop the peer.
5. **Persistence:** The `cleanup` routine in `TxFetcher` only drops peers for metadata mismatches on *requested* transactions (in `waitlist`). Unsolicited transactions are not in `waitlist`, so no penalty is applied. Even for solicited transactions, invalid proofs do not trigger the metadata mismatch drop logic.

#### Reproduction Steps
1. Connect to a victim Geth node using the `eth/68` protocol.
2. Construct a valid `PooledTransactionsResponse` message containing ~15 Blob Transactions (approx 2MB payload).
3. For each transaction:
   - Use a valid format and signature (random key).
   - Include 6 blobs.
   - Use valid commitments (random points on curve) matching the versioned hashes.
   - Use **invalid proofs**.
   - Encode using the "network encoding" (list of lists) to include the sidecar.
4. Send the message to the victim.
5. Observe the victim node consuming CPU for KZG verification.
6. Repeat continuously.
7. Scale to 10-20 connections to fully saturate a multi-core server.

#### Recommendation
1. **Enforce Request-Response:** Modify `eth/handler_eth.go` or `TxFetcher` to reject `PooledTransactionsResponse` messages that do not correspond to an active request (unless explicitly announced and waiting).
2. **Drop Malicious Peers:** In `TxFetcher.Enqueue`, if `TxPool.Add` returns a validation error implying malicious intent (like invalid signature or invalid blob proof), immediately drop the peer.
3. **Rate Limit Validation:** Implement stricter rate limiting for blob transaction validation per peer before performing the expensive KZG check.

---

## Other Analyzed Areas

### P2P UDP Discovery (v4)
**Status:** Secure
- **Amplification:** The `Ping`/`Pong` exchange has negligible amplification. `Findnode` requests are protected by a "bonding" mechanism (`checkBond`), which requires the requester to have previously echoed a `Ping` (proving IP ownership). This prevents IP spoofing attacks that could lead to amplification via `Neighbors` responses.

### RPC API Exposure
**Status:** Secure
- **Defaults:** The default `HTTPModules` are limited to `net` and `web3`. The `eth` module is added by the CLI command. Sensitive modules like `admin`, `personal`, and `debug` are not exposed by default.
- **Configuration:** The configuration loading path (`node/defaults.go` -> `cmd/geth/config.go`) correctly isolates dangerous APIs.

### EVM Memory & Gas
**Status:** Secure
- **Memory Expansion:** The `memoryGasCost` function in `core/vm/gas_table.go` correctly calculates quadratic gas costs and checks for integer overflows (`newMemSize > 0x1FFFFFFFE0`).
- **Allocation:** Memory allocation in `core/vm/memory.go` is bounded by the block gas limit, preventing OOM attacks via large allocations.
