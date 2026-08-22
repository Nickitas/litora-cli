package seabed

import (
	"bufio"
	"fmt"
	"math"
	"os"
	"strconv"
	"strings"

	"coastal-geometry/internal/domain/mesh"
)

const (
	// FlatMSHSchemaVersion помечает плоскую расчётную сетку без модели дна.
	FlatMSHSchemaVersion = "lito-mesh/v1"
	// SeabedMSHSchemaVersion помечает батиметрическую сетку по контракту проекта.
	SeabedMSHSchemaVersion = "lito-seabed/v1"

	mshNoDataSentinel = -1.0
	physicalCoastline = 1
	physicalIsland    = 2
	physicalOpen      = 3
	physicalSeabed    = 10
)

// MSHModelKind различает плоскую сетку и полноценную модель морского дна.
type MSHModelKind string

const (
	// MSHModelFlat означает, что Z не является батиметрической отметкой.
	MSHModelFlat MSHModelKind = "flat"
	// MSHModelSeabed означает, что Z равен elevation_m, а данные дополнены полями.
	MSHModelSeabed MSHModelKind = "seabed"
)

// MSHMetadata содержит явные признаки схемы MSH. Legacy показывает, что
// плоский файл был создан до появления маркеров lito_model_kind и lito_schema_version.
type MSHMetadata struct {
	ModelKind           MSHModelKind
	SchemaVersion       string
	VerticalCoordinate  string
	NoDataSentinel      float64
	NoDataSentinelSet   bool
	RegionThresholds    RegionThresholds
	RegionThresholdsSet bool
	Legacy              bool
}

// MSHDocument объединяет распознанную схему и прочитанную модель. У плоского
// документа Model содержит геометрию и узлы без глубины, Accepted остаётся false.
type MSHDocument struct {
	Metadata MSHMetadata
	Model    Model
}

// MSHCodeTables фиксирует целочисленное представление категориальных полей в
// скалярных блоках MSH 2.2. Метод возвращает новые карты, чтобы таблицы нельзя
// было изменить для последующих операций записи.
type MSHCodeTables struct {
	SamplingMethod map[SamplingMethod]int
	QualityFlag    map[QualityFlag]int
	BoundaryKind   map[BoundaryKind]int
	CellRegion     map[CellRegion]int
	CellQuality    map[CellQualityFlag]int
}

// DefaultMSHCodeTables возвращает неизменяемый по смыслу набор кодов схемы
// lito-seabed/v1 для документации, отчётов и независимых проверок экспорта.
func DefaultMSHCodeTables() MSHCodeTables {
	return MSHCodeTables{
		SamplingMethod: map[SamplingMethod]int{
			SamplingNotSampled: 0, SamplingExact: 1, SamplingBilinear: 2,
			SamplingNearest: 3, SamplingIrregular: 4, SamplingCoastlineConstraint: 5,
		},
		QualityFlag: map[QualityFlag]int{
			QualityRejected: 0, QualityNoData: 1, QualityFallback: 2,
			QualityConstrained: 3, QualityVerified: 4,
		},
		BoundaryKind: map[BoundaryKind]int{
			BoundaryNone: 0, BoundaryCoastline: 1, BoundaryIsland: 2, BoundaryOpen: 3,
		},
		CellRegion: map[CellRegion]int{
			RegionUnclassified: 0, RegionCoast: 1, RegionShelf: 2, RegionSlope: 3, RegionBasin: 4,
		},
		CellQuality: map[CellQualityFlag]int{
			CellQualityFallback: 1, CellQualityMixed: 2, CellQualityVerified: 3,
		},
	}
}

// WriteMSH2 сохраняет принятую модель дна в Gmsh MSH 2.2. Координата Z равна
// elevation_m; узловые и ячеечные характеристики записываются отдельными
// скалярными $NodeData/$ElementData, а границы получают физические группы.
func WriteMSH2(path string, model Model) error {
	if err := validateMSHModel(model); err != nil {
		return err
	}
	file, err := createOutputFile(path)
	if err != nil {
		return err
	}
	writer := bufio.NewWriterSize(file, 1024*1024)
	writeErr := writeMSHDocument(writer, model)
	if writeErr == nil {
		writeErr = writer.Flush()
	}
	closeErr := file.Close()
	if writeErr != nil {
		return fmt.Errorf("запись батиметрического MSH %q: %w", path, writeErr)
	}
	if closeErr != nil {
		return fmt.Errorf("закрытие батиметрического MSH %q: %w", path, closeErr)
	}
	return nil
}

