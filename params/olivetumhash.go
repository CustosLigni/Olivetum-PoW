package params

import "github.com/ethereum/go-ethereum/params/types/ctypes"

// OlivetumhashConfig re-exports the consensus configuration used by the
// ChainConfig types so other packages (e.g. consensus/olivetumhash) can refer
// to it through the canonical params namespace.
type OlivetumhashConfig = ctypes.OlivetumhashConfig
