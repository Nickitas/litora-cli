package seabed

import (
	"fmt"
	"math"
	"sort"
	"strings"
	"time"

	"coastal-geometry/internal/domain/mesh"
)

const (
	qualityErrorHistogramStepM = 0.1
	qualityDepthHistogramStepM = 1.0
	qualitySlopeHistogramStep  = 0.01
)

// EvaluateReliefQuality сравнивает модель с отдельной опорной моделью в одной
// LAEA-проекции. Центры опорных ячеек не совпадают с узлами проверяемой сетки
// и служат отложенными контрольными точками.
func EvaluateReliefQuality(
	reference Model,
	referenceMetadata ExportMetadata,
	evaluated Model,
	evaluatedMetadata ExportMetadata,
	passport ReliefReferencePassport,
	config ReliefQualityConfig,
) (ReliefQualityReport, error) {
	started := time.Now()
	if err := validateReliefQualityInputs(reference, referenceMetadata, evaluated, evaluatedMetadata, passport); err != nil {
		return ReliefQualityReport{}, err
	}
	config, err := normalizeReliefQualityConfig(config, passport, evaluated)
	if err != nil {
		return ReliefQualityReport{}, err
	}
	sampler, err := NewModelDepthSampler(evaluated)
	if err != nil {
		return ReliefQualityReport{}, fmt.Errorf("индекс проверяемой модели: %w", err)
	}

	report := ReliefQualityReport{
		SchemaVersion: ReliefQualitySchemaVersion,
		GeneratedAt:   time.Now().UTC().Format(time.RFC3339),
		Reference:     passport,
		Method: "центры ячеек отдельной опорной модели являются отложенными точками; " +
			"глубина проверяемой модели интерполируется по треугольникам full-quad; объём и площади взвешиваются площадью опорных ячеек",
	}
	errorHistogram := make([]int64, 50_001)
	depthHistogram := make([]int64, 5_001)
	slopeHistogram := make([]int64, 9_001)
	var errorSum, absoluteErrorSum, squaredErrorSum float64
	var weightedSquaredErrorSum, evaluatedDepthSum, referenceDepthSum float64
	var evaluatedVolumeM3, referenceVolumeM3, evaluatedAreaM2 float64
	var slopeAbsoluteErrorSum, slopeSquaredErrorSum float64
	bandBounds := append([]float64{0}, config.IsobathsM...)
	referenceBandAreaM2 := make([]float64, len(bandBounds))
	evaluatedBandAreaM2 := make([]float64, len(bandBounds))

	for _, cell := range reference.Cells {
		center, longitudeDeg, latitudeDeg, centerErr := referenceCellCenter(reference.Nodes, cell)
		if centerErr != nil {
			return ReliefQualityReport{}, centerErr
		}
		sample, sampleErr := sampler.Sample(center.X, center.Y, config.MaxNearestDistanceM)
		if sampleErr != nil {
			report.Depth.MissingCellCount++
			continue
		}
		if sample.NearestFallback {
			report.Depth.NearestFallbackCellCount++
		}
		referenceDepthM := cell.WaterDepthMeanM
		evaluatedDepthM := sample.WaterDepthM
		errorM := evaluatedDepthM - referenceDepthM
		absoluteErrorM := math.Abs(errorM)
		areaM2 := cell.AreaM2
		if !finite(areaM2) || areaM2 <= 0 {
			return ReliefQualityReport{}, fmt.Errorf("опорная ячейка %d имеет некорректную площадь", cell.ID)
		}
		report.Depth.EvaluationCellCount++
		errorSum += errorM
		absoluteErrorSum += absoluteErrorM
		squaredErrorSum += errorM * errorM
		weightedSquaredErrorSum += areaM2 * errorM * errorM
		evaluatedAreaM2 += areaM2
		referenceDepthSum += referenceDepthM
		evaluatedDepthSum += evaluatedDepthM
		referenceVolumeM3 += areaM2 * referenceDepthM
		evaluatedVolumeM3 += areaM2 * evaluatedDepthM
		incrementFixedHistogram(errorHistogram, absoluteErrorM, qualityErrorHistogramStepM)
		incrementFixedHistogram(depthHistogram, referenceDepthM, qualityDepthHistogramStepM)
		referenceBandAreaM2[depthBandIndex(referenceDepthM, config.IsobathsM)] += areaM2
		evaluatedBandAreaM2[depthBandIndex(evaluatedDepthM, config.IsobathsM)] += areaM2
		if sample.GradientAvailable {
			evaluatedSlopeDeg := math.Atan(math.Hypot(sample.GradientX, sample.GradientY)) * 180 / math.Pi
			slopeErrorDeg := math.Abs(evaluatedSlopeDeg - cell.SlopeDeg)
			report.Slope.EvaluationCellCount++
			slopeAbsoluteErrorSum += slopeErrorDeg
			slopeSquaredErrorSum += slopeErrorDeg * slopeErrorDeg
			incrementFixedHistogram(slopeHistogram, cell.SlopeDeg, qualitySlopeHistogramStep)
		}
		report.WorstCells = insertWorstReliefCell(report.WorstCells, WorstReliefCell{
			ReferenceCellID: cell.ID,
			XM:              center.X, YM: center.Y, LongitudeDeg: longitudeDeg, LatitudeDeg: latitudeDeg,
			ReferenceDepthM: referenceDepthM, EvaluatedDepthM: evaluatedDepthM, AbsoluteErrorM: absoluteErrorM,
		}, config.WorstCellCount)
	}
	if report.Depth.EvaluationCellCount == 0 {
		return ReliefQualityReport{}, fmt.Errorf("не найдено опорных ячеек с доступной глубиной")
	}

	count := float64(report.Depth.EvaluationCellCount)
	report.Depth.ReferenceMeanDepthM = referenceDepthSum / count
	report.Depth.ReferenceP95DepthM = fixedQualityHistogramQuantile(depthHistogram, int64(report.Depth.EvaluationCellCount), 0.95, qualityDepthHistogramStepM)
	report.Depth.EvaluatedMeanDepthM = evaluatedDepthSum / count
	report.Depth.BiasM = errorSum / count
	report.Depth.MAEM = absoluteErrorSum / count
	report.Depth.RMSEM = math.Sqrt(squaredErrorSum / count)
	report.Depth.P95AbsoluteErrorM = fixedQualityHistogramQuantile(errorHistogram, int64(report.Depth.EvaluationCellCount), 0.95, qualityErrorHistogramStepM)
	if evaluatedAreaM2 > 0 {
		report.Depth.AreaWeightedRMSEM = math.Sqrt(weightedSquaredErrorSum / evaluatedAreaM2)
	}
	report.Depth.NearestFallbackPercent = 100 * float64(report.Depth.NearestFallbackCellCount) / count
	report.Volume.ReferenceKM3 = referenceVolumeM3 / 1e9
	report.Volume.EvaluatedKM3 = evaluatedVolumeM3 / 1e9
	report.Volume.DeviationKM3 = (evaluatedVolumeM3 - referenceVolumeM3) / 1e9
	if referenceVolumeM3 > 0 {
		report.Volume.DeviationPercent = 100 * (evaluatedVolumeM3 - referenceVolumeM3) / referenceVolumeM3
	}
	if report.Slope.EvaluationCellCount > 0 {
		slopeCount := float64(report.Slope.EvaluationCellCount)
		report.Slope.MAEDeg = slopeAbsoluteErrorSum / slopeCount
		report.Slope.RMSEDeg = math.Sqrt(slopeSquaredErrorSum / slopeCount)
		report.Slope.ReferenceP95Deg = fixedQualityHistogramQuantile(slopeHistogram, int64(report.Slope.EvaluationCellCount), 0.95, qualitySlopeHistogramStep)
	}

	report.Isobaths = evaluateIsobathPreservation(reference, evaluated, config.IsobathsM, passport.HorizontalResolutionM)
	report.Thresholds = deriveReliefQualityThresholds(report, passport, evaluatedAreaM2, referenceVolumeM3)
	report.DepthBands = buildDepthBandMetrics(
		bandBounds, referenceBandAreaM2, evaluatedBandAreaM2,
		report.Isobaths, passport.HorizontalResolutionM,
	)
	report.Sampling = evaluateSamplingQuality(evaluated)
	report.Mesh, err = evaluateReliefMeshQuality(evaluated, config)
	if err != nil {
		return ReliefQualityReport{}, err
	}
	evaluateReliefAcceptance(&report)
	report.PublicationReady = report.MetricsAccepted && passport.ValidationClass == ReliefValidationIndependent
	report.DurationSeconds = time.Since(started).Seconds()
	return report, nil
}

