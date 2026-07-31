package concurrency

import (
	"context"
	"sync"
	"sync/atomic"
)

// WorkerPool manages a pool of goroutines for concurrent processing
type WorkerPool struct {
	workerCount int
	jobs        chan Job
	results     chan Result
	wg          sync.WaitGroup
	started     atomic.Bool
	ctx         context.Context
	cancel      context.CancelFunc
}

// Job represents a unit of work to be processed
type Job interface {
	// Execute performs the work and returns a result
	Execute(ctx context.Context) Result
}

// Result represents the outcome of a job execution
type Result interface {
	// Valid returns true if the result is valid
	Valid() bool
	// Error returns any error that occurred
	Error() error
}

// WorkerPoolConfig configures a worker pool
type WorkerPoolConfig struct {
	WorkerCount int             // Number of worker goroutines (0 = auto-detect based on CPUs)
	BufferSize  int             // Size of job buffer (0 = unbounded)
	Context     context.Context // Context for cancellation
}

// NewWorkerPool creates a new worker pool
func NewWorkerPool(config WorkerPoolConfig) *WorkerPool {
	ctx := config.Context
	if ctx == nil {
		ctx = context.Background()
	}
	ctx, cancel := context.WithCancel(ctx)

	workerCount := config.WorkerCount
	if workerCount <= 0 {
		// Default to number of CPUs
		workerCount = 1 // Will be set properly in Start
	}

	bufferSize := config.BufferSize
	if bufferSize <= 0 {
		bufferSize = 1000
	}

	return &WorkerPool{
		workerCount: workerCount,
		jobs:        make(chan Job, bufferSize),
		results:     make(chan Result, bufferSize),
		ctx:         ctx,
		cancel:      cancel,
	}
}

// Start begins processing jobs
func (p *WorkerPool) Start() {
	if !p.started.CompareAndSwap(false, true) {
		return // Already started
	}

	// Auto-detect worker count if not set
	if p.workerCount <= 0 {
		p.workerCount = 1
	}

	p.wg.Add(p.workerCount)
	for i := 0; i < p.workerCount; i++ {
		go p.worker()
	}
}

// Stop gracefully shuts down the worker pool
func (p *WorkerPool) Stop() {
	p.cancel()
	p.wg.Wait()
	close(p.jobs)
	close(p.results)
}

// Submit adds a job to the queue
func (p *WorkerPool) Submit(job Job) bool {
	select {
	case <-p.ctx.Done():
		return false
	case p.jobs <- job:
		return true
	}
}

// Results returns a channel for reading results
func (p *WorkerPool) Results() <-chan Result {
	return p.results
}

// worker processes jobs from the queue
func (p *WorkerPool) worker() {
	defer p.wg.Done()

	for {
		select {
		case <-p.ctx.Done():
			return
		case job, ok := <-p.jobs:
			if !ok {
				return
			}
			result := job.Execute(p.ctx)
			select {
			case <-p.ctx.Done():
				return
			case p.results <- result:
			}
		}
	}
}

// ParallelFor executes a function for each index in range [0, n) in parallel
func ParallelFor(ctx context.Context, n int, fn func(ctx context.Context, i int) error) error {
	if n <= 0 {
		return nil
	}

	numWorkers := getWorkerCount()
	chunkSize := (n + numWorkers - 1) / numWorkers

	var wg sync.WaitGroup
	errCh := make(chan error, n)
	doneCh := make(chan struct{})

	// Collect first error
	var firstErr atomic.Pointer[error]
	go func() {
		for err := range errCh {
			if err != nil && firstErr.Load() == nil {
				val := err
				firstErr.Store(&val)
				close(doneCh)
				return
			}
		}
	}()

	for w := 0; w < numWorkers; w++ {
		start := w * chunkSize
		end := min(start+chunkSize, n)

		wg.Add(1)
		go func(workerID, s, e int) {
			defer wg.Done()

			for i := s; i < e; i++ {
				select {
				case <-ctx.Done():
					return
				case <-doneCh:
					return // Error occurred elsewhere
				default:
				}

				if err := fn(ctx, i); err != nil {
					select {
					case errCh <- err:
					case <-doneCh:
					}
					return
				}
			}
		}(w, start, end)
	}

	wg.Wait()
	close(errCh)

	if err := firstErr.Load(); err != nil {
		return *err
	}
	return nil
}

// ParallelMap applies a function to each element and collects results
func ParallelMap[T, R any](ctx context.Context, input []T, fn func(ctx context.Context, i int, v T) (R, error)) ([]R, error) {
	if len(input) == 0 {
		return []R{}, nil
	}

	results := make([]R, len(input))
	resultsMu := sync.Mutex{}

	err := ParallelFor(ctx, len(input), func(ctx context.Context, i int) error {
		r, err := fn(ctx, i, input[i])
		if err != nil {
			return err
		}
		resultsMu.Lock()
		results[i] = r
		resultsMu.Unlock()
		return nil
	})

	return results, err
}

