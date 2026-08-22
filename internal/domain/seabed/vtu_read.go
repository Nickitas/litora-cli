package seabed

import (
	"encoding/json"
	"encoding/xml"
	"fmt"
	"math"
	"os"
	"strconv"
	"strings"

	"coastal-geometry/internal/domain/mesh"
)

// VTUDocument содержит восстановленную модель и пространственные метаданные
// из VTK XML UnstructuredGrid.
type VTUDocument struct {
	Metadata ExportMetadata
	Model    Model
}

// ReadVTU выполняет строгое обратное чтение VTU, созданного Lito. Проверяются
// XML-схема, размеры массивов, VTK_QUAD, нумерация, единицы, коды, Z и глубина.
func ReadVTU(path string) (VTUDocument, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return VTUDocument{}, fmt.Errorf("чтение VTU %q: %w", path, err)
	}
	var document vtuXMLDocument
	if err := xml.Unmarshal(data, &document); err != nil {
		return VTUDocument{}, fmt.Errorf("разбор XML VTU %q: %w", path, err)
	}
	if document.XMLName.Local != "VTKFile" || document.Type != "UnstructuredGrid" || document.Version != "0.1" || document.ByteOrder != "LittleEndian" {
		return VTUDocument{}, fmt.Errorf("VTU должен быть UnstructuredGrid 0.1 с порядком байтов LittleEndian")
	}
	pointCount, err := parseVTUNonNegativeInteger(document.Grid.Piece.NumberOfPoints, "NumberOfPoints")
	if err != nil || pointCount == 0 {
		return VTUDocument{}, fmt.Errorf("VTU не содержит корректное число узлов")
	}
	cellCount, err := parseVTUNonNegativeInteger(document.Grid.Piece.NumberOfCells, "NumberOfCells")
	if err != nil || cellCount == 0 {
		return VTUDocument{}, fmt.Errorf("VTU не содержит корректное число ячеек")
	}

	fieldArrays, err := indexVTUArrays(document.Grid.FieldData.Arrays, "FieldData")
	if err != nil {
		return VTUDocument{}, err
	}
	metadataArray, ok := fieldArrays["lito_metadata_utf8_json"]
	if !ok {
		return VTUDocument{}, fmt.Errorf("VTU не содержит FieldData lito_metadata_utf8_json")
	}
	metadataBytes, err := decodeVTUBytes(metadataArray)
	if err != nil {
		return VTUDocument{}, err
	}
	var metadata ExportMetadata
	if err := json.Unmarshal(metadataBytes, &metadata); err != nil {
		return VTUDocument{}, fmt.Errorf("декодирование метаданных VTU: %w", err)
	}
	if err := validateExportMetadata(metadata); err != nil {
		return VTUDocument{}, fmt.Errorf("метаданные VTU: %w", err)
	}

	pointArrays, err := indexVTUArrays(document.Grid.Piece.PointData.Arrays, "PointData")
	if err != nil {
		return VTUDocument{}, err
	}
	cellArrays, err := indexVTUArrays(document.Grid.Piece.CellData.Arrays, "CellData")
	if err != nil {
		return VTUDocument{}, err
	}
	pointsArrays, err := indexVTUArrays(document.Grid.Piece.Points.Arrays, "Points")
	if err != nil {
		return VTUDocument{}, err
	}
	cellsArrays, err := indexVTUArrays(document.Grid.Piece.Cells.Arrays, "Cells")
	if err != nil {
		return VTUDocument{}, err
	}

	coordinates, err := requireVTUFloats(pointsArrays, "Points", "Float64", metadata.HorizontalLinearUnit+","+metadata.HorizontalLinearUnit+","+metadata.VerticalUnit, pointCount, 3)
	if err != nil {
		return VTUDocument{}, err
	}
	pointIDs, err := requireVTUIntegers(pointArrays, "id", "Int32", "1", pointCount, 1)
	if err != nil {
		return VTUDocument{}, err
	}
	longitudes, err := requireVTUFloats(pointArrays, "longitude_deg", "Float64", "degree", pointCount, 1)
	if err != nil {
		return VTUDocument{}, err
	}
	latitudes, err := requireVTUFloats(pointArrays, "latitude_deg", "Float64", "degree", pointCount, 1)
	if err != nil {
		return VTUDocument{}, err
	}
	elevations, err := requireVTUFloats(pointArrays, "elevation_m", "Float64", metadata.VerticalUnit, pointCount, 1)
	if err != nil {
		return VTUDocument{}, err
	}
	depths, err := requireVTUFloats(pointArrays, "water_depth_m", "Float64", metadata.VerticalUnit, pointCount, 1)
	if err != nil {
		return VTUDocument{}, err
	}
	samplingCodes, err := requireVTUIntegers(pointArrays, "sampling_method_code", "Int32", "code", pointCount, 1)
	if err != nil {
		return VTUDocument{}, err
	}
	sourceDistances, err := requireVTUFloats(pointArrays, "source_distance_m", "Float64", metadata.HorizontalLinearUnit, pointCount, 1)
	if err != nil {
		return VTUDocument{}, err
	}
	qualityCodes, err := requireVTUIntegers(pointArrays, "quality_code", "Int32", "code", pointCount, 1)
	if err != nil {
		return VTUDocument{}, err
	}
	isBoundaryValues, err := requireVTUIntegers(pointArrays, "is_boundary", "UInt8", "1", pointCount, 1)
	if err != nil {
		return VTUDocument{}, err
	}
	boundaryCodes, err := requireVTUIntegers(pointArrays, "boundary_code", "Int32", "code", pointCount, 1)
	if err != nil {
		return VTUDocument{}, err
	}

	model := Model{
		Mesh:  mesh.Mesh{Nodes: make([]mesh.Point, pointCount+1)},
		Nodes: make([]Node, pointCount+1),
		CellDerivation: CellDerivationMetadata{
			AreaMethod: cellAreaMethod, ElevationMethod: cellElevationMethod,
			SlopeAspectMethod: cellSlopeAspectMethod, RoughnessMethod: cellRoughnessMethod,
			RegionMethod: cellRegionMethod, HorizontalUnit: "m", ElevationUnit: "m",
			SlopeUnit: "degree", AspectConvention: cellAspectConvention,
			RegionThresholds: metadata.RegionThresholds,
		},
	}
	for index := 0; index < pointCount; index++ {
		id := index + 1
		if pointIDs[index] != id {
			return VTUDocument{}, fmt.Errorf("PointData id[%d]=%d вместо %d", index, pointIDs[index], id)
		}
		x, y, z := coordinates[3*index], coordinates[3*index+1], coordinates[3*index+2]
		if math.Abs(z-elevations[index]) > 1e-9 {
			return VTUDocument{}, fmt.Errorf("узел %d: Z не совпадает с elevation_m", id)
		}
		if elevations[index] > 1e-9 || depths[index] < 0 || math.Abs(depths[index]-math.Max(0, -elevations[index])) > 1e-8 {
			return VTUDocument{}, fmt.Errorf("узел %d: elevation_m и water_depth_m не согласованы", id)
		}
		if longitudes[index] < -180 || longitudes[index] > 180 || latitudes[index] < -90 || latitudes[index] > 90 {
			return VTUDocument{}, fmt.Errorf("узел %d содержит координаты WGS 84 вне допустимого диапазона", id)
		}
		sampling, err := samplingMethodFromCode(float64(samplingCodes[index]))
		if err != nil || sampling == SamplingNotSampled {
			return VTUDocument{}, fmt.Errorf("узел %d содержит неприемлемый sampling_method_code %d", id, samplingCodes[index])
		}
		quality, err := qualityFlagFromCode(float64(qualityCodes[index]))
		if err != nil || quality == QualityNoData || quality == QualityRejected {
			return VTUDocument{}, fmt.Errorf("узел %d содержит неприемлемый quality_code %d", id, qualityCodes[index])
		}
		boundary, err := boundaryKindFromCode(float64(boundaryCodes[index]))
		if err != nil {
			return VTUDocument{}, fmt.Errorf("узел %d: %w", id, err)
		}
		if isBoundaryValues[index] != 0 && isBoundaryValues[index] != 1 {
			return VTUDocument{}, fmt.Errorf("узел %d содержит is_boundary=%d", id, isBoundaryValues[index])
		}
		isBoundary := isBoundaryValues[index] == 1
		if isBoundary != (boundary != BoundaryNone) {
			return VTUDocument{}, fmt.Errorf("узел %d: is_boundary не согласован с boundary_code", id)
		}
		var sourceDistance *float64
		if math.Abs(sourceDistances[index]-metadata.NoDataSentinel) > 1e-12 {
			if !validFiniteNonNegative(sourceDistances[index]) {
				return VTUDocument{}, fmt.Errorf("узел %d содержит некорректное расстояние до источника", id)
			}
			sourceDistance = floatPointer(sourceDistances[index])
		}
		model.Mesh.Nodes[id] = mesh.Point{
			X: x, Y: y, LongitudeDeg: longitudes[index], LatitudeDeg: latitudes[index], GeographicCoordinatesSet: true,
		}
		model.Nodes[id] = Node{
			ID: id, XM: x, YM: y, LongitudeDeg: longitudes[index], LatitudeDeg: latitudes[index],
			ElevationM: floatPointer(elevations[index]), WaterDepthM: floatPointer(depths[index]),
			SamplingMethod: sampling, SourceDistanceM: sourceDistance, QualityFlag: quality,
			IsBoundary: isBoundary, BoundaryKind: boundary,
		}
		incrementBoundaryCount(&model.Reconciliation.BoundaryCounts, boundary)
	}

	connectivity, err := requireVTUIntegers(cellsArrays, "connectivity", "Int32", "zero_based_point_index", 4*cellCount, 1)
	if err != nil {
		return VTUDocument{}, err
	}
	offsets, err := requireVTUIntegers(cellsArrays, "offsets", "Int32", "connectivity_offset", cellCount, 1)
	if err != nil {
		return VTUDocument{}, err
	}
	types, err := requireVTUIntegers(cellsArrays, "types", "UInt8", "VTK_cell_type", cellCount, 1)
	if err != nil {
		return VTUDocument{}, err
	}
	cellIDs, err := requireVTUIntegers(cellArrays, "id", "Int32", "1", cellCount, 1)
	if err != nil {
		return VTUDocument{}, err
	}
	area, err := requireVTUFloats(cellArrays, "area_m2", "Float64", "m2", cellCount, 1)
	if err != nil {
		return VTUDocument{}, err
	}
	elevationMin, err := requireVTUFloats(cellArrays, "elevation_min_m", "Float64", metadata.VerticalUnit, cellCount, 1)
	if err != nil {
		return VTUDocument{}, err
	}
	elevationMax, err := requireVTUFloats(cellArrays, "elevation_max_m", "Float64", metadata.VerticalUnit, cellCount, 1)
	if err != nil {
		return VTUDocument{}, err
	}
	elevationMean, err := requireVTUFloats(cellArrays, "elevation_mean_m", "Float64", metadata.VerticalUnit, cellCount, 1)
	if err != nil {
		return VTUDocument{}, err
	}
	depthMean, err := requireVTUFloats(cellArrays, "water_depth_mean_m", "Float64", metadata.VerticalUnit, cellCount, 1)
	if err != nil {
		return VTUDocument{}, err
	}
	slope, err := requireVTUFloats(cellArrays, "slope_deg", "Float64", "degree", cellCount, 1)
	if err != nil {
		return VTUDocument{}, err
	}
	aspectValues, err := requireVTUFloats(cellArrays, "aspect_deg", "Float64", "degree", cellCount, 1)
	if err != nil {
		return VTUDocument{}, err
	}
	roughness, err := requireVTUFloats(cellArrays, "roughness_m", "Float64", metadata.VerticalUnit, cellCount, 1)
	if err != nil {
		return VTUDocument{}, err
	}
	regionCodes, err := requireVTUIntegers(cellArrays, "region_code", "Int32", "code", cellCount, 1)
	if err != nil {
		return VTUDocument{}, err
	}
	cellQualityCodes, err := requireVTUIntegers(cellArrays, "cell_quality_code", "Int32", "code", cellCount, 1)
	if err != nil {
		return VTUDocument{}, err
	}
	qualityScore, err := requireVTUFloats(cellArrays, "quality_score", "Float64", "1", cellCount, 1)
	if err != nil {
		return VTUDocument{}, err
	}

	model.Mesh.Cells = make([]mesh.Cell, 0, cellCount)
	model.Cells = make([]Cell, 0, cellCount)
	for index := 0; index < cellCount; index++ {
		id := index + 1
		if cellIDs[index] != id || offsets[index] != 4*id || types[index] != vtkQuadCellType {
			return VTUDocument{}, fmt.Errorf("ячейка %d имеет несогласованные id, offset или VTK type", id)
		}
		nodeIDs := [4]int{}
		for local := 0; local < 4; local++ {
			pointIndex := connectivity[4*index+local]
			if pointIndex < 0 || pointIndex >= pointCount {
				return VTUDocument{}, fmt.Errorf("ячейка %d ссылается на отсутствующий VTK point %d", id, pointIndex)
			}
			nodeIDs[local] = pointIndex + 1
		}
		var aspect *float64
		if math.Abs(aspectValues[index]-metadata.NoDataSentinel) > 1e-12 {
			if aspectValues[index] < 0 || aspectValues[index] >= 360 {
				return VTUDocument{}, fmt.Errorf("ячейка %d содержит некорректный aspect_deg", id)
			}
			aspect = floatPointer(aspectValues[index])
		}
		region, err := cellRegionFromCode(float64(regionCodes[index]))
		if err != nil || region == RegionUnclassified {
			return VTUDocument{}, fmt.Errorf("ячейка %d содержит неприемлемый region_code %d", id, regionCodes[index])
		}
		cellQuality, err := cellQualityFromCode(float64(cellQualityCodes[index]))
		if err != nil {
			return VTUDocument{}, fmt.Errorf("ячейка %d: %w", id, err)
		}
		model.Mesh.Cells = append(model.Mesh.Cells, mesh.Cell{Nodes: nodeIDs, NodeCount: 4})
		model.Cells = append(model.Cells, Cell{
			ID: id, NodeIDs: nodeIDs, AreaM2: area[index],
			ElevationMinM: elevationMin[index], ElevationMaxM: elevationMax[index], ElevationMeanM: elevationMean[index],
			WaterDepthMeanM: depthMean[index], SlopeDeg: slope[index], AspectDeg: aspect,
			RoughnessM: roughness[index], Region: region, QualityFlag: cellQuality, QualityScore: qualityScore[index],
		})
	}
	model.Mesh.QuadCount = cellCount

	boundaryArray, ok := fieldArrays["lito_boundary_edges"]
	if !ok {
		return VTUDocument{}, fmt.Errorf("VTU не содержит FieldData lito_boundary_edges")
	}
	boundaryValues, boundaryTuples, err := decodeVTUIntegers(boundaryArray, "Int32", "index,index,code", 3)
	if err != nil {
		return VTUDocument{}, err
	}
	model.Mesh.BoundaryEdges = make([][2]int, 0, boundaryTuples)
	model.BoundaryEdges = make([]BoundaryEdge, 0, boundaryTuples)
	for index := 0; index < boundaryTuples; index++ {
		a, b := boundaryValues[3*index], boundaryValues[3*index+1]
		if a < 0 || a >= pointCount || b < 0 || b >= pointCount || a == b {
			return VTUDocument{}, fmt.Errorf("FieldData содержит некорректное граничное ребро %d–%d", a, b)
		}
		kind, err := boundaryKindFromCode(float64(boundaryValues[3*index+2]))
		if err != nil || kind == BoundaryNone {
			return VTUDocument{}, fmt.Errorf("FieldData содержит некорректный тип граничного ребра")
		}
		edge := [2]int{a + 1, b + 1}
		model.Mesh.BoundaryEdges = append(model.Mesh.BoundaryEdges, edge)
		model.BoundaryEdges = append(model.BoundaryEdges, BoundaryEdge{NodeIDs: edge, Kind: kind})
	}
	if err := validateBoundaryNodeCodes(model); err != nil {
		return VTUDocument{}, err
	}
	if err := restoreMSHSummaries(&model); err != nil {
		return VTUDocument{}, err
	}
	if err := validateMSHModel(model); err != nil {
		return VTUDocument{}, fmt.Errorf("проверка прочитанного VTU: %w", err)
	}
	return VTUDocument{Metadata: metadata, Model: model}, nil
}

