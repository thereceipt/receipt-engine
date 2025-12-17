package printer

import (
	"context"
	"fmt"
	"image"
	"sync"
	"time"
)

// PrintJob represents a print job
type PrintJob struct {
	ID        string
	PrinterID string
	Image     image.Image
	Retries   int
	Status    string // queued, printing, failed, completed
	Error     error
	CreatedAt time.Time
}

// PrintQueue manages print jobs with retry logic
type PrintQueue struct {
	jobs       []*PrintJob
	mu         sync.Mutex
	pool       *ConnectionPool
	manager    *Manager
	maxRetries int
	ctx        context.Context
	cancel     context.CancelFunc
	wg         sync.WaitGroup
}

// NewPrintQueue creates a new print queue
func NewPrintQueue(pool *ConnectionPool, manager *Manager, maxRetries int) *PrintQueue {
	ctx, cancel := context.WithCancel(context.Background())

	q := &PrintQueue{
		jobs:       make([]*PrintJob, 0),
		pool:       pool,
		manager:    manager,
		maxRetries: maxRetries,
		ctx:        ctx,
		cancel:     cancel,
	}

	// Start worker
	q.wg.Add(1)
	go q.worker()

	return q
}

// Enqueue adds a print job to the queue
func (q *PrintQueue) Enqueue(printerID string, img image.Image) string {
	q.mu.Lock()
	defer q.mu.Unlock()

	job := &PrintJob{
		ID:        fmt.Sprintf("job_%d", time.Now().UnixNano()),
		PrinterID: printerID,
		Image:     img,
		Status:    "queued",
		CreatedAt: time.Now(),
	}

	q.jobs = append(q.jobs, job)

	return job.ID
}

// worker processes print jobs
func (q *PrintQueue) worker() {
	defer q.wg.Done()

	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-q.ctx.Done():
			return
		case <-ticker.C:
			q.processNextJob()
		}
	}
}

func (q *PrintQueue) processNextJob() {
	q.mu.Lock()

	// Find next queued job (skip jobs that are already printing, completed, or failed)
	var job *PrintJob
	for _, j := range q.jobs {
		if j.Status == "queued" {
			job = j
			job.Status = "printing"
			break
		}
	}

	q.mu.Unlock()

	if job == nil {
		return // No jobs to process
	}

	// Attempt to print (only once per job)
	err := q.printJob(job)

	q.mu.Lock()
	defer q.mu.Unlock()

	// Double-check job still exists and is still in printing status
	// (prevents race conditions and ensures we only process once)
	var foundJob *PrintJob
	for _, j := range q.jobs {
		if j.ID == job.ID {
			foundJob = j
			break
		}
	}

	if foundJob == nil || foundJob.Status != "printing" {
		// Job was removed or status changed, don't update
		return
	}

	if err != nil {
		foundJob.Retries++
		foundJob.Error = err

		if foundJob.Retries >= q.maxRetries {
			foundJob.Status = "failed"
			// Job failed - error is stored in job.Error, can be viewed in TUI
		} else {
			// Retry with delay - mark as queued but add a delay before it can be processed again
			foundJob.Status = "queued"
			// Job retrying - status is tracked in job, can be viewed in TUI
			// Don't sleep here - let the worker ticker handle timing
		}
	} else {
		// Success - mark as completed immediately
		foundJob.Status = "completed"
		// Job completed - status is tracked in job, can be viewed in TUI
	}
}

func (q *PrintQueue) printJob(job *PrintJob) error {
	// Ensure printer is connected
	if !q.pool.IsConnected(job.PrinterID) {
		printer := q.manager.GetPrinter(job.PrinterID)
		if printer == nil {
			return fmt.Errorf("printer not found: %s", job.PrinterID)
		}

		if err := q.pool.Connect(printer); err != nil {
			return fmt.Errorf("failed to connect to printer: %w", err)
		}
	}

	// Print exactly once - Print should be idempotent and atomic
	// If Print succeeds, it means data was sent once
	err := q.pool.Print(job.PrinterID, job.Image)
	if err != nil {
		return fmt.Errorf("print failed: %w", err)
	}

	// Success - return nil to mark job as completed
	return nil
}

// GetJob returns a job by ID
func (q *PrintQueue) GetJob(jobID string) *PrintJob {
	q.mu.Lock()
	defer q.mu.Unlock()

	for _, job := range q.jobs {
		if job.ID == jobID {
			// Return a copy
			jobCopy := *job
			return &jobCopy
		}
	}

	return nil
}

// GetAllJobs returns all jobs
func (q *PrintQueue) GetAllJobs() []*PrintJob {
	q.mu.Lock()
	defer q.mu.Unlock()

	// Return copies
	jobs := make([]*PrintJob, len(q.jobs))
	for i, job := range q.jobs {
		jobCopy := *job
		jobs[i] = &jobCopy
	}

	return jobs
}

// ClearCompleted removes completed jobs from the queue
func (q *PrintQueue) ClearCompleted() {
	q.mu.Lock()
	defer q.mu.Unlock()

	filtered := make([]*PrintJob, 0)
	for _, job := range q.jobs {
		if job.Status != "completed" {
			filtered = append(filtered, job)
		}
	}

	q.jobs = filtered
}

// Stop stops the print queue worker
func (q *PrintQueue) Stop() {
	q.cancel()
	q.wg.Wait()
}
