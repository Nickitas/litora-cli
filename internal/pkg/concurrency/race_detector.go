package concurrency

import (
	"context"
	"fmt"
	"runtime"
	"sync"
	"sync/atomic"
	"time"
)

// RaceDetector helps identify potential race conditions in concurrent code
type RaceDetector struct {
	mu              sync.Mutex
	detectedRaces   map[string][]RaceInfo
	enabled         atomic.Bool
	verbose         atomic.Bool
	stackTraceDepth int
}

// RaceInfo contains information about a detected race
type RaceInfo struct {
	Timestamp   time.Time
	GoroutineID int
	Operation   string
	Location    string
	StackTrace  string
}

// NewRaceDetector creates a new race detector
func NewRaceDetector(enabled bool) *RaceDetector {
	rd := &RaceDetector{
		detectedRaces:   make(map[string][]RaceInfo),
		stackTraceDepth: 4,
	}
	rd.enabled.Store(enabled)
	return rd
}

// Enable enables race detection
func (rd *RaceDetector) Enable() {
	rd.enabled.Store(true)
}

// Disable disables race detection
func (rd *RaceDetector) Disable() {
	rd.enabled.Store(false)
}

// SetVerbose sets verbose mode for detailed output
func (rd *RaceDetector) SetVerbose(v bool) {
	rd.verbose.Store(v)
}

// Detect checks for potential race conditions
func (rd *RaceDetector) Detect(location string, operation string) {
	if !rd.enabled.Load() {
		return
	}

	goroutineID := getGoroutineID()
	stackTrace := getStackTrace(rd.stackTraceDepth)

	info := RaceInfo{
		Timestamp:   time.Now(),
		GoroutineID: goroutineID,
		Operation:   operation,
		Location:    location,
		StackTrace:  stackTrace,
	}

	rd.mu.Lock()
	defer rd.mu.Unlock()

	rd.detectedRaces[location] = append(rd.detectedRaces[location], info)

	if rd.verbose.Load() {
		fmt.Printf("[RACE] Potential race detected at %s:\n", location)
		fmt.Printf("  Goroutine: %d\n", goroutineID)
		fmt.Printf("  Operation: %s\n", operation)
		fmt.Printf("  Stack: %s\n", stackTrace)
	}
}

// GetRaces returns all detected races for a location
func (rd *RaceDetector) GetRaces(location string) []RaceInfo {
	rd.mu.Lock()
	defer rd.mu.Unlock()

	races := rd.detectedRaces[location]
	result := make([]RaceInfo, len(races))
	copy(result, races)
	return result
}

// GetAllRaces returns all detected races
func (rd *RaceDetector) GetAllRaces() map[string][]RaceInfo {
	rd.mu.Lock()
	defer rd.mu.Unlock()

	result := make(map[string][]RaceInfo, len(rd.detectedRaces))
	for loc, races := range rd.detectedRaces {
		result[loc] = append([]RaceInfo{}, races...)
	}
	return result
}

// Clear clears all detected races
func (rd *RaceDetector) Clear() {
	rd.mu.Lock()
	defer rd.mu.Unlock()
	rd.detectedRaces = make(map[string][]RaceInfo)
}

// Summary returns a summary of detected races
func (rd *RaceDetector) Summary() string {
	rd.mu.Lock()
	defer rd.mu.Unlock()

	total := 0
	for _, races := range rd.detectedRaces {
		total += len(races)
	}

	return fmt.Sprintf("Detected %d potential races at %d locations", total, len(rd.detectedRaces))
}

// getGoroutineID returns the current goroutine ID
func getGoroutineID() int {
	// This is a simplified version
	// In production, you might use runtime.Stack or other methods
	buf := make([]byte, 64)
	buf = buf[:runtime.Stack(buf, false)]
	var id int
	fmt.Sscanf(string(buf), "goroutine %d", &id)
	return id
}

// getStackTrace captures a stack trace
func getStackTrace(depth int) string {
	buf := make([]byte, 4096)
	n := runtime.Stack(buf, false)
	if n > 0 && n < len(buf) {
		// Parse and limit depth
		return string(buf[:n])
	}
	return ""
}

// SafeCounter provides a race-free counter with atomic operations
type SafeCounter struct {
	value atomic.Int64
}

// NewSafeCounter creates a new safe counter
func NewSafeCounter(initialValue int64) *SafeCounter {
	sc := &SafeCounter{}
	sc.value.Store(initialValue)
	return sc
}

// Increment adds 1 to the counter
func (sc *SafeCounter) Increment() int64 {
	return sc.value.Add(1)
}

// Decrement subtracts 1 from the counter
func (sc *SafeCounter) Decrement() int64 {
	return sc.value.Add(-1)
}

// Add adds delta to the counter
func (sc *SafeCounter) Add(delta int64) int64 {
	return sc.value.Add(delta)
}

// Get returns the current value
func (sc *SafeCounter) Get() int64 {
	return sc.value.Load()
}

// Reset resets the counter to 0
func (sc *SafeCounter) Reset() {
	sc.value.Store(0)
}

// SafeFloat64Accumulator provides race-free float64 accumulation
type SafeFloat64Accumulator struct {
	mu    sync.Mutex
	value float64
}

// NewSafeFloat64Accumulator creates a new float64 accumulator
func NewSafeFloat64Accumulator(initialValue float64) *SafeFloat64Accumulator {
	return &SafeFloat64Accumulator{value: initialValue}
}

