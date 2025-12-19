# Vuln 1: Pre-allocation DoS in RLPx Snappy Decompression in `p2p/rlpx/rlpx.go`

* **Severity:** Medium
* **Category:** oom_dos
* **Description:** The RLPx protocol's `Conn.Read` method reads the uncompressed length from the Snappy frame preamble and immediately allocates a buffer of that size (up to 16MB) via `growslice` before decompressing. This allocation occurs regardless of the actual size of the compressed data received.
* **Exploit Scenario:** An attacker establishes multiple connections to the victim node. On each connection, they send a Snappy frame with a preamble claiming a decoded size of 16MB (the maximum allowed), but with a very small compressed payload (e.g., a highly compressible sequence or just enough bytes to pass the length check). The node allocates 16MB per connection. Since the buffer is reused for the connection's lifetime, the attacker can keep these connections open (by sending valid highly-compressed frames), forcing the node to hold the allocated memory. With 100 connections, this consumes ~1.6GB of RAM, potentially leading to an OOM crash.
* **Recommendation:** 
    1. Enforce a maximum compression ratio check before allocation (e.g., `decodedLen < compressedLen * MAX_RATIO`).
    2. Use a streaming decompressor or allocate the buffer in smaller chunks as data is decoded.
    3. Cap the initial allocation size and grow the buffer only if decompression actually produces more data.
