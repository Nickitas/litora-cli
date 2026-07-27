package cli

import (
	"fmt"
	"io"
	"os"
	"sync"
	"sync/atomic"
	"time"
)

// Progress tracks and displays progress for long-running operations
type Progress struct {
	total    int64
	current  int64
	start    time.Time
	desc     string
	out      io.Writer
	mu       sync.Mutex
	done     chan struct{}
	lastDraw time.Time
	eta      string
}

// NewProgress creates a new progress bar
func NewProgress(total int, description string) *Progress {
	return &Progress{
		total:    int64(total),
		desc:     description,
		out:      os.Stderr,
		done:     make(chan struct{}),
		start:    time.Now(),
		lastDraw: time.Now(),
	}
}

// Add increments the progress by n
func (p *Progress) Add(n int) {
	atomic.AddInt64(&p.current, int64(n))
	p.draw()
}

// Set sets the current progress
func (p *Progress) Set(n int) {
	atomic.StoreInt64(&p.current, int64(n))
	p.draw()
}

// Increment increases progress by 1
func (p *Progress) Increment() {
	p.Add(1)
}

// Done marks the progress as complete
func (p *Progress) Done() {
	p.mu.Lock()
	defer p.mu.Unlock()
	select {
	case <-p.done:
		return
	default:
	}
	close(p.done)
	atomic.StoreInt64(&p.current, p.total)
	p.draw(true)
	fmt.Fprintln(p.out)
}

// draw renders the progress bar (if enough time passed since last draw)
func (p *Progress) draw(force ...bool) {
	const minDrawInterval = 100 * time.Millisecond
	p.mu.Lock()
	defer p.mu.Unlock()

	now := time.Now()
	if len(force) == 0 && now.Sub(p.lastDraw) < minDrawInterval {
		return
	}
	p.lastDraw = now

	current := atomic.LoadInt64(&p.current)
	total := p.total

	if total <= 0 {
		return
	}

	percent := float64(current) / float64(total) * 100

	// ETA calculation
	elapsed := now.Sub(p.start).Seconds()
	if current > 0 {
		remaining := float64(total-current) / (float64(current) / elapsed)
		p.eta = formatDuration(remaining)
	} else {
		p.eta = "--:--"
	}

	// Draw bar: [████████░░░░░░] 60% (120/200) ETA: 0:30
	barWidth := 30
	filled := int(float64(barWidth) * float64(current) / float64(total))
	if filled > barWidth {
		filled = barWidth
	}

	bar := ""
	for i := 0; i < barWidth; i++ {
		if i < filled {
			bar += "█"
		} else {
			bar += "░"
		}
	}

	// Clear line with carriage return and draw
	fmt.Fprintf(p.out, "\r%s [%s] %.0f%% (%d/%d) ETA: %s", p.desc, bar, percent, current, total, p.eta)
}

// formatDuration formats seconds as MM:SS or HH:MM:SS
func formatDuration(seconds float64) string {
	if seconds < 0 {
		return "--:--"
	}
	sec := int(seconds)
	if sec < 60 {
		return fmt.Sprintf("0:%02d", sec)
	}
	if sec < 3600 {
		min := sec / 60
		s := sec % 60
		return fmt.Sprintf("%d:%02d", min, s)
	}
	hours := sec / 3600
	min := (sec % 3600) / 60
	s := sec % 60
	return fmt.Sprintf("%d:%02d:%02d", hours, min, s)
}

// ParallelProgress tracks progress for parallel operations
type ParallelProgress struct {
	total     int64
	current   int64
	start     time.Time
	desc      string
	out       io.Writer
	mu        sync.Mutex
	lastDraw  time.Time
	eta       string
	workerCnt atomic.Int32
}

// NewParallelProgress creates a progress bar for parallel operations
func NewParallelProgress(total int, description string) *ParallelProgress {
	return &ParallelProgress{
		total:    int64(total),
		desc:     description,
		out:      os.Stderr,
		start:    time.Now(),
		lastDraw: time.Now(),
	}
}

// Add increments the progress by n
func (p *ParallelProgress) Add(n int) {
	atomic.AddInt64(&p.current, int64(n))
	p.draw()
}

// Set sets the current progress
func (p *ParallelProgress) Set(n int) {
	atomic.StoreInt64(&p.current, int64(n))
	p.draw()
}

// Increment increases progress by 1
func (p *ParallelProgress) Increment() {
	p.Add(1)
}

// Workers sets the number of active workers (for display)
func (p *ParallelProgress) Workers(n int) {
	p.workerCnt.Store(int32(n))
}

// Done marks the progress as complete
func (p *ParallelProgress) Done() {
	p.mu.Lock()
	defer p.mu.Unlock()
	atomic.StoreInt64(&p.current, p.total)
	p.draw(true)
	fmt.Fprintln(p.out)
}

// draw renders the progress bar
func (p *ParallelProgress) draw(force ...bool) {
	const minDrawInterval = 100 * time.Millisecond
	p.mu.Lock()
	defer p.mu.Unlock()

	now := time.Now()
	if len(force) == 0 && now.Sub(p.lastDraw) < minDrawInterval {
		return
	}
	p.lastDraw = now

	current := atomic.LoadInt64(&p.current)
	total := p.total
	workers := p.workerCnt.Load()

	if total <= 0 {
		return
	}

	percent := float64(current) / float64(total) * 100

	// ETA calculation
	elapsed := now.Sub(p.start).Seconds()
	if current > 0 && elapsed > 0.1 {
		remaining := float64(total-current) / (float64(current) / elapsed)
		p.eta = formatDuration(remaining)
	} else {
		p.eta = "--:--"
	}

	// Draw bar: [████████░░░░░░] 60% (120/200) [8 workers] ETA: 0:30
	barWidth := 30
	filled := int(float64(barWidth) * float64(current) / float64(total))
	if filled > barWidth {
		filled = barWidth
	}

	bar := ""
	for i := 0; i < barWidth; i++ {
		if i < filled {
			bar += "█"
		} else {
			bar += "░"
		}
	}

	// Clear line with carriage return and draw
	workerInfo := ""
	if workers > 0 {
		workerInfo = fmt.Sprintf("[%d workers] ", workers)
	}
	fmt.Fprintf(p.out, "\r%s [%s] %.0f%% (%d/%d) %sETA: %s", p.desc, bar, percent, current, total, workerInfo, p.eta)
}

// QuietProgress is a no-op progress bar for non-terminal output
type QuietProgress struct{}

// NewQuietProgress creates a silent progress bar
func NewQuietProgress(total int, description string) *QuietProgress {
	return &QuietProgress{}
}

func (p *QuietProgress) Add(n int)            {}
func (p *QuietProgress) Set(n int)             {}
func (p *QuietProgress) Increment()           {}
func (p *QuietProgress) Done()                {}
func (p *QuietProgress) Workers(n int)         {}

// IsTerminal returns true if stdout is a terminal
func IsTerminal() bool {
	fi, err := os.Stdout.Stat()
	if err != nil {
		return false
	}
	return (fi.Mode() & os.ModeCharDevice) != 0
}
