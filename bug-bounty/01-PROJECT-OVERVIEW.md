# Go-Ethereum (Geth) Project Overview

## Project Information

**Repository:** ethereum/go-ethereum  
**Project Type:** Ethereum Execution Layer Client  
**Language:** Go (version 1.24.0 required)  
**License:** LGPL-3.0, GPL-3.0  
**Official Site:** https://geth.ethereum.org  

## Project Description

Go-Ethereum (Geth) is the official Golang implementation of the Ethereum protocol. It is one of the most widely used Ethereum clients and is responsible for executing transactions, managing the blockchain state, and participating in the Ethereum network consensus.

### Core Responsibilities
- **Execution Layer Client**: Processes and validates Ethereum transactions
- **State Management**: Maintains the current state of the Ethereum network
- **P2P Networking**: Communicates with other Ethereum nodes
- **JSON-RPC API**: Provides interfaces for dApps and external applications
- **Mining/Validation**: Participates in block production and validation
- **EVM Execution**: Runs smart contract code

## Main Executables

### 1. **geth** (Main Client)
The primary Ethereum client that:
- Runs full nodes, archive nodes, or light nodes
- Syncs with the Ethereum network (mainnet, testnets, or private networks)
- Provides JSON-RPC endpoints (HTTP, WebSocket, IPC)
- Includes built-in JavaScript console
- Supports multiple sync modes: snap sync (default), full sync, light sync

**Entry Point:** [cmd/geth/main.go](cmd/geth/main.go)

### 2. **clef** (Signing Tool)
Stand-alone signing tool that:
- Acts as external signer for geth
- Provides enhanced security for key management
- Supports rule-based transaction signing
- Can be used as a backend signer

**Entry Point:** [cmd/clef/main.go](cmd/clef/main.go)

### 3. **devp2p** (P2P Utilities)
Tools for interacting with Ethereum's peer-to-peer network layer:
- Node discovery testing
- Protocol debugging
- Network diagnostics

**Entry Point:** [cmd/devp2p/main.go](cmd/devp2p/main.go)

### 4. **abigen** (Contract Bindings Generator)
Generates Go bindings for Ethereum smart contracts:
- Converts ABI to type-safe Go code
- Enables native Go interaction with smart contracts
- Supports Solidity source compilation

**Entry Point:** [cmd/abigen/main.go](cmd/abigen/main.go)

### 5. **evm** (EVM Testing Tool)
Developer utility for EVM testing:
- Runs bytecode in isolated environment
- Configurable execution context
- Useful for opcode-level debugging

**Entry Point:** [cmd/evm/main.go](cmd/evm/main.go)

### 6. **rlpdump** (RLP Decoder)
Utility for decoding RLP-encoded data:
- Converts binary RLP to readable format
- Useful for debugging protocol messages

**Entry Point:** [cmd/rlpdump/main.go](cmd/rlpdump/main.go)

### 7. **blsync** (Beacon Sync)
Light client implementation using beacon chain:
- Syncs execution layer using beacon chain
- Lightweight alternative to full sync

**Entry Point:** [cmd/blsync/main.go](cmd/blsync/main.go)

## Critical Code Modules

### Core Blockchain Components
- **[core/](core/)** - Blockchain core logic
  - `blockchain.go` - Main blockchain management
  - `state_processor.go` - Transaction execution
  - `state_transition.go` - State transitions and validations
  - `block_validator.go` - Block validation logic
  - `genesis.go` - Genesis block handling

### State Management
- **[core/state/](core/state/)** - State database and management
- **[trie/](trie/)** - Merkle Patricia Trie implementation
- **[triedb/](triedb/)** - Trie database backends

### Consensus
- **[consensus/](consensus/)** - Consensus engine interfaces
  - `beacon/` - Proof-of-Stake (Beacon chain) consensus
  - `ethash/` - Legacy Proof-of-Work consensus
  - `clique/` - Proof-of-Authority for private networks

### Transaction Pool
- **[core/txpool/](core/txpool/)** - Transaction pool management
  - Transaction validation
  - Fee market logic
  - Mempool management

