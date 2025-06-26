package concurrency

import (
	"runtime"
)

// GetVirtualCoreCount returns the number of logical CPUs (virtual cores) available on the system.
// This includes physical cores *and* hyperthreads.
func GetVirtualCoreCount() int {
	return runtime.NumCPU()
}

// GetTSSConcurrencyLimit returns the recommended maximum number of concurrent TSS sessions.
// It estimates the number of *physical* cores by dividing the virtual core count by 2,
// because each physical core typically has 2 logical threads due to hyperthreading.
//
// Threshold signing (e.g., ECDSA) is CPU-bound and does not benefit much from hyperthreads,
// so we limit concurrency based on physical core estimates.
func GetTSSConcurrencyLimit() int {
	logicalCores := GetVirtualCoreCount()

	// Estimate physical cores by dividing virtual CPUs by 2
	estimatedPhysicalCores := logicalCores / 2
	if estimatedPhysicalCores < 1 {
		estimatedPhysicalCores = 1 // always allow at least one session
	}

	return calculateAllowedSessions(estimatedPhysicalCores)
}

// calculateAllowedSessions maps physical core count to safe TSS concurrency limits.
// You can tune these thresholds depending on your latency and throughput requirements.
func calculateAllowedSessions(coreCount int) int {
	switch {
	case coreCount <= 2:
		return 1
	case coreCount <= 4:
		return 2
	case coreCount <= 8:
		return 3
	case coreCount <= 12:
		return 5
	case coreCount <= 16:
		return 6
	default:
		// For large systems, reserve some headroom for OS, logs, GC, etc.
		return coreCount / 2
	}
}
