package core_test

import (
	"crypto/ecdsa"
	"math/big"
	"testing"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/consensus/olivetumhash"
	"github.com/ethereum/go-ethereum/core"
	"github.com/ethereum/go-ethereum/core/rawdb"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/core/vm"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/params"
	"github.com/ethereum/go-ethereum/params/types/genesisT"
	"github.com/ethereum/go-ethereum/params/types/goethereum"
	"github.com/ethereum/go-ethereum/params/vars"
)

var (
	olivetumBurnSlot         = common.Hash{}
	olivetumLastDividendSlot = common.Hash{0: 1}
	olivetumRoundRateSlot    = common.Hash{0: 2}
	olivetumRoundStartSlot   = common.Hash{0: 3}
	olivetumRoundIDSlot      = common.Hash{0: 4}
)

func TestOlivetumEconomyForkTwoNodeImport(t *testing.T) {
	oldFork := params.GetEconomyForkBlock()
	params.SetEconomyForkBlock(big.NewInt(3))
	t.Cleanup(func() { params.SetEconomyForkBlock(oldFork) })

	oldMax := params.GetOffSessionMaxPerTx()
	params.SetOffSessionMaxPerTx(new(big.Int).Mul(big.NewInt(1000), big.NewInt(vars.Ether)))
	t.Cleanup(func() { params.SetOffSessionMaxPerTx(oldMax) })

	genesisTime := uint64(time.Date(2024, time.March, 3, 10, 0, 0, 0, time.UTC).Unix())
	coinbase := common.HexToAddress("0x000000000000000000000000000000000000c0fe")

	spenderKey, err := crypto.GenerateKey()
	if err != nil {
		t.Fatalf("spender key: %v", err)
	}
	spender := crypto.PubkeyToAddress(spenderKey.PublicKey)
	claimerKey, err := crypto.GenerateKey()
	if err != nil {
		t.Fatalf("claimer key: %v", err)
	}
	claimer := crypto.PubkeyToAddress(claimerKey.PublicKey)

	recipient := common.HexToAddress("0x0000000000000000000000000000000000000022")
	spenderBal := new(big.Int).Mul(big.NewInt(10000), big.NewInt(vars.Ether))
	claimerBal := new(big.Int).Mul(big.NewInt(1000), big.NewInt(vars.Ether))

	burnRate := uint64(150)
	dividendRate := uint64(50)

	genesis := olivetumTestGenesis(genesisTime, spender, spenderBal, claimer, claimerBal, burnRate, dividendRate)

	blocksWithClaim := olivetumGenerateEconomyForkChain(t, genesis, coinbase, spender, spenderKey, claimer, claimerKey, recipient, true)
	blocksWithoutClaim := olivetumGenerateEconomyForkChain(t, genesis, coinbase, spender, spenderKey, claimer, claimerKey, recipient, false)

	chainA := olivetumNewBlockchain(t, genesis)
	if _, err := chainA.InsertChain(blocksWithClaim); err != nil {
		t.Fatalf("node A insert: %v", err)
	}
	chainB := olivetumNewBlockchain(t, genesis)
	if _, err := chainB.InsertChain(blocksWithClaim); err != nil {
		t.Fatalf("node B insert: %v", err)
	}
	if chainA.CurrentBlock().Hash() != chainB.CurrentBlock().Hash() {
		t.Fatalf("nodes diverged: A=%s B=%s", chainA.CurrentBlock().Hash(), chainB.CurrentBlock().Hash())
	}

	stateA, err := chainA.State()
	if err != nil {
		t.Fatalf("node A state: %v", err)
	}
	stateB, err := chainB.State()
	if err != nil {
		t.Fatalf("node B state: %v", err)
	}

	value1 := new(big.Int).Mul(big.NewInt(600), big.NewInt(vars.Ether))
	value2 := new(big.Int).Mul(big.NewInt(300), big.NewInt(vars.Ether))
	expectedBurned := new(big.Int).Add(params.EconomyBaselineBurnedWei(), new(big.Int).Add(olivetumBurnedNet(value1, burnRate), olivetumBurnedNet(value2, burnRate)))
	if core.GetTotalBurned(stateA).Cmp(expectedBurned) != 0 {
		t.Fatalf("unexpected burned total (A): got %v want %v", core.GetTotalBurned(stateA), expectedBurned)
	}
	if core.GetTotalBurned(stateB).Cmp(expectedBurned) != 0 {
		t.Fatalf("unexpected burned total (B): got %v want %v", core.GetTotalBurned(stateB), expectedBurned)
	}

	expectedReward := new(big.Int).Mul(claimerBal, new(big.Int).SetUint64(dividendRate))
	expectedReward.Div(expectedReward, big.NewInt(10000))
	if core.GetTotalDividendsMinted(stateA).Cmp(expectedReward) != 0 {
		t.Fatalf("unexpected dividends minted: got %v want %v", core.GetTotalDividendsMinted(stateA), expectedReward)
	}

	headTime := chainA.CurrentBlock().Time
	expectedSpent := new(big.Int).Add(value1, value2)
	if got := core.GetOffSessionBudgetSpent(stateA, spender, headTime); got.Cmp(expectedSpent) != 0 {
		t.Fatalf("unexpected offsession spent: got %v want %v", got, expectedSpent)
	}

	chainNoClaim := olivetumNewBlockchain(t, genesis)
	if _, err := chainNoClaim.InsertChain(blocksWithoutClaim); err != nil {
		t.Fatalf("no-claim insert: %v", err)
	}
	stateNoClaim, err := chainNoClaim.State()
	if err != nil {
		t.Fatalf("no-claim state: %v", err)
	}

	tip := olivetumDividendTip(expectedReward, burnRate)
	withTip := new(big.Int).Set(stateA.GetBalance(coinbase).ToBig())
	noTip := new(big.Int).Set(stateNoClaim.GetBalance(coinbase).ToBig())
	delta := new(big.Int).Sub(withTip, noTip)
	if delta.Cmp(tip) != 0 {
		t.Fatalf("unexpected coinbase delta: got %v want %v", delta, tip)
	}
}

