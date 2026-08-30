package cli

import (
	"coastal-geometry/internal/domain/coastline"
	"coastal-geometry/internal/domain/fractal"
	"coastal-geometry/internal/domain/geometry"
	svgrender "coastal-geometry/internal/render/svg"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"strings"
)

type fractalSeriesOptions struct {
	Title            string
	Prefix           string
	MetricsBaseName  string
	Iterations       int
	OriginalBase     []geometry.LatLon
	ModelBase        []geometry.LatLon
	IncludeDimension bool
	Builder          func([]geometry.LatLon, int) []geometry.LatLon
}

func writeCoastlineSVG(points, renderPoints []geometry.LatLon, output, defaultName string, ctx exportContext, outputPathManager *OutputPathManager) error {
	filename := outputPathManager.SVGPath(defaultName)
	if output != "" {
		resolvedPath := outputPathManager.ResolveUserPath(output, "svg")
		if resolvedPath != "" {
			filename = resolvedPath
		}
	}

	if len(renderPoints) == 0 {
		renderPoints = points
	}

	realSummary := summarizePolyline(points)
	renderSummary := summarizePolyline(renderPoints)
	visualHints := coastline.BuildVisualizationHints(points)
	validationSummary := coastline.BuildValidationSummary(points)

	// Build base document
	doc := svgrender.Document{
		Title:    "Береговая линия",
		Subtitle: "Реальные загруженные данные: исходная географическая полилиния; SVG использует упрощённую копию только для рендера",
		Layers: []svgrender.Layer{
			{
				Label:       "Реальная исходная полилиния",
				Points:      renderPoints,
				LengthKM:    realSummary.LengthKM,
				Stroke:      "#1f6f8b",
				StrokeWidth: 3.5,
				Opacity:     1,
			},
		},
		Highlights: makeCoastlineHighlights(visualHints),
		StatCards:  makeValidationStatCards(ctx.Validation, validationSummary),
		Alerts:     makeCoastlineAlerts(ctx.Validation, visualHints),
		Meta: []string{
			fmt.Sprintf("Точек в расчёте: %d", realSummary.PointsCount),
			fmt.Sprintf("Точек в SVG: %d", renderSummary.PointsCount),
			fmt.Sprintf("Длина в расчёте: %.0f км", realSummary.LengthKM),
			fmt.Sprintf("Длина SVG-копии: %.0f км", renderSummary.LengthKM),
			fmt.Sprintf("Подсвечено длинных сегментов: %d", len(visualHints.LongSegments)),
			fmt.Sprintf("Валидация: %d исправлений, %d предупреждений", len(ctx.Validation.Fixes), len(ctx.Validation.Warnings)),
		},
	}

	// Wrap with enhanced options if enabled
	enhancedDoc := wrapDocumentForEnhanced(doc, ctx.Config, points, 0, nil, nil)

	if ctx.Config.EnableEnhanced {
		if err := svgrender.DrawEnhancedSVG(enhancedDoc, filename); err != nil {
			return err
		}
	} else {
		if err := svgrender.DrawDocument(doc, filename); err != nil {
			return err
		}
	}

	metricsPath := outputPathManager.MetricsPath("coastline.metrics.json")
	metrics := coastlineArtifactMetrics{
		GeneratedAt:          nowTimestamp(),
		Command:              canonicalCommandPath(ctx.Command),
		Dataset:              ctx.Dataset,
		Source:               ctx.Source,
		SVGFile:              filename,
		Real:                 realSummary,
		Render:               renderSummary,
		RenderSimplification: summarizeSimplification(points, renderPoints),
		Highlights:           coastlineHighlightsMetricsFromHints(visualHints),
		Validation:           validationMetricsFromData(ctx.Validation, validationSummary),
	}
	if err := writeMetricsJSON(metricsPath, metrics); err != nil {
		return err
	}

	fmt.Printf("SVG сохранён: %s\n", filename)
	fmt.Printf("Метрики сохранены: %s\n", metricsPath)
	return nil
}