func writeMSHDocument(writer *bufio.Writer, model Model) error {
	thresholds := model.CellDerivation.RegionThresholds
	if _, err := fmt.Fprintf(writer,
		"$MeshFormat\n2.2 0 8\n$EndMeshFormat\n$Comments\n"+
			"lito_model_kind=seabed\n"+
			"lito_schema_version=%s\n"+
			"lito_vertical_coordinate=elevation_m\n"+
			"lito_nodata_sentinel=%.17g\n"+
			"lito_region_coast_max_depth_m=%.17g\n"+
			"lito_region_shelf_max_depth_m=%.17g\n"+
			"lito_region_slope_max_depth_m=%.17g\n"+
			"$EndComments\n",
		SeabedMSHSchemaVersion,
		mshNoDataSentinel,
		thresholds.CoastMaxDepthM,
		thresholds.ShelfMaxDepthM,
		thresholds.SlopeMaxDepthM,
	); err != nil {
		return err
	}
	if _, err := fmt.Fprint(writer,
		"$PhysicalNames\n4\n"+
			"1 1 \"Береговая линия\"\n"+
			"1 2 \"Берег острова\"\n"+
			"1 3 \"Открытая граница\"\n"+
			"2 10 \"Морское дно\"\n"+
			"$EndPhysicalNames\n$Nodes\n",
	); err != nil {
		return err
	}
	if _, err := fmt.Fprintln(writer, len(model.Nodes)-1); err != nil {
		return err
	}
	for nodeID := 1; nodeID < len(model.Nodes); nodeID++ {
		node := model.Nodes[nodeID]
		if _, err := fmt.Fprintf(writer, "%d %.17g %.17g %.17g\n", nodeID, node.XM, node.YM, *node.ElevationM); err != nil {
			return err
		}
	}
	if _, err := fmt.Fprint(writer, "$EndNodes\n$Elements\n"); err != nil {
		return err
	}
	if _, err := fmt.Fprintln(writer, len(model.Cells)+len(model.BoundaryEdges)); err != nil {
		return err
	}
	for _, cell := range model.Cells {
		if _, err := fmt.Fprintf(writer, "%d 3 2 %d 1 %d %d %d %d\n",
			cell.ID, physicalSeabed, cell.NodeIDs[0], cell.NodeIDs[1], cell.NodeIDs[2], cell.NodeIDs[3]); err != nil {
			return err
		}
	}
	for index, edge := range model.BoundaryEdges {
		physical, _ := boundaryPhysicalCode(edge.Kind)
		if _, err := fmt.Fprintf(writer, "%d 1 2 %d %d %d %d\n",
			len(model.Cells)+index+1, physical, physical, edge.NodeIDs[0], edge.NodeIDs[1]); err != nil {
			return err
		}
	}
	if _, err := fmt.Fprintln(writer, "$EndElements"); err != nil {
		return err
	}

	tables := DefaultMSHCodeTables()
	nodeFields := []mshScalarField{
		{name: "longitude_deg", value: func(id int) float64 { return model.Nodes[id].LongitudeDeg }},
		{name: "latitude_deg", value: func(id int) float64 { return model.Nodes[id].LatitudeDeg }},
		{name: "elevation_m", value: func(id int) float64 { return *model.Nodes[id].ElevationM }},
		{name: "water_depth_m", value: func(id int) float64 { return *model.Nodes[id].WaterDepthM }},
		{name: "sampling_method_code", value: func(id int) float64 { return float64(tables.SamplingMethod[model.Nodes[id].SamplingMethod]) }},
		{name: "source_distance_m", value: func(id int) float64 {
			if model.Nodes[id].SourceDistanceM == nil {
				return mshNoDataSentinel
			}
			return *model.Nodes[id].SourceDistanceM
		}},
		{name: "quality_code", value: func(id int) float64 { return float64(tables.QualityFlag[model.Nodes[id].QualityFlag]) }},
		{name: "boundary_code", value: func(id int) float64 { return float64(tables.BoundaryKind[model.Nodes[id].BoundaryKind]) }},
	}
	for _, field := range nodeFields {
		if err := writeScalarData(writer, "$NodeData", "$EndNodeData", field.name, len(model.Nodes)-1, field.value); err != nil {
			return err
		}
	}

	elementFields := []mshScalarField{
		{name: "area_m2", value: func(id int) float64 { return model.Cells[id-1].AreaM2 }},
		{name: "elevation_min_m", value: func(id int) float64 { return model.Cells[id-1].ElevationMinM }},
		{name: "elevation_max_m", value: func(id int) float64 { return model.Cells[id-1].ElevationMaxM }},
		{name: "elevation_mean_m", value: func(id int) float64 { return model.Cells[id-1].ElevationMeanM }},
		{name: "water_depth_mean_m", value: func(id int) float64 { return model.Cells[id-1].WaterDepthMeanM }},
		{name: "slope_deg", value: func(id int) float64 { return model.Cells[id-1].SlopeDeg }},
		{name: "aspect_deg", value: func(id int) float64 {
			if model.Cells[id-1].AspectDeg == nil {
				return mshNoDataSentinel
			}
			return *model.Cells[id-1].AspectDeg
		}},
		{name: "roughness_m", value: func(id int) float64 { return model.Cells[id-1].RoughnessM }},
		{name: "region_code", value: func(id int) float64 { return float64(tables.CellRegion[model.Cells[id-1].Region]) }},
		{name: "cell_quality_code", value: func(id int) float64 { return float64(tables.CellQuality[model.Cells[id-1].QualityFlag]) }},
		{name: "quality_score", value: func(id int) float64 { return model.Cells[id-1].QualityScore }},
	}
	for _, field := range elementFields {
		if err := writeScalarData(writer, "$ElementData", "$EndElementData", field.name, len(model.Cells), field.value); err != nil {
			return err
		}
	}
	return nil
}

type mshScalarField struct {
	name  string
	value func(int) float64
}

func writeScalarData(writer *bufio.Writer, start, end, name string, count int, value func(int) float64) error {
	if _, err := fmt.Fprintf(writer, "%s\n1\n%q\n1\n0\n3\n0\n1\n%d\n", start, name, count); err != nil {
		return err
	}
	for id := 1; id <= count; id++ {
		if _, err := fmt.Fprintf(writer, "%d %.17g\n", id, value(id)); err != nil {
			return err
		}
	}
	_, err := fmt.Fprintln(writer, end)
	return err
}

