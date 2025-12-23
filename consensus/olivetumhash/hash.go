package olivetumhash

import (
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"math"
	"math/bits"
	"os"
	"path/filepath"
	"runtime"
	"sync"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/log"
	"golang.org/x/crypto/sha3"
)

const cacheEnvVar = "OLIVETUMHASH_CACHE_DIR"

var errDatasetCacheDisabled = errors.New("olivetumhash: dataset cache disabled")

type dataset struct {
	epoch uint64
	data  []byte
}

func defaultCacheDir() string {
	if custom := os.Getenv(cacheEnvVar); custom != "" {
		return custom
	}
	dir, err := os.UserCacheDir()
	if err != nil {
		return ""
	}
	return filepath.Join(dir, "olivetum", "olivetumhash")
}

func (e *Olivetumhash) dataset(epoch uint64) []byte {
	e.datasetLock.RLock()
	if ds, ok := e.datasets[epoch]; ok {
		data := ds.data
		e.datasetLock.RUnlock()
		return data
	}
	e.datasetLock.RUnlock()

	size := datasetSize(epoch, e.config)
	cachePath := ""
	if e.cacheDir != "" {
		cachePath = e.cacheFilePath(epoch, size)
	}
	loadStart := time.Now()
	if data, err := e.tryLoadDatasetFromDisk(epoch, size); err == nil {
		if cachePath != "" {
			log.Info("Loaded olivetumhash dataset from cache", "epoch", epoch, "size", common.StorageSize(size), "path", cachePath, "elapsed", common.PrettyDuration(time.Since(loadStart)))
		} else {
			log.Info("Loaded olivetumhash dataset from cache", "epoch", epoch, "size", common.StorageSize(size), "elapsed", common.PrettyDuration(time.Since(loadStart)))
		}
		e.datasetLock.Lock()
		data = e.installDatasetLocked(epoch, data)
		e.datasetLock.Unlock()
		return data
	} else if err != nil && !errors.Is(err, errDatasetCacheDisabled) && !errors.Is(err, os.ErrNotExist) {
		log.Warn("Failed to load olivetumhash dataset from cache", "epoch", epoch, "err", err)
	}

	log.Info("Building olivetumhash dataset", "epoch", epoch, "size", common.StorageSize(size))
	buildStart := time.Now()
	data := buildDataset(epoch, e.config)
	log.Info("Generated olivetumhash dataset", "epoch", epoch, "size", common.StorageSize(size), "elapsed", common.PrettyDuration(time.Since(buildStart)))

	if err := e.persistDataset(epoch, data); err != nil && !errors.Is(err, errDatasetCacheDisabled) {
		log.Warn("Failed to persist olivetumhash dataset", "epoch", epoch, "err", err)
	} else if cachePath != "" && err == nil {
		log.Info("Stored olivetumhash dataset in cache", "epoch", epoch, "size", common.StorageSize(size), "path", cachePath, "elapsed", common.PrettyDuration(time.Since(buildStart)))
	}

	e.datasetLock.Lock()
	defer e.datasetLock.Unlock()
	return e.installDatasetLocked(epoch, data)
}

func buildDataset(epoch uint64, cfg engineConfig) []byte {
	size := datasetSize(epoch, cfg)
	data, err := makeDatasetBuffer(size)
	if err != nil {
		panic(err)
	}
	chunkCount := len(data) / 64

	seed := make([]byte, 32)
	binary.LittleEndian.PutUint64(seed[:8], epoch)
	copy(seed[8:], []byte("OlivetumhashDatasetSeed.........."))

	// Fill base chunks in parallel; each chunk uses only its index-derived seed.
	workers := runtime.NumCPU()
	if workers < 1 {
		workers = 1
	}
	var wg sync.WaitGroup
	wg.Add(workers)
	for w := 0; w < workers; w++ {
		w := w
		go func() {
			defer wg.Done()
			localSeed := make([]byte, len(seed))
			copy(localSeed, seed)
			var tmp [64]byte
			h512 := sha3.NewLegacyKeccak512()
			for i := w; i < chunkCount; i += workers {
				h512.Reset()
				binary.LittleEndian.PutUint64(localSeed[16:24], uint64(i))
				h512.Write(localSeed)
				sum := h512.Sum(nil)
				copy(data[i*64:(i+1)*64], sum)
				copy(tmp[:], sum)
			}
		}()
	}
	wg.Wait()

	// Three rounds of cross-mixing enforce additional memory hardness.
	var tmp [64]byte
	h512 := sha3.NewLegacyKeccak512()
	for round := 0; round < 3; round++ {
		for i := 0; i < chunkCount; i++ {
			target := (i + (round+1)*17) % chunkCount
			base := data[i*64 : (i+1)*64]
			ref := data[target*64 : (target+1)*64]
			for j := 0; j < 64; j++ {
				tmp[j] = base[j] ^ ref[j]
			}
			h512.Reset()
			h512.Write(tmp[:])
			binary.LittleEndian.PutUint64(seed[16:24], uint64(i))
			binary.LittleEndian.PutUint64(seed[24:32], uint64(round))
			h512.Write(seed[16:32])
			copy(base, h512.Sum(nil))
		}
	}
	return data
}

