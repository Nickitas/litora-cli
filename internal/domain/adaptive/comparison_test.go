package adaptive

import (
	"bytes"
	"math"
	"os"
	"path/filepath"
	"testing"

	"coastal-geometry/internal/domain/mesh"
	"coastal-geometry/internal/domain/seabed"
)

func TestTransformTargetSizeFieldPreservesOrderAndBounds(t *testing.T) {
	field := TargetSizeField{TargetSizeM: []float64{0, 200, 600, 1000}, MinSizeM: 200, MaxSizeM: 1000}
	values, err := TransformTargetSizeField(field, TargetLevel{ID: "coarse", Name: "Укрупнённая", MinimumSizeM: 500, MaximumSizeM: 1000})
	if err != nil {
		t.Fatal(err)
	}
	if values[1] != 500 || values[2] != 750 || values[3] != 1000 {
		t.Fatalf("линейное преобразование поля неверно: %+v", values)
	}
}

func TestEvaluateBathymetryPreservationIsExactForPlane(t *testing.T) {
	depths := []float64{0, 0, 2, 4, 2}
	reference := seabed.Model{
		Mesh: mesh.Mesh{
			Nodes: []mesh.Point{{}, {X: 0, Y: 0}, {X: 2, Y: 0}, {X: 2, Y: 2}, {X: 0, Y: 2}},
			Cells: []mesh.Cell{{Nodes: [4]int{1, 2, 3, 4}, NodeCount: 4}}, QuadCount: 1,
		},
		Nodes: []seabed.Node{
			{},
			{ID: 1, XM: 0, YM: 0, WaterDepthM: &depths[1]},
			{ID: 2, XM: 2, YM: 0, WaterDepthM: &depths[2]},
			{ID: 3, XM: 2, YM: 2, WaterDepthM: &depths[3]},
			{ID: 4, XM: 0, YM: 2, WaterDepthM: &depths[4]},
		},
	}
	generated := reference.Mesh
	metrics, err := EvaluateBathymetryPreservation(reference, generated, BathymetryComparisonConfig{IsobathsM: []float64{1, 3}, WorstCellCount: 1})
	if err != nil {
		t.Fatal(err)
	}
	if metrics.EvaluationCellCount != 1 || math.Abs(metrics.DepthRMSEM) > 1e-12 || math.Abs(metrics.WaterVolumeDeviationPercent) > 1e-12 {
		t.Fatalf("плоское поле глубины должно восстановиться точно: %+v", metrics)
	}
	if metrics.NearestFallbackNodeCount != 0 || metrics.SlopeRMSEDeg > 1e-12 {
		t.Fatalf("внутри опорной ячейки не нужен fallback и не должно быть ошибки уклона: %+v", metrics)
	}
}

func TestRankComparisonLevelKeepsFailuresAndStableBest(t *testing.T) {
	level := AdaptiveComparisonLevelReport{
		Level: TargetLevel{ID: "test", Name: "Тест", MinimumSizeM: 200, MaximumSizeM: 1000},
		Runs: []GeneratorComparisonRun{
			comparisonTestRun(mesh.AlgorithmFrontalQuad, 0.80, 0.60, 2, 80, 20),
			comparisonTestRun(mesh.AlgorithmDelaunay, 0.90, 0.75, 1, 95, 10),
			{Algorithm: mesh.AlgorithmParallelograms, Success: false, Error: "тайм-аут"},
		},
	}
	if err := RankComparisonLevel(&level); err != nil {
		t.Fatal(err)
	}
	if level.BestGenerator != mesh.AlgorithmDelaunay || level.Runs[0].Algorithm != mesh.AlgorithmDelaunay || level.Runs[0].Rank != 1 {
		t.Fatalf("рейтинг выбрал неверный алгоритм: %+v", level)
	}
	if level.SuccessfulGeneratorCount != 2 || level.FailedGeneratorCount != 1 || level.Runs[2].Error != "тайм-аут" {
		t.Fatalf("неуспешный запуск должен сохраниться в рейтинге: %+v", level)
	}
	if !level.CommonBoundaryConfirmed {
		t.Fatal("одинаковые береговые метрики должны быть подтверждены")
	}
}

func TestWriteComparisonJSONUsesEmptyArraysInsteadOfNull(t *testing.T) {
	report := NewComparisonReport()
	report.Levels = append(report.Levels, AdaptiveComparisonLevelReport{
		Level: TargetLevel{ID: "test", Name: "Тест", MinimumSizeM: 200, MaximumSizeM: 1000},
		Runs:  []GeneratorComparisonRun{{Algorithm: mesh.AlgorithmParallelograms, Success: false, Error: "тайм-аут"}},
	})
	path := filepath.Join(t.TempDir(), "comparison.json")
	if err := WriteComparisonJSON(path, report); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(data, []byte(": null")) {
		t.Fatalf("машинный отчёт не должен заменять пустые коллекции null: %s", data)
	}
}

func comparisonTestRun(algorithm mesh.Algorithm, meanQuality, p05Quality, depthRMSE, targetPercent, seconds float64) GeneratorComparisonRun {
	return GeneratorComparisonRun{
		Algorithm: algorithm, Success: true,
		Resources: mesh.GenerationResourceStats{GmshDurationSeconds: seconds, PeakRSSBytes: 100 * 1024 * 1024},
		Topology:  mesh.TopologyValidation{Accepted: true, CellCount: 10},
		Geometry: mesh.QualityMetrics{
			MeanCellQuality: meanQuality, P05CellQuality: p05Quality,
			CoastalAreaDeviationPercent: 0.1, BoundaryRMSMeters: 0,
		},
		EdgeZones: []mesh.ZoneEdgeStatistics{{EdgeObservationCount: 100, WithinTolerancePct: targetPercent}},
		Bathymetry: BathymetryPreservationMetrics{
			ReferenceP95DepthM: 2000, DepthRMSEM: depthRMSE,
			WaterVolumeDeviationPercent: 0.1, MeanIsobathAreaDeviationPct: 0.2,
			SlopeRMSEDeg: 0.1,
		},
	}
}