func validateMSHModel(model Model) error {
	if !model.Accepted {
		return fmt.Errorf("батиметрический MSH можно построить только для принятой модели без NoData")
	}
	if len(model.Nodes) <= 1 || len(model.Mesh.Nodes) != len(model.Nodes) {
		return fmt.Errorf("узловой слой модели не согласован с расчётной сеткой")
	}
	if len(model.Cells) == 0 || len(model.Cells) != len(model.Mesh.Cells) {
		return fmt.Errorf("ячейки модели не согласованы с расчётной сеткой или содержат NoData")
	}
	if len(model.BoundaryEdges) != len(model.Mesh.BoundaryEdges) {
		return fmt.Errorf("физические типы заданы не для всех граничных рёбер")
	}
	if _, err := normalizeRegionThresholds(model.CellDerivation.RegionThresholds); err != nil {
		return fmt.Errorf("пороги регионов MSH: %w", err)
	}
	tables := DefaultMSHCodeTables()
	for nodeID := 1; nodeID < len(model.Nodes); nodeID++ {
		node := model.Nodes[nodeID]
		point := model.Mesh.Nodes[nodeID]
		if node.ID != nodeID || !finite(node.XM) || !finite(node.YM) ||
			math.Abs(node.XM-point.X) > 1e-9 || math.Abs(node.YM-point.Y) > 1e-9 {
			return fmt.Errorf("узел %d не согласован с геометрией сетки", nodeID)
		}
		if !point.GeographicCoordinatesSet || math.Abs(node.LongitudeDeg-point.LongitudeDeg) > 1e-10 || math.Abs(node.LatitudeDeg-point.LatitudeDeg) > 1e-10 {
			return fmt.Errorf("узел %d не согласован с геопривязкой расчётной сетки", nodeID)
		}
		if !finite(node.LongitudeDeg) || !finite(node.LatitudeDeg) || node.LongitudeDeg < -180 || node.LongitudeDeg > 180 || node.LatitudeDeg < -90 || node.LatitudeDeg > 90 {
			return fmt.Errorf("узел %d содержит некорректные координаты WGS 84", nodeID)
		}
		if node.ElevationM == nil || node.WaterDepthM == nil || !finite(*node.ElevationM) || !finite(*node.WaterDepthM) ||
			*node.ElevationM > 1e-9 || *node.WaterDepthM < 0 || math.Abs(*node.WaterDepthM-math.Max(0, -*node.ElevationM)) > 1e-8 {
			return fmt.Errorf("узел %d содержит несогласованные elevation_m и water_depth_m", nodeID)
		}
		if node.SourceDistanceM != nil && (!finite(*node.SourceDistanceM) || *node.SourceDistanceM < 0) {
			return fmt.Errorf("узел %d содержит некорректное расстояние до источника", nodeID)
		}
		if _, ok := tables.SamplingMethod[node.SamplingMethod]; !ok || node.SamplingMethod == SamplingNotSampled {
			return fmt.Errorf("узел %d содержит недопустимый способ выборки %q", nodeID, node.SamplingMethod)
		}
		if _, ok := tables.QualityFlag[node.QualityFlag]; !ok || node.QualityFlag == QualityNoData || node.QualityFlag == QualityRejected {
			return fmt.Errorf("узел %d содержит неприемлемый флаг качества %q", nodeID, node.QualityFlag)
		}
		if _, ok := tables.BoundaryKind[node.BoundaryKind]; !ok || node.IsBoundary != (node.BoundaryKind != BoundaryNone) {
			return fmt.Errorf("узел %d содержит несогласованный тип границы %q", nodeID, node.BoundaryKind)
		}
	}
	for index, cell := range model.Cells {
		source := model.Mesh.Cells[index]
		if cell.ID != index+1 || source.NodeCount != 4 || cell.NodeIDs != source.Nodes {
			return fmt.Errorf("ячейка %d не согласована с четырёхугольной топологией", index+1)
		}
		values := []float64{cell.AreaM2, cell.ElevationMinM, cell.ElevationMaxM, cell.ElevationMeanM, cell.WaterDepthMeanM, cell.SlopeDeg, cell.RoughnessM, cell.QualityScore}
		for _, value := range values {
			if !finite(value) {
				return fmt.Errorf("ячейка %d содержит неконечную производную характеристику", cell.ID)
			}
		}
		if cell.AreaM2 <= 0 || cell.WaterDepthMeanM < 0 || cell.SlopeDeg < 0 || cell.SlopeDeg >= 90 || cell.RoughnessM < 0 || cell.QualityScore < 0 || cell.QualityScore > 1 {
			return fmt.Errorf("ячейка %d содержит характеристику вне допустимого диапазона", cell.ID)
		}
		if cell.AspectDeg != nil && (!finite(*cell.AspectDeg) || *cell.AspectDeg < 0 || *cell.AspectDeg >= 360) {
			return fmt.Errorf("ячейка %d содержит некорректный азимут", cell.ID)
		}
		if _, ok := tables.CellRegion[cell.Region]; !ok || cell.Region == RegionUnclassified {
			return fmt.Errorf("ячейка %d содержит неизвестный регион %q", cell.ID, cell.Region)
		}
		if _, ok := tables.CellQuality[cell.QualityFlag]; !ok {
			return fmt.Errorf("ячейка %d содержит неизвестное качество %q", cell.ID, cell.QualityFlag)
		}
	}
	for index, boundary := range model.BoundaryEdges {
		if normalizedEdge(boundary.NodeIDs[0], boundary.NodeIDs[1]) != normalizedEdge(model.Mesh.BoundaryEdges[index][0], model.Mesh.BoundaryEdges[index][1]) {
			return fmt.Errorf("граничное ребро %d не согласовано с расчётной сеткой", index+1)
		}
		if _, ok := boundaryPhysicalCode(boundary.Kind); !ok {
			return fmt.Errorf("граничное ребро %d содержит неизвестный тип %q", index+1, boundary.Kind)
		}
	}
	return nil
}

func boundaryPhysicalCode(kind BoundaryKind) (int, bool) {
	switch kind {
	case BoundaryCoastline:
		return physicalCoastline, true
	case BoundaryIsland:
		return physicalIsland, true
	case BoundaryOpen:
		return physicalOpen, true
	default:
		return 0, false
	}
}

