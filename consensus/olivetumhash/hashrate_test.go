package olivetumhash

import "testing"

// Ensure the hashrate sampler records delta hashes between samples.
func TestHashrateSampler(t *testing.T) {
	engine := New(nil)
	engine.totalHashes.Store(100)
	engine.sampleHashrate()
	if got := engine.hashrate.Load(); got != 100 {
		t.Fatalf("hashrate after first sample, got %d want 100", got)
	}
	engine.totalHashes.Store(250)
	engine.sampleHashrate()
	if got := engine.hashrate.Load(); got != 150 {
		t.Fatalf("hashrate after second sample, got %d want 150", got)
	}
}
