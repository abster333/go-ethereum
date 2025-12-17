package main

import (
	"fmt"
	"math/big"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/crypto/kzg4844"
	"github.com/ethereum/go-ethereum/rlp"
	"github.com/holiman/uint256"
)

func main() {
	// Create a dummy 1-blob transaction
	blob := kzg4844.Blob{} // Zero blob
	commitment, _ := kzg4844.BlobToCommitment(&blob)
	proof, _ := kzg4844.ComputeBlobProof(&blob, commitment)

	sidecar := &types.BlobTxSidecar{
		Blobs:       []kzg4844.Blob{blob},
		Commitments: []kzg4844.Commitment{commitment},
		Proofs:      []kzg4844.Proof{proof},
	}

	txData := &types.BlobTx{
		ChainID:    uint256.NewInt(1),
		Nonce:      0,
		GasTipCap:  uint256.NewInt(1000000000),
		GasFeeCap:  uint256.NewInt(10000000000),
		Gas:        21000,
		To:         common.Address{},
		Value:      uint256.NewInt(0),
		Data:       nil,
		AccessList: nil,
		BlobFeeCap: uint256.NewInt(10000000000),
		BlobHashes: []common.Hash{common.Hash{}}, // Dummy hash
		Sidecar:    sidecar,
	}

	tx := types.NewTx(txData)

	// Sign it (mock signature)
	signer := types.NewCancunSigner(big.NewInt(1))
	key, _ := crypto.GenerateKey()
	signedTx, _ := types.SignTx(tx, signer, key)

	// Measure RLP size
	data, err := rlp.EncodeToBytes(signedTx)
	if err != nil {
		fmt.Printf("Error encoding: %v\n", err)
		return
	}

	fmt.Printf("RLP Size of 1-blob tx: %d bytes\n", len(data))

	// Check overhead
	blobSize := 131072
	overhead := len(data) - blobSize
	fmt.Printf("Non-blob overhead: %d bytes\n", overhead)

	// Calculate max in 10MB
	maxMsg := 10 * 1024 * 1024
	count := maxMsg / len(data)
	fmt.Printf("Max txs in 10MB: %d\n", count)
}
