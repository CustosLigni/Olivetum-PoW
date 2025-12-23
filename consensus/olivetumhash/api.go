package olivetumhash

import (
	"errors"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/rpc"
)

var errOlivetumhashStopped = errors.New("olivetumhash stopped")
var gatewayActiveWindow = 10 * time.Minute

// API exposes minimal RPC methods for Olivetumhash.
type API struct {
	olivetumhash *Olivetumhash
}

// GetHashrate returns the current hashrate for the local CPU miner.
func (api *API) GetHashrate() hexutil.Uint64 {
	return hexutil.Uint64(api.olivetumhash.Hashrate())
}

// GetWork returns a work package for external miner.
//
// The work package consists of 4 strings:
//
//	result[0] - 32 bytes hex encoded current block header pow-hash
//	result[1] - 32 bytes hex encoded seed hash used for DAG
//	result[2] - 32 bytes hex encoded boundary condition ("target"), 2^256/difficulty
//	result[3] - hex encoded block number
func (api *API) GetWork() ([4]string, error) {
	if api.olivetumhash.remote == nil {
		return [4]string{}, errors.New("not supported")
	}
	workCh := make(chan [4]string, 1)
	errc := make(chan error, 1)
	select {
	case api.olivetumhash.remote.fetchWorkCh <- &sealWork{errc: errc, res: workCh}:
	case <-api.olivetumhash.remote.exitCh:
		return [4]string{}, errOlivetumhashStopped
	}
	select {
	case work := <-workCh:
		return work, nil
	case err := <-errc:
		return [4]string{}, err
	}
}

// GetWorkFor returns a work package with coinbase set to the provided address.
func (api *API) GetWorkFor(address common.Address) ([4]string, error) {
	if api.olivetumhash.remote == nil {
		return [4]string{}, errors.New("not supported")
	}
	workCh := make(chan [4]string, 1)
	errc := make(chan error, 1)
	select {
	case api.olivetumhash.remote.fetchWorkForCh <- &sealWorkFor{address: address, errc: errc, res: workCh}:
	case <-api.olivetumhash.remote.exitCh:
		return [4]string{}, errOlivetumhashStopped
	}
	select {
	case work := <-workCh:
		return work, nil
	case err := <-errc:
		return [4]string{}, err
	}
}

// SubmitWork can be used by external miner to submit their POW solution.
// It returns an indication if the work was accepted.
// Note either an invalid solution, a stale work a non-existent work will return false.
func (api *API) SubmitWork(nonce types.BlockNonce, hash, digest common.Hash) bool {
	if api.olivetumhash.remote == nil {
		return false
	}
	errc := make(chan error, 1)
	select {
	case api.olivetumhash.remote.submitWorkCh <- &mineResult{
		nonce:     nonce,
		mixDigest: digest,
		hash:      hash,
		errc:      errc,
	}:
	case <-api.olivetumhash.remote.exitCh:
		return false
	}
	err := <-errc
	return err == nil
}

// SubmitWorkFor accepts a solution for a work package generated with a custom coinbase.
func (api *API) SubmitWorkFor(address common.Address, nonce types.BlockNonce, hash, digest common.Hash) bool {
	// address is carried for RPC symmetry and potential future validation.
	return api.SubmitWork(nonce, hash, digest)
}

// SubmitHashrate can be used for remote miners to submit their hash rate.
// This enables the node to report the combined hash rate of all miners
// which submit work through this node.
//
// It accepts the miner hash rate and an identifier which must be unique
// between nodes.
func (api *API) SubmitHashrate(rate hexutil.Uint64, id common.Hash) bool {
	if api.olivetumhash.remote == nil {
		return false
	}
	done := make(chan struct{}, 1)
	select {
	case api.olivetumhash.remote.submitRateCh <- &hashrate{done: done, rate: uint64(rate), id: id}:
	case <-api.olivetumhash.remote.exitCh:
		return false
	}
	<-done
	return true
}

// SubmitHashrateFor allows remote miners to report hashrate with a custom coinbase.
func (api *API) SubmitHashrateFor(address common.Address, rate hexutil.Uint64, id common.Hash) bool {
	if api.olivetumhash.remote == nil {
		return false
	}
	api.olivetumhash.remote.updateHashrate(address, uint64(rate))
	return true
}

// GatewayStats returns aggregated stats for miners using getWorkFor/submitWorkFor.
func (api *API) GatewayStats() GatewayStats {
	if api.olivetumhash.remote == nil {
		return GatewayStats{}
	}
	return api.olivetumhash.remote.snapshotStats(gatewayActiveWindow)
}

// PublicAPIs returns the RPC descriptors for the Olivetumhash API.
func PublicAPIs(o *Olivetumhash) []rpc.API {
	return []rpc.API{
		{
			Namespace: "olivetumhash",
			Version:   "1.0",
			Service:   &API{o},
			Public:    true,
		},
	}
}

// GatewayStats represents aggregated metrics exposed via RPC.
type GatewayStats struct {
	ActiveMiners          int                `json:"activeMiners"`
	TotalReportedHashrate uint64             `json:"totalReportedHashrate"`
	TotalWork             uint64             `json:"totalWork"`
	TotalSubmits          uint64             `json:"totalSubmits"`
	Miners                []GatewayMinerStat `json:"miners"`
}

type GatewayMinerStat struct {
	Address          common.Address `json:"address"`
	WorkCount        uint64         `json:"workCount"`
	SubmitCount      uint64         `json:"submitCount"`
	ReportedHashrate uint64         `json:"reportedHashrate"`
	LastWork         time.Time      `json:"lastWork,omitempty"`
	LastSubmit       time.Time      `json:"lastSubmit,omitempty"`
	LastHashrate     time.Time      `json:"lastHashrate,omitempty"`
	Active           bool           `json:"active"`
}
