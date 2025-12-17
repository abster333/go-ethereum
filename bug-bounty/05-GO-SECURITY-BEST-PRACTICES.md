# Go Security Best Practices for Ethereum Clients

## Overview
This document outlines security best practices specific to the Go programming language, tailored for the Go-Ethereum codebase.

## 1. Concurrency Safety

### 1.1 Race Conditions
**Risk:** Data corruption or panic due to concurrent access to shared memory.
**Mitigation:**
- Use `sync.Mutex` or `sync.RWMutex` for shared state.
- Use `atomic` package for simple counters/flags.
- Run tests with `-race` flag.
- Prefer channels for communication over shared memory.

**Example:**
```go
// UNSAFE
type Counter struct {
    val int
}
func (c *Counter) Inc() { c.val++ }

// SAFE
type Counter struct {
    mu  sync.Mutex
    val int
}
func (c *Counter) Inc() {
    c.mu.Lock()
    defer c.mu.Unlock()
    c.val++
}
```

### 1.2 Goroutine Leaks
**Risk:** Memory exhaustion due to unclosed goroutines.
**Mitigation:**
- Ensure all goroutines have a termination condition.
- Use `context.Context` for cancellation.
- Use `sync.WaitGroup` to wait for completion.
- Use `goleak` in tests.

**Example:**
```go
// UNSAFE: Goroutine might hang if ch is never written to
func process(ch <-chan int) {
    go func() {
        val := <-ch
        fmt.Println(val)
    }()
}

// SAFE: Use context for cancellation
func process(ctx context.Context, ch <-chan int) {
    go func() {
        select {
        case val := <-ch:
            fmt.Println(val)
        case <-ctx.Done():
            return
        }
    }()
}
```

### 1.3 Channel Safety
**Risk:** Panics on closed channels, deadlocks.
**Mitigation:**
- Never send on a closed channel (panic).
- Close channels only from the sender side.
- Use `select` with `default` or timeout to prevent blocking.
- Check for closed channel on receive: `val, ok := <-ch`.

## 2. Memory Safety

### 2.1 Slice Bounds
**Risk:** Panic due to out-of-bounds access.
**Mitigation:**
- Always check length before access.
- Use `copy` for safe data transfer.
- Be careful with slice re-slicing (shared backing array).

**Example:**
```go
// UNSAFE
val := data[10]

// SAFE
if len(data) > 10 {
    val := data[10]
}
```

### 2.2 Integer Overflow
**Risk:** Logic errors, infinite loops, security bypasses.
**Mitigation:**
- Check for overflow before arithmetic operations.
- Use `math/big` for large numbers (standard in Ethereum).
- Be aware of integer types (`int` is platform-dependent).

**Example:**
```go
// UNSAFE
sum := a + b

// SAFE
if b > 0 && a > math.MaxInt64 - b {
    return errorOverflow
}
sum := a + b
```

### 2.3 Pointer Safety
**Risk:** Nil pointer dereference panics.
**Mitigation:**
- Check for nil before dereferencing.
- Initialize structs properly.
- Use defensive programming.

**Example:**
```go
// UNSAFE
func (s *Service) Start() {
    s.config.Run() // Panic if s.config is nil
}

// SAFE
func (s *Service) Start() {
    if s.config == nil {
        return
    }
    s.config.Run()
}
```

## 3. Input Validation

### 3.1 External Data
**Risk:** Injection attacks, DoS, logic errors.
**Mitigation:**
- Validate all external input (RPC, P2P, files).
- Use strict parsing (e.g., JSON strict decoding).
- Sanitize strings.
- Limit input size.

### 3.2 Cryptographic Inputs
**Risk:** Invalid curve points, weak keys.
**Mitigation:**
- Validate public keys are on curve.
- Check signature length and format.
- Reject weak or invalid parameters.

## 4. Error Handling

### 4.1 Error Checking
**Risk:** Silent failures, undefined behavior.
**Mitigation:**
- Always check returned errors.
- Don't ignore errors with `_`.
- Handle errors gracefully (don't just log and continue if critical).

**Example:**
```go
// UNSAFE
file.Write(data)

// SAFE
if _, err := file.Write(data); err != nil {
    return fmt.Errorf("failed to write data: %w", err)
}
```

### 4.2 Error Wrapping
**Risk:** Loss of context.
**Mitigation:**
- Use `%w` to wrap errors.
- Add context to errors.
- Use `errors.Is` and `errors.As` for checking.

## 5. Cryptography

### 5.1 Randomness
**Risk:** Predictable keys/nonces.
**Mitigation:**
- Use `crypto/rand` for security-critical randomness.
- Avoid `math/rand` for keys/nonces.
- Seed `math/rand` properly if used for non-critical tasks.

### 5.2 Constant-Time Operations
**Risk:** Timing attacks.
**Mitigation:**
- Use `crypto/subtle.ConstantTimeCompare`.
- Avoid data-dependent branches in crypto code.

## 6. Dependency Management

### 6.1 Vulnerable Dependencies
**Risk:** Supply chain attacks.
**Mitigation:**
- Use `go mod verify`.
- Monitor dependencies for CVEs (govulncheck).
- Vendor dependencies if necessary.
- Review dependency updates.

## 7. Build & Deployment

### 7.1 Build Flags
**Risk:** Missing security features.
**Mitigation:**
- Use `-trimpath` for reproducible builds.
- Use `-ldflags` to strip debug info if needed.
- Enable ASLR/DEP (default in modern Go).

### 7.2 Panics
**Risk:** DoS due to crash.
**Mitigation:**
- Avoid `panic` in library code.
- Recover from panics at top-level boundaries (RPC, P2P handlers).
- Log panic stack traces.

## 8. Specific Geth Patterns

### 8.1 RLP Decoding
**Risk:** DoS via large allocations.
**Mitigation:**
- Use `rlp.Decode` with size limits.
- Validate decoded structures.

### 8.2 Big Integer Math
**Risk:** Performance DoS.
**Mitigation:**
- Limit bit length of inputs.
- Use `big.Int` carefully (memory allocation).

### 8.3 Database Access
**Risk:** Data corruption.
**Mitigation:**
- Use transactions/batches correctly.
- Handle database errors (disk full, corruption).
- Close iterators properly.

## 9. Tools

- **Static Analysis:** `golangci-lint`, `staticcheck`, `gosec`
- **Fuzzing:** `go test -fuzz`, `go-fuzz`
- **Race Detection:** `go test -race`
- **Vulnerability Check:** `govulncheck`

## 10. Checklist

- [ ] Race detection enabled in tests?
- [ ] Error returns checked?
- [ ] Inputs validated?
- [ ] Nil pointer checks?
- [ ] Goroutines managed/closed?
- [ ] Crypto operations constant-time?
- [ ] Dependencies updated/verified?
- [ ] Panics handled/avoided?
- [ ] Integer overflows checked?
- [ ] Memory bounds checked?
