package main

import (
	"crypto/sha256"
	"errors"
	"flag"
	"fmt"
	"math/big"
	"os"
	"runtime"
	"runtime/pprof"
	"sync"
	"sync/atomic"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/mclock"
	"github.com/ethereum/go-ethereum/core/txpool"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/crypto/kzg4844"
	"github.com/ethereum/go-ethereum/eth/fetcher"
	"github.com/ethereum/go-ethereum/params"
	"github.com/holiman/uint256"
)

func main() {
	var (
		duration   = flag.Duration("duration", 10*time.Second, "how long to run")
		peers      = flag.Int("peers", max(1, runtime.NumCPU()/2), "number of concurrent 'peers' (workers)")
		txsPerSend = flag.Int("txs", 32, "transactions per Enqueue call (<=32 avoids TxFetcher 200ms sleep)")
		cpuProfile = flag.String("cpuprofile", "", "write CPU profile to file")
		debug      = flag.Bool("debug", false, "print a single validation result and exit")
	)
	flag.Parse()

	if *peers <= 0 {
		fatalf("peers must be > 0")
	}
	if *txsPerSend <= 0 {
		fatalf("txs must be > 0")
	}

	// Configure a fork-rule set where Cancun is active and Osaka is not, so we
	// exercise legacy blob-proof verification (VerifyBlobProof).
	chainConfig := *params.TestChainConfig
	chainConfig.CancunTime = u64(0)
	chainConfig.PragueTime = nil
	chainConfig.OsakaTime = nil
	chainConfig.BlobScheduleConfig = &params.BlobScheduleConfig{
		Cancun: &params.BlobConfig{
			Target:         3,
			Max:            6,
			UpdateFraction: params.DefaultCancunBlobConfig.UpdateFraction,
		},
	}
	head := &types.Header{
		Number:     big.NewInt(1),
		Time:       1,
		Difficulty: big.NewInt(0), // PoS rules
		BaseFee:    big.NewInt(1),
		GasLimit:   30_000_000,
	}

	makeTx, err := newInvalidBlobTxMaker(&chainConfig)
	if err != nil {
		fatalf("failed to initialize invalid blob tx maker: %v", err)
	}

	opts := &txpool.ValidationOptions{
		Config:       &chainConfig,
		Accept:       1 << types.BlobTxType,
		MaxSize:      1024 * 1024,
		MaxBlobCount: 1,
		MinTip:       big.NewInt(0),
	}
	signer := types.NewCancunSigner(chainConfig.ChainID)

	// Build a TxFetcher whose addTxs callback runs stateless validation (incl. KZG),
	// and whose dropPeer callback records whether it ever gets invoked.
	var (
		validations atomic.Uint64
		failures    atomic.Uint64
		dropped     atomic.Uint64
	)

	addTxs := func(txs []*types.Transaction) []error {
		errs := make([]error, len(txs))
		for i, tx := range txs {
			validations.Add(1)
			if err := txpool.ValidateTransaction(tx, head, signer, opts); err != nil {
				failures.Add(1)
				errs[i] = err
			}
		}
		return errs
	}

	f := fetcher.NewTxFetcherForTests(
		func(common.Hash, byte) error { return nil }, // validateMeta (unused)
		addTxs, // addTxs (sync; does KZG)
		func(string, []common.Hash) error { return nil }, // fetchTxs (unused)
		func(string) { dropped.Add(1) },                  // dropPeer (should remain 0)
		mclock.System{}, time.Now, nil,
	)
	f.Start()
	defer f.Stop()

	if *cpuProfile != "" {
		fh, err := os.Create(*cpuProfile)
		if err != nil {
			fatalf("create cpuprofile: %v", err)
		}
		defer fh.Close()
		if err := pprof.StartCPUProfile(fh); err != nil {
			fatalf("start cpuprofile: %v", err)
		}
		defer pprof.StopCPUProfile()
	}

	// Debug/diagnostic mode: validate a single tx and print the error + timing.
	if *debug {
		tx, err := makeTx(0)
		if err != nil {
			fatalf("makeTx failed: %v", err)
		}
		start := time.Now()
		err = txpool.ValidateTransaction(tx, head, signer, opts)
		fmt.Printf("tx_type=%d blob_hashes=%d sidecar_nil=%v\n", tx.Type(), len(tx.BlobHashes()), tx.BlobTxSidecar() == nil)
		fmt.Printf("validate_err=%v\n", err)
		fmt.Printf("validate_took=%s\n", time.Since(start))
		return
	}

	// Run "peers" concurrently, each repeatedly calling Enqueue with batches of invalid blob txs.
	stop := time.Now().Add(*duration)

	// Pre-generate a corpus of distinct txs (nonce-differentiated) so we don't
	// spend significant time signing inside the hot loop. KZG verification is
	// the intended hot spot.
	corpusSize := max(1024, (*peers)*(*txsPerSend)*2)
	corpus := make([]*types.Transaction, 0, corpusSize)
	for i := 0; i < corpusSize; i++ {
		tx, err := makeTx(uint64(i))
		if err != nil {
			fatalf("failed to create tx corpus item: %v", err)
		}
		corpus = append(corpus, tx)
	}

	var wg sync.WaitGroup
	wg.Add(*peers)
	for peerIndex := 0; peerIndex < *peers; peerIndex++ {
		peerID := fmt.Sprintf("peer-%d", peerIndex)
		go func(peer string, nonceBase uint64) {
			defer wg.Done()
			batch := make([]*types.Transaction, 0, *txsPerSend)
			cursor := int(nonceBase % uint64(len(corpus)))

			for time.Now().Before(stop) {
				batch = batch[:0]
				for i := 0; i < *txsPerSend; i++ {
					batch = append(batch, corpus[cursor])
					cursor++
					if cursor == len(corpus) {
						cursor = 0
					}
				}
				// direct=true models PooledTransactionsMsg deliveries flowing through the
				// "direct" path (but this harness does not do any networking).
				_ = f.Enqueue(peer, batch, true)
			}
		}(peerID, uint64(peerIndex)<<32)
	}
	wg.Wait()

	v := validations.Load()
	fa := failures.Load()
	dp := dropped.Load()

	fmt.Printf("duration=%s peers=%d txs_per_enqueue=%d\n", duration.String(), *peers, *txsPerSend)
	fmt.Printf("validations=%d failures=%d dropped_peers=%d\n", v, fa, dp)
	if v > 0 {
		fmt.Printf("avg_validations_per_sec=%.2f\n", float64(v)/duration.Seconds())
	}
	if dp != 0 {
		fmt.Printf("NOTE: dropPeer callback fired (unexpected in this harness)\n")
	}
}

