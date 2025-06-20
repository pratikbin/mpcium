package tsslimiter

import (
	"sync"
	"time"

	"github.com/fystack/mpcium/pkg/logger"
)

type SessionType int

const (
	SessionKeygenECDSA SessionType = iota
	SessionReshareECDSA
	SessionSignECDSA
	SessionKeygenEDDSA
	SessionReshareEDDSA
	SessionSignEDDSA
	SessionKeygenCombined
)

// sessionCosts defines the estimated CPU cost (in points) of each session type.
// The values are based on practical benchmarks using tss-lib (ECDSA over secp256k1),
// where 100 points = 100% of a physical CPU core.
//
// These costs allow us to model CPU pressure and prevent overload by setting
// a total max budget equal to the number of physical cores × 100 points.
//
// For example, on a 4-core CPU:
//
//   - maxPoints = 400
//
//   - You could run 1 keygen (100) + 10 sign sessions (30 × 10 = 300)
//
//   - Or 4 resharing sessions (80 × 4 = 320) + 2 sign sessions (30 × 2 = 60)
//
//     Note: These values are conservative to maintain low latency and avoid timeouts.
var sessionCosts = map[SessionType]int{
	SessionKeygenECDSA:    75, // Full core
	SessionReshareECDSA:   70,
	SessionSignECDSA:      40,
	SessionKeygenEDDSA:    25, // ~25% of core
	SessionReshareEDDSA:   20,
	SessionSignEDDSA:      15,
	SessionKeygenCombined: 100, // ECDSA (100) + EDDSA (25)
}

type Limiter interface {
	// TryAcquire attempts to acquire resources for the given session type.
	// Returns true if successful, false otherwise.
	TryAcquire(t SessionType) bool

	// Acquire blocks until it successfully acquires resources for the session type.
	Acquire(t SessionType)

	// Release frees the resources for the given session type.
	Release(t SessionType)
	Stats() (int, int)
}

type WeightedLimiter struct {
	mu         sync.Mutex
	usedPoints int
	maxPoints  int
}

// NewWeightedLimiter creates a limiter with maxPoints = maxSessionsAllowed * 100
func NewWeightedLimiter(maxSessions int) *WeightedLimiter {
	return &WeightedLimiter{
		maxPoints: maxSessions * 100,
	}
}

func (l *WeightedLimiter) TryAcquire(t SessionType) bool {
	l.mu.Lock()
	defer l.mu.Unlock()

	logger.Info("TryAcquire....", "sessionType", t, "usedPoints", l.usedPoints, "maxPoints", l.maxPoints)
	cost := sessionCosts[t]
	if l.usedPoints+cost > l.maxPoints {
		return false
	}

	logger.Info("DOneACQUIRE")
	l.usedPoints += cost
	return true
}

func (l *WeightedLimiter) Acquire(t SessionType) {
	cost := sessionCosts[t]

	for {
		l.mu.Lock()
		if l.usedPoints+cost <= l.maxPoints {
			l.usedPoints += cost
			l.mu.Unlock()
			return
		}
		l.mu.Unlock()
		time.Sleep(50 * time.Millisecond) // backoff
	}
}

func (l *WeightedLimiter) Release(t SessionType) {
	l.mu.Lock()
	defer l.mu.Unlock()

	cost := sessionCosts[t]
	l.usedPoints -= cost
	if l.usedPoints < 0 {
		l.usedPoints = 0
	}
	logger.Info("Release", "sessionType", t, "usedPoints", l.usedPoints, "maxPoints", l.maxPoints)
}

func (l *WeightedLimiter) Stats() (int, int) {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.usedPoints, l.maxPoints
}