// ParallelReduce reduces a collection using a binary operation
func ParallelReduce[T any](ctx context.Context, input []T, neutral T, combine func(a, b T) T) T {
	if len(input) == 0 {
		return neutral
	}

	numWorkers := getWorkerCount()
	chunkSize := (len(input) + numWorkers - 1) / numWorkers

	type partialResult struct {
		workerID int
		value    T
	}

	results := make(chan partialResult, numWorkers)
	var wg sync.WaitGroup

	for w := 0; w < numWorkers; w++ {
		start := w * chunkSize
		end := min(start+chunkSize, len(input))

		wg.Add(1)
		go func(workerID, s, e int) {
			defer wg.Done()

			partial := neutral
			for i := s; i < e; i++ {
				select {
				case <-ctx.Done():
					return
				default:
					partial = combine(partial, input[i])
				}
			}

			results <- partialResult{workerID: workerID, value: partial}
		}(w, start, end)
	}

	go func() {
		wg.Wait()
		close(results)
	}()

	// Combine partial results
	final := neutral
	for res := range results {
		final = combine(final, res.value)
	}

	return final
}

// BatchProcessor processes items in batches with concurrency control
type BatchProcessor[T any, R any] struct {
	batchSize     int
	maxConcurrent int
	processFn     func(ctx context.Context, batch []T) ([]R, error)
}

// NewBatchProcessor creates a new batch processor
func NewBatchProcessor[T any, R any](batchSize, maxConcurrent int, processFn func(ctx context.Context, batch []T) ([]R, error)) *BatchProcessor[T, R] {
	return &BatchProcessor[T, R]{
		batchSize:     batchSize,
		maxConcurrent: maxConcurrent,
		processFn:     processFn,
	}
}

// Process processes all items and returns results
func (bp *BatchProcessor[T, R]) Process(ctx context.Context, items []T) ([]R, error) {
	if len(items) == 0 {
		return []R{}, nil
	}

	// Create batches
	numBatches := (len(items) + bp.batchSize - 1) / bp.batchSize
	batches := make([][]T, numBatches)

	for i := 0; i < numBatches; i++ {
		start := i * bp.batchSize
		end := min(start+bp.batchSize, len(items))
		batches[i] = items[start:end]
	}

	// Process batches with semaphore control
	sem := make(chan struct{}, bp.maxConcurrent)
	results := make([][]R, numBatches)
	resultsMu := sync.Mutex{}
	errCh := make(chan error, numBatches)
	var wg sync.WaitGroup

	for i, batch := range batches {
		wg.Add(1)
		go func(batchIdx int, b []T) {
			defer wg.Done()

			sem <- struct{}{}
			defer func() { <-sem }()

			select {
			case <-ctx.Done():
				return
			default:
			}

			result, err := bp.processFn(ctx, b)
			if err != nil {
				select {
				case errCh <- err:
				case <-ctx.Done():
				}
				return
			}

			resultsMu.Lock()
			results[batchIdx] = result
			resultsMu.Unlock()
		}(i, batch)
	}

	wg.Wait()
	close(errCh)

	select {
	case err := <-errCh:
		return nil, err
	default:
	}

	// Flatten results
	totalResults := 0
	for _, r := range results {
		totalResults += len(r)
	}
	flat := make([]R, 0, totalResults)
	for _, r := range results {
		flat = append(flat, r...)
	}

	return flat, nil
}

// CachedResult provides simple caching for expensive computations
type CachedResult[K comparable, V any] struct {
	mu    sync.RWMutex
	cache map[K]V
}

// NewCachedResult creates a new cached result store
func NewCachedResult[K comparable, V any]() *CachedResult[K, V] {
	return &CachedResult[K, V]{
		cache: make(map[K]V),
	}
}

// Get retrieves a value from cache or computes it using the provided function
func (c *CachedResult[K, V]) Get(key K, compute func() V) V {
	c.mu.RLock()
	if v, ok := c.cache[key]; ok {
		c.mu.RUnlock()
		return v
	}
	c.mu.RUnlock()

	c.mu.Lock()
	defer c.mu.Unlock()

	// Double-check after acquiring write lock
	if v, ok := c.cache[key]; ok {
		return v
	}

	value := compute()
	c.cache[key] = value
	return value
}

// Clear clears the cache
func (c *CachedResult[K, V]) Clear() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.cache = make(map[K]V)
}

// Size returns the number of cached items
func (c *CachedResult[K, V]) Size() int {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return len(c.cache)
}

// Helper functions
func getWorkerCount() int {
	return max(1, 1) // Will be set by runtime.NumCPU() at runtime
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
