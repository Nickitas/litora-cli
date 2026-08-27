package adaptive

import (
	"fmt"
	"math"
	"sort"
	"time"

	"coastal-geometry/internal/domain/mesh"
	"coastal-geometry/internal/domain/seabed"
)

// BathymetryComparisonConfig задаёт контрольные изобаты и число сохраняемых
// худших локальных ошибок внутренней проверки ADAPT-03.
type BathymetryComparisonConfig struct {
	IsobathsM      []float64
	WorstCellCount int
}

// DefaultBathymetryComparisonConfig возвращает единый набор контрольных
// глубин для всех генераторов и уровней детализации Чёрного моря.
func DefaultBathymetryComparisonConfig() BathymetryComparisonConfig {
	return BathymetryComparisonConfig{IsobathsM: []float64{200, 1000, 2000}, WorstCellCount: 10}
}

// IsobathAreaError описывает сохранение площади глубже заданной изобаты.
type IsobathAreaError struct {
	DepthM                   float64 `json:"depth_m"`
	ReferenceAreaKM2         float64 `json:"reference_area_km2"`
	ReconstructedAreaKM2     float64 `json:"reconstructed_area_km2"`
	AbsoluteDeviationKM2     float64 `json:"absolute_deviation_km2"`
	AbsoluteDeviationPercent float64 `json:"absolute_deviation_percent"`
}

// WorstBathymetryCell хранит одну контрольную точку с максимальной ошибкой.
type WorstBathymetryCell struct {
	CellID              int     `json:"cell_id"`
	XM                  float64 `json:"x_m"`
	YM                  float64 `json:"y_m"`
	LongitudeDeg        float64 `json:"longitude_deg"`
	LatitudeDeg         float64 `json:"latitude_deg"`
	ReferenceDepthM     float64 `json:"reference_depth_m"`
	ReconstructedDepthM float64 `json:"reconstructed_depth_m"`
	AbsoluteErrorM      float64 `json:"absolute_error_m"`
}

// BathymetryPreservationMetrics измеряет ошибку восстановления исходного
// поля BATHY-03 в центрах новых ячеек по значениям в их четырёх вершинах.
// Это внутренняя сравнительная проверка, а не внешняя валидация QA-02.
type BathymetryPreservationMetrics struct {
	Method                      string                `json:"method"`
	EvaluationCellCount         int                   `json:"evaluation_cell_count"`
	ReferenceMeanDepthM         float64               `json:"reference_mean_depth_m"`
	ReferenceP95DepthM          float64               `json:"reference_p95_depth_m"`
	DepthBiasM                  float64               `json:"depth_bias_m"`
	DepthMAEM                   float64               `json:"depth_mae_m"`
	DepthRMSEM                  float64               `json:"depth_rmse_m"`
	DepthP95AbsoluteErrorM      float64               `json:"depth_p95_absolute_error_m"`
	ReferenceWaterVolumeKM3     float64               `json:"reference_water_volume_km3"`
	ReconstructedWaterVolumeKM3 float64               `json:"reconstructed_water_volume_km3"`
	WaterVolumeDeviationPercent float64               `json:"water_volume_deviation_percent"`
	SlopeEvaluationCellCount    int                   `json:"slope_evaluation_cell_count"`
	SlopeMAEDeg                 float64               `json:"slope_mae_deg"`
	SlopeRMSEDeg                float64               `json:"slope_rmse_deg"`
	NearestFallbackNodeCount    int                   `json:"nearest_fallback_node_count"`
	NearestFallbackNodePercent  float64               `json:"nearest_fallback_node_percent"`
	MeanIsobathAreaDeviationPct float64               `json:"mean_isobath_area_deviation_percent"`
	Isobaths                    []IsobathAreaError    `json:"isobaths"`
	WorstCells                  []WorstBathymetryCell `json:"worst_cells"`
	DurationSeconds             float64               `json:"duration_seconds"`
}

