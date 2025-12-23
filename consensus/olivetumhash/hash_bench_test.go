package olivetumhash

import (
	"runtime"
	"testing"
)

var datasetSink []byte

func benchmarkBuildDataset(b *testing.B, epoch uint64) {
	cfg := engineConfig{
		epochLength:        defaultEpochLength,
		datasetInitBytes:   align64(16 * 1024 * 1024),
		datasetGrowthBytes: align64(2 * 1024 * 1024),
		mixRounds:          defaultMixRounds,
	}
	size := datasetSize(epoch, cfg)
	b.SetBytes(int64(size))
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		datasetSink = buildDataset(epoch, cfg)
		runtime.KeepAlive(datasetSink)
		datasetSink = nil
	}
}

func BenchmarkBuildDatasetEpoch0(b *testing.B)  { benchmarkBuildDataset(b, 0) }
func BenchmarkBuildDatasetEpoch16(b *testing.B) { benchmarkBuildDataset(b, 16) }
