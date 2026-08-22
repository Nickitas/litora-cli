package seabed

import (
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"coastal-geometry/internal/domain/mesh"
)

func TestMSH2RoundTripPreservesBathymetricNodesCellsAndBoundaries(t *testing.T) {
	source := threeByThreeMesh()
	model, err := Build(source, constantSampler(-80, SamplingBilinear, 25), BuildConfig{
		CoastTransitionWidthM: 200,
		BoundaryOverrides:     []BoundaryOverride{{NodeA: 1, NodeB: 2, Kind: BoundaryOpen}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if !model.Accepted {
		t.Fatalf("синтетическая модель должна быть принята: %v", model.Reasons)
	}

	path := filepath.Join(t.TempDir(), "black-sea-depth.msh")
	if err := WriteMSH2(path, model); err != nil {
		t.Fatal(err)
	}
	document, err := ReadMSH2(path)
	if err != nil {
		t.Fatal(err)
	}
	if document.Metadata.ModelKind != MSHModelSeabed || document.Metadata.SchemaVersion != SeabedMSHSchemaVersion ||
		document.Metadata.VerticalCoordinate != "elevation_m" || document.Metadata.Legacy {
		t.Fatalf("неверно распознана схема MSH: %+v", document.Metadata)
	}
	if !reflect.DeepEqual(document.Model.Nodes, model.Nodes) {
		t.Fatalf("узловые батиметрические данные изменились после round-trip:\nполучено: %#v\nожидалось: %#v", document.Model.Nodes, model.Nodes)
	}
	if !reflect.DeepEqual(document.Model.Cells, model.Cells) {
		t.Fatalf("характеристики ячеек изменились после round-trip:\nполучено: %#v\nожидалось: %#v", document.Model.Cells, model.Cells)
	}
	if !reflect.DeepEqual(document.Model.BoundaryEdges, model.BoundaryEdges) {
		t.Fatalf("физические группы границ изменились после round-trip: %#v != %#v", document.Model.BoundaryEdges, model.BoundaryEdges)
	}
	if !document.Model.Accepted {
		t.Fatalf("прочитанная модель должна оставаться принятой: %v", document.Model.Reasons)
	}

	contents, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	text := string(contents)
	for _, marker := range []string{
		"lito_model_kind=seabed", "lito_schema_version=lito-seabed/v1",
		"\"elevation_m\"", "\"water_depth_m\"", "\"quality_code\"",
		"\"elevation_mean_m\"", "\"water_depth_mean_m\"", "\"slope_deg\"",
		"\"region_code\"", "\"cell_quality_code\"", "$ElementData",
	} {
		if !strings.Contains(text, marker) {
			t.Fatalf("батиметрический MSH не содержит обязательный маркер %q", marker)
		}
	}

	flatView, err := mesh.ReadMSH2(path)
	if err != nil {
		t.Fatalf("базовый читатель геометрии должен принимать расширенный MSH: %v", err)
	}
	if len(flatView.Nodes) != len(source.Nodes) || len(flatView.Cells) != len(source.Cells) {
		t.Fatalf("базовый читатель потерял топологию: %+v", flatView)
	}
}

func TestReadMSH2DistinguishesMarkedAndLegacyFlatMeshes(t *testing.T) {
	source := threeByThreeMesh()
	markedPath := filepath.Join(t.TempDir(), "marked-flat.msh")
	if err := mesh.WriteMSH2(markedPath, source); err != nil {
		t.Fatal(err)
	}
	marked, err := ReadMSH2(markedPath)
	if err != nil {
		t.Fatal(err)
	}
	if marked.Metadata.ModelKind != MSHModelFlat || marked.Metadata.SchemaVersion != FlatMSHSchemaVersion || marked.Metadata.Legacy {
		t.Fatalf("новая плоская схема распознана неверно: %+v", marked.Metadata)
	}
	if marked.Model.Accepted || marked.Model.Nodes[1].ElevationM != nil || marked.Model.Nodes[1].QualityFlag != QualityNoData {
		t.Fatalf("плоская Z = 0 не должна считаться батиметрией: %+v", marked.Model.Nodes[1])
	}

	legacyPath := filepath.Join(t.TempDir(), "legacy-flat.msh")
	legacy := "$MeshFormat\n2.2 0 8\n$EndMeshFormat\n" +
		"$Nodes\n4\n1 0 0 0\n2 1 0 0\n3 1 1 0\n4 0 1 0\n$EndNodes\n" +
		"$Elements\n5\n1 1 0 1 2\n2 1 0 2 3\n3 1 0 3 4\n4 1 0 4 1\n5 3 0 1 2 3 4\n$EndElements\n"
	if err := os.WriteFile(legacyPath, []byte(legacy), 0o644); err != nil {
		t.Fatal(err)
	}
	legacyDocument, err := ReadMSH2(legacyPath)
	if err != nil {
		t.Fatal(err)
	}
	if legacyDocument.Metadata.ModelKind != MSHModelFlat || !legacyDocument.Metadata.Legacy || legacyDocument.Model.Accepted {
		t.Fatalf("legacy-flat распознан неверно: %+v", legacyDocument)
	}
}

func TestWriteMSH2RejectsNoDataWithoutCreatingFile(t *testing.T) {
	source := threeByThreeMesh()
	center := source.Nodes[9]
	model, err := Build(source, samplerFunc(func(latitudeDeg, longitudeDeg, _ float64) (Sample, error) {
		if latitudeDeg == center.LatitudeDeg && longitudeDeg == center.LongitudeDeg {
			return Sample{}, errors.New("нет покрытия")
		}
		return Sample{ElevationM: -20, Method: SamplingExact}, nil
	}), BuildConfig{CoastTransitionWidthM: 200})
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "incomplete.msh")
	if err := WriteMSH2(path, model); err == nil || !strings.Contains(err.Error(), "без NoData") {
		t.Fatalf("неполная модель должна быть отклонена до записи, получено: %v", err)
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("файл неполной модели не должен создаваться: %v", err)
	}
}

func TestDefaultMSHCodeTablesAreStableAndIndependent(t *testing.T) {
	first := DefaultMSHCodeTables()
	if first.SamplingMethod[SamplingCoastlineConstraint] != 5 || first.QualityFlag[QualityVerified] != 4 ||
		first.BoundaryKind[BoundaryOpen] != 3 || first.CellRegion[RegionBasin] != 4 || first.CellQuality[CellQualityVerified] != 3 {
		t.Fatalf("изменена таблица кодов lito-seabed/v1: %+v", first)
	}
	first.SamplingMethod[SamplingExact] = 99
	second := DefaultMSHCodeTables()
	if second.SamplingMethod[SamplingExact] != 1 {
		t.Fatal("возвращаемые таблицы кодов не должны разделять изменяемое состояние")
	}
}
