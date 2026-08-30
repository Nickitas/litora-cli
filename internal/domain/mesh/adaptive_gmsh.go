package mesh

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"time"
)

const defaultAdaptiveSubdivisionScale = 2.5

// AdaptiveGenerationConfig задаёт воспроизводимый запуск Gmsh с внешним
// скалярным полем размера PostView.
type AdaptiveGenerationConfig struct {
	Algorithm           Algorithm
	BackgroundFieldPath string
	GeoPath             string
	MeshPath            string
	LogPath             string
	GmshPath            string
	Timeout             time.Duration
	PreSubdivisionScale float64
	MinimumTargetSizeM  float64
	MaximumTargetSizeM  float64
}

// GenerationResourceStats фиксирует фактический объём и ресурсы запуска.
type GenerationResourceStats struct {
	GmshDurationSeconds        float64 `json:"gmsh_duration_seconds"`
	PostprocessDurationSeconds float64 `json:"postprocess_duration_seconds"`
	PeakRSSBytes               int64   `json:"peak_rss_bytes"`
	LitoHeapInUseBytes         uint64  `json:"lito_heap_in_use_bytes"`
	RawNodeCount               int     `json:"raw_node_count"`
	RawCellCount               int     `json:"raw_cell_count"`
	FinalNodeCount             int     `json:"final_node_count"`
	FinalCellCount             int     `json:"final_cell_count"`
	OutputMSHBytes             int64   `json:"output_msh_bytes"`
	OutputRoundTripVerified    bool    `json:"output_round_trip_verified"`
}

// WriteBackgroundFieldPOS потоково сохраняет узловые значения ADAPT-01 как
// скалярные четырёхугольники Gmsh PostView. Коэффициент компенсирует последующее
// согласованное деление исходных элементов на полные четырёхугольники.
func WriteBackgroundFieldPOS(path string, support Mesh, targetSizeM []float64, scale float64) error {
	if len(support.Nodes) <= 1 || len(support.Cells) == 0 || len(targetSizeM) != len(support.Nodes) {
		return fmt.Errorf("опорная сетка и узловое поле размера не согласованы")
	}
	if !finitePositive(scale) {
		return fmt.Errorf("коэффициент размера перед делением должен быть положительным")
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("создание каталога Background Field: %w", err)
	}
	file, err := os.Create(path)
	if err != nil {
		return fmt.Errorf("создание Background Field %q: %w", path, err)
	}
	writer := bufio.NewWriterSize(file, 1024*1024)
	writeErr := func() error {
		if _, err := fmt.Fprintln(writer, "View \"Lito: целевая длина ребра перед full-quad, м\" {"); err != nil {
			return err
		}
		for cellIndex, cell := range support.Cells {
			if cell.NodeCount != 4 {
				return fmt.Errorf("опорная ячейка %d не является четырёхугольником", cellIndex+1)
			}
			var points [4]Point
			var values [4]float64
			for index := 0; index < 4; index++ {
				nodeID := cell.Nodes[index]
				if nodeID <= 0 || nodeID >= len(support.Nodes) || !finitePositive(targetSizeM[nodeID]) {
					return fmt.Errorf("опорная ячейка %d ссылается на некорректный узел поля", cellIndex+1)
				}
				points[index] = support.Nodes[nodeID]
				values[index] = targetSizeM[nodeID] * scale
			}
			if _, err := fmt.Fprintf(writer,
				"SQ(%.12g,%.12g,0,%.12g,%.12g,0,%.12g,%.12g,0,%.12g,%.12g,0){%.12g,%.12g,%.12g,%.12g};\n",
				points[0].X, points[0].Y, points[1].X, points[1].Y,
				points[2].X, points[2].Y, points[3].X, points[3].Y,
				values[0], values[1], values[2], values[3]); err != nil {
				return err
			}
		}
		_, err := fmt.Fprintln(writer, "};")
		return err
	}()
	if writeErr == nil {
		writeErr = writer.Flush()
	}
	closeErr := file.Close()
	if writeErr != nil {
		return fmt.Errorf("запись Background Field %q: %w", path, writeErr)
	}
	if closeErr != nil {
		return fmt.Errorf("закрытие Background Field %q: %w", path, closeErr)
	}
	return nil
}