### P2P Networking
- **[p2p/](p2p/)** - Peer-to-peer networking layer
- **[eth/](eth/)** - Ethereum protocol implementation
- **[ethclient/](ethclient/)** - Ethereum client library

### Cryptography
- **[crypto/](crypto/)** - Cryptographic primitives
  - ECDSA signing and verification
  - Keccak256 hashing
  - BLS12-381 (for beacon chain)

### APIs
- **[rpc/](rpc/)** - JSON-RPC server implementation
- **[graphql/](graphql/)** - GraphQL API
- **[internal/ethapi/](internal/)** - Ethereum API implementations

### Accounts & Wallets
- **[accounts/](accounts/)** - Account management
  - `keystore/` - Encrypted key storage
  - `usbwallet/` - Hardware wallet support
  - `abi/` - Contract ABI encoding/decoding

### Storage
- **[ethdb/](ethdb/)** - Database interfaces
- **[core/rawdb/](core/rawdb/)** - Low-level database operations

## Hardware Requirements

### Minimum
- CPU: 4+ cores
- RAM: 8GB
- Storage: 1TB free space (for mainnet sync)
- Network: 8 Mbit/sec download

### Recommended
- CPU: Fast CPU with 8+ cores
- RAM: 16GB+
- Storage: High-performance SSD with 1TB+ free space
- Network: 25+ Mbit/sec download

## Build Information

**Build Tool:** Makefile  
**Primary Build Command:** `make geth`  
**Full Suite:** `make all`

### Key Dependencies (from go.mod)
- Go 1.24.0+
- Consensus crypto: `github.com/consensys/gnark-crypto`
- Database: `github.com/cockroachdb/pebble`, `github.com/syndtr/goleveldb`
- Cryptography: `golang.org/x/crypto`, `github.com/ethereum/c-kzg-4844`
- Networking: `github.com/gorilla/websocket`, `github.com/huin/goupnp`
- Cloud storage: AWS SDK, Azure SDK, Cloudflare

## Security Features

### Built-in Security Tools
1. **Version Check:** `geth version-check` - Checks for known vulnerabilities
2. **Vulnerability Database:** Fetches from https://geth.ethereum.org/docs/vulnerabilities/vulnerabilities.json

### Audit History
- 2017-04-25: Geth audit by Truesec
- 2018-09-14: Clef audit by NCC Group
- 2019-10-15: Discv5 audit by Least Authority
- 2020-01-24: DiscV5 audit by Cure53

Audit reports available in: [docs/audits/](docs/audits/)

## Bug Bounty Program

**Contact:** bounty@ethereum.org  
**Website:** https://bounty.ethereum.org  
**Public Disclosures:** https://github.com/ethereum/go-ethereum/security/advisories

### Reporting Guidelines
- **DO NOT** file public tickets for security issues
- Use the official disclosure process
- PGP key available for encrypted communication
- Responsible disclosure window expected

## Network Modes

1. **Mainnet** - Production Ethereum network
2. **Testnets** - Testing networks (e.g., Holesky, Sepolia)
3. **Private Networks** - Custom networks for development
4. **Dev Mode** - Local development with instant mining

## Sync Modes

1. **Snap Sync** (default) - Fast sync with state snapshots
2. **Full Sync** - Downloads and validates all blocks
3. **Light Sync** - Minimal data sync (requires light server)
4. **Archive Mode** - Stores complete historical state

## Recent Security Incidents

### 2021-08-27: Minority Chain Split
- **CVE:** Memory corruption in EVM during datacopy precompile
- **Impact:** Consensus failure leading to chain split
- **Affected:** Geth nodes that hadn't upgraded to v1.10.8
- **Root Cause:** RETURNDATA buffer corruption via overlapping memory operations
- **Response:** Emergency patch, public disclosure
- **Postmortem:** [docs/postmortems/2021-08-22-split-postmortem.md](docs/postmortems/2021-08-22-split-postmortem.md)

## Key Contact Points

- **Security:** bounty@ethereum.org, security@ethereum.org
- **Discord:** https://discord.gg/nthXNEv
- **Twitter:** @go_ethereum
- **Documentation:** https://geth.ethereum.org/docs
