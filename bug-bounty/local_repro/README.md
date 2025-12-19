## Local-only reproduction harness (no networking)

This folder contains a **local-only** CPU profiling harness that demonstrates:

1. Invalid blob sidecars still trigger expensive KZG verification during stateless tx validation.
2. The `TxFetcher.Enqueue` path does **not** disconnect/penalize a peer for “other rejects” (it only applies a small sleep when the reject ratio is high).

It intentionally **does not** implement any devp2p networking or target selection.

### Run

From the repo root:

```bash
# ~10s run, 1 "peer", 32 txs per enqueue (avoids the 200ms sleep)
go run ./bug-bounty/local_repro -duration 10s -peers 1 -txs 32

# Scale up "peers" to drive parallel validation (simulates multiple inbound peers)
go run ./bug-bounty/local_repro -duration 10s -peers 8 -txs 32
```

### Capture a CPU profile

```bash
go run ./bug-bounty/local_repro -duration 15s -peers 8 -txs 32 -cpuprofile cpu.pprof
go tool pprof -top cpu.pprof
```

In the `pprof` output you should see KZG verification functions consuming significant CPU (e.g. `VerifyBlobProof`).

