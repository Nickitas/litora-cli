package cli

import (
	"os"

	"github.com/schollz/progressbar/v3"
)

// IsTerminal проверяет, является ли stdout терминалом
func IsTerminal() bool {
	fi, err := os.Stdout.Stat()
	if err != nil {
		return false
	}
	return (fi.Mode() & os.ModeCharDevice) != 0
}

// ProgressAdapter адаптирует schollz/progressbar к существующему интерфейсу Progress
type ProgressAdapter struct {
	pb *progressbar.ProgressBar
}

// NewProgressAdapter создаёт новый адаптер индикатора прогресса
func NewProgressAdapter(total int, description string) *ProgressAdapter {
	pb := progressbar.Default(int64(total), description)
	return &ProgressAdapter{pb: pb}
}

// Add увеличивает прогресс на n единиц
func (a *ProgressAdapter) Add(n int) {
	if a.pb != nil {
		a.pb.Add(n)
	}
}

// Set устанавливает текущее значение прогресса
func (a *ProgressAdapter) Set(n int) {
	if a.pb != nil {
		a.pb.Set(n)
	}
}

// Increment увеличивает прогресс на 1
func (a *ProgressAdapter) Increment() {
	if a.pb != nil {
		a.pb.Add(1)
	}
}

// Done отмечает прогресс как завершённый
func (a *ProgressAdapter) Done() {
	if a.pb != nil {
		a.pb.Finish()
	}
}

// Workers заглушка для совместимости (schollz/progressbar не поддерживает это)
func (a *ProgressAdapter) Workers(n int) {
	// schollz/progressbar не показывает количество воркеров
}

// ParallelProgressAdapter адаптирует schollz/progressbar для параллельных операций
type ParallelProgressAdapter struct {
	*ProgressAdapter
}

// NewParallelProgressAdapter создаёт новый адаптер для параллельного индикатора прогресса
func NewParallelProgressAdapter(total int, description string) *ParallelProgressAdapter {
	if !IsTerminal() {
		return &ParallelProgressAdapter{ProgressAdapter: &ProgressAdapter{pb: nil}}
	}
	return &ParallelProgressAdapter{ProgressAdapter: NewProgressAdapter(total, description)}
}

// NewProgress создаёт индикатор прогресса с использованием нового адаптера
func NewProgress(total int, description string) *ProgressAdapter {
	return NewProgressAdapter(total, description)
}

// NewParallelProgress создаёт параллельный индикатор прогресса с использованием нового адаптера
func NewParallelProgress(total int, description string) *ParallelProgressAdapter {
	return NewParallelProgressAdapter(total, description)
}

// NewQuietProgress создаёт тихий/беззвучный индикатор прогресса (без вывода)
func NewQuietProgress(total int, description string) *ProgressAdapter {
	return &ProgressAdapter{pb: nil}
}