func writeErosionSVGSeries(originalBase, modelBase []geometry.LatLon, snapshots [][]geometry.LatLon, steps int, strength float64, seed int64, waveOptions geometry.WaveErosionOptions, output string, ctx exportContext, outputPathManager *OutputPathManager, sedimentResult *geometry.SedimentTransportResult) error {
	ctx.Report = buildReportMetadata(ctx, originalBase, fmt.Sprintf("strength=%.6f|seed=%d|steps=%d|years=%.6f", strength, seed, steps, waveOptions.YearsPerStep))
	reportKind := "Расчётный отчёт"
	if ctx.Scenario.ScenarioStatus == geometry.ScenarioStatusDemo {
		reportKind = "Демонстрационный отчёт (demo)"
	}
	fmt.Printf("🧾 %s: эксперимент %s, источник %s, версия данных %s\n", reportKind, ctx.Report.ExperimentID, ctx.Report.GeometrySource, ctx.Report.InputDataVersion[:12])
	outputDir := outputPathManager.SVGDir()

	if len(originalBase) == 0 {
		originalBase = modelBase
	}
	if len(modelBase) == 0 {
		modelBase = originalBase
	}

	referenceRender := simplifyForSeriesSVG(originalBase).Points
	if len(referenceRender) == 0 {
		referenceRender = originalBase
	}

	renderSnapshots := make([][]geometry.LatLon, len(snapshots))
	lengths := make([]float64, len(snapshots))
	areas := make([]float64, len(snapshots))
	for i, snap := range snapshots {
		renderSnapshots[i] = simplifyForSeriesSVG(snap).Points
		lengths[i] = geometry.PolylineLength(snap)
		areas[i] = geometry.Area(snap)
	}

	referenceSummary := summarizePolyline(originalBase)
	referenceRenderSummary := summarizePolyline(referenceRender)
	modelSummary := summarizePolyline(modelBase)
	modelSimplification := summarizeSimplification(originalBase, modelBase)
	visualHints := coastline.BuildVisualizationHints(originalBase)
	validationSummary := coastline.BuildValidationSummary(originalBase)

	erosionChanges := make([][]svgrender.ErosionChangePoint, len(snapshots))
	maxAbsChange := 0.0
	for step := 1; step < len(snapshots); step++ {
		erosionChanges[step] = erosionChangePoints(snapshots[step-1], snapshots[step], waveOptions.YearsPerStep)
		for _, point := range erosionChanges[step] {
			maxAbsChange = math.Max(maxAbsChange, math.Abs(point.ChangePerUnit))
		}
	}
	changeUnit := "м/шаг"
	if waveOptions.YearsPerStep > 0 && waveOptions.YearsPerStep != 1 {
		changeUnit = "м/год"
	}
	if maxAbsChange > 0 {
		fmt.Printf("🎨 Цветовая шкала эрозии: размыв — красный, накопление — синий, нейтральная зона — серая; максимум %.2f %s\n", maxAbsChange, changeUnit)
	}

	stepMetrics := make([]erosionStepMetrics, 0, len(snapshots))

	for step := 0; step < len(snapshots); step++ {
		filename := filepath.Join(outputDir, fmt.Sprintf("%s_%d.svg", "erosion_step", step))
		layers := makeErosionLayers(referenceRender, referenceSummary.LengthKM, renderSnapshots, lengths, step)

		meta := []string{
			fmt.Sprintf("Реальная линия: %.0f км, %d т.", referenceSummary.LengthKM, referenceSummary.PointsCount),
			fmt.Sprintf("База модели: %.0f км, %d т. (%+.1f%% к реальной)", modelSummary.LengthKM, modelSummary.PointsCount, modelSimplification.LengthDeltaPercent),
			fmt.Sprintf("Шаг %d: %.0f км, %d т. расчёт / %d т. SVG", step, lengths[step], len(snapshots[step]), len(renderSnapshots[step])),
			fmt.Sprintf("Площадь: %.0f км²", areas[step]),
		}
		meta = append(meta, fmt.Sprintf("Эрозия: базовый отступ %.0f м, seed=%d", strength, seed))
		meta = append(meta, fmt.Sprintf("Волны: %.0f° от севера, ветер %.1f м/с, сектор ±%.0f°, fetch <= %.0f км", waveOptions.WindSourceDirectionDeg, waveOptions.WindSpeedMetersPerSecond, waveOptions.FetchSpreadDeg, waveOptions.MaxFetchMeters/1000))
		metadataSubtitle := reportMetadataLines(ctx.Report)
		if ctx.Scenario.ScenarioStatus == geometry.ScenarioStatusDemo {
			metadataSubtitle += "\nDEMO: не использовать для оценки годового размыва, калибровки или публикации"
		}

		alerts := makeCoastlineAlerts(ctx.Validation, visualHints)
		if ctx.Scenario.ScenarioStatus == geometry.ScenarioStatusDemo {
			alerts = append([]string{"DEMO: не использовать для оценки годового размыва, калибровки или публикации"}, alerts...)
		}

		// Build document and apply enhanced options if enabled
		doc := svgrender.Document{
			Title: "Волновая эрозия береговой линии",
			Subtitle: fmt.Sprintf("Модель: волновая эрозия · шаг %d · сетка ячеек\nВолны: %.0f° от севера · ветер %.1f м/с · сектор ±%.0f° · fetch ≤ %.0f км\nОтступ %.0f м · seed=%d · масштаб глубины %.0f м · экспозиция %.2f · samples=%d\n%s",
				step, waveOptions.WindSourceDirectionDeg, waveOptions.WindSpeedMetersPerSecond, waveOptions.FetchSpreadDeg, waveOptions.MaxFetchMeters/1000,
				strength, seed, waveOptions.DepthScaleMeters, waveOptions.ExposurePower, waveOptions.FetchSamples, metadataSubtitle),
			Layers:    layers,
			StatCards: makeValidationStatCards(ctx.Validation, validationSummary),
			Alerts:    alerts,
			Meta:      meta,
		}

		if ctx.Config.EnableEnhanced {
			fmt.Printf("🔧 Расширенный режим включён для шага эрозии\n")
			// Подключаем сетку эрозионного моделирования.
			enhancedDoc := wrapErosionDocumentForEnhanced(doc, ctx.Config, referenceRender, waveOptions)
			if step > 0 && len(erosionChanges[step]) > 0 {
				enhancedDoc.ErosionChangeOptions = &svgrender.ErosionChangeOptions{
					Show:             true,
					Points:           erosionChanges[step],
					MaxAbsChange:     maxAbsChange,
					NeutralThreshold: math.Max(0.1, maxAbsChange*0.03),
					UnitLabel:        changeUnit,
				}
			}
			// Добавляем транспорт наносов, если доступны его состояния.
			if sedimentResult != nil && len(sedimentResult.States) > 0 {
				// Merge sediment options into the enhanced document
				sedimentEnhanced := wrapDocumentForEnhanced(doc, ctx.Config, referenceRender, waveOptions.WindSourceDirectionDeg, sedimentResult, renderSnapshots[step])
				enhancedDoc.SedimentTransportOptions = sedimentEnhanced.SedimentTransportOptions
			}
			if err := svgrender.DrawEnhancedSVG(enhancedDoc, filename); err != nil {
				return err
			}
		} else {
			if err := svgrender.DrawDocument(doc, filename); err != nil {
				return err
			}
		}

		stepMetrics = append(stepMetrics, erosionStepMetrics{
			Step:         step,
			SVGFile:      filename,
			Points:       len(snapshots[step]),
			LengthKM:     lengths[step],
			RenderPoints: len(renderSnapshots[step]),
			AreaKM:       areas[step],
			MeanChange:   meanErosionChange(erosionChanges[step]),
			MaxChange:    maxErosionChange(erosionChanges[step]),
			ChangeUnit:   changeUnit,
		})

		fmt.Printf("SVG сохранён: %s\n", filename)
	}

	metricsPath := outputPathManager.MetricsPath("erosion.metrics.json")
	erosionCellSize, erosionBufferKM, _ := adaptiveGridParameters(originalBase)
	reproducibility := reproducibilityForGeometry(originalBase)
	reproducibility.GridType = "erosion"
	reproducibility.GridCellSizeMeters = erosionCellSize
	reproducibility.GridBufferKM = erosionBufferKM
	fmt.Printf("🔐 Воспроизводимость: SHA-256 геометрии %s, точек %d, сетка %.0f м, буфер %.1f км\n",
		reproducibility.InputGeometrySHA256, reproducibility.InputPointCount, erosionCellSize, erosionBufferKM)
	seriesMetrics := erosionSeriesArtifactMetrics{
		GeneratedAt:         nowTimestamp(),
		Command:             canonicalCommandPath(ctx.Command),
		Dataset:             ctx.Dataset,
		Source:              ctx.Source,
		OutputDir:           outputPathManager.BaseDir(),
		ReferenceCoastline:  referenceSummary,
		ReferenceRender:     referenceRenderSummary,
		ModelBase:           modelSummary,
		ModelSimplification: modelSimplification,
		ErosionStrength:     strength,
		ErosionSeed:         seed,
		WaveDirectionDeg:    waveOptions.WindSourceDirectionDeg,
		WindSpeedMS:         waveOptions.WindSpeedMetersPerSecond,
		FetchSpreadDeg:      waveOptions.FetchSpreadDeg,
		FetchSamples:        waveOptions.FetchSamples,
		MaxFetchKM:          waveOptions.MaxFetchMeters / 1000,
		DepthScaleMeters:    waveOptions.DepthScaleMeters,
		ExposurePower:       waveOptions.ExposurePower,
		YearsPerStep:        waveOptions.YearsPerStep,
		Report:              ctx.Report,
		Steps:               stepMetrics,
		Highlights:          coastlineHighlightsMetricsFromHints(visualHints),
		Validation:          validationMetricsFromData(ctx.Validation, validationSummary),
		Reproducibility:     reproducibility,
	}

	if err := writeMetricsJSON(metricsPath, seriesMetrics); err != nil {
		return err
	}
	fmt.Printf("Метрики сохранены: %s\n", metricsPath)
	return nil
}

