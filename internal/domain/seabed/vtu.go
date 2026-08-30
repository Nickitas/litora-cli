package seabed

import (
	"bufio"
	"encoding/json"
	"encoding/xml"
	"fmt"
	"io"
	"strconv"
)

const vtkQuadCellType = 9

// WriteVTU сохраняет принятую модель дна как VTK XML UnstructuredGrid. Файл
// содержит четырёхугольную поверхность с Z = elevation_m, PointData/CellData,
// единицами VTK и полным JSON-описанием системы координат в FieldData.
func WriteVTU(path string, model Model, metadata ExportMetadata) error {
	if err := validateMSHModel(model); err != nil {
		return fmt.Errorf("проверка модели для VTU: %w", err)
	}
	normalized, err := normalizeExportMetadata(metadata, model)
	if err != nil {
		return err
	}
	metadataJSON, err := json.Marshal(normalized)
	if err != nil {
		return fmt.Errorf("кодирование метаданных VTU: %w", err)
	}

	file, err := createOutputFile(path)
	if err != nil {
		return err
	}
	writer := bufio.NewWriterSize(file, 1024*1024)
	writeErr := writeVTUDocument(writer, model, normalized, metadataJSON)
	if writeErr == nil {
		writeErr = writer.Flush()
	}
	closeErr := file.Close()
	if writeErr != nil {
		return fmt.Errorf("запись VTU %q: %w", path, writeErr)
	}
	if closeErr != nil {
		return fmt.Errorf("закрытие VTU %q: %w", path, closeErr)
	}
	return nil
}

