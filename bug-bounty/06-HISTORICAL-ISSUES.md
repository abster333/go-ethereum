# Historical Security Issues (Verified) & Vulnerability Patterns

This document focuses on **publicly disclosed, validated** vulnerabilities affecting `ethereum/go-ethereum`, and the recurring patterns they suggest for future reviews. It intentionally prefers *official advisories* and *reviewed vulnerability databases* over anecdotes.

## 1. Published advisories (Geth)

The canonical list of published advisories for this repository is:
https://github.com/ethereum/go-ethereum/security/advisories

The GitHub “Security” overview also provides a compact list:
https://github.com/ethereum/go-ethereum/security

### 1.1 Summary table (selected high-signal issues)

| Date (published) | ID(s) | Class | Remote vector | Impact | Fixed in |
| --- | --- | --- | --- | --- | --- |
| 2025-01-30 | GHSA-q26p-9cq4-7fc2 / CVE-2025-24883 | Input validation | P2P handshake | crash / forced shutdown | 1.14.13 |
| 2024-05-06 | GHSA-4xc9-8hmq-j652 | Integer underflow / resource exhaustion | `eth` protocol | memory DoS | 1.13.15 |
| 2023-09-06 | GHSA-ppjg-v974-84cm | Goroutine leak | `p2p` ping | resource exhaustion | 1.13.5 |
| 2022-05-11 | GHSA-wjxw-gh3m-7pm5 / CVE-2022-29177 | Resource exhaustion | P2P (with verbose logging) | crash | 1.10.17 |
| 2021-10-25 | GHSA-59hh-656j-3p7v / CVE-2021-41173 / GO-2022-0256 | Nil handling / panic | `snap/1` protocol | crash | 1.10.9 |
| 2021-08-24 | GHSA-9856-9gg9-qcmq | Memory overlap semantics | EVM (opcode semantics) | consensus divergence | (see advisory) |
| 2020-12-11 | GHSA-xw37-57qp-9mm4 | Consensus flaw | block processing | consensus divergence | (see advisory) |
| 2020-12-11 | GHSA-r33q-22hv-j29q | Resource exhaustion | LES server | DoS | (see advisory) |
| 2020-11-24 | GHSA-jm5c-rv3w-w83m | Underpriced computation | EVM | DoS | (see advisory) |
| 2020-11-24 | GHSA-m6gx-rhvj-fh52 / CVE-2020-28362 | Upstream (Go runtime) | (varies) | DoS | Go update |

Notes:
- “Fixed in” is the **first patched Geth release** when explicitly stated in the advisory or a reviewed database.
- Some older advisories reference a fix PR/commit rather than a tagged release; prefer the advisory details for exact versioning.

## 2. What these bugs imply for future reviews (patterns)

### 2.1 P2P message handling: untrusted sizes, counts, and nils

Recurring theme: a remote peer can trigger **panics** or **unbounded allocations** by sending structurally valid protocol messages containing edge-case values (e.g., counts of `0`, missing children, unexpected `nil` returns).

Review targets:
- Any handler that turns peer-provided sizes/counts into allocations (`make`, `new`, `append` growth) or loops.
- “Convenience” arithmetic that can underflow/overflow (`count-1`, `n+count-1`) before bounds checks.
- Code paths that treat `(nil, nil)` as impossible or use interfaces without guarding against `nil` concrete values.

### 2.2 Lifecycle leaks: goroutines, timers, and subscriptions

“Slow burn” DoS can be as damaging as a crash if it steadily increases memory or goroutine count.

Review targets:
- `p2p` protocol loops and keepalive logic (ensure close/cancel propagates everywhere).
- Subscription-style APIs (RPC/WebSocket) that maintain per-client state.
- Cleanup paths on handshake failure or protocol mismatch.

### 2.3 Consensus-critical correctness

Two high-impact categories repeatedly show up in Ethereum clients:
- **Consensus divergence** (same block processed differently across nodes).
- **DoS via underpriced operations** (protocol-level “cheap” actions that are expensive to execute).

Review targets:
- EVM opcode implementations and gas accounting (especially around hard forks and special cases).
- Block validation / execution invariants that must be deterministic across platforms and Go versions.
- “Fast paths” that assume non-overlapping buffers or strict invariants from upstream callers.

## 3. Additional publicly disclosed issues (outside the repo’s advisory list)

Not every historical bug is represented as a repository security advisory. Examples include CVE records for RPC/debug features, which may become critical when operators expose them.

- CVE-2018-16733 (fixed in 1.8.14): improper validation in tracing APIs (see OSV entry for details).
- CVE-2022-23327 (GHSA-pvx3-gm3c-gmpr): transaction stack exhaustion (DoS).
- CVE-2023-42319 (GHSA-v9jh-j8px-98vq): GraphQL query denial of service (relevant if GraphQL is enabled/exposed).

## 4. References

- Repo advisories: https://github.com/ethereum/go-ethereum/security/advisories
- Repo security overview: https://github.com/ethereum/go-ethereum/security
- GHSA-q26p-9cq4-7fc2 (CVE-2025-24883): https://github.com/advisories/GHSA-q26p-9cq4-7fc2
- GHSA-4xc9-8hmq-j652: https://github.com/ethereum/go-ethereum/security/advisories/GHSA-4xc9-8hmq-j652
- CVE-2022-29177 (GHSA-wjxw-gh3m-7pm5): https://advisories.gitlab.com/pkg/golang/github.com/ethereum/go-ethereum/CVE-2022-29177/
- GO-2022-0256 (CVE-2021-41173 / GHSA-59hh-656j-3p7v): https://pkg.go.dev/vuln/GO-2022-0256
- GHSA-ppjg-v974-84cm: https://github.com/ethereum/go-ethereum/security/advisories/GHSA-ppjg-v974-84cm
- CVE-2018-16733: https://osv.dev/vulnerability/GHSA-qr2j-wrhx-4829
- CVE-2022-23327 (GHSA-pvx3-gm3c-gmpr): https://github.com/advisories/GHSA-pvx3-gm3c-gmpr
- CVE-2023-42319 (GHSA-v9jh-j8px-98vq): https://github.com/advisories/GHSA-v9jh-j8px-98vq