// ReadMSH2 читает как новый батиметрический MSH, так и плоский MSH 2.2.
// Наличие глубины определяется только явной схемой, поэтому Z = 0 старого
// файла никогда не превращается в достоверную нулевую батиметрию.
func ReadMSH2(path string) (MSHDocument, error) {
	topology, err := mesh.ReadMSH2(path)
	if err != nil {
		return MSHDocument{}, err
	}
	raw, err := readRawMSH(path)
	if err != nil {
		return MSHDocument{}, err
	}
	metadata, err := raw.metadataValue()
	if err != nil {
		return MSHDocument{}, err
	}
	if metadata.ModelKind == MSHModelFlat {
		return newFlatMSHDocument(topology, metadata)
	}
	return newSeabedMSHDocument(topology, raw, metadata)
}

type rawMSH struct {
	comments          map[string]string
	nodeZ             map[int]float64
	surfaceElementIDs []int
	boundaryKinds     map[[2]int]BoundaryKind
	nodeData          map[string]map[int]float64
	elementData       map[string]map[int]float64
}

func readRawMSH(path string) (rawMSH, error) {
	file, err := os.Open(path)
	if err != nil {
		return rawMSH{}, fmt.Errorf("открытие MSH %q: %w", path, err)
	}
	defer file.Close()
	raw := rawMSH{
		comments:      make(map[string]string),
		nodeZ:         make(map[int]float64),
		boundaryKinds: make(map[[2]int]BoundaryKind),
		nodeData:      make(map[string]map[int]float64),
		elementData:   make(map[string]map[int]float64),
	}
	scanner := bufio.NewScanner(file)
	scanner.Buffer(make([]byte, 64*1024), 4*1024*1024)
	for scanner.Scan() {
		switch strings.TrimSpace(scanner.Text()) {
		case "$Comments":
			if err := readMSHComments(scanner, raw.comments); err != nil {
				return rawMSH{}, err
			}
		case "$Nodes":
			if err := readMSHNodeZ(scanner, raw.nodeZ); err != nil {
				return rawMSH{}, err
			}
		case "$Elements":
			if err := readMSHElementMetadata(scanner, &raw); err != nil {
				return rawMSH{}, err
			}
		case "$NodeData":
			name, values, scalar, err := readMSHDataBlock(scanner, "$EndNodeData")
			if err != nil {
				return rawMSH{}, err
			}
			if scalar {
				if _, duplicate := raw.nodeData[name]; duplicate {
					return rawMSH{}, fmt.Errorf("повторный блок $NodeData %q", name)
				}
				raw.nodeData[name] = values
			}
		case "$ElementData":
			name, values, scalar, err := readMSHDataBlock(scanner, "$EndElementData")
			if err != nil {
				return rawMSH{}, err
			}
			if scalar {
				if _, duplicate := raw.elementData[name]; duplicate {
					return rawMSH{}, fmt.Errorf("повторный блок $ElementData %q", name)
				}
				raw.elementData[name] = values
			}
		}
	}
	if err := scanner.Err(); err != nil {
		return rawMSH{}, fmt.Errorf("чтение MSH %q: %w", path, err)
	}
	return raw, nil
}

func readMSHComments(scanner *bufio.Scanner, comments map[string]string) error {
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "$EndComments" {
			return nil
		}
		key, value, ok := strings.Cut(line, "=")
		if ok {
			comments[strings.TrimSpace(key)] = strings.TrimSpace(value)
		}
	}
	return fmt.Errorf("секция $Comments не завершена маркером $EndComments")
}

func readMSHNodeZ(scanner *bufio.Scanner, values map[int]float64) error {
	count, err := scanMSHNonNegativeInteger(scanner, "число узлов $Nodes")
	if err != nil {
		return err
	}
	for index := 0; index < count; index++ {
		if !scanner.Scan() {
			return fmt.Errorf("секция $Nodes оборвана на узле %d", index+1)
		}
		fields := strings.Fields(scanner.Text())
		if len(fields) < 4 {
			return fmt.Errorf("некорректная строка узла %q", scanner.Text())
		}
		id, idErr := strconv.Atoi(fields[0])
		z, zErr := strconv.ParseFloat(fields[3], 64)
		if idErr != nil || zErr != nil || id <= 0 || !finite(z) {
			return fmt.Errorf("некорректная координата Z узла %q", scanner.Text())
		}
		if _, duplicate := values[id]; duplicate {
			return fmt.Errorf("повторный узел %d в секции $Nodes", id)
		}
		values[id] = z
	}
	if !scanner.Scan() || strings.TrimSpace(scanner.Text()) != "$EndNodes" {
		return fmt.Errorf("секция $Nodes не завершена маркером $EndNodes")
	}
	return nil
}