func validateReliefQualityInputs(reference Model, referenceMetadata ExportMetadata, evaluated Model, evaluatedMetadata ExportMetadata, passport ReliefReferencePassport) error {
	if !reference.Accepted || !evaluated.Accepted {
		return fmt.Errorf("опорная и проверяемая модели должны быть принятыми моделями %s", SeabedMSHSchemaVersion)
	}
	if len(reference.Cells) == 0 || len(evaluated.Cells) == 0 {
		return fmt.Errorf("опорная и проверяемая модели должны содержать производные характеристики ячеек")
	}
	if err := validateReliefReferencePassport(passport); err != nil {
		return err
	}
	if err := validateExportMetadata(referenceMetadata); err != nil {
		return fmt.Errorf("паспорт экспорта опорной модели: %w", err)
	}
	if err := validateExportMetadata(evaluatedMetadata); err != nil {
		return fmt.Errorf("паспорт экспорта проверяемой модели: %w", err)
	}
	if math.Abs(referenceMetadata.ProjectionReferenceLatitudeDeg-evaluatedMetadata.ProjectionReferenceLatitudeDeg) > 1e-9 ||
		math.Abs(referenceMetadata.ProjectionReferenceLongitudeDeg-evaluatedMetadata.ProjectionReferenceLongitudeDeg) > 1e-9 ||
		referenceMetadata.HorizontalMeshCRS != evaluatedMetadata.HorizontalMeshCRS {
		return fmt.Errorf("QA-02 требует общую LAEA-проекцию опорной и проверяемой моделей")
	}
	if referenceMetadata.VerticalReference != evaluatedMetadata.VerticalReference ||
		referenceMetadata.VerticalReference != passport.VerticalReference {
		return fmt.Errorf("вертикальные системы опорной, проверяемой моделей и паспорта контроля не совпадают")
	}
	return nil
}

