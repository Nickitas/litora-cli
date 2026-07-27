# Рекомендации по улучшению кодовой базы
## Litora-CLI — Industrial Readiness Assessment

---

## 1. Критическая зона: Отсутствие зависимостей в go.mod

### Проблема
```
module coastal-geometry

go 1.25.4
```

go.mod содержит только модуль и версию Go, но не указывает зависимости. Это может привести к:
- Невоспроизводимым сборкам
- Проблемам при проверке лицензий зависимостей
- Сложностям при аудите кода

### Решение
```go
module coastal-geometry

go 1.25.4

// Если есть косвенные зависимости, указать явно:
require (
    // пример: github.com/spf13/cobra v1.8.0
)
```

### Действия
1. Запустить `go mod tidy` для очистки
2. Проверить `go mod graph` для понимания дерева зависимостей
3. Проверить лицензии всех зависимостей через `go-licenses`

---

## 2. Критическая зона: Отсутствие системы версионирования

### Проблема
Версия указана только в Makefile как переменная `VERSION=v1.2`, что неудобно для:
- Автоматического определения версии в программе
- Формирования SemVer
- Отслеживания изменений

### Решение

#### 2.1 Создать файл internal/version/version.go
```go
package version

import (
    "fmt"
    "runtime"
)

// Build information (populated at build time)
var (
    Name        = "Litora-CLI"
    Version     = "1.2.0"
    BuildDate   = "unknown" // Set via ldflags
    GitCommit   = "unknown" // Set via ldflags
    GitBranch   = "unknown" // Set via ldflags
    GoVersion   = runtime.Version()
)

// FullVersion returns complete version string
func FullVersion() string {
    return fmt.Sprintf("%s v%s (build: %s, commit: %s)",
        Name, Version, BuildDate, GitCommit)
}

// ShortVersion returns short version string
func ShortVersion() string {
    return fmt.Sprintf("%s v%s", Name, Version)
}
```

#### 2.2 Добавить ldflags в Makefile
```makefile
LDFLAGS=-ldflags "-X internal/version.BuildDate=$(shell date -u +%Y-%m-%dT%H:%M:%SZ) \
                  -X internal/version.GitCommit=$(shell git rev-parse --short HEAD) \
                  -X internal/version.GitBranch=$(shell git rev-parse --abbrev-ref HEAD)"

build:
    $(GO) build $(LDFLAGS) -o $(BINARY_NAME) ./cmd/lito
```

#### 2.3 Добавить команду version в CLI
```go
// internal/cli/version_command.go
package cli

import (
    "coastal-geometry/internal/version"
    "fmt"
)

func (a *App) RunVersion() error {
    fmt.Println(version.FullVersion())
    fmt.Printf("Go version: %s\n", version.GoVersion)
    fmt.Printf("Platform: %s/%s\n", runtime.GOOS, runtime.GOARCH)
    return nil
}
```

---

## 3. Критическая зона: CLI модуль не покрыт тестами

### Проблема
```
FAIL    coastal-geometry/internal/cli [build failed]
```

CLI модуль не имеет тестового покрытия, что критично для регистрации ЭВМ.

### Решение

#### 3.1 Создать internal/cli/app_test.go
```go
package cli

import (
    "testing"
)

func TestNewApp(t *testing.T) {
    cfg := config{
        Command:  cmdVersion,
        OutputPath: "./output/test",
    }

    app, err := NewApp(cfg)
    if err != nil {
        t.Fatalf("NewApp() error = %v", err)
    }

    if app == nil {
        t.Fatal("NewApp() returned nil app")
    }
}

func TestNewAppWithMissingCoastline(t *testing.T) {
    cfg := config{
        Command:    cmdCoastline,
        InputPath:  "/nonexistent/file.json",
        OutputPath: "./output/test",
    }

    _, err := NewApp(cfg)
    if err == nil {
        t.Error("Expected error for missing input file")
    }
}
```

#### 3.2 Создать internal/cli/run_test.go
```go
package cli

import (
    "bytes"
    "testing"
)

func TestRunCoastlineCommand(t *testing.T) {
    cfg := config{
        Command:    cmdCoastline,
        InputPath:  "../testdata/coastline.json",
        OutputPath: t.TempDir(),
    }

    app, err := NewApp(cfg)
    if err != nil {
        t.Fatalf("NewApp() error = %v", err)
    }

    var stdout, stderr bytes.Buffer
    err = app.Run(&stdout, &stderr)
    if err != nil {
        t.Fatalf("Run() error = %v", err)
    }
}
```

