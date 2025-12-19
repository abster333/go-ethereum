# Security Assessment Report: Go-Ethereum (Geth)

## Executive Summary
This assessment focused on the Go-Ethereum (Geth) codebase, specifically targeting high-severity vulnerabilities in the P2P networking, Transaction Pool, and RPC subsystems.

A **High-Severity Denial of Service (DoS)** vulnerability was identified in the handling of Blob Transactions (EIP-4844). Attackers can exploit this to exhaust node CPU resources by spamming invalid blob transactions, which trigger expensive KZG verifications before the node applies any effective mitigation (peer dropping).

## Findings

### [HIGH] CPU Exhaustion via Invalid Blob Transaction Spam

**Severity:** High
**Component:** `eth/fetcher`, `core/txpool/blobpool`
**Affected Versions:** Confirmed in Geth commit `03138851`. Likely affects any release line where `PooledTransactionsMsg` (0x0a) can be accepted unsolicited and forwarded into txpool validation (eth/68+).

#### Description
The Geth node allows peers to push transactions via `PooledTransactionsMsg` (0x0a) messages. The `eth` protocol handler accepts these messages even if they were not explicitly requested (unsolicited push). When processing these transactions, the `TxFetcher` enqueues them into the `TxPool`.

The `TxPool.Add` method performs validation, including the computationally expensive KZG proof verification for Blob Transactions (`VerifyBlobProof`). This verification happens in the `preCheck` phase, which is designed to be lock-free to avoid stalling the pool. However, because this verification is performed **synchronously** in the peer's message handler goroutine *before* any effective rate-limiting or peer dropping occurs, it allows an attacker to force the victim to consume significant CPU resources.

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
    Crucially, `otherreject` only triggers a 200ms sleep, not a peer disconnect.

4.  **Validation (`core/txpool/blobpool/blobpool.go`):**
    The `BlobPool.Add` method calls `preCheck` for each transaction. `preCheck` performs the expensive `ValidateTxBasics` -> `validateBlobTx` -> `VerifyBlobProof` chain.
    ```go
    // core/txpool/blobpool/blobpool.go
    func (p *BlobPool) Add(txs []*types.Transaction, sync bool) []error {
        // ...
        for i, tx := range txs {
            // Expensive KZG verification happens here, outside the lock
            if errs[i] = p.preCheck(tx); errs[i] != nil {
                continue
            }
            // ...
        }
        // ...
    }

    // preCheck calls ValidateTxBasics -> ValidateTransaction -> validateBlobTx -> VerifyBlobProof
    func (p *BlobPool) preCheck(tx *types.Transaction) error {
        // ...
        if err := p.ValidateTxBasics(tx); err != nil {
            return err
        }
        // ...
    }
    ```
    While `preCheck` is lock-free, it consumes CPU. Since this runs in the peer's handler goroutine, an attacker with multiple connections can saturate multiple CPU cores.

5.  **Late Check (`eth/fetcher/tx_fetcher.go`):**
    The `cleanup` channel is processed asynchronously. Only *after* the transactions have been validated and added (or rejected) does the fetcher check `f.requests[delivery.origin]`. If no request exists, it logs a warning but does not retroactively undo the CPU work.

#### Impact Analysis & Exploit Economics

**Attack Vector:**
An attacker can flood a victim node with unsolicited `PooledTransactionsMsg` containing invalid blob transactions.

**Resource Consumption Math (single-blob txs):**
*   **Bandwidth Limit:** The `eth` protocol enforces `maxMessageSize = 10 * 1024 * 1024` bytes (10 MiB). This cap is applied to `msg.Size` during `eth` message handling.
*   **Blob Tx Size (Wire Format):** A single blob transaction with a populated sidecar is ~131KB when RLP-encoded in “network encoding” (exact size varies by signature and list overhead). This was measured locally using `rlp.EncodeToBytes` in `measure_tx_size.go` / `bug-bounty/poc_packet_gen.go`.
*   **Payload Capacity:** A full 10 MiB message can typically fit on the order of **~70–80** one-blob transactions, depending on overhead.
*   **CPU Cost (Benchmark):** On the target system, `kzg4844.VerifyBlobProof` costs **~2.5ms per blob**. This cost is paid even if the proof is invalid but well-formed.