func normalizeReliefQualityConfig(config ReliefQualityConfig, passport ReliefReferencePassport, evaluated Model) (ReliefQualityConfig, error) {
	if len(config.IsobathsM) == 0 {
		config.IsobathsM = []float64{20, 200, 1000, 2000}
	}
	config.IsobathsM = append([]float64(nil), config.IsobathsM...)
	sort.Float64s(config.IsobathsM)
	for index, value := range config.IsobathsM {
		if !finite(value) || value <= 0 || (index > 0 && value == config.IsobathsM[index-1]) {
			return ReliefQualityConfig{}, fmt.Errorf("изобаты QA-02 должны быть уникальными конечными положительными глубинами")
		}
	}
	if config.WorstCellCount <= 0 {
		config.WorstCellCount = 20
	}
	if config.MaxNearestDistanceM == 0 {
		config.MaxNearestDistanceM = 2 * passport.HorizontalResolutionM
	}
	if !finite(config.MaxNearestDistanceM) || config.MaxNearestDistanceM <= 0 {
		return ReliefQualityConfig{}, fmt.Errorf("максимальное расстояние до ближайшей глубины должно быть положительным")
	}
	if len(config.TargetSizeM) != len(evaluated.Nodes) || len(config.TargetZones) != len(evaluated.Nodes) {
		return ReliefQualityConfig{}, fmt.Errorf("QA-02 требует поле целевого размера и зоны для каждого узла проверяемой модели")
	}
	if config.TargetZoneNames == nil {
		config.TargetZoneNames = map[string]string{}
	}
	return config, nil
}

func referenceCellCenter(nodes []Node, cell Cell) (mesh.Point, float64, float64, error) {
	center := mesh.Point{}
	longitudeDeg, latitudeDeg := 0.0, 0.0
	for _, nodeID := range cell.NodeIDs {
		if nodeID <= 0 || nodeID >= len(nodes) {
			return mesh.Point{}, 0, 0, fmt.Errorf("опорная ячейка %d содержит некорректный узел %d", cell.ID, nodeID)
		}
		node := nodes[nodeID]
		center.X += node.XM
		center.Y += node.YM
		longitudeDeg += node.LongitudeDeg
		latitudeDeg += node.LatitudeDeg
	}
	center.X /= 4
	center.Y /= 4
	return center, longitudeDeg / 4, latitudeDeg / 4, nil
}

