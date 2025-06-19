package tsslimiter

import (
	"sync"

	"github.com/fystack/mpcium/pkg/logger"
)

// SessionJob represents a queued job with type and execution logic
type SessionJob struct {
	Type SessionType
	Run  func()
}

// Queue defines the interface for a job queue that manages TSS session jobs.
type Queue interface {
	// Enqueue adds a new session job to the queue for processing.
	Enqueue(job SessionJob)

	// Stop gracefully shuts down the queue and waits for background workers to finish.
	Stop()
}

// WeightedQueue buffers and processes session jobs using the WeightedLimiter
type WeightedQueue struct {
	queue    chan SessionJob
	limiter  *WeightedLimiter
	stopChan chan struct{}
	wg       sync.WaitGroup
}

// NewWeightedQueue initializes a buffered job queue
func NewWeightedQueue(limiter *WeightedLimiter, bufferSize int) *WeightedQueue {
	q := &WeightedQueue{
		queue:    make(chan SessionJob, bufferSize),
		limiter:  limiter,
		stopChan: make(chan struct{}),
	}

	// Start the background worker to process queue
	q.wg.Add(1)
	go q.run()
	return q
}

// Enqueue adds a job to the queue
func (q *WeightedQueue) Enqueue(job SessionJob) {
	q.queue <- job
}

// run continuously processes jobs based on limiter capacity, logging counters
func (q *WeightedQueue) run() {
	defer q.wg.Done()

	for {
		select {
		case job := <-q.queue:
			// Log queue length and limiter state before acquire
			usedBefore, max := q.limiter.Stats()
			logger.Info("Before Acquire", "usedPoints", usedBefore, "maxPoints", max, "pendingJobs", len(q.queue))

			// Block until we can acquire budget
			q.limiter.Acquire(job.Type)

			// Log limiter state after acquire
			usedAfter, _ := q.limiter.Stats()
			logger.Info("After Acquire", "usedPoints", usedAfter, "jobType", job.Type)

			// Launch job
			q.wg.Add(1)
			go func(j SessionJob) {
				defer q.wg.Done()
				defer q.limiter.Release(j.Type)

				usedExec, _ := q.limiter.Stats()
				logger.Info("Executing Job", "usedPoints", usedExec, "jobType", j.Type)
				j.Run()
				logger.Info("Pending Jobs", "num", len(q.queue))
			}(job)

		case <-q.stopChan:
			return
		}
	}
}

// Stop shuts down the queue processing loop and waits for running jobs
func (q *WeightedQueue) Stop() {
	close(q.stopChan)
	q.wg.Wait()
}