// GenerateAdaptiveGmsh создаёт адаптивную сетку из PostView, затем применяет
// единое full-quad деление и сохраняет географическую привязку и физические
// метки границ в итоговом MSH 2.2.
func GenerateAdaptiveGmsh(domain PreparedDomain, support Mesh, targetSizeM []float64, config AdaptiveGenerationConfig) (Mesh, GenerationResourceStats, error) {
	var stats GenerationResourceStats
	if _, err := config.Algorithm.options(); err != nil {
		return Mesh{}, stats, err
	}
	if err := executableFile(config.GmshPath); err != nil {
		return Mesh{}, stats, err
	}
	if !finitePositive(config.MinimumTargetSizeM) || config.MaximumTargetSizeM < config.MinimumTargetSizeM {
		return Mesh{}, stats, fmt.Errorf("диапазон целевого размера некорректен")
	}
	scale := config.PreSubdivisionScale
	if scale == 0 {
		scale = defaultAdaptiveSubdivisionScale
	}
	for _, path := range []string{config.BackgroundFieldPath, config.GeoPath, config.MeshPath, config.LogPath} {
		if strings.TrimSpace(path) == "" {
			return Mesh{}, stats, fmt.Errorf("пути POS, GEO, MSH и журнала должны быть заданы")
		}
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			return Mesh{}, stats, fmt.Errorf("создание каталога для %q: %w", path, err)
		}
	}
	if err := WriteBackgroundFieldPOS(config.BackgroundFieldPath, support, targetSizeM, scale); err != nil {
		return Mesh{}, stats, err
	}
	geo, err := buildAdaptiveGeo(domain, config.Algorithm, config.BackgroundFieldPath, scale*config.MinimumTargetSizeM, scale*config.MaximumTargetSizeM)
	if err != nil {
		return Mesh{}, stats, err
	}
	if err := os.WriteFile(config.GeoPath, geo, 0o644); err != nil {
		return Mesh{}, stats, fmt.Errorf("сохранение адаптивного GEO-сценария: %w", err)
	}

	timeout := config.Timeout
	if timeout <= 0 {
		timeout = 10 * time.Minute
	}
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	command := exec.CommandContext(ctx, config.GmshPath, config.GeoPath, "-2", "-format", "msh2", "-o", config.MeshPath, "-v", "3")
	started := time.Now()
	output, runErr := command.CombinedOutput()
	stats.GmshDurationSeconds = time.Since(started).Seconds()
	stats.PeakRSSBytes = processPeakRSSBytes(command)
	log := append([]byte(fmt.Sprintf("Алгоритм: %s\nBackground Field: %s\nЦелевой диапазон после full-quad: %.6g–%.6g м\nКоэффициент до деления: %.6g\n",
		config.Algorithm.RussianName(), config.BackgroundFieldPath, config.MinimumTargetSizeM, config.MaximumTargetSizeM, scale)), output...)
	if err := os.WriteFile(config.LogPath, log, 0o644); err != nil && runErr == nil {
		return Mesh{}, stats, fmt.Errorf("сохранение журнала Gmsh: %w", err)
	}
	if runErr != nil {
		_ = os.Remove(config.MeshPath)
		if ctx.Err() == context.DeadlineExceeded {
			return Mesh{}, stats, fmt.Errorf("Gmsh превысил лимит времени %s; журнал: %s", timeout, config.LogPath)
		}
		return Mesh{}, stats, fmt.Errorf("Gmsh завершился с ошибкой: %w; журнал: %s", runErr, config.LogPath)
	}

	postprocessStarted := time.Now()
	generated, err := ReadMSH2(config.MeshPath)
	if err != nil {
		return Mesh{}, stats, fmt.Errorf("чтение адаптивного результата Gmsh: %w", err)
	}
	stats.RawNodeCount = len(generated.Nodes) - 1
	stats.RawCellCount = len(generated.Cells)
	if stats.RawCellCount == 0 {
		return Mesh{}, stats, fmt.Errorf("Gmsh не создал поверхностных ячеек")
	}
	fullQuads := SubdivideToFullQuads(generated)
	if err := domain.Projection.AssignGeographicCoordinates(fullQuads.Nodes); err != nil {
		return Mesh{}, stats, fmt.Errorf("географическая привязка адаптивной сетки: %w", err)
	}
	if err := WriteMSH2(config.MeshPath, fullQuads); err != nil {
		return Mesh{}, stats, fmt.Errorf("сохранение адаптивной full-quad сетки: %w", err)
	}
	expectedNodeCount := len(fullQuads.Nodes) - 1
	expectedCellCount := len(fullQuads.Cells)
	expectedBoundaryCount := len(fullQuads.BoundaryEdges)
	// Проверяется именно записанный файл. Ссылка на исходную структуру
	// освобождается перед повторным чтением, чтобы не удваивать рабочий набор.
	fullQuads = Mesh{}
	runtime.GC()
	written, err := ReadMSH2(config.MeshPath)
	if err != nil {
		return Mesh{}, stats, fmt.Errorf("контрольное чтение итогового MSH: %w", err)
	}
	if len(written.Nodes)-1 != expectedNodeCount || len(written.Cells) != expectedCellCount || len(written.BoundaryEdges) != expectedBoundaryCount {
		return Mesh{}, stats, fmt.Errorf("контрольное чтение итогового MSH изменило число узлов, ячеек или граничных рёбер")
	}
	fullQuads = written
	stats.OutputRoundTripVerified = true
	stats.PostprocessDurationSeconds = time.Since(postprocessStarted).Seconds()
	stats.FinalNodeCount = len(fullQuads.Nodes) - 1
	stats.FinalCellCount = len(fullQuads.Cells)
	var memory runtime.MemStats
	runtime.ReadMemStats(&memory)
	stats.LitoHeapInUseBytes = memory.HeapInuse
	if info, statErr := os.Stat(config.MeshPath); statErr == nil {
		stats.OutputMSHBytes = info.Size()
	}
	return fullQuads, stats, nil
}