// erosionChangePoints вычисляет подписанное смещение береговой линии по нормали.
// Положительное значение означает отступ берега (размыв), отрицательное — накопление.
func erosionChangePoints(previous, current []geometry.LatLon, yearsPerStep float64) []svgrender.ErosionChangePoint {
	if len(previous) < 2 || len(previous) != len(current) {
		return nil
	}

	projection := geometry.NewLocalMetricProjection(previous)
	clockwise := geometry.SignedArea(previous) < 0
	points := make([]svgrender.ErosionChangePoint, 0, len(current))
	for index := range current {
		previousProjected := projection.Project(previous[index])
		currentProjected := projection.Project(current[index])
		displacementX := currentProjected.X - previousProjected.X
		displacementY := currentProjected.Y - previousProjected.Y

		leftIndex := index - 1
		rightIndex := index + 1
		if leftIndex < 0 {
			leftIndex = 0
		}
		if rightIndex >= len(previous) {
			rightIndex = len(previous) - 1
		}
		left := projection.Project(previous[leftIndex])
		right := projection.Project(previous[rightIndex])
		tangentX := right.X - left.X
		tangentY := right.Y - left.Y
		tangentLength := math.Hypot(tangentX, tangentY)
		if tangentLength == 0 {
			continue
		}
		tangentX /= tangentLength
		tangentY /= tangentLength

		outwardX, outwardY := tangentY, -tangentX
		if clockwise {
			outwardX, outwardY = -tangentY, tangentX
		}
		change := displacementX*outwardX + displacementY*outwardY
		if yearsPerStep > 0 && yearsPerStep != 1 {
			change /= yearsPerStep
		}
		points = append(points, svgrender.ErosionChangePoint{Point: current[index], ChangePerUnit: change})
	}
	return points
}

