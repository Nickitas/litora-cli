package cli

import (
	"fmt"
	"os"
	"path/filepath"
)

// Подкаталоги выходного каталога
const (
	subdirSVG        = "svg"
	subdirMetrics    = "metrics"
	subdirCSV        = "csv"
	subdirGIF        = "gif"
	subdirErosion    = "erosion"
	subdirBenchmark  = "benchmark"
	subdirMesh       = "mesh"
	defaultOutputDir = "output"
)

// OutputPathManager управляет структурой выходных каталогов и путями
// Автоматически создаёт необходимую структуру директорий и разрешает пути к файлам
type OutputPathManager struct {
	baseDir string
}

// NewOutputPathManager создаёт новый менеджер путей вывода
// Если baseDir пуст, использует каталог "output" по умолчанию
func NewOutputPathManager(baseDir string) *OutputPathManager {
	if baseDir == "" {
		baseDir = defaultOutputDir
	}
	return &OutputPathManager{
		baseDir: baseDir,
	}
}

// EnsureDirectories создаёт все выходные подкаталоги, если они не существуют.
// Создаёт каталоги SVG, метрик, CSV, GIF, эрозии, benchmark и сеток.
func (opm *OutputPathManager) EnsureDirectories() error {
	dirs := []string{
		opm.SVGDir(),
		opm.MetricsDir(),
		opm.CSVDir(),
		opm.GIFDir(),
		opm.ErosionDir(),
		opm.BenchmarkDir(),
		opm.MeshDir(),
	}

	for _, dir := range dirs {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return fmt.Errorf("создать каталог %s: %w", dir, err)
		}
	}

	return nil
}

// MeshDir возвращает каталог расчётных 2D-сеток и отчётов их сравнения.
func (opm *OutputPathManager) MeshDir() string {
	return filepath.Join(opm.baseDir, subdirMesh)
}

// MeshPath возвращает путь к файлу внутри каталога расчётных 2D-сеток.
func (opm *OutputPathManager) MeshPath(filename string) string {
	return filepath.Join(opm.MeshDir(), filename)
}

// BaseDir возвращает базовый выходной каталог
func (opm *OutputPathManager) BaseDir() string {
	return opm.baseDir
}

// SVGDir возвращает каталог для вывода SVG файлов
func (opm *OutputPathManager) SVGDir() string {
	return filepath.Join(opm.baseDir, subdirSVG)
}

// MetricsDir возвращает каталог для вывода файлов метрик
func (opm *OutputPathManager) MetricsDir() string {
	return filepath.Join(opm.baseDir, subdirMetrics)
}

// CSVDir возвращает каталог для вывода CSV файлов
func (opm *OutputPathManager) CSVDir() string {
	return filepath.Join(opm.baseDir, subdirCSV)
}

// GIFDir возвращает каталог для вывода GIF-анимаций.
func (opm *OutputPathManager) GIFDir() string {
	return filepath.Join(opm.baseDir, subdirGIF)
}

// ErosionDir возвращает каталог расчётных результатов эрозии и транспорта.
func (opm *OutputPathManager) ErosionDir() string {
	return filepath.Join(opm.baseDir, subdirErosion)
}

// ErosionPath возвращает путь к файлу результатов расчёта эрозии.
func (opm *OutputPathManager) ErosionPath(filename string) string {
	return filepath.Join(opm.ErosionDir(), filename)
}

// BenchmarkDir возвращает каталог параметрических и калибровочных отчётов.
func (opm *OutputPathManager) BenchmarkDir() string {
	return filepath.Join(opm.baseDir, subdirBenchmark)
}

// BenchmarkPath возвращает путь к файлу отчёта в каталоге benchmark.
func (opm *OutputPathManager) BenchmarkPath(filename string) string {
	return filepath.Join(opm.BenchmarkDir(), filename)
}

// ResolveBenchmarkPath помещает относительное имя отчёта в каталог benchmark.
// Абсолютный путь сохраняется как явно заданный пользователем.
func (opm *OutputPathManager) ResolveBenchmarkPath(userPath, defaultFilename string) string {
	if userPath == "" {
		return opm.BenchmarkPath(defaultFilename)
	}
	if filepath.IsAbs(userPath) {
		return userPath
	}
	return opm.BenchmarkPath(filepath.Base(userPath))
}

// SVGPath возвращает полный путь к SVG файлу
func (opm *OutputPathManager) SVGPath(filename string) string {
	return filepath.Join(opm.SVGDir(), filename)
}

// MetricsPath возвращает полный путь к файлу метрик
func (opm *OutputPathManager) MetricsPath(filename string) string {
	return filepath.Join(opm.MetricsDir(), filename)
}

// CSVPath возвращает полный путь к CSV файлу
func (opm *OutputPathManager) CSVPath(filename string) string {
	return filepath.Join(opm.CSVDir(), filename)
}

// GIFPath возвращает полный путь к GIF-файлу.
func (opm *OutputPathManager) GIFPath(filename string) string {
	return filepath.Join(opm.GIFDir(), filename)
}

// ResolveUserPath преобразует пользовательский путь в соответствующий подкаталог
// Если путь абсолютный, использует его как есть
// Если путь относительный и начинается с имени подкаталога (svg/, metrics/, csv/, gif/),
// помещает его в соответствующий подкаталог
// Иначе использует базовый каталог
func (opm *OutputPathManager) ResolveUserPath(userPath string, fileType string) string {
	if userPath == "" {
		return ""
	}

	// Если абсолютный путь, используем как есть
	if filepath.IsAbs(userPath) {
		return userPath
	}

	// Если путь уже включает префикс подкаталога, используем как есть
	base := filepath.Base(userPath)
	dir := filepath.Dir(userPath)

	switch fileType {
	case "svg":
		if dir == subdirSVG {
			return filepath.Join(opm.baseDir, userPath)
		}
		return opm.SVGPath(base)
	case "metrics":
		if dir == subdirMetrics {
			return filepath.Join(opm.baseDir, userPath)
		}
		return opm.MetricsPath(base)
	case "csv":
		if dir == subdirCSV {
			return filepath.Join(opm.baseDir, userPath)
		}
		return opm.CSVPath(base)
	case "gif":
		if dir == subdirGIF {
			return filepath.Join(opm.baseDir, userPath)
		}
		return opm.GIFPath(base)
	default:
		// Неизвестный тип файла, помещаем в базовый каталог
		return filepath.Join(opm.baseDir, userPath)
	}
}

// ParseFileType определяет тип файла по имени/расширению
// Возвращает: "svg", "metrics" (JSON), "csv", "gif" или "unknown"
func ParseFileType(filename string) string {
	ext := filepath.Ext(filename)
	switch ext {
	case ".svg":
		return "svg"
	case ".json":
		return "metrics"
	case ".csv":
		return "csv"
	case ".gif":
		return "gif"
	default:
		return "unknown"
	}
}