func depthBandIndex(depthM float64, isobaths []float64) int {
	return sort.Search(len(isobaths), func(index int) bool { return depthM < isobaths[index] })
}

func incrementFixedHistogram(histogram []int64, value, step float64) {
	bin := int(math.Floor(math.Max(0, value) / step))
	if bin >= len(histogram) {
		bin = len(histogram) - 1
	}
	histogram[bin]++
}

func fixedQualityHistogramQuantile(histogram []int64, count int64, probability, step float64) float64 {
	target := int64(math.Ceil(probability * float64(count)))
	if target < 1 {
		target = 1
	}
	seen := int64(0)
	for index, value := range histogram {
		seen += value
		if seen >= target {
			return (float64(index) + 0.5) * step
		}
	}
	return float64(len(histogram)-1) * step
}

func insertWorstReliefCell(values []WorstReliefCell, candidate WorstReliefCell, limit int) []WorstReliefCell {
	values = append(values, candidate)
	sort.SliceStable(values, func(left, right int) bool {
		if values[left].AbsoluteErrorM == values[right].AbsoluteErrorM {
			return values[left].ReferenceCellID < values[right].ReferenceCellID
		}
		return values[left].AbsoluteErrorM > values[right].AbsoluteErrorM
	})
	if len(values) > limit {
		values = values[:limit]
	}
	return values
}

func deriveReliefQualityThresholds(report ReliefQualityReport, passport ReliefReferencePassport, evaluatedAreaM2, referenceVolumeM3 float64) ReliefQualityThresholds {
	horizontalVerticalM := math.Tan(report.Slope.ReferenceP95Deg*math.Pi/180) * passport.HorizontalResolutionM
	depthRMSEM := math.Hypot(2*passport.VerticalUncertaintyM, horizontalVerticalM)
	volumePercent := 0.0
	if referenceVolumeM3 > 0 {
		volumePercent = 100 * 2 * passport.VerticalUncertaintyM * evaluatedAreaM2 / referenceVolumeM3
	}
	return ReliefQualityThresholds{
		Method: "две вертикальные неопределённости плюс перенос горизонтального шага через P95 уклона; " +
			"изобаты — два горизонтальных шага; объём — две вертикальные неопределённости по площади; " +
			"геометрические пороги являются фиксированными воротами проекта",
		DepthRMSEMaxM:              depthRMSEM,
		DepthP95AbsoluteErrorMaxM:  1.5 * depthRMSEM,
		VolumeDeviationMaxPercent:  volumePercent,
		SlopeRMSEMaxDeg:            math.Atan2(2*passport.VerticalUncertaintyM, passport.HorizontalResolutionM) * 180 / math.Pi,
		IsobathP95DistanceMaxM:     2 * passport.HorizontalResolutionM,
		NearestNodeMaxPercent:      5,
		P05QuadQualityMin:          0.2,
		TargetSizeComplianceMinPct: 80,
	}
}

func buildDepthBandMetrics(bounds, referenceAreaM2, evaluatedAreaM2 []float64, isobaths []IsobathPreservationMetrics, resolutionM float64) []DepthBandPreservationMetrics {
	contourLengthsM := make(map[float64]float64, len(isobaths))
	for _, metric := range isobaths {
		contourLengthsM[metric.DepthM] = metric.ReferenceLengthKM * 1000
	}
	result := make([]DepthBandPreservationMetrics, 0, len(bounds))
	for index, lower := range bounds {
		var upper *float64
		boundaryLengthM := contourLengthsM[lower]
		if index+1 < len(bounds) {
			value := bounds[index+1]
			upper = &value
			boundaryLengthM += contourLengthsM[value]
		}
		referenceM2, evaluatedM2 := referenceAreaM2[index], evaluatedAreaM2[index]
		deviationM2 := math.Abs(evaluatedM2 - referenceM2)
		deviationPercent, tolerancePercent := 0.0, 0.0
		if referenceM2 > 0 {
			deviationPercent = 100 * deviationM2 / referenceM2
			tolerancePercent = 100 * 2 * resolutionM * boundaryLengthM / referenceM2
		}
		accepted := deviationM2 == 0 || (referenceM2 > 0 && deviationPercent <= tolerancePercent)
		result = append(result, DepthBandPreservationMetrics{
			LowerDepthM: lower, UpperDepthM: upper,
			ReferenceAreaKM2: referenceM2 / 1e6, EvaluatedAreaKM2: evaluatedM2 / 1e6,
			AbsoluteDeviationKM2: deviationM2 / 1e6, AbsoluteDeviationPct: deviationPercent,
			ResolutionTolerancePct: tolerancePercent, Accepted: accepted,
		})
	}
	return result
}