type vtuXMLDocument struct {
	XMLName   xml.Name               `xml:"VTKFile"`
	Type      string                 `xml:"type,attr"`
	Version   string                 `xml:"version,attr"`
	ByteOrder string                 `xml:"byte_order,attr"`
	Grid      vtuXMLUnstructuredGrid `xml:"UnstructuredGrid"`
}

type vtuXMLUnstructuredGrid struct {
	FieldData vtuXMLSection `xml:"FieldData"`
	Piece     vtuXMLPiece   `xml:"Piece"`
}

type vtuXMLPiece struct {
	NumberOfPoints string        `xml:"NumberOfPoints,attr"`
	NumberOfCells  string        `xml:"NumberOfCells,attr"`
	PointData      vtuXMLSection `xml:"PointData"`
	CellData       vtuXMLSection `xml:"CellData"`
	Points         vtuXMLSection `xml:"Points"`
	Cells          vtuXMLSection `xml:"Cells"`
}

type vtuXMLSection struct {
	Arrays []vtuXMLDataArray `xml:"DataArray"`
}

type vtuXMLDataArray struct {
	Type               string              `xml:"type,attr"`
	Name               string              `xml:"Name,attr"`
	NumberOfComponents string              `xml:"NumberOfComponents,attr"`
	NumberOfTuples     string              `xml:"NumberOfTuples,attr"`
	Format             string              `xml:"format,attr"`
	Information        []vtuXMLInformation `xml:"InformationKey"`
	Value              string              `xml:",chardata"`
}