func readMSHElementMetadata(scanner *bufio.Scanner, raw *rawMSH) error {
	count, err := scanMSHNonNegativeInteger(scanner, "число элементов $Elements")
	if err != nil {
		return err
	}
	for index := 0; index < count; index++ {
		if !scanner.Scan() {
			return fmt.Errorf("секция $Elements оборвана на элементе %d", index+1)
		}
		fields := strings.Fields(scanner.Text())
		if len(fields) < 4 {
			return fmt.Errorf("некорректная строка элемента %q", scanner.Text())
		}
		id, idErr := strconv.Atoi(fields[0])
		elementType, typeErr := strconv.Atoi(fields[1])
		tagCount, tagsErr := strconv.Atoi(fields[2])
		nodeStart := 3 + tagCount
		if idErr != nil || typeErr != nil || tagsErr != nil || id <= 0 || tagCount < 0 || len(fields) < nodeStart {
			return fmt.Errorf("некорректный заголовок элемента %q", scanner.Text())
		}
		switch elementType {
		case 1:
			if len(fields) < nodeStart+2 {
				return fmt.Errorf("линейный элемент %d не содержит два узла", id)
			}
			a, aErr := strconv.Atoi(fields[nodeStart])
			b, bErr := strconv.Atoi(fields[nodeStart+1])
			if aErr != nil || bErr != nil {
				return fmt.Errorf("линейный элемент %d содержит некорректные узлы", id)
			}
			if tagCount > 0 {
				physical, physicalErr := strconv.Atoi(fields[3])
				if physicalErr != nil {
					return fmt.Errorf("линейный элемент %d содержит некорректную физическую группу", id)
				}
				if kind, ok := physicalBoundaryKind(physical); ok {
					key := normalizedEdge(a, b)
					if _, duplicate := raw.boundaryKinds[key]; duplicate {
						return fmt.Errorf("повторное физическое граничное ребро %d–%d", a, b)
					}
					raw.boundaryKinds[key] = kind
				}
			}
		case 2, 3:
			raw.surfaceElementIDs = append(raw.surfaceElementIDs, id)
		}
	}
	if !scanner.Scan() || strings.TrimSpace(scanner.Text()) != "$EndElements" {
		return fmt.Errorf("секция $Elements не завершена маркером $EndElements")
	}
	return nil
}

func readMSHDataBlock(scanner *bufio.Scanner, endMarker string) (string, map[int]float64, bool, error) {
	stringCount, err := scanMSHNonNegativeInteger(scanner, "число строковых тегов блока данных")
	if err != nil {
		return "", nil, false, err
	}
	name := ""
	for index := 0; index < stringCount; index++ {
		if !scanner.Scan() {
			return "", nil, false, fmt.Errorf("блок данных оборван на строковом теге")
		}
		if index == 0 {
			name = strings.Trim(strings.TrimSpace(scanner.Text()), "\"")
		}
	}
	realCount, err := scanMSHNonNegativeInteger(scanner, "число вещественных тегов блока данных")
	if err != nil {
		return "", nil, false, err
	}
	for index := 0; index < realCount; index++ {
		if !scanner.Scan() {
			return "", nil, false, fmt.Errorf("блок данных оборван на вещественном теге")
		}
		if _, err := strconv.ParseFloat(strings.TrimSpace(scanner.Text()), 64); err != nil {
			return "", nil, false, fmt.Errorf("некорректный вещественный тег блока %q", name)
		}
	}
	integerCount, err := scanMSHNonNegativeInteger(scanner, "число целочисленных тегов блока данных")
	if err != nil {
		return "", nil, false, err
	}
	integerTags := make([]int, integerCount)
	for index := range integerTags {
		if !scanner.Scan() {
			return "", nil, false, fmt.Errorf("блок данных %q оборван на целочисленном теге", name)
		}
		integerTags[index], err = strconv.Atoi(strings.TrimSpace(scanner.Text()))
		if err != nil {
			return "", nil, false, fmt.Errorf("некорректный целочисленный тег блока %q", name)
		}
	}
	if len(integerTags) < 3 || integerTags[1] <= 0 || integerTags[2] < 0 {
		return "", nil, false, fmt.Errorf("блок данных %q не содержит числа компонентов и записей", name)
	}
	componentCount, entryCount := integerTags[1], integerTags[2]
	values := make(map[int]float64, entryCount)
	for index := 0; index < entryCount; index++ {
		if !scanner.Scan() {
			return "", nil, false, fmt.Errorf("блок данных %q оборван на записи %d", name, index+1)
		}
		fields := strings.Fields(scanner.Text())
		if len(fields) < componentCount+1 {
			return "", nil, false, fmt.Errorf("некорректная запись блока данных %q", scanner.Text())
		}
		id, idErr := strconv.Atoi(fields[0])
		if idErr != nil || id <= 0 {
			return "", nil, false, fmt.Errorf("некорректный идентификатор записи блока %q", name)
		}
		for component := 0; component < componentCount; component++ {
			value, valueErr := strconv.ParseFloat(fields[component+1], 64)
			if valueErr != nil || !finite(value) {
				return "", nil, false, fmt.Errorf("некорректное значение %d записи %d блока %q", component+1, id, name)
			}
			if componentCount == 1 {
				if _, duplicate := values[id]; duplicate {
					return "", nil, false, fmt.Errorf("повторная запись %d в блоке %q", id, name)
				}
				values[id] = value
			}
		}
	}
	if !scanner.Scan() || strings.TrimSpace(scanner.Text()) != endMarker {
		return "", nil, false, fmt.Errorf("блок данных %q не завершён маркером %s", name, endMarker)
	}
	return name, values, componentCount == 1, nil
}

func scanMSHNonNegativeInteger(scanner *bufio.Scanner, label string) (int, error) {
	if !scanner.Scan() {
		return 0, fmt.Errorf("после %s отсутствует значение", label)
	}
	value, err := strconv.Atoi(strings.TrimSpace(scanner.Text()))
	if err != nil || value < 0 {
		return 0, fmt.Errorf("некорректное значение %s %q", label, scanner.Text())
	}
	return value, nil
}

