package mesh

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// ResolveGmshPath ищет явно заданный бинарник, переменную LITO_GMSH_PATH,
// системный PATH и локальный SDK в output/mesh/tools.
func ResolveGmshPath(explicitPath, outputBase string) (string, error) {
	candidates := []string{strings.TrimSpace(explicitPath), strings.TrimSpace(os.Getenv("LITO_GMSH_PATH"))}
	for _, candidate := range candidates {
		if candidate == "" {
			continue
		}
		if err := executableFile(candidate); err != nil {
			return "", err
		}
		return candidate, nil
	}

	if path, err := exec.LookPath("gmsh"); err == nil {
		return path, nil
	}
	if outputBase == "" {
		outputBase = "output"
	}
	matches, _ := filepath.Glob(filepath.Join(outputBase, "mesh", "tools", "gmsh-*", "bin", "gmsh"))
	sort.Strings(matches)
	for index := len(matches) - 1; index >= 0; index-- {
		if executableFile(matches[index]) == nil {
			return matches[index], nil
		}
	}
	return "", fmt.Errorf("Gmsh не найден: выполните scripts/install-gmsh.sh или укажите --gmsh")
}

// GmshVersion возвращает версию фактически запускаемого генератора.
func GmshVersion(path string) (string, error) {
	command := exec.Command(path, "--version")
	output, err := command.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("проверка версии Gmsh: %w: %s", err, strings.TrimSpace(string(output)))
	}
	return strings.TrimSpace(string(output)), nil
}

// GenerateGmsh создаёт GEO-сценарий, запускает Gmsh и читает MSH 2.2.
func GenerateGmsh(domain PreparedDomain, config GenerationConfig) (Mesh, error) {
	if config.TargetEdgeMeters <= 0 {
		return Mesh{}, fmt.Errorf("целевая длина ребра должна быть положительной")
	}
	if _, err := config.Algorithm.options(); err != nil {
		return Mesh{}, err
	}
	if err := executableFile(config.GmshPath); err != nil {
		return Mesh{}, err
	}
	for _, path := range []string{config.GeoPath, config.MeshPath, config.LogPath} {
		if strings.TrimSpace(path) == "" {
			return Mesh{}, fmt.Errorf("пути GEO, MSH и журнала должны быть заданы")
		}
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			return Mesh{}, fmt.Errorf("создание каталога для %q: %w", path, err)
		}
	}

	// После генерации каждый исходный элемент топологически делится на
	// четырёхугольники. Коэффициент 2,5 компенсирует рёбра от центра к серединам
	// сторон: по контрольным сеткам средняя итоговая длина близка к заданной.
	geo, err := buildGeo(domain, config.Algorithm, 2.5*config.TargetEdgeMeters)
	if err != nil {
		return Mesh{}, err
	}
	if err := os.WriteFile(config.GeoPath, geo, 0o644); err != nil {
		return Mesh{}, fmt.Errorf("сохранение GEO-сценария: %w", err)
	}

	timeout := config.Timeout
	if timeout <= 0 {
		timeout = 5 * time.Minute
	}
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	command := exec.CommandContext(ctx, config.GmshPath, config.GeoPath, "-2", "-format", "msh2", "-o", config.MeshPath, "-v", "3")
	output, runErr := command.CombinedOutput()
	log := append([]byte(fmt.Sprintf("Алгоритм: %s\nЦелевая длина ребра: %.0f м\n", config.Algorithm.RussianName(), config.TargetEdgeMeters)), output...)
	if err := os.WriteFile(config.LogPath, log, 0o644); err != nil && runErr == nil {
		return Mesh{}, fmt.Errorf("сохранение журнала Gmsh: %w", err)
	}
	if runErr != nil {
		// Частичный MSH после ошибки или тайм-аута не должен выглядеть как
		// пригодный расчётный результат; воспроизводимый GEO и журнал остаются.
		_ = os.Remove(config.MeshPath)
		if ctx.Err() == context.DeadlineExceeded {
			return Mesh{}, fmt.Errorf("Gmsh превысил лимит времени %s; журнал: %s", timeout, config.LogPath)
		}
		return Mesh{}, fmt.Errorf("Gmsh завершился с ошибкой: %w; журнал: %s", runErr, config.LogPath)
	}

	generated, err := ReadMSH2(config.MeshPath)
	if err != nil {
		return Mesh{}, fmt.Errorf("чтение результата Gmsh: %w", err)
	}
	if len(generated.Cells) == 0 {
		return Mesh{}, fmt.Errorf("Gmsh не создал поверхностных ячеек")
	}
	fullQuads := SubdivideToFullQuads(generated)
	if err := WriteMSH2(config.MeshPath, fullQuads); err != nil {
		return Mesh{}, fmt.Errorf("сохранение итоговой четырёхугольной сетки: %w", err)
	}
	return fullQuads, nil
}

