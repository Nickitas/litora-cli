package seabed

import (
	"fmt"
	"math"

	"coastal-geometry/internal/domain/mesh"
)

const (
	cellAreaMethod        = "формула Гаусса по X/Y в LAEA"
	cellElevationMethod   = "минимум, максимум и среднее четырёх узлов"
	cellSlopeAspectMethod = "плоскость наименьших квадратов z = a·x + b·y + c"
	cellRoughnessMethod   = "RMS остатков узловых отметок относительно плоскости ячейки"
	cellRegionMethod      = "береговая топология и пороги средней положительной глубины"
	cellAspectConvention  = "азимут наибольшего понижения: 0° север, 90° восток, по часовой стрелке"
)

func normalizeRegionThresholds(thresholds RegionThresholds) (RegionThresholds, error) {
	if thresholds == (RegionThresholds{}) {
		return DefaultRegionThresholds(), nil
	}
	if !finite(thresholds.CoastMaxDepthM) || !finite(thresholds.ShelfMaxDepthM) || !finite(thresholds.SlopeMaxDepthM) {
		return RegionThresholds{}, fmt.Errorf("пороги морфометрических зон должны быть конечными")
	}
	if thresholds.CoastMaxDepthM < 0 || thresholds.ShelfMaxDepthM <= thresholds.CoastMaxDepthM || thresholds.SlopeMaxDepthM <= thresholds.ShelfMaxDepthM {
		return RegionThresholds{}, fmt.Errorf(
			"пороги глубины должны удовлетворять 0 ≤ coast < shelf < slope, получено %.3f, %.3f, %.3f м",
			thresholds.CoastMaxDepthM,
			thresholds.ShelfMaxDepthM,
			thresholds.SlopeMaxDepthM,
		)
	}
	return thresholds, nil
}

func (model *Model) deriveCells(thresholds RegionThresholds) error {
	metadata := CellDerivationMetadata{
		AreaMethod:        cellAreaMethod,
		ElevationMethod:   cellElevationMethod,
		SlopeAspectMethod: cellSlopeAspectMethod,
		RoughnessMethod:   cellRoughnessMethod,
		RegionMethod:      cellRegionMethod,
		HorizontalUnit:    "m",
		ElevationUnit:     "m",
		SlopeUnit:         "degree",
		AspectConvention:  cellAspectConvention,
		RegionThresholds:  thresholds,
		Summary: CellSummary{
			TotalCellCount: len(model.Mesh.Cells),
		},
	}
	model.Cells = make([]Cell, 0, len(model.Mesh.Cells))
	slopeSum := 0.0
	roughnessSum := 0.0
	maxSlope := 0.0
	maxRoughness := 0.0
	for cellIndex, sourceCell := range model.Mesh.Cells {
		if sourceCell.NodeCount != 4 {
			return fmt.Errorf("ячейка %d содержит %d узлов: BATHY-03 принимает итоговую четырёхугольную сетку", cellIndex+1, sourceCell.NodeCount)
		}
		if !cellHasCompleteDepth(model.Nodes, sourceCell) {
			metadata.Summary.NoDataCellCount++
			continue
		}
		cell, err := deriveCell(cellIndex+1, sourceCell, model.Nodes, thresholds)
		if err != nil {
			return err
		}
		model.Cells = append(model.Cells, cell)
		metadata.Summary.AssignedCellCount++
		incrementRegionCount(&metadata.Summary.RegionCounts, cell.Region)
		incrementCellQualityCount(&metadata.Summary.QualityCounts, cell.QualityFlag)
		slopeSum += cell.SlopeDeg
		roughnessSum += cell.RoughnessM
		if cell.SlopeDeg > maxSlope {
			maxSlope = cell.SlopeDeg
		}
		if cell.RoughnessM > maxRoughness {
			maxRoughness = cell.RoughnessM
		}
	}
	if metadata.Summary.TotalCellCount > 0 {
		metadata.Summary.CoveragePercent = 100 * float64(metadata.Summary.AssignedCellCount) / float64(metadata.Summary.TotalCellCount)
	}
	if metadata.Summary.AssignedCellCount > 0 {
		metadata.Summary.MeanSlopeDeg = floatPointer(slopeSum / float64(metadata.Summary.AssignedCellCount))
		metadata.Summary.MaxSlopeDeg = floatPointer(maxSlope)
		metadata.Summary.MeanRoughnessM = floatPointer(roughnessSum / float64(metadata.Summary.AssignedCellCount))
		metadata.Summary.MaxRoughnessM = floatPointer(maxRoughness)
	}
	model.CellDerivation = metadata
	return nil
}