// Add adds a value to the accumulator
func (sfa *SafeFloat64Accumulator) Add(v float64) float64 {
	sfa.mu.Lock()
	sfa.value += v
	result := sfa.value
	sfa.mu.Unlock()
	return result
}

// Get returns the current value
func (sfa *SafeFloat64Accumulator) Get() float64 {
	sfa.mu.Lock()
	defer sfa.mu.Unlock()
	return sfa.value
}

// Reset resets the accumulator to 0
func (sfa *SafeFloat64Accumulator) Reset() {
	sfa.mu.Lock()
	defer sfa.mu.Unlock()
	sfa.value = 0
}

// SafeMap provides a concurrent map with fine-grained locking
type SafeMap[K comparable, V any] struct {
	mu     sync.RWMutex
	shards []mapShard[K, V]
	size   int
}

type mapShard[K comparable, V any] struct {
	mu   sync.RWMutex
	data map[K]V
}

// NewSafeMap creates a new concurrent map
func NewSafeMap[K comparable, V any](sizeHint int) *SafeMap[K, V] {
	shardCount := 16 // Number of shards for reduced contention
	sm := &SafeMap[K, V]{
		shards: make([]mapShard[K, V], shardCount),
		size:   shardCount,
	}

	for i := 0; i < shardCount; i++ {
		sm.shards[i] = mapShard[K, V]{
			data: make(map[K]V),
		}
	}

	return sm
}

// getShard returns the appropriate shard for a key
func (sm *SafeMap[K, V]) getShard(key K) *mapShard[K, V] {
	// Simple hash using built-in hash function
	h := uintptr(any(key).(uintptr))
	return &sm.shards[int(h)%sm.size]
}

// Get retrieves a value
func (sm *SafeMap[K, V]) Get(key K) (V, bool) {
	shard := sm.getShard(key)
	shard.mu.RLock()
	defer shard.mu.RUnlock()
	val, ok := shard.data[key]
	return val, ok
}

// Set stores a value
func (sm *SafeMap[K, V]) Set(key K, value V) {
	shard := sm.getShard(key)
	shard.mu.Lock()
	defer shard.mu.Unlock()
	shard.data[key] = value
}

// Delete removes a key
func (sm *SafeMap[K, V]) Delete(key K) {
	shard := sm.getShard(key)
	shard.mu.Lock()
	defer shard.mu.Unlock()
	delete(shard.data, key)
}

// Len returns the number of entries
func (sm *SafeMap[K, V]) Len() int {
	total := 0
	for i := 0; i < sm.size; i++ {
		shard := &sm.shards[i]
		shard.mu.RLock()
		total += len(shard.data)
		shard.mu.RUnlock()
	}
	return total
}

// Range iterates over all key-value pairs
func (sm *SafeMap[K, V]) Range(fn func(key K, value V) bool) {
	for i := 0; i < sm.size; i++ {
		shard := &sm.shards[i]
		shard.mu.RLock()
		for k, v := range shard.data {
			if !fn(k, v) {
				shard.mu.RUnlock()
				return
			}
		}
		shard.mu.RUnlock()
	}
}

// Clear removes all entries
func (sm *SafeMap[K, V]) Clear() {
	for i := 0; i < sm.size; i++ {
		shard := &sm.shards[i]
		shard.mu.Lock()
		shard.data = make(map[K]V)
		shard.mu.Unlock()
	}
}

// ConcurrentLimiter limits concurrent operations using a semaphore
type ConcurrentLimiter struct {
	sem chan struct{}
}

// NewConcurrentLimiter creates a new concurrent operation limiter
func NewConcurrentLimiter(maxConcurrent int) *ConcurrentLimiter {
	return &ConcurrentLimiter{
		sem: make(chan struct{}, maxConcurrent),
	}
}

// Acquire acquires a permit
func (cl *ConcurrentLimiter) Acquire(ctx context.Context) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	case cl.sem <- struct{}{}:
		return nil
	}
}

// TryAcquire tries to acquire without blocking
func (cl *ConcurrentLimiter) TryAcquire() bool {
	select {
	case cl.sem <- struct{}{}:
		return true
	default:
		return false
	}
}

// Release releases a permit
func (cl *ConcurrentLimiter) Release() {
	<-cl.sem
}

// Run executes fn with concurrency limit
func (cl *ConcurrentLimiter) Run(ctx context.Context, fn func() error) error {
	if err := cl.Acquire(ctx); err != nil {
		return err
	}
	defer cl.Release()
	return fn()
}

// Available returns the number of available permits
func (cl *ConcurrentLimiter) Available() int {
	return cap(cl.sem) - len(cl.sem)
}

// WaitGroupWrapper provides a WaitGroup with context support
type WaitGroupWrapper struct {
	wg  sync.WaitGroup
	ctx context.Context
}

// NewWaitGroupWrapper creates a new wait group wrapper
func NewWaitGroupWrapper(ctx context.Context) *WaitGroupWrapper {
	if ctx == nil {
		ctx = context.Background()
	}
	return &WaitGroupWrapper{ctx: ctx}
}

// Add increments the counter
func (w *WaitGroupWrapper) Add(delta int) {
	w.wg.Add(delta)
}

// Done decrements the counter
func (w *WaitGroupWrapper) Done() {
	w.wg.Done()
}

// Wait waits for all goroutines to finish or context cancellation
func (w *WaitGroupWrapper) Wait() error {
	done := make(chan struct{})
	go func() {
		w.wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		return nil
	case <-w.ctx.Done():
		return w.ctx.Err()
	}
}