type vtuXMLInformation struct {
	Name     string `xml:"name,attr"`
	Location string `xml:"location,attr"`
	Value    string `xml:",chardata"`
}

func indexVTUArrays(arrays []vtuXMLDataArray, section string) (map[string]vtuXMLDataArray, error) {
	result := make(map[string]vtuXMLDataArray, len(arrays))
	for _, array := range arrays {
		if strings.TrimSpace(array.Name) == "" {
			return nil, fmt.Errorf("%s содержит DataArray без имени", section)
		}
		if _, duplicate := result[array.Name]; duplicate {
			return nil, fmt.Errorf("%s содержит повторный DataArray %q", section, array.Name)
		}
		result[array.Name] = array
	}
	return result, nil
}

func requireVTUFloats(arrays map[string]vtuXMLDataArray, name, dataType, unit string, tuples, components int) ([]float64, error) {
	array, ok := arrays[name]
	if !ok {
		return nil, fmt.Errorf("VTU не содержит DataArray %q", name)
	}
	values, actualTuples, err := decodeVTUFloats(array, dataType, unit, components)
	if err != nil {
		return nil, err
	}
	if actualTuples != tuples {
		return nil, fmt.Errorf("DataArray %q содержит %d tuples вместо %d", name, actualTuples, tuples)
	}
	return values, nil
}

