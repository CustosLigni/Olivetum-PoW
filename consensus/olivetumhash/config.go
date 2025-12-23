package olivetumhash

import "github.com/ethereum/go-ethereum/params"

const (
	// defaultEpochLength targets ~2 days at 15s blocks.
	defaultEpochLength        = 11520
	defaultDatasetInitBytes   = 32 * 1024 * 1024
	defaultDatasetGrowthBytes = 2 * 1024 * 1024
	defaultMixRounds          = 64
	maxCachedDatasets         = 2
)

type engineConfig struct {
	epochLength        uint64
	datasetInitBytes   uint64
	datasetGrowthBytes uint64
	mixRounds          uint64
}

func resolveConfig(cfg *params.OlivetumhashConfig) engineConfig {
	conf := engineConfig{
		epochLength:        defaultEpochLength,
		datasetInitBytes:   defaultDatasetInitBytes,
		datasetGrowthBytes: defaultDatasetGrowthBytes,
		mixRounds:          defaultMixRounds,
	}
	if cfg != nil {
		if cfg.EpochLength != 0 {
			conf.epochLength = cfg.EpochLength
		}
		if cfg.DatasetInitBytes != 0 {
			conf.datasetInitBytes = cfg.DatasetInitBytes
		}
		if cfg.DatasetGrowthBytes != 0 {
			conf.datasetGrowthBytes = cfg.DatasetGrowthBytes
		}
		if cfg.MixRounds != 0 {
			conf.mixRounds = cfg.MixRounds
		}
	}

	// Ensure all byte-sizes are multiples of 64 so dataset slices align with 64-byte chunks.
	conf.datasetInitBytes = align64(conf.datasetInitBytes)
	conf.datasetGrowthBytes = align64(conf.datasetGrowthBytes)
	if conf.datasetInitBytes == 0 {
		conf.datasetInitBytes = align64(defaultDatasetInitBytes)
	}
	if conf.datasetGrowthBytes == 0 {
		conf.datasetGrowthBytes = align64(defaultDatasetGrowthBytes)
	}
	if conf.epochLength == 0 {
		conf.epochLength = defaultEpochLength
	}
	if conf.mixRounds < 16 {
		conf.mixRounds = 16
	}
	return conf
}

func align64(value uint64) uint64 {
	if value%64 == 0 {
		return value
	}
	return (value/64 + 1) * 64
}
