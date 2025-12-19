# Security Assessment Report: Go-Ethereum (Geth)

## Executive Summary
This assessment focused on the Go-Ethereum (Geth) codebase, specifically targeting high-severity vulnerabilities in the P2P networking, Transaction Pool, and RPC subsystems.

A **High-Severity Denial of Service (DoS)** vulnerability was identified in the handling of Blob Transactions (EIP-4844). Attackers can exploit this to exhaust node CPU resources by spamming invalid blob transactions, which trigger expensive KZG verifications before any effective mitigation (disconnect/ban) is applied.

## Findings

### [HIGH] CPU Exhaustion via Invalid Blob Transaction Spam

**Severity:** High
**Component:** `eth/protocols/eth`, `eth/handler_eth`, `eth/fetcher`, `core/txpool` (blob validation)
**Affected Versions:** Reproduced against go-ethereum source commit `3f02256db` (this checkout). Likely affects any release line where `PooledTransactionsMsg` (0x0a) is accepted and forwarded into txpool stateless validation without being tied to an outstanding `GetPooledTransactionsMsg` request (eth/68+).

#### Description
The Geth node allows peers to push transactions via `PooledTransactionsMsg` (0x0a) messages. The `eth` protocol handler accepts these messages even if they were not explicitly requested (unsolicited push). When processing these transactions, the `TxFetcher` enqueues them into the `TxPool`.

The txpool’s stateless validation performs computationally expensive KZG proof verification for blob transactions (`kzg4844.VerifyBlobProof`). In the current `eth` handling path, this expensive work is performed **synchronously** in the peer’s message handling flow, and repeated invalid proofs are not treated as a protocol violation (no disconnect/ban), enabling CPU exhaustion by an untrusted peer.

If the verification fails (e.g., due to an invalid proof), the `TxFetcher` counts the rejection but **does not drop the peer**. Instead, it applies a short sleep (200ms) if the rejection rate is high. This throttling is insufficient to prevent CPU exhaustion, as the expensive work (KZG verification) is already completed before the sleep.

An attacker can establish multiple P2P connections and spam `PooledTransactionsMsg` messages containing batches of invalid blob transactions, saturating the victim's CPU cores.

#### Technical Details
1.  **Transport Layer (`p2p/rlpx/rlpx.go`):**
    The `Conn.Read()` method reads messages at the RLPx layer. The relevant application-layer bound for this issue is the `eth` protocol’s `maxMessageSize` of **10 MiB** (defined in `eth/protocols/eth/protocol.go` and enforced in `eth/protocols/eth/handler.go`), which caps the size of a single decoded `eth` message.

2.  **Protocol Handler (`eth/protocols/eth/handlers.go`):**
    The `handlePooledTransactions` function decodes the incoming message. It calls `requestTracker.Fulfil` to track request latency, but this function does not error on unsolicited messages. The handler then unconditionally forwards the transactions to the backend.

    Crucially, the backend handler (`eth/handler_eth.go`) treats these pooled transactions as direct deliveries (`direct=true`), which bypasses some checks but eventually leads to a "Unexpected transaction delivery" warning in `eth/fetcher/tx_fetcher.go`—but only *after* the CPU-intensive validation has occurred.

    ```go
    func handlePooledTransactions(backend Backend, msg Decoder, peer *Peer) error {
        // ...
        if err := msg.Decode(&txs); err != nil {
            return err
        }
        // ...
        // Fulfil tracks metrics but does not block unsolicited messages
        requestTracker.Fulfil(peer.id, peer.version, PooledTransactionsMsg, txs.RequestId)

        // Unconditionally forward to backend
        return backend.Handle(peer, &txs.PooledTransactionsResponse)
    }
    ```