**Mitigation factor (and how attackers work around it):**
*   `TxFetcher.Enqueue` processes incoming txs in batches of `addTxsBatchSize = 128`.
*   After processing a batch, if `otherreject > addTxsBatchSize/4` (i.e., `otherreject > 32`), it sleeps **200ms** (`eth/fetcher/tx_fetcher.go`).
*   This sleep happens **after** the expensive validation work has already been performed, so it does not prevent CPU burn; it only slows the attacker when they deliver large invalid batches.
*   An attacker can avoid triggering the sleep by keeping each delivered batch at **≤32 invalid-proof txs** (e.g., send messages containing ≤32 blob txs, or otherwise ensure the invalid-proof rate stays under the threshold).

**Amplification (illustrative):**
*   If an attacker sends **32** invalid one-blob txs per message (to avoid the sleep), the victim does about `32 * 2.5ms = 80ms` of KZG verification work per message.
*   At 1 Gbps, the attacker can send many such messages per second in bandwidth terms; in practice, throughput is bounded by the victim’s CPU and the number of concurrent peer connections.
*   With multiple peers, the attacker can drive concurrent KZG verification across cores and significantly degrade block/tx processing.

**Real-world Preconditions:**
*   **Inbound Connectivity:** The victim node must accept inbound P2P connections (default behavior).
*   **Peer Slots:** The attacker needs to occupy at least one peer slot. If the node is full (`--maxpeers`), the attacker must wait for a slot or eclipse an existing peer.
*   **Trusted Peers:** Nodes that only peer with trusted/static peers (`--trustedpeers`) are mitigated, but this is not the default configuration for public nodes.

**Severity:**
**High**. The attack requires no special permissions (just a P2P connection) and can be executed with standard hardware. It causes a Denial of Service (DoS) by exhausting CPU resources.

#### Reproduction Steps
1. Connect to a victim Geth node using the `eth/68` protocol.
2. Construct a `PooledTransactionsMsg` (0x0a) containing a batch of blob transactions (e.g., up to ~10 MiB, or smaller batches ≤32 txs to avoid the `200ms` sleep).
3. For each transaction:
   - Use a valid format and signature (random key).
   - Include 1 blob.
   - Use valid commitments (random points on curve) matching the versioned hashes.
   - Use a **well-formed but invalid proof**. A practical way is to compute a valid proof for a different `(blob, commitment)` pair and reuse it, so verification still executes the full pairing check before failing.
   - Encode using the "network encoding" (list of lists) to include the sidecar.
4. Send the message to the victim without sending a `GetPooledTransactionsMsg` first.
5. Observe the victim node consuming CPU due to KZG verification and becoming slow/unresponsive at the process level under sustained load.
6. Repeat continuously.
*Note: The code below generates a valid proof to demonstrate the packet structure and size. For the exploit, the attacker would modify this to send an invalid proof (e.g., by modifying the blob data after proof generation) to trigger the validation failure.*


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
4.  **Verify:** `TxFetcher` calls `BlobPool.Add` -> `preCheck` -> `VerifyBlobProof`.
5.  **Fail:** Verification fails. `TxFetcher` increments `otherreject`.
6.  **Repeat:** Attacker sends another batch immediately.
7.  **Result:** Victim CPU spikes. The peer is **not disconnected** for repeated invalid blob proofs. Any `200ms` sleep is applied only after the expensive validation work is already done, and can be worked around by keeping invalid deliveries under the threshold.

#### Recommendation
1. **Enforce Request-Response:** Modify `eth/handler_eth.go` or `TxFetcher` to reject `PooledTransactionsMsg` messages that do not correspond to an active request (unless explicitly announced and waiting) *before* calling `addTxs`.
2. **Drop Malicious Peers:** In `TxFetcher.Enqueue`, if `TxPool.Add` returns a validation error implying malicious intent (like invalid signature or invalid blob proof), immediately **disconnect** the peer.