func requireVTUIntegers(arrays map[string]vtuXMLDataArray, name, dataType, unit string, tuples, components int) ([]int, error) {
	array, ok := arrays[name]
	if !ok {
		return nil, fmt.Errorf("VTU не содержит DataArray %q", name)
	}
	values, actualTuples, err := decodeVTUIntegers(array, dataType, unit, components)
	if err != nil {
		return nil, err
	}
	if actualTuples != tuples {
		return nil, fmt.Errorf("DataArray %q содержит %d tuples вместо %d", name, actualTuples, tuples)
	}
	return values, nil
}

func decodeVTUFloats(array vtuXMLDataArray, dataType, unit string, components int) ([]float64, int, error) {
	tuples, fields, err := validateVTUArray(array, dataType, unit, components)
	if err != nil {
		return nil, 0, err
	}
	values := make([]float64, len(fields))
	for index, field := range fields {
		values[index], err = strconv.ParseFloat(field, 64)
		if err != nil || !finite(values[index]) {
			return nil, 0, fmt.Errorf("DataArray %q содержит некорректное число %q", array.Name, field)
		}
	}
	return values, tuples, nil
}

func decodeVTUIntegers(array vtuXMLDataArray, dataType, unit string, components int) ([]int, int, error) {
	tuples, fields, err := validateVTUArray(array, dataType, unit, components)
	if err != nil {
		return nil, 0, err
	}
	values := make([]int, len(fields))
	for index, field := range fields {
		values[index], err = strconv.Atoi(field)
		if err != nil {
			return nil, 0, fmt.Errorf("DataArray %q содержит некорректное целое %q", array.Name, field)
		}
	}
	return values, tuples, nil
}