func buildGeo(domain PreparedDomain, algorithm Algorithm, edgeMeters float64) ([]byte, error) {
	options, err := algorithm.options()
	if err != nil {
		return nil, err
	}
	if len(domain.SimplifiedRings) == 0 {
		return nil, fmt.Errorf("нет колец для построения сетки")
	}

	var out bytes.Buffer
	fmt.Fprintln(&out, "// Сценарий создан Lito; координаты заданы в метрах в проекции LAEA.")
	fmt.Fprintln(&out, "SetFactory(\"Built-in\");")
	fmt.Fprintf(&out, "lc = %.12g;\n", edgeMeters)
	pointID, lineID := 1, 1
	loopIDs := make([]int, 0, len(domain.SimplifiedRings))
	for ringIndex, sourceRing := range domain.SimplifiedRings {
		ring := append([]Point(nil), openMetricRing(sourceRing)...)
		if len(ring) < 3 {
			return nil, fmt.Errorf("кольцо %d содержит меньше трёх вершин", ringIndex)
		}
		wantPositive := ringIndex == 0
		if (signedRingArea(closeMetricRing(ring)) > 0) != wantPositive {
			reversePoints(ring)
		}
		firstPointID := pointID
		for _, point := range ring {
			fmt.Fprintf(&out, "Point(%d) = {%.12g, %.12g, 0, lc};\n", pointID, point.X, point.Y)
			pointID++
		}

		lineIDs := make([]int, 0, len(ring))
		for index := range ring {
			from := firstPointID + index
			to := firstPointID + (index+1)%len(ring)
			fmt.Fprintf(&out, "Line(%d) = {%d, %d};\n", lineID, from, to)
			lineIDs = append(lineIDs, lineID)
			lineID++
		}
		loopID := ringIndex + 1
		loopIDs = append(loopIDs, loopID)
		fmt.Fprintf(&out, "Curve Loop(%d) = {%s};\n", loopID, joinIntegers(lineIDs))
	}
	fmt.Fprintf(&out, "Plane Surface(1) = {%s};\n", joinIntegers(loopIDs))
	fmt.Fprintln(&out, "Physical Surface(\"Водоём\") = {1};")
	fmt.Fprintf(&out, "Mesh.Algorithm = %d;\n", options.MeshAlgorithm)
	fmt.Fprintln(&out, "Mesh.AlgorithmSwitchOnFailure = 0;")
	// Gmsh строит согласованную исходную сетку выбранным алгоритмом. Единое
	// разбиение элементов в Go затем гарантирует только четырёхугольные ячейки
	// и не зависит от эвристической рекомбинации Blossom.
	fmt.Fprintln(&out, "Mesh.RecombineAll = 0;")
	fmt.Fprintln(&out, "Mesh.MeshSizeMin = lc;")
	fmt.Fprintln(&out, "Mesh.MeshSizeMax = lc;")
	fmt.Fprintln(&out, "Mesh.MeshSizeFromCurvature = 0;")
	fmt.Fprintln(&out, "Mesh.MeshSizeExtendFromBoundary = 0;")
	fmt.Fprintln(&out, "Mesh.Smoothing = 10;")
	fmt.Fprintln(&out, "Mesh.RandomSeed = 1;")
	fmt.Fprintln(&out, "Mesh.Reproducible = 1;")
	fmt.Fprintln(&out, "Mesh.MshFileVersion = 2.2;")
	fmt.Fprintln(&out, "Mesh.Binary = 0;")
	fmt.Fprintln(&out, "Mesh.SaveAll = 1;")
	return out.Bytes(), nil
}

func executableFile(path string) error {
	info, err := os.Stat(path)
	if err != nil {
		return fmt.Errorf("Gmsh %q недоступен: %w", path, err)
	}
	if info.IsDir() || info.Mode()&0o111 == 0 {
		return fmt.Errorf("Gmsh %q не является исполняемым файлом", path)
	}
	return nil
}

func reversePoints(points []Point) {
	for left, right := 0, len(points)-1; left < right; left, right = left+1, right-1 {
		points[left], points[right] = points[right], points[left]
	}
}

func joinIntegers(values []int) string {
	parts := make([]string, len(values))
	for index, value := range values {
		parts[index] = fmt.Sprintf("%d", value)
	}
	return strings.Join(parts, ", ")
}
