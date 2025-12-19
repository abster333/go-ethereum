# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Build Commands

**Build geth (main client):**
```bash
make geth
# Binary output: ./build/bin/geth
```

**Build all executables:**
```bash
make all
```

**Build specific tool:**
```bash
make evm  # Build the EVM developer utility
# Other tools are in cmd/ directory
```

**Run tests:**
```bash
make test  # Runs all tests via build/ci.go
```

**Run specific package tests:**
```bash
go test ./core/...           # Test core package and subpackages
go test -run TestFuncName ./path/to/package  # Run specific test
go test -v ./eth             # Verbose output for eth package
```

**Linting:**
```bash
make lint  # Run pre-selected linters
```

**Code formatting:**
```bash
make fmt   # Format all Go files with gofmt
```

**Install developer tools:**
```bash
make devtools  # Install stringer, gencodec, protoc-gen-go, abigen
```

## Repository Structure

**Main executables** (`cmd/`):
- `geth`: Main Ethereum client
- `clef`: External signer for account management
- `devp2p`: P2P networking utilities
- `abigen`: Contract ABI to Go binding generator
- `evm`: EVM bytecode runner for debugging
- `rlpdump`: RLP encoding/decoding utility

**Core packages:**
- `core/`: Blockchain, state management, EVM execution, transaction pool
- `eth/`: Ethereum protocol implementation, sync, mining
- `p2p/`: Peer-to-peer networking layer
- `rpc/`: JSON-RPC server implementation
- `node/`: Service lifecycle and coordination
- `consensus/`: Consensus engine interfaces (ethash, clique)
- `accounts/`: Account management and signing
- `params/`: Network parameters and fork configuration

## Architecture Overview

### Layered Architecture

```
Application Layer (cmd/*)
    ↓
Node Stack (node/) - Service coordination, RPC setup
    ↓
Protocol Layer (eth/) - Sync, mining, protocol handling
    ↓
Core Layer (core/) - BlockChain, StateDB, EVM, TxPool
    ↓
Storage Layer (ethdb/, trie/, triedb/) - Merkle tries, key-value storage
```

### Key Components

**BlockChain** (`core/blockchain.go`):
- Manages canonical chain and handles block insertion
- Validates blocks through consensus engine
- Performs chain reorganizations when needed
- Thread-safe for concurrent reads, locked for writes

**StateDB** (`core/state/statedb.go`):
- In-memory account and contract state
- Manages nonces, balances, code, and storage
- Snapshots for transaction atomicity
- NOT thread-safe - single-use per state transition

**TxPool** (`core/txpool/`):
- Coordinates multiple transaction subpools (legacy, blob)
- Validates and orders transactions by gas price
- Broadcasts transactions to peers
- Thread-safe with internal locking

**EVM** (`core/vm/`):
- Executes smart contract bytecode
- Manages gas accounting and metering
- Handles precompiled contracts
- Isolated execution environment per transaction

**P2P Networking** (`p2p/`):
- Manages peer discovery and connections
- Protocol capability negotiation
- Multiple protocol versions (eth/66, eth/67, eth/68, snap)

### Data Flow: Block Import

1. P2P layer receives block from peer (`p2p/`)
2. Protocol handler validates and schedules (`eth/handler.go`)
3. Downloader/fetcher retrieves full block data (`eth/downloader/`, `eth/fetcher/`)
4. BlockChain.InsertChain validates and executes:
   - Validates block header and body
   - Creates isolated StateDB
   - Executes transactions through EVM
   - Commits state to Merkle trie
   - Updates canonical chain or side chain
5. Broadcasts to peers if canonical

### Data Flow: Transaction Submission

1. Transaction received via RPC or P2P
2. TxPool validates (signature, nonce, balance, gas)
3. Added to pending (executable) or queued (future nonce)
4. Broadcast to interested peers
5. Included in block by miner
6. Removed from pool upon block import

## Important Patterns

### Dependency Injection via Interfaces

The codebase uses interfaces extensively to decouple components:
- `Backend` interface (`internal/ethapi/`) abstracts protocol implementation
- `Engine` interface (`consensus/`) supports multiple consensus algorithms
- `Database` interface (`ethdb/`) abstracts storage backend

### Event-Driven Communication

- **event.Feed**: Type-safe event publishing/subscription
- **event.TypeMux**: Type-based event multiplexing (legacy)
- Key events: `ChainHeadEvent`, `NewTxsEvent`, `ChainEvent`

### Service Lifecycle

All services implement a lifecycle pattern managed by `node/`:
```go
type Lifecycle interface {
    Start() error
    Stop() error
}
```

### State Snapshots

State modifications are journaled and can be reverted:
```go
snapshot := state.Snapshot()
// ... modifications ...
state.RevertToSnapshot(snapshot)  // Undo changes
```

## Development Conventions

### Code Formatting
- Use `gofmt` for all code (enforced by `make fmt`)
- Follow official Go formatting guidelines

### Commit Messages
- Prefix with package name(s) being modified
- Example: `eth, rpc: make trace configs optional`

### Thread Safety
- **StateDB**: NOT thread-safe (single-use per block execution)
- **BlockChain**: Concurrent reads OK, writes protected by mutex
- **TxPool**: Fully thread-safe
- **Trie**: NOT thread-safe (create new instance per state transition)

### Testing
- Table-driven tests preferred for multiple test cases
- Use `testdata/` directories for test fixtures
- Mock implementations via interfaces
- Simulated backend for contract testing

## Key Database Abstractions

### Ancient Store (Freezer)
- Immutable historical data stored separately
- Blocks, receipts, headers older than recent state
- Reduces main database size

### Trie Database
- **PathDB**: Path-based trie (newer, recommended)
- **HashDB**: Hash-based trie (legacy)
- Configurable via `--db.engine` flag

### Snapshots
- Flat state representation for fast sync
- Enables snap sync protocol
- Improves state access performance

## Configuration

### Chain Parameters (`params/`)
- `config.go`: Chain configuration and fork rules
- `protocol_params.go`: Gas costs and limits
- `bootnodes.go`: Network bootstrap nodes

### Genesis Files
- Define initial chain state
- Network ID, consensus engine, pre-funded accounts
- Located in `core/genesis.go` and `params/`

## Common Modification Points

1. **Add RPC endpoint**: Register new API service in `node/` or `eth/api.go`
2. **Custom consensus**: Implement `consensus.Engine` interface
3. **Transaction type**: Extend `core/types/transaction.go`
4. **Protocol change**: Modify handlers in `eth/handler.go`
5. **State storage**: Implement `ethdb.Database` interface

## Performance Considerations

- **Trie prefetching**: Predictively loads nodes during execution
- **State snapshots**: Fast flat-file lookups avoid trie traversal
- **LRU caching**: Extensive caching at blockchain, state, and trie layers
- **Batch writes**: Database writes batched for efficiency
- **Parallel validation**: Block validation parallelized where possible

## Go Version

Requires Go 1.24.0 or later (see `go.mod`).
