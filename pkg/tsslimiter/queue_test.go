package tsslimiter_test

import (
	"sync/atomic"
	"testing"
	"time"

	"github.com/fystack/mpcium/pkg/tsslimiter"
	"github.com/stretchr/testify/assert"
)

func TestWeightedQueue_SingleJobExecution(t *testing.T) {
	limiter := tsslimiter.NewWeightedLimiter(2) // 2 cores = 200 points
	queue := tsslimiter.NewWeightedQueue(limiter, 10)
	defer queue.Stop()

	var executed int32 = 0

	job := tsslimiter.SessionJob{
		Type: tsslimiter.SessionSignECDSA,
		Run: func() {
			atomic.AddInt32(&executed, 1)
		},
	}

	queue.Enqueue(job)
	time.Sleep(200 * time.Millisecond) // Give time to process

	assert.Equal(t, int32(1), executed, "Expected job to execute")
}

func TestWeightedQueue_RespectsConcurrency(t *testing.T) {
	limiter := tsslimiter.NewWeightedLimiter(1) // 1 core = 100 points
	queue := tsslimiter.NewWeightedQueue(limiter, 10)
	defer queue.Stop()

	var executing int32 = 0
	var completed int32 = 0

	// 3 jobs each costing 100 (keygen) → only 1 should run at a time
	for i := 0; i < 3; i++ {
		queue.Enqueue(tsslimiter.SessionJob{
			Type: tsslimiter.SessionKeygenECDSA,
			Run: func() {
				current := atomic.AddInt32(&executing, 1)
				assert.LessOrEqual(t, current, int32(1), "Too many concurrent jobs running")
				time.Sleep(100 * time.Millisecond)
				atomic.AddInt32(&executing, -1)
				atomic.AddInt32(&completed, 1)
			},
		})
	}

	time.Sleep(500 * time.Millisecond)
	assert.Equal(t, int32(3), completed, "All jobs should complete sequentially")
}

func TestWeightedQueue_MixedSessions(t *testing.T) {
	limiter := tsslimiter.NewWeightedLimiter(2) // 2 cores = 200 points
	queue := tsslimiter.NewWeightedQueue(limiter, 10)
	defer queue.Stop()

	var completed int32 = 0

	// Sign (40) + Keygen (100) + Sign (40) = 180 total, fits under 200
	queue.Enqueue(tsslimiter.SessionJob{
		Type: tsslimiter.SessionSignECDSA,
		Run: func() {
			time.Sleep(50 * time.Millisecond)
			atomic.AddInt32(&completed, 1)
		},
	})
	queue.Enqueue(tsslimiter.SessionJob{
		Type: tsslimiter.SessionKeygenECDSA,
		Run: func() {
			time.Sleep(50 * time.Millisecond)
			atomic.AddInt32(&completed, 1)
		},
	})
	queue.Enqueue(tsslimiter.SessionJob{
		Type: tsslimiter.SessionSignECDSA,
		Run: func() {
			time.Sleep(50 * time.Millisecond)
			atomic.AddInt32(&completed, 1)
		},
	})

	time.Sleep(300 * time.Millisecond)
	assert.Equal(t, int32(3), completed, "All mixed jobs should run within capacity")
}

func TestWeightedQueue_BackpressureBuffering(t *testing.T) {
	limiter := tsslimiter.NewWeightedLimiter(1) // 1 core = 100
	queue := tsslimiter.NewWeightedQueue(limiter, 10)
	defer queue.Stop()

	var completed int32 = 0

	// First job blocks the CPU
	queue.Enqueue(tsslimiter.SessionJob{
		Type: tsslimiter.SessionKeygenECDSA,
		Run: func() {
			time.Sleep(150 * time.Millisecond)
			atomic.AddInt32(&completed, 1)
		},
	})

	// Second job should wait in the queue
	queue.Enqueue(tsslimiter.SessionJob{

		Type: tsslimiter.SessionSignECDSA,
		Run: func() {
			atomic.AddInt32(&completed, 1)
		},
	})

	time.Sleep(400 * time.Millisecond)
	assert.Equal(t, int32(2), completed, "Both jobs should run in sequence due to backpressure")
}
