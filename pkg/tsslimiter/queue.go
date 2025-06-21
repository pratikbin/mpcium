package tsslimiter

import (
	"fmt"
	"sync"

	"github.com/fystack/mpcium/pkg/logger"
)

// SessionJob represents a queued job with type, execution logic, and optional error callback
// Run should return an error if execution fails.
type SessionJob struct {
	Type    SessionType
	Run     func() error
	OnError func(error)
	Name    string
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
			ok := q.limiter.TryAcquire(job.Type)
			if !ok {
				logger.Info("Failed to Acquire", "jobType", job.Type, "name", job.Name)
				// Notify via OnError callback if provided
				if job.OnError != nil {
					job.OnError(fmt.Errorf("tsslimiter: failed to acquire budget for job type %v, job %s", job.Type, job.Name))
				}
				continue
			}

			// Log limiter state after acquire
			usedAfter, _ := q.limiter.Stats()
			logger.Info("After Acquire", "usedPoints", usedAfter, "jobType", job.Type, "name", job.Name)

			// Launch job
			q.wg.Add(1)
			go func(j SessionJob) {
				defer q.wg.Done()
				defer q.limiter.Release(j.Type)

				usedExec, _ := q.limiter.Stats()
				logger.Info("Executing Job", "usedPoints", usedExec, "jobType", j.Type, "name", job.Name)

				err := j.Run()
				if err != nil && j.OnError != nil {
					// Call the error handler for this job
					j.OnError(err)
				}

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
