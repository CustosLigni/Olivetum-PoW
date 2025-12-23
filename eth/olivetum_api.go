package eth

import (
	"context"
	"fmt"
	"math/big"

	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/core"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/params"
)

// OlivetumRuntimeConfig exposes the currently active runtime parameters.
type OlivetumRuntimeConfig struct {
	BlockPeriod            uint64       `json:"blockPeriod"`
	GasLimit               uint64       `json:"gasLimit"`
	MinTxAmount            *hexutil.Big `json:"minTxAmount"`
	TxRateLimit            uint64       `json:"txRateLimit"`
	OffSessionTxRate       uint64       `json:"offSessionTxRate"`
	OffSessionMaxPerTx     *hexutil.Big `json:"offSessionMaxPerTx"`
	SessionTzOffsetSeconds int32        `json:"sessionTzOffsetSeconds"`
	BurnRate               uint64       `json:"burnRate"`
	DividendRate           uint64       `json:"dividendRate"`
}

// OlivetumSupply exposes supply-related stats.
type OlivetumSupply struct {
	TotalMinted  *hexutil.Big `json:"totalMinted"`
	MaxSupply    *hexutil.Big `json:"maxSupply"`
	Remaining    *hexutil.Big `json:"remaining"`
	BurnRate     uint64       `json:"burnRate"`
	DividendRate uint64       `json:"dividendRate"`
	Burned       *hexutil.Big `json:"burned"`
	Dividends    *hexutil.Big `json:"dividendsMinted"`
}

// OlivetumAPI exposes chain-specific helper RPCs (read-only, non-consensus).
type OlivetumAPI struct {
	eth *Ethereum
}

// NewOlivetumAPI wires an OlivetumAPI instance.
func NewOlivetumAPI(eth *Ethereum) *OlivetumAPI {
	return &OlivetumAPI{eth: eth}
}

// GetRuntimeConfig returns the current Olivetum runtime configuration.
func (api *OlivetumAPI) GetRuntimeConfig(ctx context.Context) (*OlivetumRuntimeConfig, error) {
	if !params.IsOlivetumConfig(api.eth.blockchain.Config()) {
		return nil, fmt.Errorf("not an Olivetum chain")
	}
	state, err := api.eth.BlockChain().State()
	if err != nil {
		return nil, err
	}
	return &OlivetumRuntimeConfig{
		BlockPeriod:            params.GetBlockPeriod(),
		GasLimit:               params.GetGasLimit(),
		MinTxAmount:            (*hexutil.Big)(params.GetMinTxAmount()),
		TxRateLimit:            params.GetTxRateLimit(),
		OffSessionTxRate:       params.GetOffSessionTxRate(),
		OffSessionMaxPerTx:     (*hexutil.Big)(params.GetOffSessionMaxPerTx()),
		SessionTzOffsetSeconds: params.GetSessionTzOffsetSeconds(),
		BurnRate:               core.GetBurnRate(state),
		DividendRate:           core.GetDividendRate(state),
	}, nil
}

// GetFinalizedHeight returns the current finalized height watermark (monotonic).
func (api *OlivetumAPI) GetFinalizedHeight(ctx context.Context) hexutil.Uint64 {
	return hexutil.Uint64(api.eth.blockchain.FinalizedHeight())
}

// GetSupply returns the minted supply stats.
func (api *OlivetumAPI) GetSupply(ctx context.Context) (*OlivetumSupply, error) {
	if !params.IsOlivetumConfig(api.eth.blockchain.Config()) {
		return nil, fmt.Errorf("not an Olivetum chain")
	}
	state, err := api.eth.BlockChain().State()
	if err != nil {
		return nil, err
	}
	totalMinted := core.GetTotalMinted(state)
	maxSupply := params.MaxSupply()
	remaining := new(big.Int).Sub(maxSupply, totalMinted)
	if remaining.Sign() < 0 {
		remaining = new(big.Int)
	}
	return &OlivetumSupply{
		TotalMinted:  (*hexutil.Big)(totalMinted),
		MaxSupply:    (*hexutil.Big)(maxSupply),
		Remaining:    (*hexutil.Big)(remaining),
		BurnRate:     core.GetBurnRate(state),
		DividendRate: core.GetDividendRate(state),
		Burned:       (*hexutil.Big)(big.NewInt(0)), // not tracked on-chain
		Dividends:    (*hexutil.Big)(big.NewInt(0)), // not tracked on-chain
	}, nil
}

// GetNetworkHashrate returns an estimate of the network hashrate (H/s) averaged
// over the given window of blocks. If blocks is nil or 0, defaults to 120.
func (api *OlivetumAPI) GetNetworkHashrate(ctx context.Context, blocks *hexutil.Uint64) (*hexutil.Big, error) {
	if !params.IsOlivetumConfig(api.eth.blockchain.Config()) {
		return nil, fmt.Errorf("not an Olivetum chain")
	}
	window := uint64(120)
	if blocks != nil && uint64(*blocks) > 0 {
		window = uint64(*blocks)
	}
	chain := api.eth.BlockChain()
	head := chain.CurrentBlock()
	if head == nil {
		return (*hexutil.Big)(big.NewInt(0)), nil
	}
	headNum := head.Number.Uint64()
	if headNum == 0 {
		return (*hexutil.Big)(big.NewInt(0)), nil
	}
	if window > headNum {
		window = headNum
	}
	var base *types.Block
	if window == headNum {
		base = chain.Genesis()
	} else {
		base = chain.GetBlockByNumber(headNum - window)
	}
	if base == nil {
		return nil, fmt.Errorf("could not load base block")
	}
	tdHead := chain.GetTd(head.Hash(), headNum)
	tdBase := chain.GetTd(base.Hash(), base.NumberU64())
	if tdHead == nil || tdBase == nil {
		return nil, fmt.Errorf("missing total difficulty data")
	}
	timeDiff := int64(head.Time) - int64(base.Time())
	if timeDiff <= 0 {
		return (*hexutil.Big)(big.NewInt(0)), nil
	}
	tdDiff := new(big.Int).Sub(tdHead, tdBase)
	if tdDiff.Sign() <= 0 {
		return (*hexutil.Big)(big.NewInt(0)), nil
	}
	hashrate := tdDiff.Div(tdDiff, big.NewInt(timeDiff))
	return (*hexutil.Big)(hashrate), nil
}
