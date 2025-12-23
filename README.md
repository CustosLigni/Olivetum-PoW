## Olivetum (CoreGeth-based PoW)

This repository contains the Go implementation of the Olivetum node. Olivetum is a Proof-of-Work (Olivetumhash) blockchain. The native currency is Olivo (18 decimals). This codebase is production-ready.

### Quick links
- Binary: `geth` (Olivetum node)
- Genesis / chain config: `olivetum_pow_genesis.json`
- Module system: Go modules (vendored deps not included by default)

### Requirements
- Go 1.21 or newer (see `go.mod`)
- Git and a C toolchain for CGO packages
- Supported: Linux, macOS, Windows

### Build from source
- Using Makefile (recommended):
  - `make geth` builds `build/bin/geth`
  - `make all` builds all tools
- Directly via helper script (used in Dockerfile):
  - `go run build/ci.go install -static ./cmd/geth`

### Docker image
- Build: `docker build -t olivetum/geth .`
- Run: `docker run --rm -it olivetum/geth --help`

### Initialize and run a node
- Initialize a data directory with the Olivetum genesis:  
  `build/bin/geth init --datadir /path/to/data olivetum_pow_genesis.json`
- Start the node (adjust flags as needed):  
  `build/bin/geth --datadir /path/to/data --http`

### Notes
- Contract creation is disabled by consensus; only value transfers and built-in management logic are permitted.
- Gas is accounted but transactions are accepted with `gasPrice=0`.
- Chain settings (including chain ID 30216931) are defined in `olivetum_pow_genesis.json`.

### Contributing and issues
Please open issues and pull requests in this repository. Standard Go formatting and linting rules apply.

### License
See `COPYING` and `COPYING.LESSER`.
