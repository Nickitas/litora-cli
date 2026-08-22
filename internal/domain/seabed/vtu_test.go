package seabed

import (
	"encoding/csv"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"testing"

	"coastal-geometry/internal/domain/mesh"
)

func TestVTURoundTripPreservesModelFieldsAndMetadata(t *testing.T) {
	model, err := Build(threeByThreeMesh(), constantSampler(-80, SamplingBilinear, 25), BuildConfig{
		CoastTransitionWidthM: 200,
		BoundaryOverrides:     []BoundaryOverride{{NodeA: 1, NodeB: 2, Kind: BoundaryOpen}},
	})
	if err != nil {
		t.Fatal(err)
	}
	metadata := testExportMetadata()
	expectedMetadata, err := normalizeExportMetadata(metadata, model)
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "black-sea-depth.vtu")
	if err := WriteVTU(path, model, metadata); err != nil {
		t.Fatal(err)
	}
	document, err := ReadVTU(path)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(document.Metadata, expectedMetadata) {
		t.Fatalf("метаданные изменились после round-trip:\nполучено: %#v\nожидалось: %#v", document.Metadata, expectedMetadata)
	}
	if !reflect.DeepEqual(document.Model.Nodes, model.Nodes) {
		t.Fatalf("узлы изменились после round-trip VTU:\nполучено: %#v\nожидалось: %#v", document.Model.Nodes, model.Nodes)
	}
	if !reflect.DeepEqual(document.Model.Cells, model.Cells) {
		t.Fatalf("ячейки изменились после round-trip VTU:\nполучено: %#v\nожидалось: %#v", document.Model.Cells, model.Cells)
	}
	if !reflect.DeepEqual(document.Model.BoundaryEdges, model.BoundaryEdges) {
		t.Fatalf("границы изменились после round-trip VTU: %#v != %#v", document.Model.BoundaryEdges, model.BoundaryEdges)
	}
	contents, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	text := string(contents)
	for _, marker := range []string{
		`type="UnstructuredGrid"`, `Name="lito_metadata_utf8_json"`,
		`Name="water_depth_m"`, `Name="quality_code"`, `Name="water_depth_mean_m"`,
		`Name="cell_quality_code"`, `name="UNITS_LABEL"`, `Name="types"`,
	} {
		if !strings.Contains(text, marker) {
			t.Fatalf("VTU не содержит обязательный маркер %q", marker)
		}
	}
}

func TestReadVTURejectsMissingRequiredDepthArray(t *testing.T) {
	model, err := Build(oneCellMesh(), constantSampler(-100, SamplingBilinear, 50), BuildConfig{})
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "corrupted.vtu")
	if err := WriteVTU(path, model, testExportMetadata()); err != nil {
		t.Fatal(err)
	}
	contents, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	corrupted := strings.Replace(string(contents), `Name="water_depth_m"`, `Name="missing_depth"`, 1)
	if err := os.WriteFile(path, []byte(corrupted), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := ReadVTU(path); err == nil || !strings.Contains(err.Error(), "water_depth_m") {
		t.Fatalf("VTU без глубины должен быть отклонён, получено: %v", err)
	}
}

func TestWriteVTURejectsMissingVerticalReferenceBeforeCreatingFile(t *testing.T) {
	model, err := Build(oneCellMesh(), constantSampler(-100, SamplingBilinear, 50), BuildConfig{})
	if err != nil {
		t.Fatal(err)
	}
	metadata := testExportMetadata()
	metadata.VerticalReference = ""
	path := filepath.Join(t.TempDir(), "invalid.vtu")
	if err := WriteVTU(path, model, metadata); err == nil || !strings.Contains(err.Error(), "вертикальная система") {
		t.Fatalf("VTU без вертикальной системы должен быть отклонён, получено: %v", err)
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("некорректный VTU не должен создаваться: %v", err)
	}
}

