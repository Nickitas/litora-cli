package mesh

import (
	"bufio"
	"fmt"
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
		}
	}
	if err := scanner.Err(); err != nil {
		return Mesh{}, fmt.Errorf("чтение MSH %q: %w", path, err)
	}
	if len(result.Nodes) == 0 {
		return Mesh{}, fmt.Errorf("MSH не содержит узлов")
	}
	return result, nil
}

// WriteMSH2 сохраняет фактическую итоговую сетку в текстовом формате Gmsh
// MSH 2.2. Граничные рёбра и полные четырёхугольные ячейки записываются без
// прореживания, поэтому файл пригоден для последующих численных расчётов.
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
		if _, err := fmt.Fprint(writer, "$MeshFormat\n2.2 0 8\n$EndMeshFormat\n$Nodes\n"); err != nil {
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