func evaluateSamplingQuality(model Model) SamplingQualityMetrics {
	metrics := SamplingQualityMetrics{}
	for nodeID := 1; nodeID < len(model.Nodes); nodeID++ {
		node := model.Nodes[nodeID]
		if node.WaterDepthM == nil {
			continue
		}
		metrics.AssignedNodeCount++
		if node.SamplingMethod == SamplingNearest {
			metrics.NearestNodeCount++
		}
		if node.SourceDistanceM != nil {
			metrics.MaxSourceDistanceM = math.Max(metrics.MaxSourceDistanceM, *node.SourceDistanceM)
		}
	}
	if metrics.AssignedNodeCount > 0 {
		metrics.NearestNodePercent = 100 * float64(metrics.NearestNodeCount) / float64(metrics.AssignedNodeCount)
	}
	return metrics
}

type meshQualityAccumulator struct {
	count      int
	qualitySum float64
	histogram  [101]int64
}

func evaluateReliefMeshQuality(model Model, config ReliefQualityConfig) (MeshReliefQualityMetrics, error) {
	result := MeshReliefQualityMetrics{CellCount: len(model.Cells)}
	global := &meshQualityAccumulator{}
	regions := make(map[CellRegion]*meshQualityAccumulator)
	for _, cell := range model.Cells {
		if cell.ID <= 0 || cell.ID > len(model.Mesh.Cells) {
			return MeshReliefQualityMetrics{}, fmt.Errorf("производная ячейка %d не согласована с топологией", cell.ID)
		}
		quality := mesh.QuadrilateralQuality(model.Mesh.Nodes, model.Mesh.Cells[cell.ID-1])
		accumulateMeshQuality(global, quality)
		accumulator := regions[cell.Region]
		if accumulator == nil {
			accumulator = &meshQualityAccumulator{}
			regions[cell.Region] = accumulator
		}
		accumulateMeshQuality(accumulator, quality)
	}
	if global.count > 0 {
		result.MeanQuadQuality = global.qualitySum / float64(global.count)
		result.P05QuadQuality = meshQualityP05(global.histogram, int64(global.count))
	}
	for _, region := range []CellRegion{RegionCoast, RegionShelf, RegionSlope, RegionBasin, RegionUnclassified} {
		accumulator := regions[region]
		if accumulator == nil {
			continue
		}
		result.Regions = append(result.Regions, MeshRegionQualityMetrics{
			Region: region, CellCount: accumulator.count,
			MeanQuadQuality: accumulator.qualitySum / float64(accumulator.count),
			P05QuadQuality:  meshQualityP05(accumulator.histogram, int64(accumulator.count)),
		})
	}
	zoneStats, err := mesh.EvaluateAdaptiveEdges(model.Mesh, model.Mesh, config.TargetSizeM, config.TargetZones, config.TargetZoneNames)
	if err != nil {
		return MeshReliefQualityMetrics{}, err
	}
	result.TargetZones = zoneStats
	var observations int64
	for _, zone := range zoneStats {
		result.TargetSizeCompliancePercent += zone.WithinTolerancePct * float64(zone.EdgeObservationCount)
		observations += zone.EdgeObservationCount
	}
	if observations > 0 {
		result.TargetSizeCompliancePercent /= float64(observations)
	}
	return result, nil
}