---

## 4. Зона улучшения: Отсутствие CHANGELOG

### Проблема
История изменений не документирована, что затрудняет:
- Отслеживание новых функций
- Обнаружение регрессий
- Подготовку релизов

### Решение

#### 4.1 Создать CHANGELOG.md по Keep a Changelog
```markdown
# Changelog

All notable changes to Litora-CLI will be documented in this file.

## [Unreleased]

### Added
- Placeholder for upcoming features

## [1.2.0] - 2025-07-25

### Added
- Sediment transport with caching and batching
- Dynamic lithology model with weathering
- Coastline segmentation analysis

### Changed
- Improved erosion model performance

### Fixed
- Memory leak in bathymetry processing

## [1.1.0] - 2025-06-15

### Added
- GIF animation export
- CSV metrics export
- Model quality metrics

## [1.0.0] - 2025-05-01

### Added
- Initial release
- Wave erosion modeling
- Fractal analysis
- Bathymetry integration
```

---

## 5. Зона улучшения: Отсутствие интеграционных тестов

### Проблема
Отсутствуют тесты, проверяющие полный цикл работы программы.

### Решение

#### 5.1 Создать tests/integration/full_scenario_test.go
```go
// +build integration

package integration

import (
    "os"
    "os/exec"
    "testing"
    "path/filepath"
)

func TestFullScenario(t *testing.T) {
    tempDir := t.TempDir()

    // Run full scenario
    cmd := exec.Command("../lito", "all",
        "--iterations", "3",
        "--steps", "5",
        "--output", tempDir,
    )

    output, err := cmd.CombinedOutput()
    if err != nil {
        t.Fatalf("Command failed: %v\nOutput: %s", err, output)
    }

    // Check outputs
    checkFileExists(t, filepath.Join(tempDir, "svg", "coastline.svg"))
    checkFileExists(t, filepath.Join(tempDir, "metrics", "coastline.metrics.json"))
}

func checkFileExists(t *testing.T, path string) {
    if _, err := os.Stat(path); os.IsNotExist(err) {
        t.Errorf("Expected file %s to exist", path)
    }
}
```

#### 5.2 Добавить make target
```makefile
test-integration:
    $(GO) test -v -tags=integration ./tests/integration/...
```

---

## 6. Зона улучшения: Отсутствие бенчмарков

### Проблема
Невозможно измерить производительность и обнаружить регрессии.

### Решение

#### 6.1 Создать бенчмарки
```go
// internal/domain/geometry/erosion_bench_test.go
package geometry

import (
    "testing"
)

func BenchmarkWaveErosion(b *testing.B) {
    coast := generateTestCoastline(10000)

    b.ResetTimer()
    for i := 0; i < b.N; i++ {
        _, _ = WaveErosion(coast, WaveErosionOptions{
            Steps:           10,
            ErosionStrength: 50,
        })
    }
}

func BenchmarkHaversine(b *testing.B) {
    p1 := LatLon{Lat: 43.5, Lon: 40.0}
    p2 := LatLon{Lat: 44.5, Lon: 41.0}

    b.ResetTimer()
    for i := 0; i < b.N; i++ {
        _ = Haversine(p1, p2)
    }
}
```

---

## 7. Зона улучшения: Обработка ошибок

### Проблема
В некоторых местах ошибки могут быть не обработаны корректно.

### Решение

#### 7.1 Проверить обработку ошибок
```go
// Плохо
data, _ := ioutil.ReadFile(file)

// Хорошо
data, err := ioutil.ReadFile(file)
if err != nil {
    return fmt.Errorf("read file %s: %w", file, err)
}
```

---

## 8. Зона улучшения: Валидация входных данных

### Проблема
Отсутствует валидация пользовательского ввода.

### Решение

#### 8.1 Добавить валидацию
```go
// internal/cli/validation.go
package cli

func validateSteps(steps int) error {
    if steps < 1 {
        return fmt.Errorf("steps must be at least 1, got %d", steps)
    }
    if steps > 1000 {
        return fmt.Errorf("steps cannot exceed 1000, got %d", steps)
    }
    return nil
}

func validateErosionStrength(strength float64) error {
    if strength <= 0 {
        return fmt.Errorf("erosion strength must be positive, got %f", strength)
    }
    if strength > 1000 {
        return fmt.Errorf("erosion strength cannot exceed 1000 m, got %f", strength)
    }
    return nil
}
```

