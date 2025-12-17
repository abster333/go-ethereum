# Ethereum Foundation Bug Bounty (Geth-relevant notes)

This document summarizes the Ethereum Foundation (EF) bug bounty information that is most relevant when reporting vulnerabilities in **Go-Ethereum (Geth)**.

Primary references:
- EF bug bounty page: https://ethereum.org/en/bug-bounty/
- EF bug bounty portal: https://bounty.ethereum.org
- Geth `SECURITY.md`: `SECURITY.md` (repo root)

## 1. Scope: where Geth fits

EF’s bug bounty scope covers multiple targets and is run via a public bug bounty program (managed through a third-party platform).

For clients, EF’s public scope list includes **Execution Layer clients**, including:
- Besu
- Erigon
- **Geth (go-ethereum)**
- Nethermind
- Reth

Implication for reviews:
- Bugs in **consensus-critical execution** and **network protocol handling** are typically eligible if they can realistically impact mainnet/testnet operation.
- Bugs limited to optional / non-default features may still be valid, but scope and severity will depend on how commonly they are enabled.

## 2. What tends to be “high impact” for clients

From the EF bug bounty severity guidance, the following impact categories are the most relevant to Geth:

- **Critical:** consensus failures (chain splits), large-scale theft, or other outcomes that could compromise Ethereum at scale.
- **High:** substantial remote DoS against nodes/clients, significant integrity or availability loss, or other serious client-level compromises.
- **Medium/Low:** narrower-scope DoS or correctness issues, usually requiring additional constraints (configuration, partial exposure, limited blast radius).

Treat these as *reporting heuristics*, not strict rules: EF will still apply judgement based on exploitability and ecosystem impact.

EF also advertises bounties up to **$2,000,000** (final amounts depend on severity and triage).

## 3. Common exclusions that matter during triage

The EF page explicitly calls out categories that are commonly **out of scope** (or reduced in severity). These matter when deciding whether a finding is bounty-eligible vs. still worth fixing:

- **Issues that require an operator to expose privileged APIs to the public internet** (e.g., unsafe RPC/admin exposure).
- **High-effort, sustained DoS** that is not practically exploitable at scale.
- **Purely theoretical / non-exploitable issues**, missing best practices, or “should be more hardened” reports without a clear impact path.
- **Known issues** or reports without a demonstrable security impact.

For internal security reviews, “out of scope” does not mean “unimportant”; it often means “not eligible for payout” or “not considered a security bug under bounty rules”.

## 4. Reporting channels (Geth)

Follow the repository’s `SECURITY.md` policy:

- Do **not** open a public GitHub issue for a security vulnerability.
- Use the EF bug bounty process (`https://bounty.ethereum.org`) or email `bounty@ethereum.org`.
- Published disclosures (after coordinated release) are tracked at: https://github.com/ethereum/go-ethereum/security/advisories

## 5. Practical guidance for writing a good report (client bugs)

To maximize triage quality, a report should usually include:

- Clear **impact statement** (what breaks, and why it matters for Ethereum).
- **Attack preconditions** (protocols enabled, ports exposed, config assumptions, attacker capabilities).
- A minimal **reproduction** (ideally a small PoC or message trace) that demonstrates the behavior without relying on undefined behavior.
- **Affected versions** and the earliest known good/bad release (if possible).
- Suggested fix direction (bounds checks, accounting, early reject, protocol ban rules, etc.).