func (raw rawMSH) metadataValue() (MSHMetadata, error) {
	kind := raw.comments["lito_model_kind"]
	schema := raw.comments["lito_schema_version"]
	if kind == "" && schema == "" {
		return MSHMetadata{ModelKind: MSHModelFlat, Legacy: true}, nil
	}
	if kind == string(MSHModelFlat) {
		if schema != "" && schema != FlatMSHSchemaVersion {
			return MSHMetadata{}, fmt.Errorf("неподдерживаемая схема плоского MSH %q", schema)
		}
		return MSHMetadata{ModelKind: MSHModelFlat, SchemaVersion: schema}, nil
	}
	if kind != string(MSHModelSeabed) || schema != SeabedMSHSchemaVersion {
		return MSHMetadata{}, fmt.Errorf("несогласованные маркеры MSH: model_kind=%q, schema_version=%q", kind, schema)
	}
	metadata := MSHMetadata{
		ModelKind:          MSHModelSeabed,
		SchemaVersion:      schema,
		VerticalCoordinate: raw.comments["lito_vertical_coordinate"],
	}
	if metadata.VerticalCoordinate != "elevation_m" {
		return MSHMetadata{}, fmt.Errorf("батиметрический MSH должен объявлять Z = elevation_m")
	}
	sentinel, err := strconv.ParseFloat(raw.comments["lito_nodata_sentinel"], 64)
	if err != nil || !finite(sentinel) || sentinel >= 0 {
		return MSHMetadata{}, fmt.Errorf("батиметрический MSH содержит некорректный NoData sentinel")
	}
	metadata.NoDataSentinel = sentinel
	metadata.NoDataSentinelSet = true
	thresholds := RegionThresholds{}
	thresholds.CoastMaxDepthM, err = parseCommentFloat(raw.comments, "lito_region_coast_max_depth_m")
	if err != nil {
		return MSHMetadata{}, err
	}
	thresholds.ShelfMaxDepthM, err = parseCommentFloat(raw.comments, "lito_region_shelf_max_depth_m")
	if err != nil {
		return MSHMetadata{}, err
	}
	thresholds.SlopeMaxDepthM, err = parseCommentFloat(raw.comments, "lito_region_slope_max_depth_m")
	if err != nil {
		return MSHMetadata{}, err
	}
	if _, err := normalizeRegionThresholds(thresholds); err != nil {
		return MSHMetadata{}, fmt.Errorf("метаданные порогов регионов MSH: %w", err)
	}
	metadata.RegionThresholds = thresholds
	metadata.RegionThresholdsSet = true
	return metadata, nil
}

func parseCommentFloat(comments map[string]string, name string) (float64, error) {
	value, err := strconv.ParseFloat(comments[name], 64)
	if err != nil || !finite(value) {
		return 0, fmt.Errorf("метаданные MSH не содержат корректное поле %s", name)
	}
	return value, nil
}

func newFlatMSHDocument(topology mesh.Mesh, metadata MSHMetadata) (MSHDocument, error) {
	classification, err := classifyBoundaries(topology, nil)
	if err != nil {
		return MSHDocument{}, fmt.Errorf("классификация границы плоского MSH: %w", err)
	}
	model := Model{
		Mesh:          topology,
		Nodes:         make([]Node, len(topology.Nodes)),
		BoundaryEdges: make([]BoundaryEdge, 0, len(topology.BoundaryEdges)),
		Reasons:       []string{"плоский MSH не содержит батиметрических метаданных lito-seabed/v1"},
	}
	for id := 1; id < len(topology.Nodes); id++ {
		point := topology.Nodes[id]
		kind := classification.nodeKinds[id]
		if kind == "" {
			kind = BoundaryNone
		}
		model.Nodes[id] = Node{
			ID: id, XM: point.X, YM: point.Y,
			LongitudeDeg: point.LongitudeDeg, LatitudeDeg: point.LatitudeDeg,
			SamplingMethod: SamplingNotSampled, QualityFlag: QualityNoData,
			IsBoundary: kind != BoundaryNone, BoundaryKind: kind,
		}
	}
	for _, edge := range topology.BoundaryEdges {
		kind, ok := classification.edgeKinds[normalizedEdge(edge[0], edge[1])]
		if !ok {
			return MSHDocument{}, fmt.Errorf("граничное ребро плоского MSH %d–%d не получило тип", edge[0], edge[1])
		}
		model.BoundaryEdges = append(model.BoundaryEdges, BoundaryEdge{NodeIDs: edge, Kind: kind})
	}
	return MSHDocument{Metadata: metadata, Model: model}, nil
}