func writeVTUDocument(writer *bufio.Writer, model Model, metadata ExportMetadata, metadataJSON []byte) error {
	if _, err := fmt.Fprintf(writer,
		"<?xml version=\"1.0\"?>\n"+
			"<VTKFile type=\"UnstructuredGrid\" version=\"0.1\" byte_order=\"LittleEndian\" header_type=\"UInt64\">\n"+
			"  <UnstructuredGrid>\n"+
			"    <FieldData>\n",
	); err != nil {
		return err
	}
	if err := writeVTUArray(writer, vtuArraySpec{
		dataType: "UInt8", name: "lito_metadata_utf8_json", components: 1, tuples: len(metadataJSON), includeTuples: true,
		values: func(writer io.Writer) error {
			for index, value := range metadataJSON {
				if err := writeVTUInteger(writer, index, int(value), 24); err != nil {
					return err
				}
			}
			return nil
		},
	}); err != nil {
		return err
	}
	if err := writeVTUArray(writer, vtuArraySpec{
		dataType: "Int32", name: "lito_boundary_edges", components: 3, tuples: len(model.BoundaryEdges), includeTuples: true, unit: "index,index,code",
		values: func(writer io.Writer) error {
			for index, edge := range model.BoundaryEdges {
				code := metadata.CodeTables.BoundaryKind[edge.Kind]
				if _, err := fmt.Fprintf(writer, "%d %d %d", edge.NodeIDs[0]-1, edge.NodeIDs[1]-1, code); err != nil {
					return err
				}
				if (index+1)%4 == 0 || index == len(model.BoundaryEdges)-1 {
					if _, err := fmt.Fprint(writer, "\n"); err != nil {
						return err
					}
				} else if _, err := fmt.Fprint(writer, " "); err != nil {
					return err
				}
			}
			return nil
		},
	}); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(writer,
		"    </FieldData>\n"+
			"    <Piece NumberOfPoints=\"%d\" NumberOfCells=\"%d\">\n"+
			"      <PointData Scalars=\"water_depth_m\">\n",
		len(model.Nodes)-1, len(model.Cells),
	); err != nil {
		return err
	}

	tables := metadata.CodeTables
	pointArrays := []vtuArraySpec{
		integerVTUArray("Int32", "id", "1", len(model.Nodes)-1, func(id int) int { return model.Nodes[id].ID }),
		floatVTUArray("longitude_deg", "degree", len(model.Nodes)-1, func(id int) float64 { return model.Nodes[id].LongitudeDeg }),
		floatVTUArray("latitude_deg", "degree", len(model.Nodes)-1, func(id int) float64 { return model.Nodes[id].LatitudeDeg }),
		floatVTUArray("elevation_m", metadata.VerticalUnit, len(model.Nodes)-1, func(id int) float64 { return *model.Nodes[id].ElevationM }),
		floatVTUArray("water_depth_m", metadata.VerticalUnit, len(model.Nodes)-1, func(id int) float64 { return *model.Nodes[id].WaterDepthM }),
		integerVTUArray("Int32", "sampling_method_code", "code", len(model.Nodes)-1, func(id int) int {
			return tables.SamplingMethod[model.Nodes[id].SamplingMethod]
		}),
		floatVTUArray("source_distance_m", metadata.HorizontalLinearUnit, len(model.Nodes)-1, func(id int) float64 {
			if model.Nodes[id].SourceDistanceM == nil {
				return metadata.NoDataSentinel
			}
			return *model.Nodes[id].SourceDistanceM
		}),
		integerVTUArray("Int32", "quality_code", "code", len(model.Nodes)-1, func(id int) int {
			return tables.QualityFlag[model.Nodes[id].QualityFlag]
		}),
		integerVTUArray("UInt8", "is_boundary", "1", len(model.Nodes)-1, func(id int) int {
			if model.Nodes[id].IsBoundary {
				return 1
			}
			return 0
		}),
		integerVTUArray("Int32", "boundary_code", "code", len(model.Nodes)-1, func(id int) int {
			return tables.BoundaryKind[model.Nodes[id].BoundaryKind]
		}),
	}
	for _, array := range pointArrays {
		if err := writeVTUArray(writer, array); err != nil {
			return err
		}
	}
	if _, err := fmt.Fprint(writer, "      </PointData>\n      <CellData Scalars=\"water_depth_mean_m\">\n"); err != nil {
		return err
	}
	cellArrays := []vtuArraySpec{
		integerVTUArray("Int32", "id", "1", len(model.Cells), func(id int) int { return model.Cells[id-1].ID }),
		floatVTUArray("area_m2", "m2", len(model.Cells), func(id int) float64 { return model.Cells[id-1].AreaM2 }),
		floatVTUArray("elevation_min_m", metadata.VerticalUnit, len(model.Cells), func(id int) float64 { return model.Cells[id-1].ElevationMinM }),
		floatVTUArray("elevation_max_m", metadata.VerticalUnit, len(model.Cells), func(id int) float64 { return model.Cells[id-1].ElevationMaxM }),
		floatVTUArray("elevation_mean_m", metadata.VerticalUnit, len(model.Cells), func(id int) float64 { return model.Cells[id-1].ElevationMeanM }),
		floatVTUArray("water_depth_mean_m", metadata.VerticalUnit, len(model.Cells), func(id int) float64 { return model.Cells[id-1].WaterDepthMeanM }),
		floatVTUArray("slope_deg", "degree", len(model.Cells), func(id int) float64 { return model.Cells[id-1].SlopeDeg }),
		floatVTUArray("aspect_deg", "degree", len(model.Cells), func(id int) float64 {
			if model.Cells[id-1].AspectDeg == nil {
				return metadata.NoDataSentinel
			}
			return *model.Cells[id-1].AspectDeg
		}),
		floatVTUArray("roughness_m", metadata.VerticalUnit, len(model.Cells), func(id int) float64 { return model.Cells[id-1].RoughnessM }),
		integerVTUArray("Int32", "region_code", "code", len(model.Cells), func(id int) int {
			return tables.CellRegion[model.Cells[id-1].Region]
		}),
		integerVTUArray("Int32", "cell_quality_code", "code", len(model.Cells), func(id int) int {
			return tables.CellQuality[model.Cells[id-1].QualityFlag]
		}),
		floatVTUArray("quality_score", "1", len(model.Cells), func(id int) float64 { return model.Cells[id-1].QualityScore }),
	}
	for _, array := range cellArrays {
		if err := writeVTUArray(writer, array); err != nil {
			return err
		}
	}
	if _, err := fmt.Fprint(writer, "      </CellData>\n      <Points>\n"); err != nil {
		return err
	}
	if err := writeVTUArray(writer, vtuArraySpec{
		dataType: "Float64", name: "Points", components: 3, tuples: len(model.Nodes) - 1, unit: metadata.HorizontalLinearUnit + "," + metadata.HorizontalLinearUnit + "," + metadata.VerticalUnit,
		values: func(writer io.Writer) error {
			for id := 1; id < len(model.Nodes); id++ {
				node := model.Nodes[id]
				if _, err := fmt.Fprintf(writer, "%s %s %s\n", formatVTUFloat(node.XM), formatVTUFloat(node.YM), formatVTUFloat(*node.ElevationM)); err != nil {
					return err
				}
			}
			return nil
		},
	}); err != nil {
		return err
	}
	if _, err := fmt.Fprint(writer, "      </Points>\n      <Cells>\n"); err != nil {
		return err
	}
	connectivity := vtuArraySpec{
		dataType: "Int32", name: "connectivity", components: 1, tuples: 4 * len(model.Cells), unit: "zero_based_point_index",
		values: func(writer io.Writer) error {
			for _, cell := range model.Cells {
				if _, err := fmt.Fprintf(writer, "%d %d %d %d\n", cell.NodeIDs[0]-1, cell.NodeIDs[1]-1, cell.NodeIDs[2]-1, cell.NodeIDs[3]-1); err != nil {
					return err
				}
			}
			return nil
		},
	}
	offsets := integerVTUArray("Int32", "offsets", "connectivity_offset", len(model.Cells), func(id int) int { return 4 * id })
	types := integerVTUArray("UInt8", "types", "VTK_cell_type", len(model.Cells), func(int) int { return vtkQuadCellType })
	for _, array := range []vtuArraySpec{connectivity, offsets, types} {
		if err := writeVTUArray(writer, array); err != nil {
			return err
		}
	}
	_, err := fmt.Fprint(writer,
		"      </Cells>\n"+
			"    </Piece>\n"+
			"  </UnstructuredGrid>\n"+
			"</VTKFile>\n",
	)
	return err
}