// EvaluateBathymetryPreservation переносит глубины опорной модели на узлы
// новой сетки, а затем использует центры ячеек как независимые от её узлов
// контрольные точки. Все генераторы проходят одну и ту же процедуру.
func EvaluateBathymetryPreservation(reference seabed.Model, generated mesh.Mesh, config BathymetryComparisonConfig) (BathymetryPreservationMetrics, error) {
	started := time.Now()
	if len(generated.Nodes) <= 1 || len(generated.Cells) == 0 {
		return BathymetryPreservationMetrics{}, fmt.Errorf("сравниваемая сетка не содержит узлов или ячеек")
	}
	if len(config.IsobathsM) == 0 {
		config.IsobathsM = DefaultBathymetryComparisonConfig().IsobathsM
	}
	if config.WorstCellCount <= 0 {
		config.WorstCellCount = DefaultBathymetryComparisonConfig().WorstCellCount
	}
	index, err := seabed.NewModelDepthSampler(reference)
	if err != nil {
		return BathymetryPreservationMetrics{}, err
	}

	nodeDepths := make([]float64, len(generated.Nodes))
	metrics := BathymetryPreservationMetrics{
		Method: "глубины узлов интерполируются по full-quad каркасу BATHY-03; центры новых ячеек являются контрольными точками; восстановление в центре равно среднему четырёх вершин",
	}
	for nodeID := 1; nodeID < len(generated.Nodes); nodeID++ {
		point := generated.Nodes[nodeID]
		sample, sampleErr := index.Sample(point.X, point.Y, math.MaxFloat64)
		if sampleErr != nil {
			return BathymetryPreservationMetrics{}, sampleErr
		}
		nodeDepths[nodeID] = sample.WaterDepthM
		if sample.NearestFallback {
			metrics.NearestFallbackNodeCount++
		}
	}
	metrics.NearestFallbackNodePercent = 100 * float64(metrics.NearestFallbackNodeCount) / float64(len(generated.Nodes)-1)

	isobaths := append([]float64(nil), config.IsobathsM...)
	sort.Float64s(isobaths)
	referenceIsobathArea := make([]float64, len(isobaths))
	reconstructedIsobathArea := make([]float64, len(isobaths))
	errorHistogram := make([]int64, 20_001) // шаг 0,1 м, последний интервал ≥ 2000 м.
	depthHistogram := make([]int64, 5_001)  // шаг 1 м, последний интервал ≥ 5000 м.
	var errorSum, absoluteErrorSum, squaredErrorSum float64
	var referenceDepthSum, referenceVolumeM3, reconstructedVolumeM3 float64
	var slopeAbsoluteErrorSum, slopeSquaredErrorSum float64

	for cellIndex, cell := range generated.Cells {
		if cell.NodeCount != 4 {
			return BathymetryPreservationMetrics{}, fmt.Errorf("ячейка %d не является четырёхугольником", cellIndex+1)
		}
		center := mesh.Point{}
		reconstructedDepthM := 0.0
		longitudeDeg, latitudeDeg := 0.0, 0.0
		for corner := 0; corner < 4; corner++ {
			nodeID := cell.Nodes[corner]
			if nodeID <= 0 || nodeID >= len(generated.Nodes) {
				return BathymetryPreservationMetrics{}, fmt.Errorf("ячейка %d содержит некорректный узел", cellIndex+1)
			}
			point := generated.Nodes[nodeID]
			center.X += point.X
			center.Y += point.Y
			longitudeDeg += point.LongitudeDeg
			latitudeDeg += point.LatitudeDeg
			reconstructedDepthM += nodeDepths[nodeID]
		}
		center.X /= 4
		center.Y /= 4
		longitudeDeg /= 4
		latitudeDeg /= 4
		reconstructedDepthM /= 4
		referenceSample, sampleErr := index.Sample(center.X, center.Y, math.MaxFloat64)
		if sampleErr != nil {
			return BathymetryPreservationMetrics{}, sampleErr
		}
		referenceDepthM := referenceSample.WaterDepthM
		areaM2 := quadArea(generated.Nodes, cell)
		if areaM2 <= 0 {
			continue
		}
		errorM := reconstructedDepthM - referenceDepthM
		absoluteErrorM := math.Abs(errorM)
		metrics.EvaluationCellCount++
		errorSum += errorM
		absoluteErrorSum += absoluteErrorM
		squaredErrorSum += errorM * errorM
		referenceDepthSum += referenceDepthM
		referenceVolumeM3 += areaM2 * referenceDepthM
		reconstructedVolumeM3 += areaM2 * reconstructedDepthM
		errorBin := int(math.Min(float64(len(errorHistogram)-1), math.Floor(absoluteErrorM/0.1)))
		errorHistogram[errorBin]++
		depthBin := int(math.Min(float64(len(depthHistogram)-1), math.Floor(referenceDepthM)))
		depthHistogram[depthBin]++
		for index, isobathM := range isobaths {
			if referenceDepthM >= isobathM {
				referenceIsobathArea[index] += areaM2
			}
			if reconstructedDepthM >= isobathM {
				reconstructedIsobathArea[index] += areaM2
			}
		}
		if referenceSample.GradientAvailable {
			if gradientX, gradientY, ok := quadDepthGradient(generated.Nodes, cell, nodeDepths); ok {
				referenceSlopeDeg := math.Atan(math.Hypot(referenceSample.GradientX, referenceSample.GradientY)) * 180 / math.Pi
				reconstructedSlopeDeg := math.Atan(math.Hypot(gradientX, gradientY)) * 180 / math.Pi
				slopeErrorDeg := math.Abs(reconstructedSlopeDeg - referenceSlopeDeg)
				metrics.SlopeEvaluationCellCount++
				slopeAbsoluteErrorSum += slopeErrorDeg
				slopeSquaredErrorSum += slopeErrorDeg * slopeErrorDeg
			}
		}
		metrics.WorstCells = insertWorstCell(metrics.WorstCells, WorstBathymetryCell{
			CellID: cellIndex + 1, XM: center.X, YM: center.Y,
			LongitudeDeg: longitudeDeg, LatitudeDeg: latitudeDeg,
			ReferenceDepthM: referenceDepthM, ReconstructedDepthM: reconstructedDepthM, AbsoluteErrorM: absoluteErrorM,
		}, config.WorstCellCount)
	}
	if metrics.EvaluationCellCount == 0 {
		return BathymetryPreservationMetrics{}, fmt.Errorf("не найдено контрольных ячеек для сравнения батиметрии")
	}
	count := float64(metrics.EvaluationCellCount)
	metrics.ReferenceMeanDepthM = referenceDepthSum / count
	metrics.ReferenceP95DepthM = fixedHistogramQuantile(depthHistogram, int64(metrics.EvaluationCellCount), 0.95, 1)
	metrics.DepthBiasM = errorSum / count
	metrics.DepthMAEM = absoluteErrorSum / count
	metrics.DepthRMSEM = math.Sqrt(squaredErrorSum / count)
	metrics.DepthP95AbsoluteErrorM = fixedHistogramQuantile(errorHistogram, int64(metrics.EvaluationCellCount), 0.95, 0.1)
	metrics.ReferenceWaterVolumeKM3 = referenceVolumeM3 / 1e9
	metrics.ReconstructedWaterVolumeKM3 = reconstructedVolumeM3 / 1e9
	if referenceVolumeM3 > 0 {
		metrics.WaterVolumeDeviationPercent = 100 * (reconstructedVolumeM3 - referenceVolumeM3) / referenceVolumeM3
	}
	if metrics.SlopeEvaluationCellCount > 0 {
		slopeCount := float64(metrics.SlopeEvaluationCellCount)
		metrics.SlopeMAEDeg = slopeAbsoluteErrorSum / slopeCount
		metrics.SlopeRMSEDeg = math.Sqrt(slopeSquaredErrorSum / slopeCount)
	}
	for index, isobathM := range isobaths {
		referenceAreaM2 := referenceIsobathArea[index]
		reconstructedAreaM2 := reconstructedIsobathArea[index]
		deviationM2 := math.Abs(reconstructedAreaM2 - referenceAreaM2)
		deviationPercent := 0.0
		if referenceAreaM2 > 0 {
			deviationPercent = 100 * deviationM2 / referenceAreaM2
		}
		metrics.Isobaths = append(metrics.Isobaths, IsobathAreaError{
			DepthM: isobathM, ReferenceAreaKM2: referenceAreaM2 / 1e6,
			ReconstructedAreaKM2: reconstructedAreaM2 / 1e6,
			AbsoluteDeviationKM2: deviationM2 / 1e6, AbsoluteDeviationPercent: deviationPercent,
		})
		metrics.MeanIsobathAreaDeviationPct += deviationPercent
	}
	if len(metrics.Isobaths) > 0 {
		metrics.MeanIsobathAreaDeviationPct /= float64(len(metrics.Isobaths))
	}
	metrics.DurationSeconds = time.Since(started).Seconds()
	return metrics, nil
}