func datasetSize(epoch uint64, cfg engineConfig) uint64 {
	size := cfg.datasetInitBytes + cfg.datasetGrowthBytes*epoch
	if size < 64 {
		size = 64
	}
	return align64(size)
}

func makeDatasetBuffer(size uint64) ([]byte, error) {
	if size > uint64(math.MaxInt) {
		return nil, fmt.Errorf("olivetumhash: dataset too large (%d bytes)", size)
	}
	return make([]byte, int(size)), nil
}

func (e *Olivetumhash) installDatasetLocked(epoch uint64, data []byte) []byte {
	if existing, ok := e.datasets[epoch]; ok {
		return existing.data
	}
	if e.datasets == nil {
		e.datasets = make(map[uint64]*dataset)
	}
	e.datasets[epoch] = &dataset{epoch: epoch, data: data}
	e.evictOldDatasetsLocked(epoch)
	return data
}

func (e *Olivetumhash) evictOldDatasetsLocked(current uint64) {
	if len(e.datasets) <= maxCachedDatasets {
		return
	}
	var oldest uint64
	var hasOldest bool
	for k := range e.datasets {
		if !hasOldest || k < oldest {
			oldest = k
			hasOldest = true
		}
	}
	if hasOldest && oldest != current {
		delete(e.datasets, oldest)
	}
}

func (e *Olivetumhash) tryLoadDatasetFromDisk(epoch, size uint64) ([]byte, error) {
	if e.cacheDir == "" {
		return nil, errDatasetCacheDisabled
	}
	path := e.cacheFilePath(epoch, size)
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}

	info, err := file.Stat()
	if err != nil {
		file.Close()
		_ = os.Remove(path)
		return nil, err
	}
	if uint64(info.Size()) != size {
		file.Close()
		_ = os.Remove(path)
		return nil, fmt.Errorf("olivetumhash: cache size mismatch (%d != %d)", info.Size(), size)
	}

	data, err := makeDatasetBuffer(size)
	if err != nil {
		file.Close()
		return nil, err
	}
	if _, err := io.ReadFull(file, data); err != nil {
		file.Close()
		_ = os.Remove(path)
		return nil, err
	}
	if err := file.Close(); err != nil {
		return nil, err
	}
	return data, nil
}

func (e *Olivetumhash) persistDataset(epoch uint64, data []byte) error {
	if e.cacheDir == "" {
		return errDatasetCacheDisabled
	}
	size := uint64(len(data))
	if err := os.MkdirAll(e.cacheDir, 0o755); err != nil {
		return err
	}
	path := e.cacheFilePath(epoch, size)

	tmp, err := os.CreateTemp(e.cacheDir, fmt.Sprintf("epoch-%06d-*.tmp", epoch))
	if err != nil {
		return err
	}
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		_ = os.Remove(tmp.Name())
		return err
	}
	if err := tmp.Sync(); err != nil {
		tmp.Close()
		_ = os.Remove(tmp.Name())
		return err
	}
	if err := tmp.Chmod(0o644); err != nil {
		tmp.Close()
		_ = os.Remove(tmp.Name())
		return err
	}
	if err := tmp.Close(); err != nil {
		_ = os.Remove(tmp.Name())
		return err
	}
	if err := os.Rename(tmp.Name(), path); err != nil {
		_ = os.Remove(path)
		if err := os.Rename(tmp.Name(), path); err != nil {
			_ = os.Remove(tmp.Name())
			return err
		}
		return nil
	}
	return nil
}

