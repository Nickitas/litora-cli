package mesh

import (
	"bufio"
	"fmt"
	"math"
	"os"
	"strconv"
	"strings"
)

// ReadMSH2 читает текстовый формат Gmsh MSH 2.2 и сохраняет линейные
// граничные элементы, треугольники и четырёхугольники первого порядка.
func ReadMSH2(path string) (Mesh, error) {
	file, err := os.Open(path)
	if err != nil {
		return Mesh{}, fmt.Errorf("открытие MSH %q: %w", path, err)
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	scanner.Buffer(make([]byte, 64*1024), 4*1024*1024)
	var result Mesh
	var longitudeData, latitudeData bool
	for scanner.Scan() {
		switch strings.TrimSpace(scanner.Text()) {
		case "$Nodes":
			if err := readNodes(scanner, &result); err != nil {
				return Mesh{}, err
			}
		case "$Elements":
			if err := readElements(scanner, &result); err != nil {
				return Mesh{}, err
			}
		case "$NodeData":
			name, err := readNodeData(scanner, &result)
			if err != nil {
				return Mesh{}, err
			}
			switch name {
			case "longitude_deg":
				longitudeData = true
			case "latitude_deg":
				latitudeData = true
			}
		}
	}
	if err := scanner.Err(); err != nil {
		return Mesh{}, fmt.Errorf("чтение MSH %q: %w", path, err)
	}
	if len(result.Nodes) == 0 {
		return Mesh{}, fmt.Errorf("MSH не содержит узлов")
	}
	if longitudeData != latitudeData {
		return Mesh{}, fmt.Errorf("MSH должен содержать оба блока WGS 84: longitude_deg и latitude_deg")
	}
	if longitudeData {
		for nodeID := 1; nodeID < len(result.Nodes); nodeID++ {
			point := result.Nodes[nodeID]
			if point.LongitudeDeg < -180 || point.LongitudeDeg > 180 || point.LatitudeDeg < -90 || point.LatitudeDeg > 90 {
				return Mesh{}, fmt.Errorf("узел %d содержит координаты WGS 84 вне допустимого диапазона", nodeID)
			}
			result.Nodes[nodeID].GeographicCoordinatesSet = true
		}
	}
	return result, nil
}

// WriteMSH2 сохраняет фактическую итоговую сетку в текстовом формате Gmsh
// MSH 2.2. Маркеры lito-mesh/v1 явно показывают, что Z = 0 является плоской
// геометрией, а не достоверной батиметрией. Граничные рёбра и полные
// четырёхугольные ячейки записываются без прореживания.
func WriteMSH2(path string, generated Mesh) error {
	if len(generated.Nodes) <= 1 {
		return fmt.Errorf("итоговая сетка не содержит узлов")
	}
	file, err := os.Create(path)
	if err != nil {
		return fmt.Errorf("создание MSH %q: %w", path, err)
	}
	writer := bufio.NewWriterSize(file, 1024*1024)
	writeErr := func() error {
		if _, err := fmt.Fprint(writer,
			"$MeshFormat\n2.2 0 8\n$EndMeshFormat\n"+
				"$Comments\n"+
				"lito_model_kind=flat\n"+
				"lito_schema_version=lito-mesh/v1\n"+
				"$EndComments\n"+
				"$Nodes\n",
		); err != nil {
			return err
		}
		if _, err := fmt.Fprintln(writer, len(generated.Nodes)-1); err != nil {
			return err
		}
		for node := 1; node < len(generated.Nodes); node++ {
			point := generated.Nodes[node]
			if _, err := fmt.Fprintf(writer, "%d %.12g %.12g 0\n", node, point.X, point.Y); err != nil {
				return err
			}
		}
		if _, err := fmt.Fprint(writer, "$EndNodes\n$Elements\n"); err != nil {
			return err
		}
		elementCount := len(generated.BoundaryEdges) + len(generated.Cells)
		if _, err := fmt.Fprintln(writer, elementCount); err != nil {
			return err
		}
		elementID := 1
		for _, edge := range generated.BoundaryEdges {
			if _, err := fmt.Fprintf(writer, "%d 1 0 %d %d\n", elementID, edge[0], edge[1]); err != nil {
				return err
			}
			elementID++
		}
		for _, cell := range generated.Cells {
			if cell.NodeCount != 4 {
				return fmt.Errorf("итоговый MSH содержит ячейку не из четырёх узлов")
			}
			if _, err := fmt.Fprintf(writer, "%d 3 0 %d %d %d %d\n", elementID, cell.Nodes[0], cell.Nodes[1], cell.Nodes[2], cell.Nodes[3]); err != nil {
				return err
			}
			elementID++
		}
		if _, err := fmt.Fprintln(writer, "$EndElements"); err != nil {
			return err
		}
		geographicNodeCount := 0
		for nodeID := 1; nodeID < len(generated.Nodes); nodeID++ {
			point := generated.Nodes[nodeID]
			if !point.GeographicCoordinatesSet {
				continue
			}
			if math.IsNaN(point.LongitudeDeg) || math.IsInf(point.LongitudeDeg, 0) || point.LongitudeDeg < -180 || point.LongitudeDeg > 180 ||
				math.IsNaN(point.LatitudeDeg) || math.IsInf(point.LatitudeDeg, 0) || point.LatitudeDeg < -90 || point.LatitudeDeg > 90 {
				return fmt.Errorf("узел %d содержит некорректные координаты WGS 84", nodeID)
			}
			geographicNodeCount++
		}
		if geographicNodeCount != 0 && geographicNodeCount != len(generated.Nodes)-1 {
			return fmt.Errorf("координаты WGS 84 заданы только для %d из %d узлов", geographicNodeCount, len(generated.Nodes)-1)
		}
		if geographicNodeCount > 0 {
			if err := writeScalarNodeData(writer, "longitude_deg", generated.Nodes, func(point Point) float64 { return point.LongitudeDeg }); err != nil {
				return err
			}
			if err := writeScalarNodeData(writer, "latitude_deg", generated.Nodes, func(point Point) float64 { return point.LatitudeDeg }); err != nil {
				return err
			}
		}
		return writer.Flush()
	}()
	closeErr := file.Close()
	if writeErr != nil {
		return fmt.Errorf("запись MSH %q: %w", path, writeErr)
	}
	if closeErr != nil {
		return fmt.Errorf("закрытие MSH %q: %w", path, closeErr)
	}
	return nil
}

func writeScalarNodeData(writer *bufio.Writer, name string, nodes []Point, value func(Point) float64) error {
	if _, err := fmt.Fprintf(writer, "$NodeData\n1\n%q\n1\n0\n3\n0\n1\n%d\n", name, len(nodes)-1); err != nil {
		return err
	}
	for nodeID := 1; nodeID < len(nodes); nodeID++ {
		if _, err := fmt.Fprintf(writer, "%d %.15g\n", nodeID, value(nodes[nodeID])); err != nil {
			return err
		}
	}
	_, err := fmt.Fprintln(writer, "$EndNodeData")
	return err
}

func readNodes(scanner *bufio.Scanner, result *Mesh) error {
	if !scanner.Scan() {
		return fmt.Errorf("после $Nodes отсутствует число узлов")
	}
	count, err := strconv.Atoi(strings.TrimSpace(scanner.Text()))
	if err != nil || count < 0 {
		return fmt.Errorf("некорректное число узлов %q", scanner.Text())
	}
	result.Nodes = make([]Point, count+1)
	for index := 0; index < count; index++ {
		if !scanner.Scan() {
			return fmt.Errorf("секция $Nodes оборвана на узле %d", index+1)
		}
		fields := strings.Fields(scanner.Text())
		if len(fields) < 4 {
			return fmt.Errorf("некорректная строка узла %q", scanner.Text())
		}
		id, idErr := strconv.Atoi(fields[0])
		x, xErr := strconv.ParseFloat(fields[1], 64)
		y, yErr := strconv.ParseFloat(fields[2], 64)
		if idErr != nil || xErr != nil || yErr != nil || id <= 0 || id >= len(result.Nodes) {
			return fmt.Errorf("некорректный узел %q", scanner.Text())
		}
		result.Nodes[id] = Point{X: x, Y: y}
	}
	if !scanner.Scan() || strings.TrimSpace(scanner.Text()) != "$EndNodes" {
		return fmt.Errorf("секция $Nodes не завершена маркером $EndNodes")
	}
	return nil
}

func readElements(scanner *bufio.Scanner, result *Mesh) error {
	if !scanner.Scan() {
		return fmt.Errorf("после $Elements отсутствует число элементов")
	}
	count, err := strconv.Atoi(strings.TrimSpace(scanner.Text()))
	if err != nil || count < 0 {
		return fmt.Errorf("некорректное число элементов %q", scanner.Text())
	}
	for index := 0; index < count; index++ {
		if !scanner.Scan() {
			return fmt.Errorf("секция $Elements оборвана на элементе %d", index+1)
		}
		fields := strings.Fields(scanner.Text())
		if len(fields) < 4 {
			return fmt.Errorf("некорректная строка элемента %q", scanner.Text())
		}
		elementType, typeErr := strconv.Atoi(fields[1])
		tagCount, tagsErr := strconv.Atoi(fields[2])
		if typeErr != nil || tagsErr != nil || tagCount < 0 {
			return fmt.Errorf("некорректный заголовок элемента %q", scanner.Text())
		}
		nodeStart := 3 + tagCount
		switch elementType {
		case 1:
			if len(fields) < nodeStart+2 {
				return fmt.Errorf("линейный элемент не содержит два узла")
			}
			a, aErr := strconv.Atoi(fields[nodeStart])
			b, bErr := strconv.Atoi(fields[nodeStart+1])
			if aErr != nil || bErr != nil {
				return fmt.Errorf("некорректные узлы граничного элемента")
			}
			result.BoundaryEdges = append(result.BoundaryEdges, [2]int{a, b})
		case 2, 3:
			nodeCount := 3
			if elementType == 3 {
				nodeCount = 4
			}
			if len(fields) < nodeStart+nodeCount {
				return fmt.Errorf("поверхностный элемент не содержит %d узлов", nodeCount)
			}
			cell := Cell{NodeCount: nodeCount}
			for nodeIndex := 0; nodeIndex < nodeCount; nodeIndex++ {
				node, nodeErr := strconv.Atoi(fields[nodeStart+nodeIndex])
				if nodeErr != nil || node <= 0 || node >= len(result.Nodes) {
					return fmt.Errorf("некорректный индекс узла поверхностного элемента")
				}
				cell.Nodes[nodeIndex] = node
			}
			result.Cells = append(result.Cells, cell)
			if nodeCount == 4 {
				result.QuadCount++
			} else {
				result.TriangleCount++
			}
		}
	}
	if !scanner.Scan() || strings.TrimSpace(scanner.Text()) != "$EndElements" {
		return fmt.Errorf("секция $Elements не завершена маркером $EndElements")
	}
	return nil
}

func readNodeData(scanner *bufio.Scanner, result *Mesh) (string, error) {
	stringTagCount, err := scanNonNegativeInteger(scanner, "число строковых тегов $NodeData")
	if err != nil {
		return "", err
	}
	stringTags := make([]string, stringTagCount)
	for index := range stringTags {
		if !scanner.Scan() {
			return "", fmt.Errorf("секция $NodeData оборвана на строковом теге")
		}
		stringTags[index] = strings.Trim(strings.TrimSpace(scanner.Text()), "\"")
	}
	name := ""
	if len(stringTags) > 0 {
		name = stringTags[0]
	}

	realTagCount, err := scanNonNegativeInteger(scanner, "число вещественных тегов $NodeData")
	if err != nil {
		return "", err
	}
	for index := 0; index < realTagCount; index++ {
		if !scanner.Scan() {
			return "", fmt.Errorf("секция $NodeData оборвана на вещественном теге")
		}
		if _, err := strconv.ParseFloat(strings.TrimSpace(scanner.Text()), 64); err != nil {
			return "", fmt.Errorf("некорректный вещественный тег $NodeData %q", scanner.Text())
		}
	}

	integerTagCount, err := scanNonNegativeInteger(scanner, "число целочисленных тегов $NodeData")
	if err != nil {
		return "", err
	}
	integerTags := make([]int, integerTagCount)
	for index := range integerTags {
		if !scanner.Scan() {
			return "", fmt.Errorf("секция $NodeData оборвана на целочисленном теге")
		}
		value, valueErr := strconv.Atoi(strings.TrimSpace(scanner.Text()))
		if valueErr != nil {
			return "", fmt.Errorf("некорректный целочисленный тег $NodeData %q", scanner.Text())
		}
		integerTags[index] = value
	}
	if len(integerTags) < 3 || integerTags[1] <= 0 || integerTags[2] < 0 {
		return "", fmt.Errorf("$NodeData %q не содержит корректные теги компонентов и узлов", name)
	}
	componentCount, entryCount := integerTags[1], integerTags[2]
	isGeographic := name == "longitude_deg" || name == "latitude_deg"
	if isGeographic {
		if componentCount != 1 {
			return "", fmt.Errorf("блок %s должен содержать одно значение на узел", name)
		}
		if len(result.Nodes) <= 1 || entryCount != len(result.Nodes)-1 {
			return "", fmt.Errorf("блок %s содержит %d записей вместо %d", name, entryCount, len(result.Nodes)-1)
		}
	}

	seen := make(map[int]bool, entryCount)
	for index := 0; index < entryCount; index++ {
		if !scanner.Scan() {
			return "", fmt.Errorf("секция $NodeData %q оборвана на записи %d", name, index+1)
		}
		fields := strings.Fields(scanner.Text())
		if len(fields) < componentCount+1 {
			return "", fmt.Errorf("некорректная запись $NodeData %q", scanner.Text())
		}
		nodeID, idErr := strconv.Atoi(fields[0])
		if idErr != nil {
			return "", fmt.Errorf("некорректный идентификатор узла $NodeData %q", fields[0])
		}
		if !isGeographic {
			continue
		}
		if nodeID <= 0 || nodeID >= len(result.Nodes) || seen[nodeID] {
			return "", fmt.Errorf("некорректный или повторный узел %d в блоке %s", nodeID, name)
		}
		value, valueErr := strconv.ParseFloat(fields[1], 64)
		if valueErr != nil || math.IsNaN(value) || math.IsInf(value, 0) {
			return "", fmt.Errorf("некорректное значение узла %d в блоке %s", nodeID, name)
		}
		if name == "longitude_deg" {
			result.Nodes[nodeID].LongitudeDeg = value
		} else {
			result.Nodes[nodeID].LatitudeDeg = value
		}
		seen[nodeID] = true
	}
	if !scanner.Scan() || strings.TrimSpace(scanner.Text()) != "$EndNodeData" {
		return "", fmt.Errorf("секция $NodeData %q не завершена маркером $EndNodeData", name)
	}
	return name, nil
}

func scanNonNegativeInteger(scanner *bufio.Scanner, label string) (int, error) {
	if !scanner.Scan() {
		return 0, fmt.Errorf("после %s отсутствует значение", label)
	}
	value, err := strconv.Atoi(strings.TrimSpace(scanner.Text()))
	if err != nil || value < 0 {
		return 0, fmt.Errorf("некорректное %s %q", label, scanner.Text())
	}
	return value, nil
}
