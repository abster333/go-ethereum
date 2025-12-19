# Tested / Reviewed Vectors (Notes)

This file captures secondary review notes gathered while investigating the primary finding(s). It is **not** a claim of comprehensive security coverage.

## CPU Exhaustion via Invalid Blob Transaction Spam (Primary Finding)

**Status:** Confirmed (High Severity)
**Component:** `eth/fetcher`, `core/txpool/blobpool`

**Vector:**
Unsolicited `PooledTransactionsMsg` (0x0a) containing invalid blob transactions (valid format, invalid KZG proof) are processed by the victim node.

**Observation:**
The `TxFetcher` enqueues these transactions into the `BlobPool`. The `BlobPool` performs synchronous KZG verification (`VerifyBlobProof`) in the peer's handler goroutine. This verification is expensive (~2.5ms).

**Impact:**
An attacker can saturate the victim's CPU by flooding these messages. The node does not disconnect the peer upon verification failure, only applying a minor sleep (200ms) which is insufficient to mitigate the attack when scaled across multiple connections.

**Reproduction:**
See `bug-bounty/poc_packet_gen.go` for payload generation.

## P2P UDP Discovery (v4)

**Status:** Not the primary focus of the blob-tx DoS report  
**Note:** The main report focuses on an exploit path in `eth` pooled transaction delivery. No additional high-confidence UDP discovery findings are included here.

## RPC API Exposure

**Status:** Not the primary focus of the blob-tx DoS report  
**Note:** RPC exposure is deployment-specific (flags, bind address, reverse proxies). No additional RPC-auth findings are included here.

## EVM Memory & Gas

**Status:** Not the primary focus of the blob-tx DoS report  
**Note:** No additional EVM/consensus correctness findings are included here.