func meanErosionChange(points []svgrender.ErosionChangePoint) float64 {
	if len(points) == 0 {
		return 0
	}
	total := 0.0
	for _, point := range points {
		total += point.ChangePerUnit
	}
	return total / float64(len(points))
}

func maxErosionChange(points []svgrender.ErosionChangePoint) float64 {
	maximum := 0.0
	for _, point := range points {
		maximum = math.Max(maximum, math.Abs(point.ChangePerUnit))
	}
	return maximum
}

func writeFractalSeries(opts fractalSeriesOptions, output string, ctx exportContext, outputPathManager *OutputPathManager) error {
	metadataPoints := opts.OriginalBase
	if len(metadataPoints) == 0 {
		metadataPoints = opts.ModelBase
	}
	reportParameters := "method=box-counting|geometry=observed"
	ctx.Report = buildReportMetadata(ctx, metadataPoints, reportParameters)
	reportKind := "Научный отчёт по наблюдениям"
	if ctx.Scenario.ScenarioStatus == geometry.ScenarioStatusDemo {
		reportKind = "Демонстрационный отчёт (demo)"
	} else if ctx.Scenario.ScenarioStatus == geometry.ScenarioStatusUnclassified {
		reportKind = "Расчётный отчёт без подтверждённого исследовательского статуса"
	}
	fmt.Printf("🧾 %s: эксперимент %s, источник %s, версия данных %s\n", reportKind, ctx.Report.ExperimentID, ctx.Report.GeometrySource, ctx.Report.InputDataVersion[:12])
	outputDir := outputPathManager.SVGDir()

	originalBase := opts.OriginalBase
	if len(originalBase) == 0 {
		originalBase = opts.ModelBase
	}
	modelBase := opts.ModelBase
	if len(modelBase) == 0 {
		modelBase = originalBase
	}

	referenceRender := simplifyForSeriesSVG(originalBase).Points
	if len(referenceRender) == 0 {
		referenceRender = originalBase
	}

	iterations := opts.Iterations

	curves := make([][]geometry.LatLon, iterations+1)
	renderCurves := make([][]geometry.LatLon, iterations+1)
	lengths := make([]float64, iterations+1)
	dimensions := make([]*dimensionMetrics, iterations+1)
	maxRawPoints := 0
	maxRenderPoints := 0

	for iter := 0; iter <= iterations; iter++ {
		curves[iter] = opts.Builder(modelBase, iter)
		renderCurves[iter] = simplifyForSeriesSVG(curves[iter]).Points
		lengths[iter] = geometry.PolylineLength(curves[iter])
		if len(curves[iter]) > maxRawPoints {
			maxRawPoints = len(curves[iter])
		}
		if len(renderCurves[iter]) > maxRenderPoints {
			maxRenderPoints = len(renderCurves[iter])
		}
		if opts.IncludeDimension {
			dimensions[iter] = dimensionMetricsFromAnalysis(fractal.AnalyzeBoxCounting(curves[iter]))
		}
	}

	if maxRawPoints > maxRenderPoints {
		fmt.Printf("ℹ️  Упрощение наблюдаемой линии для SVG: %d → %d точек; расчёт выполнен по всем исходным точкам\n", maxRawPoints, maxRenderPoints)
	}

	referenceSummary := summarizePolyline(originalBase)
	referenceRenderSummary := summarizePolyline(referenceRender)
	modelSummary := summarizePolyline(modelBase)
	modelSimplification := summarizeSimplification(originalBase, modelBase)
	visualHints := coastline.BuildVisualizationHints(originalBase)
	validationSummary := coastline.BuildValidationSummary(originalBase)

	iterationsMetrics := make([]fractalIterationMetrics, 0, iterations+1)
	for iter := 0; iter <= iterations; iter++ {
		filename := filepath.Join(outputDir, fmt.Sprintf("%s_%d.svg", opts.Prefix, iter))
		logLogFilename := ""
		if dimensions[iter] != nil && len(dimensions[iter].Samples) > 1 {
			logLogFilename = filepath.Join(outputDir, fmt.Sprintf("%s_loglog_%d.svg", opts.Prefix, iter))
			analysis := fractal.AnalyzeBoxCounting(curves[iter])
			if err := svgrender.DrawLogLogSVG(logLogPlotOptionsFromAnalysis(analysis), logLogFilename); err != nil {
				return err
			}
			dimensions[iter].LogLogSVGFile = logLogFilename
			fmt.Printf("Log-log график сохранён: %s\n", logLogFilename)
		}
		layers := makeFractalLayers(renderCurves[iter:iter+1], lengths[iter:iter+1], iter)
		charts := makeSeriesCharts(dimensions[:iter+1])
		meta := []string{
			fmt.Sprintf("Наблюдаемая линия: %.0f км, %d т.", referenceSummary.LengthKM, referenceSummary.PointsCount),
			fmt.Sprintf("Расчёт: %d исходных точек; SVG: %d точек", len(curves[iter]), len(renderCurves[iter])),
		}
		if dimension := dimensions[iter]; dimension != nil {
			if dimension.Valid {
				stability := "нет"
				if dimension.StableAcrossScales {
					stability = "да"
				}
				meta = append(meta, fmt.Sprintf("D: %.5f, R²=%.4f, устойчива=%s", dimension.Dimension, dimension.RegressionRSquared, stability))
			} else {
				meta = append(meta, fmt.Sprintf("D: н/д, масштабов=%d", dimension.SampleCount))
			}
		}
		subtitle := "Наблюдаемая береговая линия без синтетического преобразования · сетка box-counting"
		if dimensions[iter] != nil {
			subtitle += fmt.Sprintf("\nМасштабов ε: %d · εпредст.=%.0f м · D=%.5f · R²=%.4f\nУстойчивость по масштабам: %t",
				dimensions[iter].SampleCount, dimensions[iter].BoxSizeMeters, dimensions[iter].Dimension, dimensions[iter].RegressionRSquared, dimensions[iter].StableAcrossScales)
			if dimensions[iter].DimensionCI95High > dimensions[iter].DimensionCI95Low {
				subtitle += fmt.Sprintf("\n95%% ДИ размерности: [%.5f; %.5f]", dimensions[iter].DimensionCI95Low, dimensions[iter].DimensionCI95High)
			} else {
				subtitle += "\n95% ДИ размерности: не оценён"
			}
		}
		subtitle += "\n" + reportMetadataLines(ctx.Report)
		if ctx.Scenario.ScenarioStatus == geometry.ScenarioStatusDemo {
			subtitle += "\nDEMO: стартовый сценарий не является исследовательским отчётом"
		}

		// Build document and apply enhanced options if enabled
		doc := svgrender.Document{
			Title:     opts.Title,
			Subtitle:  subtitle,
			Layers:    layers,
			StatCards: makeValidationStatCards(ctx.Validation, validationSummary),
			Charts:    charts,
			Meta:      meta,
		}

		if ctx.Config.EnableEnhanced {
			modeName := "научный режим наблюдений"
			if ctx.Scenario.ScenarioStatus == geometry.ScenarioStatusDemo {
				modeName = "демонстрационный режим (demo)"
			} else if ctx.Scenario.ScenarioStatus == geometry.ScenarioStatusUnclassified {
				modeName = "расчётный режим без подтверждённого исследовательского статуса"
			}
			fmt.Printf("🔧 Включён %s, шаг %d\n", modeName, iter)
			// Для фрактального анализа направление ветра не используется.
			renderConfig := ctx.Config
			if dimensions[iter] != nil {
				renderConfig.BoxCountingBoxSize = dimensions[iter].BoxSizeMeters
				renderConfig.BoxCountingRegressionMin, renderConfig.BoxCountingRegressionMax = regressionBoxSizeRange(dimensions[iter])
				renderConfig.BoxCountingLogLogSVGFile = logLogFilename
			}
			enhancedDoc := wrapDocumentForEnhanced(doc, renderConfig, referenceRender, 0, nil, nil)
			if err := svgrender.DrawEnhancedSVG(enhancedDoc, filename); err != nil {
				return err
			}
		} else {
			if err := svgrender.DrawDocument(doc, filename); err != nil {
				return err
			}
		}

		iterationMetrics := fractalIterationMetrics{
			Iteration:           iter,
			SVGFile:             filename,
			PointsCount:         len(curves[iter]),
			RenderPointsCount:   len(renderCurves[iter]),
			LengthKM:            lengths[iter],
			RelativeToModelBase: safeRatio(lengths[iter], modelSummary.LengthKM),
			RelativeToReference: safeRatio(lengths[iter], referenceSummary.LengthKM),
			Dimension:           dimensions[iter],
		}
		iterationsMetrics = append(iterationsMetrics, iterationMetrics)

		fmt.Printf("SVG сохранён: %s\n", filename)
	}

	metricsPath := outputPathManager.MetricsPath(opts.MetricsBaseName + ".metrics.json")
	reproducibility := reproducibilityForGeometry(originalBase)
	reproducibility.GridType = "box-counting"
	if len(dimensions) > 0 && dimensions[len(dimensions)-1] != nil {
		reproducibility.GridCellSizeMeters = dimensions[len(dimensions)-1].BoxSizeMeters
	} else {
		reproducibility.GridCellSizeMeters, _, _ = adaptiveGridParameters(originalBase)
	}
	_, reproducibility.GridBufferKM, _ = adaptiveGridParameters(originalBase)
	fmt.Printf("🔐 Воспроизводимость: SHA-256 геометрии %s, точек %d, box-counting %.0f м, буфер %.1f км\n",
		reproducibility.InputGeometrySHA256, reproducibility.InputPointCount, reproducibility.GridCellSizeMeters, reproducibility.GridBufferKM)
	seriesMetrics := fractalSeriesArtifactMetrics{
		GeneratedAt:         nowTimestamp(),
		Command:             canonicalCommandPath(ctx.Command),
		Dataset:             ctx.Dataset,
		Source:              ctx.Source,
		Title:               opts.Title,
		GeometryKind:        "наблюдаемая",
		Interpretation:      "Оценка box-counting относится к наблюдаемому набору данных и доступному диапазону масштабов.",
		OutputDir:           outputPathManager.BaseDir(),
		ReferenceCoastline:  referenceSummary,
		ReferenceRender:     referenceRenderSummary,
		ModelBase:           modelSummary,
		ModelSimplification: modelSimplification,
		Report:              ctx.Report,
		Iterations:          iterationsMetrics,
		Highlights:          coastlineHighlightsMetricsFromHints(visualHints),
		Validation:          validationMetricsFromData(ctx.Validation, validationSummary),
		Reproducibility:     reproducibility,
	}
	if err := writeMetricsJSON(metricsPath, seriesMetrics); err != nil {
		return err
	}

	fmt.Printf("Метрики сохранены: %s\n", metricsPath)
	return nil
}