func newSeabedMSHDocument(topology mesh.Mesh, raw rawMSH, metadata MSHMetadata) (MSHDocument, error) {
	requiredNodeData := []string{
		"longitude_deg", "latitude_deg", "elevation_m", "water_depth_m",
		"sampling_method_code", "source_distance_m", "quality_code", "boundary_code",
	}
	if err := requireScalarBlocks(raw.nodeData, requiredNodeData, len(topology.Nodes)-1, "узловой"); err != nil {
		return MSHDocument{}, err
	}
	requiredElementData := []string{
		"area_m2", "elevation_min_m", "elevation_max_m", "elevation_mean_m",
		"water_depth_mean_m", "slope_deg", "aspect_deg", "roughness_m",
		"region_code", "cell_quality_code", "quality_score",
	}
	if err := requireScalarBlocks(raw.elementData, requiredElementData, len(topology.Cells), "ячеечный"); err != nil {
		return MSHDocument{}, err
	}
	if len(raw.nodeZ) != len(topology.Nodes)-1 {
		return MSHDocument{}, fmt.Errorf("секция $Nodes не содержит Z для всех узлов")
	}
	if len(raw.surfaceElementIDs) != len(topology.Cells) {
		return MSHDocument{}, fmt.Errorf("число поверхностных элементов не совпадает с числом ячеек")
	}
	model := Model{
		Mesh:          topology,
		Nodes:         make([]Node, len(topology.Nodes)),
		Cells:         make([]Cell, 0, len(topology.Cells)),
		BoundaryEdges: make([]BoundaryEdge, 0, len(topology.BoundaryEdges)),
		CellDerivation: CellDerivationMetadata{
			AreaMethod: cellAreaMethod, ElevationMethod: cellElevationMethod,
			SlopeAspectMethod: cellSlopeAspectMethod, RoughnessMethod: cellRoughnessMethod,
			RegionMethod: cellRegionMethod, HorizontalUnit: "m", ElevationUnit: "m",
			SlopeUnit: "degree", AspectConvention: cellAspectConvention,
			RegionThresholds: metadata.RegionThresholds,
		},
	}
	for id := 1; id < len(topology.Nodes); id++ {
		elevation := raw.nodeData["elevation_m"][id]
		depth := raw.nodeData["water_depth_m"][id]
		if z, ok := raw.nodeZ[id]; !ok || math.Abs(z-elevation) > 1e-9 {
			return MSHDocument{}, fmt.Errorf("узел %d: координата Z не совпадает с elevation_m", id)
		}
		if elevation > 1e-9 || depth < 0 || math.Abs(depth-math.Max(0, -elevation)) > 1e-8 {
			return MSHDocument{}, fmt.Errorf("узел %d: elevation_m и water_depth_m не согласованы", id)
		}
		sampling, err := samplingMethodFromCode(raw.nodeData["sampling_method_code"][id])
		if err != nil {
			return MSHDocument{}, fmt.Errorf("узел %d: %w", id, err)
		}
		if sampling == SamplingNotSampled {
			return MSHDocument{}, fmt.Errorf("узел %d: sampling_method_code указывает на отсутствие выборки", id)
		}
		quality, err := qualityFlagFromCode(raw.nodeData["quality_code"][id])
		if err != nil || quality == QualityNoData || quality == QualityRejected {
			return MSHDocument{}, fmt.Errorf("узел %d: неприемлемый quality_code", id)
		}
		boundary, err := boundaryKindFromCode(raw.nodeData["boundary_code"][id])
		if err != nil {
			return MSHDocument{}, fmt.Errorf("узел %d: %w", id, err)
		}
		sourceDistanceValue := raw.nodeData["source_distance_m"][id]
		var sourceDistance *float64
		if math.Abs(sourceDistanceValue-metadata.NoDataSentinel) > 1e-12 {
			if sourceDistanceValue < 0 {
				return MSHDocument{}, fmt.Errorf("узел %d: отрицательное расстояние до источника", id)
			}
			sourceDistance = floatPointer(sourceDistanceValue)
		}
		longitude := raw.nodeData["longitude_deg"][id]
		latitude := raw.nodeData["latitude_deg"][id]
		if longitude < -180 || longitude > 180 || latitude < -90 || latitude > 90 {
			return MSHDocument{}, fmt.Errorf("узел %d содержит координаты WGS 84 вне допустимого диапазона", id)
		}
		point := topology.Nodes[id]
		model.Nodes[id] = Node{
			ID: id, XM: point.X, YM: point.Y, LongitudeDeg: longitude, LatitudeDeg: latitude,
			ElevationM: floatPointer(elevation), WaterDepthM: floatPointer(depth),
			SamplingMethod: sampling, SourceDistanceM: sourceDistance, QualityFlag: quality,
			IsBoundary: boundary != BoundaryNone, BoundaryKind: boundary,
		}
		incrementBoundaryCount(&model.Reconciliation.BoundaryCounts, boundary)
	}
	for _, edge := range topology.BoundaryEdges {
		kind, ok := raw.boundaryKinds[normalizedEdge(edge[0], edge[1])]
		if !ok {
			return MSHDocument{}, fmt.Errorf("граничное ребро %d–%d не имеет физической группы", edge[0], edge[1])
		}
		model.BoundaryEdges = append(model.BoundaryEdges, BoundaryEdge{NodeIDs: edge, Kind: kind})
	}
	if err := validateBoundaryNodeCodes(model); err != nil {
		return MSHDocument{}, err
	}
	for index, source := range topology.Cells {
		if source.NodeCount != 4 {
			return MSHDocument{}, fmt.Errorf("поверхностный элемент %d не является четырёхугольником", index+1)
		}
		elementID := raw.surfaceElementIDs[index]
		aspectValue := raw.elementData["aspect_deg"][elementID]
		var aspect *float64
		if math.Abs(aspectValue-metadata.NoDataSentinel) > 1e-12 {
			if aspectValue < 0 || aspectValue >= 360 {
				return MSHDocument{}, fmt.Errorf("ячейка %d содержит некорректный азимут", index+1)
			}
			aspect = floatPointer(aspectValue)
		}
		region, err := cellRegionFromCode(raw.elementData["region_code"][elementID])
		if err != nil || region == RegionUnclassified {
			return MSHDocument{}, fmt.Errorf("ячейка %d: некорректный region_code", index+1)
		}
		quality, err := cellQualityFromCode(raw.elementData["cell_quality_code"][elementID])
		if err != nil {
			return MSHDocument{}, fmt.Errorf("ячейка %d: %w", index+1, err)
		}
		cell := Cell{
			ID: index + 1, NodeIDs: source.Nodes,
			AreaM2:          raw.elementData["area_m2"][elementID],
			ElevationMinM:   raw.elementData["elevation_min_m"][elementID],
			ElevationMaxM:   raw.elementData["elevation_max_m"][elementID],
			ElevationMeanM:  raw.elementData["elevation_mean_m"][elementID],
			WaterDepthMeanM: raw.elementData["water_depth_mean_m"][elementID],
			SlopeDeg:        raw.elementData["slope_deg"][elementID], AspectDeg: aspect,
			RoughnessM: raw.elementData["roughness_m"][elementID], Region: region,
			QualityFlag: quality, QualityScore: raw.elementData["quality_score"][elementID],
		}
		model.Cells = append(model.Cells, cell)
	}
	if err := restoreMSHSummaries(&model); err != nil {
		return MSHDocument{}, err
	}
	if err := validateMSHModel(model); err != nil {
		return MSHDocument{}, fmt.Errorf("проверка прочитанного батиметрического MSH: %w", err)
	}
	return MSHDocument{Metadata: metadata, Model: model}, nil
}

