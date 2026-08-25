package adaptive

import (
	"os"
	"path/filepath"
	"testing"

	"coastal-geometry/internal/domain/mesh"
	"coastal-geometry/internal/domain/seabed"
)

func TestReadTargetSizeFieldCSVStreamsAndValidatesNodes(t *testing.T) {
	model := seabed.Model{Nodes: []seabed.Node{
		{},
		{ID: 1, XM: 10, YM: 20},
		{ID: 2, XM: 30, YM: 40},
	}, Mesh: mesh.Mesh{Nodes: []mesh.Point{{}, {X: 10, Y: 20}, {X: 30, Y: 40}}}}
	path := filepath.Join(t.TempDir(), "field.csv")
	data := "schema_version,node_id,x_m,y_m,target_size_m,zone\n" +
		SchemaVersion + ",1,10,20,200,coastline\n" +
		SchemaVersion + ",2,30,40,800,basin_flat\n"
	if err := os.WriteFile(path, []byte(data), 0o644); err != nil {
		t.Fatal(err)
	}
	field, err := ReadTargetSizeFieldCSV(path, model)
	if err != nil {
		t.Fatal(err)
	}
	if field.NodeCount != 2 || field.MinSizeM != 200 || field.MaxSizeM != 800 || field.Zones[2] != "basin_flat" {
		t.Fatalf("поле прочитано неверно: %+v", field)
	}
	report := Report{SchemaVersion: SchemaVersion, HorizontalUnit: "m", TargetSizeUnit: "m", Summary: Summary{
		NodeCount: 2, Target: SizeStatistics{MinM: 200, MaxM: 800},
	}}
	if err := field.ValidateAgainstReport(report); err != nil {
		t.Fatal(err)
	}
}