func deriveCell(cellID int, source mesh.Cell, nodes []Node, thresholds RegionThresholds) (Cell, error) {
	seen := make(map[int]bool, 4)
	coordinates := make([]mesh.Point, 4)
	elevations := make([]float64, 4)
	elevationSum := 0.0
	elevationMin := math.Inf(1)
	elevationMax := math.Inf(-1)
	coastal := false
	for index, nodeID := range source.Nodes {
		if nodeID <= 0 || nodeID >= len(nodes) || seen[nodeID] {
			return Cell{}, fmt.Errorf("ячейка %d содержит некорректный или повторный узел %d", cellID, nodeID)
		}
		seen[nodeID] = true
		node := nodes[nodeID]
		if node.ElevationM == nil {
			return Cell{}, fmt.Errorf("ячейка %d передана на расчёт без полной глубины", cellID)
		}
		coordinates[index] = mesh.Point{X: node.XM, Y: node.YM}
		elevations[index] = *node.ElevationM
		elevationSum += *node.ElevationM
		elevationMin = math.Min(elevationMin, *node.ElevationM)
		elevationMax = math.Max(elevationMax, *node.ElevationM)
		if node.BoundaryKind == BoundaryCoastline || node.BoundaryKind == BoundaryIsland {
			coastal = true
		}
	}
	areaM2 := polygonAreaM2(coordinates)
	if !finite(areaM2) || areaM2 <= 0 {
		return Cell{}, fmt.Errorf("ячейка %d имеет нулевую или некорректную площадь", cellID)
	}
	elevationMean := elevationSum / 4
	a, b, c, err := fitElevationPlane(coordinates, elevations)
	if err != nil {
		return Cell{}, fmt.Errorf("ячейка %d: %w", cellID, err)
	}
	gradient := math.Hypot(a, b)
	slopeDeg := math.Atan(gradient) * 180 / math.Pi
	var aspectDeg *float64
	if gradient > 1e-12 {
		aspect := math.Atan2(-a, -b) * 180 / math.Pi
		if aspect < 0 {
			aspect += 360
		}
		if aspect >= 360-1e-12 {
			aspect = 0
		}
		aspectDeg = floatPointer(aspect)
	}
	residualSquares := 0.0
	for index, coordinate := range coordinates {
		residual := elevations[index] - (a*coordinate.X + b*coordinate.Y + c)
		residualSquares += residual * residual
	}
	roughnessM := math.Sqrt(residualSquares / 4)
	waterDepthMeanM := math.Max(0, -elevationMean)
	qualityFlag, qualityScore := aggregateCellQuality(source, nodes)
	return Cell{
		ID:              cellID,
		NodeIDs:         source.Nodes,
		AreaM2:          areaM2,
		ElevationMinM:   elevationMin,
		ElevationMaxM:   elevationMax,
		ElevationMeanM:  elevationMean,
		WaterDepthMeanM: waterDepthMeanM,
		SlopeDeg:        slopeDeg,
		AspectDeg:       aspectDeg,
		RoughnessM:      roughnessM,
		Region:          classifyCellRegion(waterDepthMeanM, coastal, thresholds),
		QualityFlag:     qualityFlag,
		QualityScore:    qualityScore,
	}, nil
}