func (e *Olivetumhash) cacheFilePath(epoch, size uint64) string {
	filename := fmt.Sprintf("epoch-%06d-%d.dat", epoch, size)
	return filepath.Join(e.cacheDir, filename)
}

func (e *Olivetumhash) computeSeal(headerHash common.Hash, nonce types.BlockNonce, epoch uint64) (common.Hash, common.Hash) {
	data := e.datasetWithPrefetch(epoch)
	mix, digest := oliveMix(headerHash, nonce, data, e.config.mixRounds)
	return mix, digest
}

// datasetWithPrefetch returns the dataset for the given epoch and kicks off a
// background build of the next epoch to smooth over epoch transitions.
func (e *Olivetumhash) datasetWithPrefetch(epoch uint64) []byte {
	data := e.dataset(epoch)
	e.prefetchDataset(epoch + 1)
	return data
}

// prefetchDataset builds the dataset for the given epoch in the background if
// it is not already cached or being prefetched.
func (e *Olivetumhash) prefetchDataset(epoch uint64) {
	e.datasetLock.Lock()
	if _, ok := e.datasets[epoch]; ok {
		e.datasetLock.Unlock()
		return
	}
	if e.prefetching == nil {
		e.prefetching = make(map[uint64]struct{})
	}
	if _, ok := e.prefetching[epoch]; ok {
		e.datasetLock.Unlock()
		return
	}
	e.prefetching[epoch] = struct{}{}
	log.Info("Prefetching olivetumhash dataset", "epoch", epoch, "role", "next")
	e.datasetLock.Unlock()

	go func() {
		data := e.dataset(epoch)
		log.Info("Finished olivetumhash dataset prefetch", "epoch", epoch, "size", common.StorageSize(len(data)))
		e.datasetLock.Lock()
		delete(e.prefetching, epoch)
		e.datasetLock.Unlock()
	}()
}

var mixPool = sync.Pool{
	New: func() interface{} {
		buf := make([]byte, 64)
		return &buf
	},
}

