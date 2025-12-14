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
	jobs      []*PrintJob
	mu        sync.Mutex
	pool      *ConnectionPool
	manager   *Manager
	maxRetries int
	ctx       context.Context
	cancel    context.CancelFunc
	wg        sync.WaitGroup
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
	
	// Find next queued job
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
	
	// Attempt to print
	err := q.printJob(job)
	
	q.mu.Lock()
	defer q.mu.Unlock()
	
	if err != nil {
		job.Retries++
		job.Error = err
		
		if job.Retries >= q.maxRetries {
			job.Status = "failed"
			fmt.Printf("❌ Print job %s failed after %d retries: %v\n", job.ID, job.Retries, err)
		} else {
			job.Status = "queued" // Retry
			fmt.Printf("⚠️  Print job %s failed, retrying (%d/%d): %v\n", 
				job.ID, job.Retries, q.maxRetries, err)
			time.Sleep(time.Second) // Brief delay before retry
		}
	} else {
		job.Status = "completed"
		fmt.Printf("✅ Print job %s completed\n", job.ID)
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
	
	// Print
	return q.pool.Print(job.PrinterID, job.Image)
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