// regressionBoxSizeRange возвращает диапазон размеров ячеек, использованных
// в линейной регрессии log-log box-counting.
func regressionBoxSizeRange(metrics *dimensionMetrics) (float64, float64) {
	if metrics == nil || metrics.RegressionStart < 0 || metrics.RegressionEnd < metrics.RegressionStart || metrics.RegressionEnd >= len(metrics.Samples) {
		return 0, 0
	}
	minimum := metrics.Samples[metrics.RegressionStart].BoxSizeMeters
	maximum := minimum
	for index := metrics.RegressionStart; index <= metrics.RegressionEnd; index++ {
		minimum = math.Min(minimum, metrics.Samples[index].BoxSizeMeters)
		maximum = math.Max(maximum, metrics.Samples[index].BoxSizeMeters)
	}
	return minimum, maximum
}

func makeFractalLayers(curves [][]geometry.LatLon, lengths []float64, iteration int) []svgrender.Layer {
	palette := []string{
		"#1f6f8b",
		"#2c7a7b",
		"#c06c3f",
		"#8b3f5c",
		"#6f5f1f",
		"#3f6b4b",
		"#4a5d23",
	}

	layers := make([]svgrender.Layer, 0, len(curves))
	for i := range curves {
		layers = append(layers, svgrender.Layer{
			Label:       "Наблюдаемая береговая линия",
			Points:      curves[i],
			LengthKM:    lengths[i],
			Stroke:      palette[iteration%len(palette)],
			StrokeWidth: 3.2,
			Opacity:     1,
		})
	}

	return layers
}