func cellHasCompleteDepth(nodes []Node, cell mesh.Cell) bool {
	for index := 0; index < cell.NodeCount; index++ {
		nodeID := cell.Nodes[index]
		if nodeID <= 0 || nodeID >= len(nodes) || nodes[nodeID].ElevationM == nil || nodes[nodeID].WaterDepthM == nil {
			return false
		}
	}
	return true
}

func polygonAreaM2(points []mesh.Point) float64 {
	doubleArea := 0.0
	for index, point := range points {
		next := points[(index+1)%len(points)]
		doubleArea += point.X*next.Y - next.X*point.Y
	}
	return math.Abs(doubleArea) / 2
}

func fitElevationPlane(points []mesh.Point, elevations []float64) (a, b, c float64, err error) {
	meanX, meanY, meanZ := 0.0, 0.0, 0.0
	for index, point := range points {
		meanX += point.X
		meanY += point.Y
		meanZ += elevations[index]
	}
	count := float64(len(points))
	meanX /= count
	meanY /= count
	meanZ /= count
	sxx, syy, sxy, sxz, syz := 0.0, 0.0, 0.0, 0.0, 0.0
	for index, point := range points {
		dx := point.X - meanX
		dy := point.Y - meanY
		dz := elevations[index] - meanZ
		sxx += dx * dx
		syy += dy * dy
		sxy += dx * dy
		sxz += dx * dz
		syz += dy * dz
	}
	determinant := sxx*syy - sxy*sxy
	if determinant <= 1e-12*math.Max(1, sxx*syy) {
		return 0, 0, 0, fmt.Errorf("узлы не задают устойчивую плоскость")
	}
	a = (sxz*syy - syz*sxy) / determinant
	b = (syz*sxx - sxz*sxy) / determinant
	c = meanZ - a*meanX - b*meanY
	return a, b, c, nil
}

func classifyCellRegion(meanDepthM float64, coastal bool, thresholds RegionThresholds) CellRegion {
	if coastal || meanDepthM <= thresholds.CoastMaxDepthM {
		return RegionCoast
	}
	if meanDepthM <= thresholds.ShelfMaxDepthM {
		return RegionShelf
	}
	if meanDepthM <= thresholds.SlopeMaxDepthM {
		return RegionSlope
	}
	return RegionBasin
}

func aggregateCellQuality(cell mesh.Cell, nodes []Node) (CellQualityFlag, float64) {
	allVerified := true
	allFallback := true
	score := 0.0
	for _, nodeID := range cell.Nodes {
		node := nodes[nodeID]
		allVerified = allVerified && node.QualityFlag == QualityVerified
		allFallback = allFallback && node.QualityFlag == QualityFallback
		score += nodeQualityScore(node)
	}
	flag := CellQualityMixed
	if allVerified {
		flag = CellQualityVerified
	} else if allFallback {
		flag = CellQualityFallback
	}
	return flag, score / 4
}

func nodeQualityScore(node Node) float64 {
	switch node.QualityFlag {
	case QualityVerified:
		switch node.SamplingMethod {
		case SamplingExact:
			return 1
		case SamplingBilinear:
			return 0.95
		case SamplingIrregular:
			return 0.9
		default:
			return 0.9
		}
	case QualityConstrained:
		return 0.8
	case QualityFallback:
		return 0.6
	default:
		return 0
	}
}

func incrementRegionCount(counts *RegionCounts, region CellRegion) {
	switch region {
	case RegionCoast:
		counts.Coast++
	case RegionShelf:
		counts.Shelf++
	case RegionSlope:
		counts.Slope++
	case RegionBasin:
		counts.Basin++
	default:
		counts.Unclassified++
	}
}

func incrementCellQualityCount(counts *CellQualityCounts, quality CellQualityFlag) {
	switch quality {
	case CellQualityVerified:
		counts.Verified++
	case CellQualityFallback:
		counts.Fallback++
	default:
		counts.Mixed++
	}
}