type vtuArraySpec struct {
	dataType      string
	name          string
	components    int
	tuples        int
	includeTuples bool
	unit          string
	values        func(io.Writer) error
}

func floatVTUArray(name, unit string, count int, value func(int) float64) vtuArraySpec {
	return vtuArraySpec{
		dataType: "Float64", name: name, components: 1, tuples: count, unit: unit,
		values: func(writer io.Writer) error {
			for id := 1; id <= count; id++ {
				if err := writeVTUFloat(writer, id-1, value(id), 8); err != nil {
					return err
				}
			}
			return nil
		},
	}
}

func integerVTUArray(dataType, name, unit string, count int, value func(int) int) vtuArraySpec {
	return vtuArraySpec{
		dataType: dataType, name: name, components: 1, tuples: count, unit: unit,
		values: func(writer io.Writer) error {
			for id := 1; id <= count; id++ {
				if err := writeVTUInteger(writer, id-1, value(id), 16); err != nil {
					return err
				}
			}
			return nil
		},
	}
}

func writeVTUArray(writer *bufio.Writer, array vtuArraySpec) error {
	if _, err := fmt.Fprintf(writer, "        <DataArray type=\"%s\" Name=\"%s\" NumberOfComponents=\"%d\"", array.dataType, array.name, array.components); err != nil {
		return err
	}
	if array.includeTuples {
		if _, err := fmt.Fprintf(writer, " NumberOfTuples=\"%d\"", array.tuples); err != nil {
			return err
		}
	}
	if _, err := fmt.Fprint(writer, " format=\"ascii\">\n"); err != nil {
		return err
	}
	if _, err := fmt.Fprint(writer, "          "); err != nil {
		return err
	}
	if err := array.values(writer); err != nil {
		return err
	}
	if array.unit != "" {
		if _, err := fmt.Fprint(writer, "\n          <InformationKey name=\"UNITS_LABEL\" location=\"vtkDataArray\">"); err != nil {
			return err
		}
		if err := xml.EscapeText(writer, []byte(array.unit)); err != nil {
			return err
		}
		if _, err := fmt.Fprint(writer, "</InformationKey>\n"); err != nil {
			return err
		}
	}
	_, err := fmt.Fprint(writer, "        </DataArray>\n")
	return err
}

func writeVTUFloat(writer io.Writer, index int, value float64, lineWidth int) error {
	if _, err := fmt.Fprint(writer, formatVTUFloat(value)); err != nil {
		return err
	}
	return writeVTUSeparator(writer, index, lineWidth)
}

func writeVTUInteger(writer io.Writer, index, value, lineWidth int) error {
	if _, err := fmt.Fprint(writer, value); err != nil {
		return err
	}
	return writeVTUSeparator(writer, index, lineWidth)
}

func writeVTUSeparator(writer io.Writer, index, lineWidth int) error {
	separator := " "
	if (index+1)%lineWidth == 0 {
		separator = "\n          "
	}
	_, err := fmt.Fprint(writer, separator)
	return err
}

func formatVTUFloat(value float64) string {
	return strconv.FormatFloat(value, 'g', 17, 64)
}