func accumulateMeshQuality(accumulator *meshQualityAccumulator, quality float64) {
	quality = math.Max(0, math.Min(1, quality))
	accumulator.count++
	accumulator.qualitySum += quality
	accumulator.histogram[int(math.Round(quality*100))]++
}

func meshQualityP05(histogram [101]int64, count int64) float64 {
	target := int64(math.Ceil(0.05 * float64(count)))
	seen := int64(0)
	for index, value := range histogram {
		seen += value
		if seen >= target {
			return float64(index) / 100
		}
	}
	return 0
}

func evaluateReliefAcceptance(report *ReliefQualityReport) {
	add := func(format string, values ...any) {
		report.Reasons = append(report.Reasons, fmt.Sprintf(format, values...))
	}
	if report.Depth.MissingCellCount > 0 {
		add("%d опорных ячеек остались без выборки проверяемой глубины", report.Depth.MissingCellCount)
	}
	if report.Depth.RMSEM > report.Thresholds.DepthRMSEMaxM {
		add("RMSE глубины %.3f м превышает порог %.3f м", report.Depth.RMSEM, report.Thresholds.DepthRMSEMaxM)
	}
	if report.Depth.P95AbsoluteErrorM > report.Thresholds.DepthP95AbsoluteErrorMaxM {
		add("P95 ошибки глубины %.3f м превышает порог %.3f м", report.Depth.P95AbsoluteErrorM, report.Thresholds.DepthP95AbsoluteErrorMaxM)
	}
	if math.Abs(report.Volume.DeviationPercent) > report.Thresholds.VolumeDeviationMaxPercent {
		add("отклонение объёма %.3f%% превышает порог %.3f%%", report.Volume.DeviationPercent, report.Thresholds.VolumeDeviationMaxPercent)
	}
	if report.Slope.EvaluationCellCount == 0 {
		add("не рассчитана ошибка уклона")
	} else if report.Slope.RMSEDeg > report.Thresholds.SlopeRMSEMaxDeg {
		add("RMSE уклона %.3f° превышает порог %.3f°", report.Slope.RMSEDeg, report.Thresholds.SlopeRMSEMaxDeg)
	}
	for _, band := range report.DepthBands {
		if !band.Accepted {
			upper := "∞"
			if band.UpperDepthM != nil {
				upper = fmt.Sprintf("%.0f", *band.UpperDepthM)
			}
			add("площадь зоны %.0f–%s м отклонена на %.3f%% при допуске %.3f%%", band.LowerDepthM, upper, band.AbsoluteDeviationPct, band.ResolutionTolerancePct)
		}
	}
	for _, isobath := range report.Isobaths {
		if !isobath.Comparable {
			add("изобата %.0f м несопоставима: %s", isobath.DepthM, isobath.Reason)
		} else if isobath.P95DistanceM > report.Thresholds.IsobathP95DistanceMaxM {
			add("P95 расстояния изобаты %.0f м: %.3f м превышает порог %.3f м", isobath.DepthM, isobath.P95DistanceM, report.Thresholds.IsobathP95DistanceMaxM)
		}
	}
	if report.Sampling.NearestNodePercent > report.Thresholds.NearestNodeMaxPercent {
		add("доля ближайших замен %.3f%% превышает порог %.3f%%", report.Sampling.NearestNodePercent, report.Thresholds.NearestNodeMaxPercent)
	}
	if report.Mesh.P05QuadQuality < report.Thresholds.P05QuadQualityMin {
		add("P05 качества ячеек %.3f ниже порога %.3f", report.Mesh.P05QuadQuality, report.Thresholds.P05QuadQualityMin)
	}
	if report.Mesh.TargetSizeCompliancePercent < report.Thresholds.TargetSizeComplianceMinPct {
		add("соответствие целевому размеру %.3f%% ниже порога %.3f%%", report.Mesh.TargetSizeCompliancePercent, report.Thresholds.TargetSizeComplianceMinPct)
	}
	report.Reasons = compactQualityReasons(report.Reasons)
	report.MetricsAccepted = len(report.Reasons) == 0
}

func compactQualityReasons(values []string) []string {
	seen := make(map[string]bool, len(values))
	result := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value != "" && !seen[value] {
			seen[value] = true
			result = append(result, value)
		}
	}
	if result == nil {
		return []string{}
	}
	return result
}