func buildAdaptiveGeo(domain PreparedDomain, algorithm Algorithm, fieldPath string, minSizeM, maxSizeM float64) ([]byte, error) {
	options, err := algorithm.options()
	if err != nil {
		return nil, err
	}
	if len(domain.SimplifiedRings) == 0 || !finitePositive(minSizeM) || maxSizeM < minSizeM {
		return nil, fmt.Errorf("граница или диапазон адаптивного размера некорректны")
	}
	absoluteFieldPath, err := filepath.Abs(fieldPath)
	if err != nil {
		return nil, fmt.Errorf("абсолютный путь Background Field: %w", err)
	}
	absoluteFieldPath = strings.ReplaceAll(strings.ReplaceAll(absoluteFieldPath, "\\", "/"), "\"", "\\\"")
	var out bytes.Buffer
	fmt.Fprintln(&out, "// ADAPT-02: воспроизводимая адаптивная сетка Lito в метрической LAEA.")
	fmt.Fprintln(&out, "SetFactory(\"Built-in\");")
	fmt.Fprintf(&out, "lcMin = %.12g; lcMax = %.12g;\n", minSizeM, maxSizeM)
	pointID, lineID := 1, 1
	loopIDs := make([]int, 0, len(domain.SimplifiedRings))
	boundaryLineIDs := make([][]int, 0, len(domain.SimplifiedRings))
	for ringIndex, sourceRing := range domain.SimplifiedRings {
		ring := append([]Point(nil), openMetricRing(sourceRing)...)
		if len(ring) < 3 {
			return nil, fmt.Errorf("кольцо %d содержит меньше трёх вершин", ringIndex)
		}
		if (signedRingArea(closeMetricRing(ring)) > 0) != (ringIndex == 0) {
			reversePoints(ring)
		}
		firstPointID := pointID
		for _, point := range ring {
			fmt.Fprintf(&out, "Point(%d) = {%.12g, %.12g, 0, lcMax};\n", pointID, point.X, point.Y)
			pointID++
		}
		lineIDs := make([]int, 0, len(ring))
		for index := range ring {
			fmt.Fprintf(&out, "Line(%d) = {%d, %d};\n", lineID, firstPointID+index, firstPointID+(index+1)%len(ring))
			lineIDs = append(lineIDs, lineID)
			lineID++
		}
		loopID := ringIndex + 1
		loopIDs = append(loopIDs, loopID)
		boundaryLineIDs = append(boundaryLineIDs, lineIDs)
		fmt.Fprintf(&out, "Curve Loop(%d) = {%s};\n", loopID, joinIntegers(lineIDs))
	}
	fmt.Fprintf(&out, "Plane Surface(1) = {%s};\n", joinIntegers(loopIDs))
	fmt.Fprintf(&out, "Physical Surface(\"Водоём\", %d) = {1};\n", PhysicalWaterSurface)
	fmt.Fprintf(&out, "Physical Curve(\"Внешний берег\", %d) = {%s};\n", PhysicalCoastline, joinIntegers(boundaryLineIDs[0]))
	if len(boundaryLineIDs) > 1 {
		islandLines := make([]int, 0)
		for _, lines := range boundaryLineIDs[1:] {
			islandLines = append(islandLines, lines...)
		}
		fmt.Fprintf(&out, "Physical Curve(\"Острова\", %d) = {%s};\n", PhysicalIsland, joinIntegers(islandLines))
	}
	fmt.Fprintf(&out, "Merge \"%s\";\n", absoluteFieldPath)
	fmt.Fprintln(&out, "Field[1] = PostView;")
	fmt.Fprintln(&out, "Field[1].ViewIndex = 0;")
	fmt.Fprintln(&out, "Field[1].UseClosest = 1;")
	fmt.Fprintln(&out, "Background Field = 1;")
	fmt.Fprintf(&out, "Mesh.Algorithm = %d;\n", options.MeshAlgorithm)
	fmt.Fprintln(&out, "Mesh.AlgorithmSwitchOnFailure = 0;")
	fmt.Fprintln(&out, "Mesh.RecombineAll = 0;")
	fmt.Fprintln(&out, "Mesh.MeshSizeFromPoints = 0;")
	fmt.Fprintln(&out, "Mesh.MeshSizeFromCurvature = 0;")
	fmt.Fprintln(&out, "Mesh.MeshSizeExtendFromBoundary = 0;")
	fmt.Fprintln(&out, "Mesh.MeshSizeMin = lcMin;")
	fmt.Fprintln(&out, "Mesh.MeshSizeMax = lcMax;")
	fmt.Fprintln(&out, "Mesh.Smoothing = 10;")
	fmt.Fprintln(&out, "Mesh.RandomSeed = 1;")
	fmt.Fprintln(&out, "Mesh.Reproducible = 1;")
	fmt.Fprintln(&out, "Mesh.MshFileVersion = 2.2;")
	fmt.Fprintln(&out, "Mesh.Binary = 0;")
	fmt.Fprintln(&out, "Mesh.SaveAll = 0;")
	return out.Bytes(), nil
}

func processPeakRSSBytes(command *exec.Cmd) int64 {
	if command == nil || command.ProcessState == nil || command.ProcessState.SysUsage() == nil {
		return 0
	}
	value := reflect.ValueOf(command.ProcessState.SysUsage())
	if value.Kind() == reflect.Pointer {
		value = value.Elem()
	}
	if !value.IsValid() || value.Kind() != reflect.Struct {
		return 0
	}
	field := value.FieldByName("Maxrss")
	if !field.IsValid() {
		return 0
	}
	var maxRSS int64
	if field.Kind() >= reflect.Int && field.Kind() <= reflect.Int64 {
		maxRSS = field.Int()
	} else if field.Kind() >= reflect.Uint && field.Kind() <= reflect.Uint64 {
		maxRSS = int64(field.Uint())
	}
	if maxRSS <= 0 {
		return 0
	}
	if runtime.GOOS != "darwin" {
		maxRSS *= 1024
	}
	return maxRSS
}

func finitePositive(value float64) bool {
	return value > 0 && !math.IsNaN(value) && !math.IsInf(value, 0)
}