func oliveMix(headerHash common.Hash, nonce types.BlockNonce, dataset []byte, rounds uint64) (common.Hash, common.Hash) {
	h512 := sha3.NewLegacyKeccak512()
	var seed [40]byte
	copy(seed[:32], headerHash[:])
	copy(seed[32:], nonce[:])
	h512.Write(seed[:])
	initial := h512.Sum(nil)
	h512.Reset()

	mixPtr := mixPool.Get().(*[]byte)
	defer mixPool.Put(mixPtr)
	mix := *mixPtr
	copy(mix, initial)

	var mixWords [8]uint64
	for i := 0; i < 8; i++ {
		mixWords[i] = binary.LittleEndian.Uint64(mix[i*8 : (i+1)*8])
	}

	chunkCount := uint64(len(dataset) / 64)
	if chunkCount == 0 {
		chunkCount = 1
	}

	// Precompute a pseudo-random operation schedule derived from the header/nonce.
	progHasher := sha3.NewLegacyKeccak512()
	progHasher.Write(headerHash[:])
	progHasher.Write(nonce[:])
	progBytes := progHasher.Sum(nil)
	progHasher.Reset()
	progHasher.Write(progBytes)
	progHasher.Write(headerHash[:])
	progHasher.Write(nonce[:])
	progBytes = append(progBytes, progHasher.Sum(nil)...)
	program := make([]uint64, len(progBytes)/8)
	for i := range program {
		program[i] = binary.LittleEndian.Uint64(progBytes[i*8 : (i+1)*8])
	}
	if len(program) == 0 {
		program = []uint64{0}
	}

	// dynamicSalt and periodic program refresh increase cross-round coupling,
	// making the access pattern harder to pipeline in hardware.
	dynamicSalt := binary.LittleEndian.Uint64(headerHash[8:16]) ^ binary.LittleEndian.Uint64(nonce[:8])
	const refreshInterval = 8

	for i := uint64(0); i < rounds; i++ {
		if refreshInterval != 0 && i != 0 && i%refreshInterval == 0 {
			h512.Reset()
			h512.Write(mix)
			h512.Write(headerHash[:])
			h512.Write(nonce[:])
			sum := h512.Sum(nil)
			for j := range program {
				off := (j * 8) % len(sum)
				word := binary.LittleEndian.Uint64(sum[off : off+8])
				program[j] ^= word
			}
			dynamicSalt ^= binary.LittleEndian.Uint64(sum[:8])
		}

		progWord := program[i%uint64(len(program))] ^ (i * 0x9e3779b97f4a7c15)
		sourceLane := int((progWord >> 5) & 7)
		rotateAmt := int(progWord&63) + 1

		index := mixWords[sourceLane] ^ progWord ^ binary.LittleEndian.Uint64(headerHash[0:8])
		index ^= (i + uint64(sourceLane)) * 0x517cc1b727220a95
		chunkOffset := (index % chunkCount) * 64
		chunk := dataset[chunkOffset : chunkOffset+64]

		var chunkWords [8]uint64
		for j := 0; j < 8; j++ {
			chunkWords[j] = binary.LittleEndian.Uint64(chunk[j*8 : (j+1)*8])
		}

		// Second, differently indexed read to raise memory bandwidth pressure.
		index2 := mixWords[(sourceLane+3)&7] ^ progWord ^ dynamicSalt ^ (bits.RotateLeft64(uint64(i), sourceLane) & 0xffff)
		index2 ^= binary.LittleEndian.Uint64(headerHash[16:24])
		index2 ^= (i + uint64(sourceLane*3+1)) * 0x94d049bb133111eb
		chunkOffset2 := (index2 % chunkCount) * 64
		chunk2 := dataset[chunkOffset2 : chunkOffset2+64]

		var chunkWords2 [8]uint64
		for j := 0; j < 8; j++ {
			chunkWords2[j] = binary.LittleEndian.Uint64(chunk2[j*8 : (j+1)*8])
		}

		// Third read, different stride, to further stress random access.
		index3 := mixWords[(sourceLane+5)&7] ^ dynamicSalt ^ progWord ^ binary.LittleEndian.Uint64(headerHash[24:32])
		index3 ^= (i * 0x2545f4914f6cdd1d) + uint64(sourceLane<<3)
		chunkOffset3 := (index3 % chunkCount) * 64
		chunk3 := dataset[chunkOffset3 : chunkOffset3+64]

		var chunkWords3 [8]uint64
		for j := 0; j < 8; j++ {
			chunkWords3[j] = binary.LittleEndian.Uint64(chunk3[j*8 : (j+1)*8])
		}

		for lane := 0; lane < 8; lane++ {
			data1 := chunkWords[(lane+sourceLane)&7]
			data2 := chunkWords2[(lane+(sourceLane^3))&7]
			data3 := chunkWords3[(lane+(sourceLane^5))&7]
			mixWords[lane] ^= data1 ^ data3
			mixWords[lane] = bits.RotateLeft64(mixWords[lane]+data1*0x9e3779b97f4a7c15+data2+data3*0x6a09e667f3bcc908, rotateAmt+(lane&7))
			mixWords[lane] ^= bits.RotateLeft64(progWord^dynamicSalt^data2^data3, lane+1)
		}

		// Evolve dynamicSalt using fresh mix state to perturb future indexing.
		dynamicSalt ^= mixWords[(sourceLane+1)&7] + mixWords[(sourceLane+2)&7]
		dynamicSalt = bits.RotateLeft64(dynamicSalt, rotateAmt&31)
	}

	for i := 0; i < 8; i++ {
		binary.LittleEndian.PutUint64(mix[i*8:(i+1)*8], mixWords[i])
	}

	h256 := sha3.NewLegacyKeccak256()
	h256.Write(mix)
	mixDigestBytes := h256.Sum(nil)

	h256.Reset()
	h256.Write(mixDigestBytes)
	h256.Write(headerHash[:])
	h256.Write(nonce[:])
	finalDigestBytes := h256.Sum(nil)

	var mixDigest, finalDigest common.Hash
	copy(mixDigest[:], mixDigestBytes)
	copy(finalDigest[:], finalDigestBytes)
	return mixDigest, finalDigest
}
