package etherman

import (
	"context"
	"errors"
	"fmt"
	"math/big"
	"strings"

	"github.com/0xPolygonHermez/zkevm-aggregator/encoding"
	ethmanTypes "github.com/0xPolygonHermez/zkevm-aggregator/etherman/types"
	"github.com/0xPolygonHermez/zkevm-aggregator/log"
	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/rpc"
)

// GetLatestVerifiedBatchNum gets latest verified batch from ethereum
func (etherMan *Client) GetLatestVerifiedBatchNum() (uint64, error) {
	var lastVerifiedBatchNum uint64
	rollupData, err := etherMan.RollupManager.RollupIDToRollupData(&bind.CallOpts{Pending: false}, etherMan.RollupID)
	if err != nil {
		log.Debug("error getting lastVerifiedBatchNum from rollupManager. Trying old zkevm smc... Error: ", err)
		lastVerifiedBatchNum, err = etherMan.OldZkEVM.LastVerifiedBatch(&bind.CallOpts{Pending: false})
		if err != nil {
			return lastVerifiedBatchNum, err
		}
	} else {
		lastVerifiedBatchNum = rollupData.LastVerifiedBatch
	}
	return lastVerifiedBatchNum, nil
}

// BuildTrustedVerifyBatchesTxData builds a []bytes to be sent to the PoE SC method TrustedVerifyBatches.
func (etherMan *Client) BuildTrustedVerifyBatchesTxData(lastVerifiedBatch, newVerifiedBatch uint64, inputs *ethmanTypes.FinalProofInputs, beneficiary common.Address) (to *common.Address, data []byte, err error) {
	opts, err := etherMan.generateRandomAuth()
	if err != nil {
		return nil, nil, fmt.Errorf("failed to build trusted verify batches, err: %w", err)
	}
	opts.NoSend = true
	// force nonce, gas limit and gas price to avoid querying it from the chain
	opts.Nonce = big.NewInt(1)
	opts.GasLimit = uint64(1)
	opts.GasPrice = big.NewInt(1)

	var newLocalExitRoot [32]byte
	copy(newLocalExitRoot[:], inputs.NewLocalExitRoot)

	var newStateRoot [32]byte
	copy(newStateRoot[:], inputs.NewStateRoot)

	proof, err := convertProof(inputs.FinalProof.Proof)
	if err != nil {
		log.Errorf("error converting proof. Error: %v, Proof: %s", err, inputs.FinalProof.Proof)
		return nil, nil, err
	}

	const pendStateNum = 0 // TODO hardcoded for now until we implement the pending state feature

	tx, err := etherMan.RollupManager.VerifyBatchesTrustedAggregator(
		&opts,
		etherMan.RollupID,
		pendStateNum,
		lastVerifiedBatch,
		newVerifiedBatch,
		newLocalExitRoot,
		newStateRoot,
		beneficiary,
		proof,
	)
	if err != nil {
		if parsedErr, ok := tryParseError(err); ok {
			err = parsedErr
		}
		return nil, nil, err
	}

	return tx.To(), tx.Data(), nil
}

// GetLatestBlockHeader gets the latest block header from the ethereum
func (etherMan *Client) GetLatestBlockHeader(ctx context.Context) (*types.Header, error) {
	header, err := etherMan.EthClient.HeaderByNumber(ctx, big.NewInt(int64(rpc.LatestBlockNumber)))
	if err != nil || header == nil {
		return nil, err
	}
	return header, nil
}

// GetBatchAccInputHash gets the batch accumulated input hash from the ethereum
func (etherman *Client) GetBatchAccInputHash(ctx context.Context, batchNumber uint64) (common.Hash, error) {
	rollupData, err := etherman.RollupManager.GetRollupSequencedBatches(&bind.CallOpts{Pending: false}, etherman.RollupID, batchNumber)
	if err != nil {
		return common.Hash{}, err
	}
	return rollupData.AccInputHash, nil
}

// GetRollupId returns the rollup id
func (etherMan *Client) GetRollupId() uint32 {
	return etherMan.RollupID
}

// generateRandomAuth generates an authorization instance from a
// randomly generated private key to be used to estimate gas for PoE
// operations NOT restricted to the Trusted Sequencer
func (etherMan *Client) generateRandomAuth() (bind.TransactOpts, error) {
	privateKey, err := crypto.GenerateKey()
	if err != nil {
		return bind.TransactOpts{}, errors.New("failed to generate a private key to estimate L1 txs")
	}
	chainID := big.NewInt(0).SetUint64(etherMan.l1Cfg.L1ChainID)
	auth, err := bind.NewKeyedTransactorWithChainID(privateKey, chainID)
	if err != nil {
		return bind.TransactOpts{}, errors.New("failed to generate a fake authorization to estimate L1 txs")
	}

	return *auth, nil
}

func convertProof(p string) ([24][32]byte, error) {
	if len(p) != 24*32*2+2 {
		return [24][32]byte{}, fmt.Errorf("invalid proof length. Length: %d", len(p))
	}
	p = strings.TrimPrefix(p, "0x")
	proof := [24][32]byte{}
	for i := 0; i < 24; i++ {
		data := p[i*64 : (i+1)*64]
		p, err := encoding.DecodeBytes(&data)
		if err != nil {
			return [24][32]byte{}, fmt.Errorf("failed to decode proof, err: %w", err)
		}
		var aux [32]byte
		copy(aux[:], p)
		proof[i] = aux
	}
	return proof, nil
}

// GetL2ChainID returns L2 Chain ID
func (etherMan *Client) GetL2ChainID() (uint64, error) {
	chainID, err := etherMan.OldZkEVM.ChainID(&bind.CallOpts{Pending: false})
	log.Debug("chainID read from oldZkevm: ", chainID)
	if err != nil || chainID == 0 {
		log.Debug("error from oldZkevm: ", err)
		rollupData, err := etherMan.RollupManager.RollupIDToRollupData(&bind.CallOpts{Pending: false}, etherMan.RollupID)
		log.Debugf("ChainID read from rollupManager: %d using rollupID: %d", rollupData.ChainID, etherMan.RollupID)
		if err != nil {
			log.Debug("error from rollupManager: ", err)
			return 0, err
		} else if rollupData.ChainID == 0 {
			return rollupData.ChainID, fmt.Errorf("error: chainID received is 0!!")
		}
		return rollupData.ChainID, nil
	}
	return chainID, nil
}
