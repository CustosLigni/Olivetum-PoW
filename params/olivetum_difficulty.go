package params

import "math/big"

// Default parameters for the Olivetum difficulty fork and post-fork tuning.
//
// One hard fork is expected on mainnet; adjust the values below only if you
// intentionally plan another fork and rebuild binaries before reaching it.
var (
	// Height at which the post-fork difficulty rules activate.
	difficultyForkBlock = big.NewInt(57900)
	// Height at which live gap-drop (ceil) activates for stuck blocks (nil = disabled).
	difficultyLiveDropForkBlock *big.Int
	// Height at which ETC-style difficulty + step drop (every 60s) activates.
	difficultyEtcForkBlock = big.NewInt(61500)
	// Height at which the milder ETC-style step drop (post-ETC) activates.
	difficultyEtcStepForkBlock = big.NewInt(76000)

	// Post-fork clamps: max increase per block is *num/den, max decrease is /div.
	difficultyPostForkMaxIncreaseNum uint64 = 3
	difficultyPostForkMaxIncreaseDen uint64 = 2 // 1.5x
	difficultyPostForkMaxDecreaseDiv uint64 = 4096

	// Post-fork minimum timestamp increment as a fraction of the target period.
	// 1/6 keeps the legacy ~3s minimum when target is 15s.
	difficultyMinTimestampNum uint64 = 1
	difficultyMinTimestampDen uint64 = 6

	// Emergency gap drop: if no block for gapSeconds, difficulty is divided by
	// roughly (timeDiff / gapSeconds), capped by difficultyGapDropMaxDivisor.
	difficultyGapDropSeconds    uint64 = 60
	difficultyGapDropMaxDivisor uint64 = 65536

	// Step-drop tuning for the new fork: first drop after startSeconds, then
	// every intervalSeconds reduce difficulty by dropBps (basis points), capped
	// by maxDropBps for a single block.
	difficultyStepDropStartSeconds    uint64 = 120
	difficultyStepDropIntervalSeconds uint64 = 60
	difficultyStepDropBps             uint64 = 200  // 2% per step
	difficultyStepDropMaxBps          uint64 = 5000 // max 50% drop on one block
)

// Get/Set helpers for fork height.
func GetDifficultyForkBlock() *big.Int {
	if difficultyForkBlock == nil {
		return nil
	}
	return new(big.Int).Set(difficultyForkBlock)
}

func SetDifficultyForkBlock(block *big.Int) {
	if block == nil {
		difficultyForkBlock = nil
		return
	}
	difficultyForkBlock = new(big.Int).Set(block)
}

func IsAfterDifficultyFork(number uint64) bool {
	if difficultyForkBlock == nil {
		return false
	}
	if difficultyForkBlock.Sign() == 0 {
		return true
	}
	return number >= difficultyForkBlock.Uint64()
}

// Live drop fork helpers.
func GetDifficultyLiveDropForkBlock() *big.Int {
	if difficultyLiveDropForkBlock == nil {
		return nil
	}
	return new(big.Int).Set(difficultyLiveDropForkBlock)
}

func SetDifficultyLiveDropForkBlock(block *big.Int) {
	if block == nil {
		difficultyLiveDropForkBlock = nil
		return
	}
	difficultyLiveDropForkBlock = new(big.Int).Set(block)
}

func IsAfterDifficultyLiveDropFork(number uint64) bool {
	if difficultyLiveDropForkBlock == nil {
		return false
	}
	if difficultyLiveDropForkBlock.Sign() == 0 {
		return true
	}
	return number >= difficultyLiveDropForkBlock.Uint64()
}

// ETC-style fork helpers.
func GetDifficultyEtcForkBlock() *big.Int {
	if difficultyEtcForkBlock == nil {
		return nil
	}
	return new(big.Int).Set(difficultyEtcForkBlock)
}

func SetDifficultyEtcForkBlock(block *big.Int) {
	if block == nil {
		difficultyEtcForkBlock = nil
		return
	}
	difficultyEtcForkBlock = new(big.Int).Set(block)
}

func IsAfterDifficultyEtcFork(number uint64) bool {
	if difficultyEtcForkBlock == nil {
		return false
	}
	if difficultyEtcForkBlock.Sign() == 0 {
		return true
	}
	return number >= difficultyEtcForkBlock.Uint64()
}

// ETC-style (milder) fork helpers.
func GetDifficultyEtcStepForkBlock() *big.Int {
	if difficultyEtcStepForkBlock == nil {
		return nil
	}
	return new(big.Int).Set(difficultyEtcStepForkBlock)
}

func SetDifficultyEtcStepForkBlock(block *big.Int) {
	if block == nil {
		difficultyEtcStepForkBlock = nil
		return
	}
	difficultyEtcStepForkBlock = new(big.Int).Set(block)
}

func IsAfterDifficultyEtcStepFork(number uint64) bool {
	if difficultyEtcStepForkBlock == nil {
		return false
	}
	if difficultyEtcStepForkBlock.Sign() == 0 {
		return true
	}
	return number >= difficultyEtcStepForkBlock.Uint64()
}

// Post-fork clamps.
func GetPostForkDifficultyClamps() (incNum, incDen, decDiv uint64) {
	return difficultyPostForkMaxIncreaseNum, difficultyPostForkMaxIncreaseDen, difficultyPostForkMaxDecreaseDiv
}

func SetPostForkDifficultyClamps(incNum, incDen, decDiv uint64) {
	if incNum > 0 {
		difficultyPostForkMaxIncreaseNum = incNum
	}
	if incDen > 0 {
		difficultyPostForkMaxIncreaseDen = incDen
	}
	if decDiv > 0 {
		difficultyPostForkMaxDecreaseDiv = decDiv
	}
}

// Timestamp fraction used after the fork.
func GetPostForkTimestampFraction() (num, den uint64) {
	return difficultyMinTimestampNum, difficultyMinTimestampDen
}

func SetPostForkTimestampFraction(num, den uint64) {
	if num > 0 {
		difficultyMinTimestampNum = num
	}
	if den > 0 {
		difficultyMinTimestampDen = den
	}
}

// Gap-drop configuration.
func GetDifficultyGapDrop() (seconds, maxDiv uint64) {
	return difficultyGapDropSeconds, difficultyGapDropMaxDivisor
}

func SetDifficultyGapDrop(seconds, maxDiv uint64) {
	difficultyGapDropSeconds = seconds
	if maxDiv > 0 {
		difficultyGapDropMaxDivisor = maxDiv
	}
}

// Step-drop configuration for the post-ETC fork.
func GetDifficultyStepDrop() (startSeconds, intervalSeconds, dropBps, maxDropBps uint64) {
	return difficultyStepDropStartSeconds, difficultyStepDropIntervalSeconds, difficultyStepDropBps, difficultyStepDropMaxBps
}

func SetDifficultyStepDrop(startSeconds, intervalSeconds, dropBps, maxDropBps uint64) {
	if startSeconds > 0 {
		difficultyStepDropStartSeconds = startSeconds
	}
	if intervalSeconds > 0 {
		difficultyStepDropIntervalSeconds = intervalSeconds
	}
	if dropBps > 0 {
		difficultyStepDropBps = dropBps
	}
	if maxDropBps > 0 {
		difficultyStepDropMaxBps = maxDropBps
	}
}