func requireScalarBlocks(blocks map[string]map[int]float64, names []string, expectedCount int, label string) error {
	for _, name := range names {
		values, ok := blocks[name]
		if !ok {
			return fmt.Errorf("батиметрический MSH не содержит %s блок %q", label, name)
		}
		if len(values) != expectedCount {
			return fmt.Errorf("%s блок %q содержит %d записей вместо %d", label, name, len(values), expectedCount)
		}
		for id := 1; id <= expectedCount; id++ {
			if _, ok := values[id]; !ok {
				return fmt.Errorf("%s блок %q не содержит запись %d", label, name, id)
			}
		}
	}
	return nil
}

func validateBoundaryNodeCodes(model Model) error {
	strongest := make(map[int]BoundaryKind)
	for _, edge := range model.BoundaryEdges {
		strongest[edge.NodeIDs[0]] = strongerBoundaryKind(strongest[edge.NodeIDs[0]], edge.Kind)
		strongest[edge.NodeIDs[1]] = strongerBoundaryKind(strongest[edge.NodeIDs[1]], edge.Kind)
	}
	for nodeID := 1; nodeID < len(model.Nodes); nodeID++ {
		expected := strongest[nodeID]
		if expected == "" {
			expected = BoundaryNone
		}
		if model.Nodes[nodeID].BoundaryKind != expected {
			return fmt.Errorf("узел %d: boundary_code=%q не согласован с физическими группами рёбер (%q)", nodeID, model.Nodes[nodeID].BoundaryKind, expected)
		}
	}
	return nil
}

func restoreMSHSummaries(model *Model) error {
	summary := CellSummary{TotalCellCount: len(model.Cells), AssignedCellCount: len(model.Cells), CoveragePercent: 100}
	slopeSum, roughnessSum := 0.0, 0.0
	maxSlope, maxRoughness := 0.0, 0.0
	for _, cell := range model.Cells {
		incrementRegionCount(&summary.RegionCounts, cell.Region)
		incrementCellQualityCount(&summary.QualityCounts, cell.QualityFlag)
		slopeSum += cell.SlopeDeg
		roughnessSum += cell.RoughnessM
		maxSlope = math.Max(maxSlope, cell.SlopeDeg)
		maxRoughness = math.Max(maxRoughness, cell.RoughnessM)
	}
	if len(model.Cells) > 0 {
		summary.MeanSlopeDeg = floatPointer(slopeSum / float64(len(model.Cells)))
		summary.MaxSlopeDeg = floatPointer(maxSlope)
		summary.MeanRoughnessM = floatPointer(roughnessSum / float64(len(model.Cells)))
		summary.MaxRoughnessM = floatPointer(maxRoughness)
	}
	model.CellDerivation.Summary = summary
	model.finishSummaries()
	if !model.Accepted {
		return fmt.Errorf("прочитанный батиметрический MSH не прошёл проверку полноты: %v", model.Reasons)
	}
	return nil
}

func physicalBoundaryKind(code int) (BoundaryKind, bool) {
	switch code {
	case physicalCoastline:
		return BoundaryCoastline, true
	case physicalIsland:
		return BoundaryIsland, true
	case physicalOpen:
		return BoundaryOpen, true
	default:
		return BoundaryNone, false
	}
}

func integerMSHCode(value float64) (int, error) {
	code := math.Round(value)
	if !finite(value) || math.Abs(value-code) > 1e-9 {
		return 0, fmt.Errorf("код %.17g не является целым числом", value)
	}
	return int(code), nil
}

func samplingMethodFromCode(value float64) (SamplingMethod, error) {
	code, err := integerMSHCode(value)
	if err != nil {
		return "", err
	}
	for method, candidate := range DefaultMSHCodeTables().SamplingMethod {
		if candidate == code {
			return method, nil
		}
	}
	return "", fmt.Errorf("неизвестный sampling_method_code %d", code)
}

func qualityFlagFromCode(value float64) (QualityFlag, error) {
	code, err := integerMSHCode(value)
	if err != nil {
		return "", err
	}
	for flag, candidate := range DefaultMSHCodeTables().QualityFlag {
		if candidate == code {
			return flag, nil
		}
	}
	return "", fmt.Errorf("неизвестный quality_code %d", code)
}

func boundaryKindFromCode(value float64) (BoundaryKind, error) {
	code, err := integerMSHCode(value)
	if err != nil {
		return "", err
	}
	for kind, candidate := range DefaultMSHCodeTables().BoundaryKind {
		if candidate == code {
			return kind, nil
		}
	}
	return "", fmt.Errorf("неизвестный boundary_code %d", code)
}

func cellRegionFromCode(value float64) (CellRegion, error) {
	code, err := integerMSHCode(value)
	if err != nil {
		return "", err
	}
	for region, candidate := range DefaultMSHCodeTables().CellRegion {
		if candidate == code {
			return region, nil
		}
	}
	return "", fmt.Errorf("неизвестный region_code %d", code)
}

func cellQualityFromCode(value float64) (CellQualityFlag, error) {
	code, err := integerMSHCode(value)
	if err != nil {
		return "", err
	}
	for quality, candidate := range DefaultMSHCodeTables().CellQuality {
		if candidate == code {
			return quality, nil
		}
	}
	return "", fmt.Errorf("неизвестный cell_quality_code %d", code)
}