---

## 9. Зона улучшения: Логирование

### Проблема
Отсутствует структурированное логирование.

### Решение

#### 9.1 Добавить логирование
```go
// internal/log/log.go
package log

import (
    "io"
    "log"
    "os"
)

var (
    Info    *log.Logger
    Warning *log.Logger
    Error   *log.Logger
    Debug   *log.Logger
)

func init() {
    Info = log.New(os.Stdout, "INFO: ", log.Ldate|log.Ltime)
    Warning = log.New(os.Stdout, "WARN: ", log.Ldate|log.Ltime)
    Error = log.New(os.Stderr, "ERROR: ", log.Ldate|log.Ltime)
    Debug = log.New(io.Discard, "DEBUG: ", log.Ldate|log.Ltime)
}

func SetDebug(enable bool) {
    if enable {
        Debug = log.New(os.Stdout, "DEBUG: ", log.Ldate|log.Ltime)
    }
}
```

---

## 10. Зона улучшения: Конфигурационные файлы

### Преблема
Все параметры передаются только через CLI флаги.

### Решение

#### 10.1 Добавить поддержку конфигурационного файла
```go
// internal/cli/config_file.go
package cli

type ConfigFile struct {
    Common CommonConfig `json:"common"`
    Model  ModelConfig   `json:"model"`
    Output OutputConfig  `json:"output"`
}

type CommonConfig struct {
    InputPath  string `json:"input_path"`
    OutputPath string `json:"output_path"`
}

type ModelConfig struct {
    Iterations      int     `json:"iterations"`
    Steps           int     `json:"steps"`
    ErosionStrength float64 `json:"erosion_strength"`
}

// LoadConfigFile loads configuration from JSON file
func LoadConfigFile(path string) (*ConfigFile, error) {
    data, err := os.ReadFile(path)
    if err != nil {
        return nil, err
    }

    var cfg ConfigFile
    if err := json.Unmarshal(data, &cfg); err != nil {
        return nil, err
    }

    return &cfg, nil
}
```

---

## 11. Зона улучшения: CI/CD

### Проблема
Отсутствует автоматическая проверка качества кода.

### Решение

#### 11.1 Добавить GitHub Actions
```yaml
# .github/workflows/ci.yml
name: CI

on:
  push:
    branches: [ main, develop ]
  pull_request:

jobs:
  test:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v3

      - name: Set up Go
        uses: actions/setup-go@v4
        with:
          go-version: '1.25'

      - name: Run tests
        run: go test -v -race -coverprofile=coverage.out ./...

      - name: Upload coverage
        uses: codecov/codecov-action@v3

  lint:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v3

      - name: Run golangci-lint
        uses: golangci/golangci-lint-action@v3
```

---

## 12. Приоритеты реализации

| Приоритет | Задача | Срок | Влияние |
|----------|--------|------|---------|
| P0 | Версионирование (version.go) | 2 дня | Критично |
| P0 | CHANGELOG.md | 1 день | Критично |
| P0 | CLI тесты (app_test.go) | 5 дней | Критично |
| P1 | Интеграционные тесты | 3 дня | Важно |
| P1 | Валидация входных данных | 2 дня | Важно |
| P2 | Логирование | 2 дня | Полезно |
| P2 | Конфигурационные файлы | 3 дня | Полезно |
| P3 | CI/CD | 2 дня | Опционально |

---

## 13. Чек-лист готовности кода

- [x] go.mod содержит только зависимости с известными лицензиями
- [ ] Версия определена в коде (internal/version)
- [ ] CHANGELOG.md создан и обновляется
- [ ] CLI модуль имеет тестовое покрытие ≥ 60%
- [ ] Интеграционные тесты покрывают основные сценарии
- [ ] Бенчмарки позволяют отслеживать производительность
- [ ] Все ошибки обрабатываются корректно
- [ ] Входные данные валидируются
- [ ] Логирование структурировано
- [ ] Конфигурационные файлы поддерживаются
- [ ] CI/CD настроен
- [ ] Code coverage ≥ 60%

---

**Дата:** 2025-07-25
**Автор:** NeDatsky
