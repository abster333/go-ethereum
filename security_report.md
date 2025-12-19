# Security Assessment Report

## 1. CPU Exhaustion via Invalid Blob Transaction Spam

**Severity:** High
**Component:** `eth/fetcher`, `core/txpool/blobpool`
**File:** `eth/fetcher/tx_fetcher.go`, `core/txpool/blobpool/blobpool.go`

### Description
The `TxFetcher` component handles incoming transaction announcements and deliveries. When receiving a batch of transactions, it enqueues them into the transaction pool. For EIP-4844 blob transactions, the `BlobPool` performs synchronous KZG proof verification during the `Add` operation. This verification is computationally expensive (~2-3ms per blob).

The `TxFetcher` implements a penalty mechanism where it sleeps for 200ms if more than 25% of the transactions in a batch are rejected with "other" errors (which includes invalid KZG proofs). However, the batch size is 128 (`addTxsBatchSize`). If an attacker sends a batch of 32 or fewer invalid blob transactions, the `otherreject` count (32) will not exceed `addTxsBatchSize/4` (32), and the sleep is bypassed.

### Exploit Scenario
1. Attacker connects to the victim node.
2. Attacker sends `PooledTransactionsMsg` containing 32 invalid blob transactions (valid format, invalid KZG proof).
3. The victim node verifies all 32 proofs, consuming ~64-100ms of CPU time.
4. The `TxFetcher` checks the reject count: 32 is not greater than 32, so it does not sleep.
5. The attacker immediately sends another batch.
6. By repeating this, the attacker can saturate a CPU core with a single connection. With multiple connections, the attacker can exhaust all CPU resources, causing a Denial of Service.

### Recommendation
- Decrease the threshold for the sleep penalty, or make it proportional to the validation cost.
- Implement a stricter ban mechanism for peers sending invalid blob proofs (e.g., disconnect after a few invalid blobs).
- Perform KZG verification asynchronously or with a global rate limiter.

## 2. Pre-allocation DoS in RLPx Snappy Decompression

**Severity:** Medium
**Component:** `p2p/rlpx`
**File:** `p2p/rlpx/rlpx.go`

### Description
The RLPx protocol uses Snappy compression for messages. The `Conn.Read` method reads the uncompressed length from the Snappy frame header and allocates a buffer of that size before decompressing the data.

```go
// p2p/rlpx/rlpx.go
actualSize, err = snappy.DecodedLen(data)
// ...
c.snappyReadBuffer = growslice(c.snappyReadBuffer, actualSize)
data, err = snappy.Decode(c.snappyReadBuffer, data)
```

The `actualSize` can be up to `maxUint24` (16MB). An attacker can send a small message (e.g., a few bytes) that claims to have a decompressed size of 16MB. The node will allocate 16MB for `snappyReadBuffer`. Since this buffer is reused and associated with the connection, the memory remains allocated.

### Exploit Scenario
1. Attacker establishes multiple connections to the victim node (up to `MaxPeers`).
2. On each connection, the attacker sends a malformed Snappy message with a header claiming 16MB decompressed size.
3. The victim node allocates 16MB per connection.
4. If the node allows 100 peers, this consumes 1.6GB of memory. If the attacker can bypass peer limits or if the node has high limits, this can lead to Out-of-Memory (OOM) crash.

### Recommendation
- Limit the pre-allocation size to a smaller value (e.g., 1MB) and grow the buffer as needed during decompression (if the library supports it).
- Or, enforce a minimum compression ratio check (e.g., compressed size must be at least 1/20 of decompressed size) before allocating the full buffer.