3.  **Transaction Fetcher (`eth/fetcher/tx_fetcher.go`):**
    The `Enqueue` method immediately adds the transactions to the pool via `f.addTxs(batch)`.
    ```go
    func (f *TxFetcher) Enqueue(peer string, txs []*types.Transaction, direct bool) error {
        // ...
        for j, err := range f.addTxs(batch) { // Triggers validation
            // ...
            switch {
            case err == nil:
            case errors.Is(err, txpool.ErrAlreadyKnown):
                duplicate++
            case errors.Is(err, txpool.ErrUnderpriced) || ...:
                underpriced++
            default:
                otherreject++ // Invalid blob proofs fall here
            }
        }
        // ...
        // If 'other reject' is >25% of the deliveries in any batch, sleep a bit.
        if otherreject > addTxsBatchSize/4 {
            time.Sleep(200 * time.Millisecond)
            // No disconnect logic here!
        }
    }
    ```
    Crucially:
    - Validation errors returned by `addTxs` are not propagated as a protocol-level error (so the peer is not disconnected by `handleMessage`).
    - `otherreject` only triggers a 200ms sleep, not a peer disconnect/ban.

4.  **Backend Handler (`eth/handler_eth.go`):**
    Pooled transactions are handled as “direct” deliveries (`direct=true`). There are explicit disconnect-triggering checks for missing sidecars and commitment-hash mismatch, but **no** KZG proof verification at this layer:
    - Sidecar-less blob tx => error (disconnect)
    - Commitment-hash mismatch => error (disconnect)
    - Invalid KZG proof => not checked here; falls through into txpool validation

5.  **Stateless Validation (`core/txpool/validation.go`):**
    For legacy blob sidecars, the expensive proof verification happens in `validateBlobSidecarLegacy`:
    - `kzg4844.VerifyBlobProof(&sidecar.Blobs[i], sidecar.Commitments[i], sidecar.Proofs[i])`
    This is invoked from the blob pool’s lock-free precheck path via:
    - `core/txpool/blobpool/blobpool.go: ValidateTxBasics -> txpool.ValidateTransaction`

6.  **Late Check (`eth/fetcher/tx_fetcher.go`):**
    The `cleanup` channel is processed asynchronously. Only *after* the transactions have been validated and added (or rejected) does the fetcher check `f.requests[delivery.origin]`. If no request exists, it logs a warning but does not retroactively undo the CPU work.

**Why the peer stays connected (code-level):**
`eth/protocols/eth/handler.go` documents that the remote connection is torn down upon returning any error from a message handler. In this path, invalid blob proofs result in txpool validation errors, but those are not returned as handler errors; `TxFetcher.Enqueue` accepts the batch, records rejection statistics, and returns `nil`. As a result, repeated invalid proofs do not trigger disconnect/ban at the protocol level.

#### Why this differs from “just sending invalid junk”
There are many protocol and implementation guards that make *other* classes of invalid data cheap to reject or disconnect-worthy. This blob-proof case stands out because it is **expensive per-item**, **easy to make well-formed**, and **not penalized as a protocol violation**.

Concrete examples of existing guards (contrast cases):
* **Oversized messages are rejected early:** `eth/protocols/eth/handler.go` enforces `maxMessageSize` (10 MiB) and disconnects on violation before higher-level decoding/processing.
* **Malformed / duplicate tx deliveries can disconnect:** `eth/protocols/eth/handlers.go` rejects nil txs and duplicate tx hashes as errors in both `handleTransactions` and `handlePooledTransactions` (handler error => disconnect).
* **Many responses are tied to an explicit request/response dispatcher:** `eth/protocols/eth/dispatcher.go` treats “dangling responses” (untracked request IDs) as protocol errors. However, `handlePooledTransactions` does not use the dispatcher; it calls `requestTracker.Fulfil` (metrics) and forwards unconditionally, so unsolicited `PooledTransactionsMsg` is not treated as a protocol violation.
* **Blob txs are intentionally not accepted as broadcasts:** `eth/handler_eth.go` rejects blob transactions delivered via `TransactionsMsg` (“broadcast”) as a handler error (disconnect). The intent is that blob txs should be **pulled** (requested) rather than **pushed**.
* **Some blob-sidecar invalidity is treated as disconnect-worthy:** `eth/handler_eth.go` disconnects for missing sidecars or commitment-hash mismatch (cheap checks), but it does **not** verify KZG proofs at this layer.
* **Fetcher has DoS protections for the announce→request path:** `eth/fetcher/tx_fetcher.go` includes per-peer announce caps (`maxTxAnnounces`) and retrieval sizing controls (`maxTxRetrievalSize`), but unsolicited `PooledTransactionsMsg` bypasses that “pull-based” flow and lands directly in txpool validation.

