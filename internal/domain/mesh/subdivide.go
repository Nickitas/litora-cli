package mesh

// SubdivideToFullQuads выполняет согласованное топологическое деление каждого
// треугольника или четырёхугольника через общие середины рёбер и центр ячейки.
// В результате не остаётся треугольников и не возникают висячие узлы.
func SubdivideToFullQuads(source Mesh) Mesh {
	result := Mesh{Nodes: append([]Point(nil), source.Nodes...), SurfacePhysicalTag: source.SurfacePhysicalTag}
	if len(source.Cells) == 0 {
		return result
	}

	edgeMidpoints := make(map[[2]int]int, len(source.Cells)*2)
	midpoint := func(a, b int) int {
		key := normalizedEdge(a, b)
		if node, ok := edgeMidpoints[key]; ok {
			return node
		}
		point := Point{
			X: (result.Nodes[a].X + result.Nodes[b].X) / 2,
			Y: (result.Nodes[a].Y + result.Nodes[b].Y) / 2,
		}
		result.Nodes = append(result.Nodes, point)
		node := len(result.Nodes) - 1
		edgeMidpoints[key] = node
		return node
	}

	for _, cell := range source.Cells {
		if cell.NodeCount < 3 {
			continue
		}
		center := Point{}
		for index := 0; index < cell.NodeCount; index++ {
			center.X += result.Nodes[cell.Nodes[index]].X
			center.Y += result.Nodes[cell.Nodes[index]].Y
		}
		center.X /= float64(cell.NodeCount)
		center.Y /= float64(cell.NodeCount)
		result.Nodes = append(result.Nodes, center)
		centerNode := len(result.Nodes) - 1

		for index := 0; index < cell.NodeCount; index++ {
			current := cell.Nodes[index]
			next := cell.Nodes[(index+1)%cell.NodeCount]
			previous := cell.Nodes[(index+cell.NodeCount-1)%cell.NodeCount]
			result.Cells = append(result.Cells, Cell{
				Nodes:     [4]int{current, midpoint(current, next), centerNode, midpoint(previous, current)},
				NodeCount: 4,
			})
		}
	}

	for edgeIndex, edge := range source.BoundaryEdges {
		middle := midpoint(edge[0], edge[1])
		result.BoundaryEdges = append(result.BoundaryEdges, [2]int{edge[0], middle}, [2]int{middle, edge[1]})
		if edgeIndex < len(source.BoundaryPhysicalTags) {
			tag := source.BoundaryPhysicalTags[edgeIndex]
			result.BoundaryPhysicalTags = append(result.BoundaryPhysicalTags, tag, tag)
		}
	}
	result.QuadCount = len(result.Cells)
	return result
}

func normalizedEdge(a, b int) [2]int {
	if a > b {
		a, b = b, a
	}
	return [2]int{a, b}
}
