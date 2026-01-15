package params

import "math/big"

var (
	economyBaselineBurnedWei = func() *big.Int {
		v, ok := new(big.Int).SetString("1166812191093119704612", 10)
		if !ok {
			panic("invalid economy baseline burned value")
		}
		return v
	}()
	economyBaselineBurnedTransfersWei = new(big.Int).Set(economyBaselineBurnedWei)
	economyBaselineBurnedGasWei       = new(big.Int)
	economyBaselineMinerBurnShareWei  = func() *big.Int {
		v, ok := new(big.Int).SetString("2924341331060450388", 10)
		if !ok {
			panic("invalid economy baseline miner burn share value")
		}
		return v
	}()
	economyBaselineDividendsMintedWei = new(big.Int)
)

func EconomyBaselineBurnedWei() *big.Int {
	return new(big.Int).Set(economyBaselineBurnedWei)
}

func EconomyBaselineBurnedTransfersWei() *big.Int {
	return new(big.Int).Set(economyBaselineBurnedTransfersWei)
}

func EconomyBaselineBurnedGasWei() *big.Int {
	return new(big.Int).Set(economyBaselineBurnedGasWei)
}

func EconomyBaselineMinerBurnShareWei() *big.Int {
	return new(big.Int).Set(economyBaselineMinerBurnShareWei)
}

func EconomyBaselineDividendsMintedWei() *big.Int {
	return new(big.Int).Set(economyBaselineDividendsMintedWei)
}