What makes this specific attack path strong:
* The attacker can craft blob txs that pass the cheap disconnect checks (sidecar present + commitment-hash matches) and then reliably force the expensive KZG verification to run and fail.
* The failure is counted as an “other reject” and only triggers a small post-hoc sleep under certain ratios; there is no “proof-invalid => disconnect/ban” policy.

#### Impact Analysis & Exploit Economics

**Attack Vector:**
An attacker can flood a victim node with unsolicited `PooledTransactionsMsg` containing invalid blob transactions.

**Resource Consumption Math (single-blob txs):**
*   **Bandwidth Limit:** The `eth` protocol enforces `maxMessageSize = 10 * 1024 * 1024` bytes (10 MiB). This cap is applied to `msg.Size` during `eth` message handling.
*   **Blob Tx Size (Wire Format):** A single blob transaction with a populated sidecar is ~131KB when RLP-encoded in “network encoding” (exact size varies by signature and list overhead). This was measured locally using `rlp.EncodeToBytes` in `measure_tx_size.go` / `bug-bounty/poc_packet_gen.go`.
*   **Payload Capacity:** A full 10 MiB message can typically fit on the order of **~70–80** one-blob transactions, depending on overhead.
*   **CPU Cost (Benchmark):** On this test machine, a single invalid-proof blob tx validation took ~`1.8ms` (see “Local Reproduction (No Network)”). This cost is paid even if the proof is invalid but well-formed.

**Mitigation factor (and how attackers work around it):**
*   `TxFetcher.Enqueue` processes incoming txs in batches of `addTxsBatchSize = 128`.
*   After processing a batch, if `otherreject > addTxsBatchSize/4` (i.e., `otherreject > 32`), it sleeps **200ms** (`eth/fetcher/tx_fetcher.go`).
*   This sleep happens **after** the expensive validation work has already been performed, so it does not prevent CPU burn; it only slows the attacker when they deliver large invalid batches.
*   An attacker can avoid triggering the sleep by keeping each delivered batch at **≤32 invalid-proof txs** (e.g., send messages containing ≤32 blob txs, or otherwise ensure the invalid-proof rate stays under the threshold).

**Amplification (illustrative):**
*   On this test machine, validating a single blob tx with an invalid proof took ~`1.8ms` (measured via the local harness; see “Local Reproduction (No Network)” below).
*   If an attacker sends **32** invalid one-blob txs per delivery (to avoid the sleep), the victim does roughly `32 * 1.8ms ≈ 58ms` of KZG verification CPU per delivery per peer.
*   With multiple peers, the work scales across cores because each peer can drive validation in parallel.

**Real-world Preconditions:**
*   **Inbound Connectivity:** The victim node must accept inbound P2P connections (default behavior).
*   **Peer Slots:** The attacker needs to occupy at least one peer slot. If the node is full (`--maxpeers`), the attacker must wait for a slot or eclipse an existing peer.
*   **Trusted Peers:** Nodes that only peer with trusted/static peers (`--trustedpeers`) are mitigated, but this is not the default configuration for public nodes.

**Severity:**
**High**. The attack requires no special permissions (just a P2P connection) and can be executed with standard hardware. It causes a Denial of Service (DoS) by exhausting CPU resources.

#### Local Reproduction (No Network)
This repository includes a local-only harness that demonstrates:
1) invalid blob proofs still trigger expensive KZG verification during txpool stateless validation, and
2) the fetcher path does not disconnect/ban on repeated invalid proofs.

