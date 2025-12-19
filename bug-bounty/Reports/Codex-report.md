# Security Assessment Report: Go-Ethereum (Geth)

## Executive Summary

This report documents a **high-confidence, practically exploitable** remote Denial-of-Service issue in Geth’s devp2p `eth` protocol transaction delivery path. A remote peer can send **unsolicited** `PooledTransactionsMsg` (0x0a) messages containing blob transactions with invalid KZG proofs. Geth will still perform expensive KZG verification during stateless tx validation, and the current mitigation does **not** disconnect or strongly throttle the malicious peer. With multiple inbound peer connections, an attacker can exhaust CPU and degrade node availability.

**Repository state assessed:** `03138851a` (from `git describe --tags --always`).

## Findings

### [HIGH] CPU DoS via unsolicited `PooledTransactionsMsg` triggering KZG verification

**Severity:** High  
**Category:** `oom_dos`  
**Component:** `eth/protocols/eth`, `eth/fetcher`, `core/txpool` / `core/txpool/blobpool`  
**Affected versions:** Present in this repo state (`03138851a`). Likely affects any version where `PooledTransactionsMsg` is accepted unsolicited and forwarded into txpool validation.

#### Description

Geth accepts `PooledTransactionsMsg` (0x0a) from peers and forwards it to the backend without verifying it corresponds to an outstanding request (`GetPooledTransactionsMsg`). Specifically, `handlePooledTransactions` decodes the packet, calls `requestTracker.Fulfil(...)` (metrics only), then unconditionally calls `backend.Handle(...)` (`eth/protocols/eth/handlers.go:514`).

The `eth` backend treats this packet type as a “direct” pooled-transaction delivery and calls `TxFetcher.Enqueue(..., direct=true)` (`eth/handler_eth.go:71`). `TxFetcher.Enqueue` synchronously calls into the txpool via its `addTxs` callback (`eth/fetcher/tx_fetcher.go:289`), which is wired to `h.txpool.Add(txs, false)` (`eth/handler.go:189`).

For blob transactions, txpool stateless validation includes expensive KZG proof checks (`core/txpool/validation.go:195`). In the blob pool specifically, this heavy work happens in the lock-free `preCheck` path via `ValidateTxBasics` (which calls `txpool.ValidateTransaction`) (`core/txpool/blobpool/blobpool.go:1652` and `core/txpool/blobpool/blobpool.go:1663`).

If the proof verification fails, the error is counted as an “other reject”, and the only mitigation is a fixed `200ms` sleep when `otherreject > addTxsBatchSize/4` (`eth/fetcher/tx_fetcher.go:355`). There is **no** peer disconnect/ban for repeated invalid blob proofs in this path.

#### Why this is exploitable

- The work is triggered by an **untrusted remote peer** (default configuration accepts inbound peers).
- KZG verification cost is paid even for invalid proofs.
- Unsolicited replies are not treated as protocol violations; after-the-fact request correlation only logs a warning for `direct` deliveries with no matching request (`eth/fetcher/tx_fetcher.go:699`, warning at `eth/fetcher/tx_fetcher.go:706`).
- The attacker can scale linearly by opening more peer connections, causing many goroutines to perform expensive verification concurrently.

#### Exploit scenario

1. Attacker opens multiple devp2p peer connections to the victim.
2. For each connection, attacker repeatedly sends `PooledTransactionsMsg` messages containing blob transactions with:
   - Correct blob commitment-hash relationships (passes cheap `ValidateBlobCommitmentHashes` check in `eth/handler_eth.go:80` and `core/txpool/validation.go:179`).
   - **Invalid** KZG proofs (fails in `core/txpool/validation.go:195` after doing the expensive verification).
3. The victim repeatedly performs KZG verification; peers are not disconnected, so the attacker can sustain CPU exhaustion.

#### Relevant limits / notes

- `eth` caps a single protocol message size at **10 MiB** (`eth/protocols/eth/protocol.go:49`) and enforces it in `handleMessage` (`eth/protocols/eth/handler.go:206`).
- `TxFetcher.Enqueue` processes deliveries in batches of 128 txs (`addTxsBatchSize`), and sleeps `200ms` if more than 25% of a batch are “other rejects” (`eth/fetcher/tx_fetcher.go:355`). An attacker can avoid triggering the sleep by keeping each delivery batch small (e.g., ≤32 invalid-proof txs per batch), and can always scale by adding more peers.
- On networks where blob sidecars are versioned with cell proofs, the corresponding verification path is `core/txpool/validation.go:206` (not required for this exploit on legacy blob sidecars).

#### Fix recommendations

1. **Enforce request/response pairing for `PooledTransactionsMsg`:** reject/disconnect when `RequestId` does not match an outstanding `GetPooledTransactionsMsg` request (treat as protocol violation) in `eth/protocols/eth/handlers.go:514` before forwarding to `backend.Handle`.
2. **Disconnect/penalize peers for invalid blob proofs:** classify repeated blob-proof failures as malicious and drop peers early in the `TxFetcher.Enqueue` path (`eth/fetcher/tx_fetcher.go:289`) or in the `eth` backend handler (`eth/handler_eth.go:71`).
3. **Add per-peer pre-verification budgeting:** rate-limit blob-proof verification by peer ID before expensive KZG work, rather than sleeping only after the work is done.