func makeErosionLayers(reference []geometry.LatLon, referenceLength float64, snapshots [][]geometry.LatLon, lengths []float64, current int) []svgrender.Layer {
	palette := []string{
		"#1f6f8b",
		"#c06c3f",
		"#2c7a7b",
		"#8b3f5c",
		"#6f5f1f",
		"#3f6b4b",
	}

	if current < 0 || current >= len(snapshots) {
		return nil
	}
	return []svgrender.Layer{{
		Label:       fmt.Sprintf("Эрозионная модель, шаг %d", current),
		Points:      snapshots[current],
		LengthKM:    lengths[current],
		Stroke:      palette[current%len(palette)],
		StrokeWidth: 3.2,
		Opacity:     1,
	}}
}

func safeRatio(value, base float64) float64 {
	if base == 0 {
		return 0
	}
	return value / base
}

func logLogPlotOptionsFromAnalysis(analysis fractal.BoxCountingAnalysis) svgrender.LogLogPlotOptions {
	points := make([]svgrender.LogLogPoint, 0, len(analysis.Samples))
	for index, sample := range analysis.Samples {
		points = append(points, svgrender.LogLogPoint{
			LogInverseScale: sample.LogInvScale,
			LogBoxes:        sample.LogBoxes,
			BoxSizeMeters:   sample.BoxSizeMeters,
			InRegression:    index >= analysis.RegressionStart && index <= analysis.RegressionEnd,
		})
	}
	return svgrender.LogLogPlotOptions{
		Title:          "Аудит box-counting — наблюдаемая линия",
		Subtitle:       "Все масштабы ε и значения N(ε); пунктир — окно линейной регрессии",
		Points:         points,
		Dimension:      analysis.Dimension,
		RSquared:       analysis.RegressionRSquared,
		RegressionFrom: analysis.RegressionStart,
		RegressionTo:   analysis.RegressionEnd,
	}
}