func olivetumTestGenesis(ts uint64, spender common.Address, spenderBal *big.Int, claimer common.Address, claimerBal *big.Int, burnRate uint64, dividendRate uint64) *genesisT.Genesis {
	cfg := &goethereum.ChainConfig{
		ChainID:             big.NewInt(30216931),
		HomesteadBlock:      big.NewInt(0),
		EIP150Block:         big.NewInt(0),
		EIP155Block:         big.NewInt(0),
		EIP158Block:         big.NewInt(0),
		ByzantiumBlock:      big.NewInt(0),
		ConstantinopleBlock: big.NewInt(0),
		PetersburgBlock:     big.NewInt(0),
		IstanbulBlock:       big.NewInt(0),
		BerlinBlock:         big.NewInt(0),
	}
	params.ApplyOlivetumDefaults(cfg)

	dividendStorage := map[common.Hash]common.Hash{
		olivetumLastDividendSlot: common.BigToHash(new(big.Int).SetUint64(ts)),
		olivetumRoundRateSlot:    common.BigToHash(new(big.Int).SetUint64(dividendRate)),
		olivetumRoundStartSlot:   common.BigToHash(new(big.Int).SetUint64(ts)),
		olivetumRoundIDSlot:      common.BigToHash(new(big.Int).SetUint64(1)),
	}
	burnStorage := map[common.Hash]common.Hash{
		olivetumBurnSlot: common.BigToHash(new(big.Int).SetUint64(burnRate)),
	}
	return &genesisT.Genesis{
		Config:     cfg,
		Timestamp:  ts,
		GasLimit:   15_000_000,
		Difficulty: big.NewInt(1),
		Alloc: genesisT.GenesisAlloc{
			spender:               {Balance: new(big.Int).Set(spenderBal)},
			claimer:               {Balance: new(big.Int).Set(claimerBal)},
			core.BurnContract:     {Balance: new(big.Int), Storage: burnStorage},
			core.DividendContract: {Balance: new(big.Int), Storage: dividendStorage},
		},
	}
}

func olivetumGenerateEconomyForkChain(t *testing.T, genesis *genesisT.Genesis, coinbase common.Address, spender common.Address, spenderKey *ecdsa.PrivateKey, claimer common.Address, claimerKey *ecdsa.PrivateKey, recipient common.Address, withClaim bool) []*types.Block {
	t.Helper()
	engine := olivetumhash.NewFaker()
	_, blocks, _ := core.GenerateChainWithGenesis(genesis, engine, 4, func(i int, gen *core.BlockGen) {
		gen.SetCoinbase(coinbase)
		signer := gen.Signer()
		gasPrice := new(big.Int)
		switch i {
		case 2:
			v := new(big.Int).Mul(big.NewInt(600), big.NewInt(vars.Ether))
			tx, err := types.SignTx(types.NewTransaction(gen.TxNonce(spender), recipient, v, vars.TxGas, gasPrice, nil), signer, spenderKey)
			if err != nil {
				t.Fatalf("sign tx1: %v", err)
			}
			gen.AddTx(tx)
			if withClaim {
				claim, err := types.SignTx(types.NewTransaction(gen.TxNonce(claimer), core.DividendContract, new(big.Int), vars.TxGas, gasPrice, nil), signer, claimerKey)
				if err != nil {
					t.Fatalf("sign claim: %v", err)
				}
				gen.AddTx(claim)
			}
		case 3:
			v := new(big.Int).Mul(big.NewInt(300), big.NewInt(vars.Ether))
			tx, err := types.SignTx(types.NewTransaction(gen.TxNonce(spender), recipient, v, vars.TxGas, gasPrice, nil), signer, spenderKey)
			if err != nil {
				t.Fatalf("sign tx2: %v", err)
			}
			gen.AddTx(tx)
		}
	})
	return blocks
}

func olivetumNewBlockchain(t *testing.T, genesis *genesisT.Genesis) *core.BlockChain {
	t.Helper()
	engine := olivetumhash.NewFaker()
	chain, err := core.NewBlockChain(rawdb.NewMemoryDatabase(), core.DefaultCacheConfigWithScheme(rawdb.HashScheme), genesis, nil, engine, vm.Config{}, nil, nil)
	if err != nil {
		t.Fatalf("new chain: %v", err)
	}
	t.Cleanup(chain.Stop)
	return chain
}

func olivetumBurnedNet(value *big.Int, burnRate uint64) *big.Int {
	burn := new(big.Int).Mul(value, new(big.Int).SetUint64(burnRate))
	burn.Div(burn, big.NewInt(10000))
	minerShare := new(big.Int).Mul(burn, big.NewInt(int64(core.MinerBurnShareBps)))
	minerShare.Div(minerShare, big.NewInt(10000))
	return burn.Sub(burn, minerShare)
}

func olivetumDividendTip(reward *big.Int, burnRate uint64) *big.Int {
	virtualBurn := new(big.Int).Mul(reward, new(big.Int).SetUint64(burnRate))
	virtualBurn.Div(virtualBurn, big.NewInt(10000))
	tip := new(big.Int).Mul(virtualBurn, big.NewInt(int64(core.MinerBurnShareBps)))
	tip.Div(tip, big.NewInt(10000))
	return tip
}