func newInvalidBlobTxMaker(chainConfig *params.ChainConfig) (func(nonce uint64) (*types.Transaction, error), error) {
	key, err := crypto.GenerateKey()
	if err != nil {
		return nil, err
	}
	chainID := uint256.MustFromBig(chainConfig.ChainID)

	// Start from an all-zero blob (canonical), compute commitment+proof for it,
	// then mutate the blob so the proof becomes invalid while remaining well-formed.
	var blob kzg4844.Blob
	commitment, err := kzg4844.BlobToCommitment(&blob)
	if err != nil {
		return nil, err
	}
	proof, err := kzg4844.ComputeBlobProof(&blob, commitment)
	if err != nil {
		return nil, err
	}
	mutated := blob
	mutated[31] = 1 // keeps the first field element canonical, but changes the blob

	vhash := kzg4844.CalcBlobHashV1(sha256.New(), &commitment)
	sidecar := types.NewBlobTxSidecar(
		types.BlobSidecarVersion0,
		[]kzg4844.Blob{mutated},
		[]kzg4844.Commitment{commitment},
		[]kzg4844.Proof{proof},
	)
	if err := sidecar.ValidateBlobCommitmentHashes([]common.Hash{vhash}); err != nil {
		return nil, fmt.Errorf("unexpected commitment-hash validation failure: %w", err)
	}

	// Sanity: the proof must fail (otherwise we didn’t construct the intended case).
	if err := kzg4844.VerifyBlobProof(&sidecar.Blobs[0], sidecar.Commitments[0], sidecar.Proofs[0]); err == nil {
		return nil, errors.New("constructed blob proof unexpectedly verifies")
	}

	signer := types.NewCancunSigner(chainID.ToBig())
	minBlobFeeCap := new(big.Int).Add(big.NewInt(params.BlobTxMinBlobGasprice), big.NewInt(1))

	return func(nonce uint64) (*types.Transaction, error) {
		txData := &types.BlobTx{
			ChainID:    chainID,
			Nonce:      nonce,
			GasTipCap:  uint256.NewInt(1),
			GasFeeCap:  uint256.NewInt(2),
			Gas:        21_000,
			To:         common.Address{},
			Value:      uint256.NewInt(0),
			Data:       nil,
			AccessList: nil,
			BlobFeeCap: uint256.MustFromBig(minBlobFeeCap),
			BlobHashes: []common.Hash{vhash},
			Sidecar:    sidecar,
		}
		return types.SignNewTx(key, signer, txData)
	}, nil
}

func u64(v uint64) *uint64 { return &v }

func fatalf(format string, args ...any) {
	fmt.Fprintf(os.Stderr, format+"\n", args...)
	os.Exit(2)
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
