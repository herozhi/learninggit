package stats

import "runtime"

// GCPauseNs cals max(set(new.PauseNs) - set(old.PauseNs))
func GCPauseNs(new runtime.MemStats, old runtime.MemStats) uint64 {
	if new.NumGC <= old.NumGC {
		return new.PauseNs[(new.NumGC+255)%256]
	}
	n := new.NumGC - old.NumGC
	if n > 256 {
		n = 256
	}
	// max PauseNs since last GC
	var maxPauseNs uint64
	for i := uint32(0); i < n; i++ {
		if pauseNs := new.PauseNs[(new.NumGC-i+255)%256]; pauseNs > maxPauseNs {
			maxPauseNs = pauseNs
		}
	}
	return maxPauseNs
}
