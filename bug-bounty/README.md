# Go-Ethereum Bug Bounty / Security Review Notes

This folder is a **security-review workspace** for `ethereum/go-ethereum` (Geth). It is designed to give a future reviewer (human or LLM) enough context to:

1. Understand what the Ethereum Foundation (EF) bug bounty considers **in-scope** / **out-of-scope** for Geth.
2. Prioritize review areas that have historically produced **exploitable** bugs (remote crash, resource exhaustion, consensus divergence, etc.).
3. Quickly find the right reporting channel for sensitive findings.

## Reporting (do not disclose publicly)

Follow `SECURITY.md` in the repository root.

- EF bug bounty portal: https://bounty.ethereum.org
- Email: `bounty@ethereum.org`
- Public disclosures (published advisories): https://github.com/ethereum/go-ethereum/security/advisories
- Known-vuln feed used by `geth version-check`: https://geth.ethereum.org/docs/vulnerabilities/vulnerabilities.json

## Docs index

- `01-PROJECT-OVERVIEW.md` — architecture overview and critical modules.
- `02-ATTACK-SURFACE.md` — externally reachable surfaces (P2P, RPC, GraphQL, etc.).
- `03-DEPENDENCIES.md` — third-party components worth auditing.
- `04-ETHEREUM-SECURITY-PRACTICES.md` — protocol/security context.
- `05-GO-SECURITY-BEST-PRACTICES.md` — Go-specific security pitfalls and checks.
- `06-HISTORICAL-ISSUES.md` — **verified** historical vulnerabilities and recurring exploit patterns for Geth.
- `07-ETHEREUM-FOUNDATION-BUG-BOUNTY.md` — EF bug bounty details relevant to Geth (scope, severity, exclusions).

## Quick triage: what “exploitable” usually means for Geth

Examples of impact classes that typically matter for the ecosystem:

- **Consensus divergence / chain split**: different nodes accept different blocks or compute different state roots.
- **Remote crash / forced shutdown**: a peer or request causes a panic or fatal error in default deployments.
- **Resource exhaustion (CPU / memory / disk / goroutines)**: remote parties can drive unbounded work or allocations.
- **Network isolation / eclipse-enablement**: practical paths to isolate nodes or bias their view of the network.
- **Key compromise / signing abuse**: issues affecting `clef`, keystore, signer APIs, or transaction authorization.

Where possible, tie findings to the EF bounty scope in `07-ETHEREUM-FOUNDATION-BUG-BOUNTY.md` and to historical patterns in `06-HISTORICAL-ISSUES.md`.