func makeCoastlineHighlights(hints coastline.VisualizationHints) []svgrender.HighlightSegment {
	highlights := make([]svgrender.HighlightSegment, 0, len(hints.LongSegments))
	for _, segment := range hints.LongSegments {
		highlights = append(highlights, svgrender.HighlightSegment{
			Start:       segment.Start,
			End:         segment.End,
			Stroke:      "#c2410c",
			StrokeWidth: 4.8,
			Opacity:     0.95,
		})
	}
	return highlights
}

func makeCoastlineAlerts(report coastline.ValidationReport, hints coastline.VisualizationHints) []string {
	alerts := make([]string, 0, len(report.Warnings)+2)
	if len(hints.LongSegments) > 0 {
		alerts = append(alerts, fmt.Sprintf("Длинные сегменты > 450 км: %d", len(hints.LongSegments)))
		for i, segment := range hints.LongSegments {
			if i >= 3 {
				break
			}
			alerts = append(alerts, fmt.Sprintf("сегмент %d-%d: %.0f км", segment.StartIndex, segment.EndIndex, segment.LengthKM))
		}
	}

	for _, warning := range report.Warnings {
		if strings.HasPrefix(warning, "сегмент ") {
			continue
		}
		alerts = append(alerts, warning)
		if len(alerts) >= 5 {
			break
		}
	}

	if len(alerts) == 0 && len(report.Fixes) > 0 {
		alerts = append(alerts, fmt.Sprintf("Автоисправления: %d", len(report.Fixes)))
	}

	return alerts
}

func makeValidationStatCards(report coastline.ValidationReport, summary coastline.ValidationSummary) []svgrender.StatCard {
	longSegments, threshold := validationIssueCount(summary, coastline.WarningTypeLongSegment)
	duplicateLocations, _ := validationIssueCount(summary, coastline.WarningTypeDuplicateLocation)

	return []svgrender.StatCard{
		{
			Title: "Контроль геометрии",
			Items: []svgrender.StatItem{
				{
					Label: fmt.Sprintf("Сегменты > %.0f км", threshold),
					Value: fmt.Sprintf("%d", longSegments),
					Tone:  warningStatTone(longSegments),
				},
				{
					Label: "Повторы ориентиров",
					Value: fmt.Sprintf("%d", duplicateLocations),
					Tone:  warningStatTone(duplicateLocations),
				},
				{
					Label: "Автоисправления",
					Value: fmt.Sprintf("%d", len(report.Fixes)),
					Tone:  fixStatTone(len(report.Fixes)),
				},
			},
		},
	}
}

func validationIssueCount(summary coastline.ValidationSummary, warningType string) (int, float64) {
	for _, issue := range summary.Issues {
		if issue.WarningType == warningType {
			return issue.Count, issue.ThresholdKM
		}
	}
	return 0, 0
}

func warningStatTone(count int) string {
	if count > 0 {
		return "#c2410c"
	}
	return "#3f6b4b"
}

func fixStatTone(count int) string {
	if count > 0 {
		return "#1f6f8b"
	}
	return "#3f6b4b"
}

func makeSeriesCharts(dimensions []*dimensionMetrics) []svgrender.Chart {
	charts := make([]svgrender.Chart, 0, 1)
	dimensionChart := buildDimensionChart(dimensions)
	if len(dimensionChart.Series) > 0 {
		charts = append(charts, dimensionChart)
	}
	return charts
}

func buildDimensionChart(dimensions []*dimensionMetrics) svgrender.Chart {
	values := make([]float64, len(dimensions))
	hasValues := false
	for i, dimension := range dimensions {
		if dimension == nil || !dimension.Valid {
			values[i] = math.NaN()
			continue
		}
		values[i] = dimension.Dimension
		hasValues = true
	}
	if !hasValues {
		return svgrender.Chart{}
	}

	chart := svgrender.Chart{
		Title: "Размерность D",
		Series: []svgrender.ChartSeries{
			{
				Label:  "Оценка",
				Values: values,
				Stroke: "#8b3f5c",
			},
		},
	}
	return chart
}