func decodeVTUBytes(array vtuXMLDataArray) ([]byte, error) {
	values, _, err := decodeVTUIntegers(array, "UInt8", "", 1)
	if err != nil {
		return nil, err
	}
	result := make([]byte, len(values))
	for index, value := range values {
		if value < 0 || value > 255 {
			return nil, fmt.Errorf("DataArray %q содержит байт вне диапазона", array.Name)
		}
		result[index] = byte(value)
	}
	return result, nil
}

func validateVTUArray(array vtuXMLDataArray, dataType, unit string, components int) (int, []string, error) {
	if array.Type != dataType || array.Format != "ascii" {
		return 0, nil, fmt.Errorf("DataArray %q должен иметь type=%s и format=ascii", array.Name, dataType)
	}
	actualComponents, err := parseVTUNonNegativeInteger(array.NumberOfComponents, "NumberOfComponents")
	if err != nil || actualComponents != components {
		return 0, nil, fmt.Errorf("DataArray %q должен иметь %d компонентов", array.Name, components)
	}
	if unit != "" {
		actualUnit, err := vtuArrayUnit(array)
		if err != nil {
			return 0, nil, err
		}
		if actualUnit != unit {
			return 0, nil, fmt.Errorf("DataArray %q имеет единицу %q вместо %q", array.Name, actualUnit, unit)
		}
	}
	fields := strings.Fields(array.Value)
	tuples := 0
	if strings.TrimSpace(array.NumberOfTuples) == "" {
		if len(fields)%components != 0 {
			return 0, nil, fmt.Errorf("DataArray %q содержит неполный tuple", array.Name)
		}
		tuples = len(fields) / components
	} else {
		tuples, err = parseVTUNonNegativeInteger(array.NumberOfTuples, "NumberOfTuples")
		if err != nil {
			return 0, nil, fmt.Errorf("DataArray %q содержит некорректное NumberOfTuples", array.Name)
		}
	}
	if len(fields) != tuples*components {
		return 0, nil, fmt.Errorf("DataArray %q содержит %d значений вместо %d", array.Name, len(fields), tuples*components)
	}
	return tuples, fields, nil
}

func vtuArrayUnit(array vtuXMLDataArray) (string, error) {
	unit := ""
	for _, information := range array.Information {
		if information.Name != "UNITS_LABEL" || information.Location != "vtkDataArray" {
			continue
		}
		if unit != "" {
			return "", fmt.Errorf("DataArray %q повторяет UNITS_LABEL", array.Name)
		}
		unit = strings.TrimSpace(information.Value)
	}
	if unit == "" {
		return "", fmt.Errorf("DataArray %q не содержит UNITS_LABEL", array.Name)
	}
	return unit, nil
}

func parseVTUNonNegativeInteger(value, label string) (int, error) {
	parsed, err := strconv.Atoi(strings.TrimSpace(value))
	if err != nil || parsed < 0 {
		return 0, fmt.Errorf("некорректное %s %q", label, value)
	}
	return parsed, nil
}