func TestExportBundleKeepsCSVAndVTUIdentifiersAligned(t *testing.T) {
	model, err := Build(threeByThreeMesh(), constantSampler(-60, SamplingExact, 0), BuildConfig{CoastTransitionWidthM: 200})
	if err != nil {
		t.Fatal(err)
	}
	directory := t.TempDir()
	artifacts, err := WriteExportBundle(model, ExportBundleConfig{
		Directory: directory,
		Metadata:  testExportMetadata(),
		Profiles: []Profile{{
			ID: "контрольный-01", Name: "Берег — внутренняя часть", NodeIDs: []int{1, 9, 5},
		}},
	})
	if err != nil {
		t.Fatal(err)
	}
	mshPath := filepath.Join(directory, "black-sea-depth.msh")
	if err := WriteMSH2(mshPath, model); err != nil {
		t.Fatal(err)
	}
	mshDocument, err := ReadMSH2(mshPath)
	if err != nil {
		t.Fatal(err)
	}
	vtuDocument, err := ReadVTU(artifacts.VTUPath)
	if err != nil {
		t.Fatal(err)
	}
	if len(vtuDocument.Model.Nodes) != len(mshDocument.Model.Nodes) || len(vtuDocument.Model.Cells) != len(mshDocument.Model.Cells) {
		t.Fatal("VTU и MSH содержат разное число узлов или ячеек")
	}

	nodeRows := readCSVRows(t, artifacts.NodesCSVPath)
	cellRows := readCSVRows(t, artifacts.CellsCSVPath)
	profileRows := readCSVRows(t, artifacts.ProfilesCSVPath)
	if len(nodeRows) != len(model.Nodes)+1 || len(cellRows) != len(model.Cells)+2 || len(profileRows) != 5 {
		t.Fatalf("неверное число строк CSV: nodes=%d cells=%d profiles=%d", len(nodeRows), len(cellRows), len(profileRows))
	}
	nodeHeader := csvHeaderIndex(nodeRows[0])
	cellHeader := csvHeaderIndex(cellRows[0])
	profileHeader := csvHeaderIndex(profileRows[0])
	assertCSVMetadata(t, nodeRows[1], nodeHeader)
	assertCSVMetadata(t, cellRows[1], cellHeader)
	assertCSVMetadata(t, profileRows[1], profileHeader)
	for rowIndex, row := range nodeRows[2:] {
		expectedID := rowIndex + 1
		if row[nodeHeader["id"]] != strconv.Itoa(expectedID) || vtuDocument.Model.Nodes[expectedID].ID != expectedID || mshDocument.Model.Nodes[expectedID].ID != expectedID {
			t.Fatalf("идентификатор узла %d не согласован между CSV/VTU/MSH", expectedID)
		}
		assertCSVDataRecord(t, row, nodeHeader)
	}
	for rowIndex, row := range cellRows[2:] {
		expectedID := rowIndex + 1
		if row[cellHeader["id"]] != strconv.Itoa(expectedID) || row[cellHeader["node_ids"]] != formatCellNodeIDs(model.Cells[rowIndex].NodeIDs) {
			t.Fatalf("ячейка %d не согласована с моделью: %v", expectedID, row)
		}
		assertCSVDataRecord(t, row, cellHeader)
	}
	lastDistance := -1.0
	for _, row := range profileRows[2:] {
		nodeID, err := strconv.Atoi(row[profileHeader["node_id"]])
		if err != nil || nodeID <= 0 || nodeID >= len(model.Nodes) {
			t.Fatalf("profiles.csv содержит неизвестный node_id: %v", row)
		}
		distance, err := strconv.ParseFloat(row[profileHeader["distance_m"]], 64)
		if err != nil || distance <= lastDistance {
			t.Fatalf("расстояние профиля не возрастает: %v", row)
		}
		lastDistance = distance
		assertCSVDataRecord(t, row, profileHeader)
	}

	metadataData, err := os.ReadFile(artifacts.MetadataJSONPath)
	if err != nil {
		t.Fatal(err)
	}
	var metadata ExportMetadata
	if err := json.Unmarshal(metadataData, &metadata); err != nil {
		t.Fatal(err)
	}
	if metadata.SchemaVersion != SeabedMSHSchemaVersion || metadata.VerticalReference == "" || metadata.RegionThresholds != model.CellDerivation.RegionThresholds {
		t.Fatalf("export-metadata.json неполон: %+v", metadata)
	}
}

func TestExportBundleRejectsMissingProfilesBeforeCreatingFiles(t *testing.T) {
	model, err := Build(oneCellMesh(), constantSampler(-100, SamplingExact, 0), BuildConfig{})
	if err != nil {
		t.Fatal(err)
	}
	directory := filepath.Join(t.TempDir(), "export")
	if _, err := WriteExportBundle(model, ExportBundleConfig{Directory: directory, Metadata: testExportMetadata()}); err == nil || !strings.Contains(err.Error(), "VIEW-03") {
		t.Fatalf("пустой набор профилей должен быть отклонён явно: %v", err)
	}
	if _, err := os.Stat(filepath.Join(directory, "black-sea-depth.vtu")); !os.IsNotExist(err) {
		t.Fatalf("проверка должна завершиться до создания файлов: %v", err)
	}
}

func testExportMetadata() ExportMetadata {
	return NewExportMetadata(
		mesh.EqualAreaProjection{ReferenceLat: 43.2, ReferenceLon: 34.1},
		"Средний уровень моря по допущению GEBCO",
		"В мелководье исходные данные могут относиться к иной вертикальной системе.",
	)
}

func readCSVRows(t *testing.T, path string) [][]string {
	t.Helper()
	file, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	rows, err := csv.NewReader(file).ReadAll()
	_ = file.Close()
	if err != nil {
		t.Fatal(err)
	}
	return rows
}

func csvHeaderIndex(header []string) map[string]int {
	result := make(map[string]int, len(header))
	for index, name := range header {
		result[name] = index
	}
	return result
}

func assertCSVMetadata(t *testing.T, row []string, header map[string]int) {
	t.Helper()
	if row[header["record_type"]] != "metadata" || row[header["schema_version"]] != SeabedMSHSchemaVersion ||
		row[header["horizontal_mesh_crs"]] != "spherical_laea" ||
		row[header["horizontal_linear_unit"]] != "m" ||
		row[header["vertical_reference"]] == "" || row[header["vertical_unit"]] != "m" {
		t.Fatalf("строка CSV потеряла пространственные метаданные: %v", row)
	}
}

func assertCSVDataRecord(t *testing.T, row []string, header map[string]int) {
	t.Helper()
	if row[header["record_type"]] != "data" || row[header["schema_version"]] != "" || row[header["vertical_reference"]] != "" {
		t.Fatalf("строка данных CSV не отделена от metadata-записи: %v", row)
	}
}

func formatCellNodeIDs(ids [4]int) string {
	return "[" + strconv.Itoa(ids[0]) + "," + strconv.Itoa(ids[1]) + "," + strconv.Itoa(ids[2]) + "," + strconv.Itoa(ids[3]) + "]"
}