1. Run a single validation and see the specific error + timing:
   ```bash
   go run ./bug-bounty/local_repro -debug
   ```
   Expected: `validate_err=invalid blob 0: ...` and `validate_took` on the order of milliseconds.
   Example output:
   ```
   validate_err=invalid blob 0: can't verify opening proof
   validate_took=1.831875ms
   ```

2. Run sustained load with multiple “peer” workers and capture a CPU profile:
   ```bash
   go run ./bug-bounty/local_repro -duration 15s -peers 8 -txs 32 -cpuprofile cpu.pprof
   go tool pprof -top cpu.pprof
   ```
   Expected: `pprof` output dominated by KZG/BLS12-381 math functions, demonstrating that invalid proofs still force expensive verification work.
   Example `pprof -top` excerpts:
   ```
   github.com/consensys/gnark-crypto/ecc/bls12-381/fp.mul
   github.com/consensys/gnark-crypto/ecc/bls12-381/fr.mul
   github.com/crate-crypto/go-eth-kzg/internal/domain.(*Domain).EvaluateLagrangePolynomialWithIndex
   ```

3. Observe that the harness reports `dropped_peers=0`, matching the code-level behavior that invalid-proof rejections do not lead to disconnect/ban in `TxFetcher.Enqueue`.


#### Proof of Concept (Packet Construction)
The following Go code demonstrates that a `BlobTx` with a sidecar encodes to the network format accepted by `PooledTransactionsMsg`. The `BlobTx.decode` method in `core/types/tx_blob.go` explicitly checks for this list-of-lists format and populates the `Sidecar` field, which subsequently triggers validation in the `BlobPool`.

```go
// See bug-bounty/poc_packet_gen.go for full source
// ...
    // 4. Create the Blob Transaction with Sidecar
    txData := &types.BlobTx{
        // ... fields ...
        Sidecar: &types.BlobTxSidecar{
            Blobs:       []kzg4844.Blob{blob},
            Commitments: []kzg4844.Commitment{commit},
            Proofs:      []kzg4844.Proof{proof},
        },
    }
    // ... sign ...

    // 6. Encode to RLP (Network Format)
    // The presence of tx.Sidecar ensures BlobTx.encode uses the network format (wrapper struct)
    encoded, err := rlp.EncodeToBytes(tx)
    // Result: 131,336 bytes
```

#### Proof of Concept (Attack Flow)
1.  **Connect:** Attacker connects to victim node using `eth/68`.
2.  **Send:** Attacker sends unsolicited `PooledTransactionsMsg` (0x0a) containing invalid blob transactions (valid format, well-formed but invalid proof). **No `GetPooledTransactionsMsg` is sent.** To maximize throughput, the attacker can keep deliveries to **≤32 invalid txs per message** (or otherwise keep `otherreject` ≤ 32 per `addTxsBatchSize` batch) to avoid the post-batch `200ms` sleep.
3.  **Process:** Victim's `handlePooledTransactions` forwards to `TxFetcher`.
4.  **Verify:** `TxFetcher` calls into txpool stateless validation, which performs KZG verification (`kzg4844.VerifyBlobProof`) for legacy blob sidecars.
5.  **Fail:** Verification fails. `TxFetcher` increments `otherreject`.
6.  **Repeat:** Attacker sends another batch immediately.
7.  **Result:** Victim CPU spikes. The peer is **not disconnected** for repeated invalid blob proofs. Any `200ms` sleep is applied only after the expensive validation work is already done, and can be worked around by keeping invalid deliveries under the threshold.

#### Recommendation
1. **Enforce Request-Response:** Modify `eth/handler_eth.go` or `TxFetcher` to reject `PooledTransactionsMsg` messages that do not correspond to an active request (unless explicitly announced and waiting) *before* calling `addTxs`.
2. **Drop Malicious Peers:** In `TxFetcher.Enqueue`, if `TxPool.Add` returns a validation error implying malicious intent (like invalid signature or invalid blob proof), immediately **disconnect** the peer.