func quadArea(nodes []mesh.Point, cell mesh.Cell) float64 {
	area2 := 0.0
	for side := 0; side < 4; side++ {
		a, b := nodes[cell.Nodes[side]], nodes[cell.Nodes[(side+1)%4]]
		area2 += a.X*b.Y - b.X*a.Y
	}
	return math.Abs(area2) / 2
}

func quadDepthGradient(nodes []mesh.Point, cell mesh.Cell, depths []float64) (float64, float64, bool) {
	centerX, centerY, centerDepth := 0.0, 0.0, 0.0
	for corner := 0; corner < 4; corner++ {
		nodeID := cell.Nodes[corner]
		centerX += nodes[nodeID].X
		centerY += nodes[nodeID].Y
		centerDepth += depths[nodeID]
	}
	centerX, centerY, centerDepth = centerX/4, centerY/4, centerDepth/4
	var xx, xy, yy, xz, yz float64
	for corner := 0; corner < 4; corner++ {
		nodeID := cell.Nodes[corner]
		dx, dy, dz := nodes[nodeID].X-centerX, nodes[nodeID].Y-centerY, depths[nodeID]-centerDepth
		xx += dx * dx
		xy += dx * dy
		yy += dy * dy
		xz += dx * dz
		yz += dy * dz
	}
	determinant := xx*yy - xy*xy
	if math.Abs(determinant) <= 1e-12 {
		return 0, 0, false
	}
	return (xz*yy - yz*xy) / determinant, (yz*xx - xz*xy) / determinant, true
}

func insertWorstCell(values []WorstBathymetryCell, candidate WorstBathymetryCell, limit int) []WorstBathymetryCell {
	values = append(values, candidate)
	sort.SliceStable(values, func(left, right int) bool {
		if values[left].AbsoluteErrorM == values[right].AbsoluteErrorM {
			return values[left].CellID < values[right].CellID
		}
		return values[left].AbsoluteErrorM > values[right].AbsoluteErrorM
	})
	if len(values) > limit {
		values = values[:limit]
	}
	return values
}

func fixedHistogramQuantile(histogram []int64, count int64, probability, step float64) float64 {
	threshold := int64(math.Ceil(float64(count) * probability))
	if threshold < 1 {
		threshold = 1
	}
	seen := int64(0)
	for index, value := range histogram {
		seen += value
		if seen >= threshold {
			return (float64(index) + 0.5) * step
		}
	}
	return float64(len(histogram)-1) * step
}