func resolveOutputPath(output, defaultName, command string) (string, error) {
	if output == "" {
		output = defaultOutputDir
	}

	if strings.HasSuffix(strings.ToLower(output), ".svg") {
		if command == cmdAll {
			return "", fmt.Errorf("command %q generates multiple SVG files, so --output must be a directory", command)
		}

		dir := filepath.Dir(output)
		if dir != "." {
			if err := os.MkdirAll(dir, 0o755); err != nil {
				return "", fmt.Errorf("create output directory %q: %w", dir, err)
			}
		}

		return filepath.Abs(output)
	}

	if err := os.MkdirAll(output, 0o755); err != nil {
		return "", fmt.Errorf("create output directory %q: %w", output, err)
	}

	return filepath.Abs(filepath.Join(output, defaultName))
}

func resolveSeriesOutputDir(output string) (string, error) {
	if output == "" {
		output = defaultOutputDir
	}

	if strings.HasSuffix(strings.ToLower(output), ".svg") {
		dir := filepath.Dir(output)
		if dir == "." {
			dir = defaultOutputDir
		}
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return "", fmt.Errorf("create output directory %q: %w", dir, err)
		}
		return filepath.Abs(dir)
	}

	if err := os.MkdirAll(output, 0o755); err != nil {
		return "", fmt.Errorf("create output directory %q: %w", output, err)
	}

	return filepath.Abs(output)
}

// Публичные функции-обёртки для cobra-команд

// WriteCoastlineSVG создаёт SVG-визуализацию береговой линии
func WriteCoastlineSVG(points []geometry.LatLon, validation coastline.ValidationReport, outputPathManager *OutputPathManager) error {
	if len(points) == 0 {
		return fmt.Errorf("нет точек для рендеринга")
	}
	if outputPathManager == nil {
		return fmt.Errorf("менеджер выходных путей не задан")
	}
	if err := outputPathManager.EnsureDirectories(); err != nil {
		return err
	}

	ctx := exportContext{
		Command:    "source",
		Dataset:    "coastline",
		Source:     "unknown",
		Validation: validation,
		Config:     config{},
	}

	renderPoints := simplifyForSeriesSVG(points)
	if len(renderPoints.Points) == 0 {
		renderPoints.Points = points
	}

	return writeCoastlineSVG(points, renderPoints.Points, "", "coastline.svg", ctx, outputPathManager)
}

// WriteDimensionSVG создаёт SVG-артефакты оценки box-counting для неизменённой
// наблюдаемой береговой линии. Необязательная классификация маркирует весь
// внешний сценарий как демонстрационный, не меняя геометрический расчёт.
func WriteDimensionSVG(points []geometry.LatLon, outputPathManager *OutputPathManager, dataset, source string, validation coastline.ValidationReport, classifications ...geometry.ScenarioClassification) error {
	if len(points) == 0 {
		return fmt.Errorf("нет наблюдаемых точек")
	}
	if outputPathManager == nil {
		return fmt.Errorf("менеджер выходных путей не задан")
	}
	if err := outputPathManager.EnsureDirectories(); err != nil {
		return err
	}

	ctx := newExportContext("dimension", dataset, source, validation)
	if len(classifications) > 0 {
		ctx = withScenarioClassification(ctx, classifications[0])
	}
	return writeFractalSeries(fractalSeriesOptions{
		Title:            "Фрактальная размерность наблюдаемой береговой линии",
		Prefix:           "dimension",
		MetricsBaseName:  "dimension",
		Iterations:       0,
		OriginalBase:     points,
		ModelBase:        points,
		IncludeDimension: true,
		Builder: func(base []geometry.LatLon, _ int) []geometry.LatLon {
			return append([]geometry.LatLon(nil), base...)
		},
	}, "", ctx, outputPathManager)
}

// WriteErosionSVGSeries создаёт SVG-файлы для каждого шага эрозии и переносит
// необязательный статус сценария в подписи и JSON-метрики.
func WriteErosionSVGSeries(originalBase []geometry.LatLon, snapshots [][]geometry.LatLon, steps int, strength float64, seed int64, waveOptions geometry.WaveErosionOptions, outputPathManager *OutputPathManager, dataset, source string, validation coastline.ValidationReport, classifications ...geometry.ScenarioClassification) error {
	if len(originalBase) == 0 && len(snapshots) == 0 {
		return fmt.Errorf("нет данных для рендеринга")
	}
	if outputPathManager == nil {
		return fmt.Errorf("менеджер выходных путей не задан")
	}
	if err := outputPathManager.EnsureDirectories(); err != nil {
		return err
	}

	ctx := newExportContext("erosion", dataset, source, validation)
	if len(classifications) > 0 {
		ctx = withScenarioClassification(ctx, classifications[0])
	}

	return writeErosionSVGSeries(originalBase, nil, snapshots, steps, strength, seed, waveOptions, "", ctx, outputPathManager, nil)
}
