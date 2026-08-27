package cobra

import (
	"testing"

	adaptivemodel "coastal-geometry/internal/domain/adaptive"
	mesh2d "coastal-geometry/internal/domain/mesh"
)

func TestPrepareExpertSetJobsAnonymizesOnlySuccessfulRuns(t *testing.T) {
	level := adaptivemodel.AdaptiveComparisonLevelReport{
		Level:                    adaptivemodel.TargetLevel{ID: "test", Name: "Тест", MinimumSizeM: 200, MaximumSizeM: 1000},
		CommonBoundaryConfirmed:  true,
		SuccessfulGeneratorCount: 2,
		Runs: []adaptivemodel.GeneratorComparisonRun{
			{Algorithm: mesh2d.AlgorithmDelaunay, Success: true, Artifacts: adaptivemodel.ComparisonArtifacts{MSH: "delaunay.msh"}},
			{Algorithm: mesh2d.AlgorithmFrontalQuad, Success: true, Artifacts: adaptivemodel.ComparisonArtifacts{MSH: "frontal.msh"}},
			{Algorithm: mesh2d.AlgorithmParallelograms, Success: false, Error: "тайм-аут"},
		},
	}
	jobs, excluded, err := prepareExpertSetJobs([]adaptivemodel.AdaptiveComparisonLevelReport{level}, adaptivemodel.DefaultExpertFragments(), "fixed-seed")
	if err != nil {
		t.Fatal(err)
	}
	if len(jobs) != 16 || len(excluded) != 1 {
		t.Fatalf("неверное число карточек или исключений: cards=%d excluded=%d", len(jobs), len(excluded))
	}
	seen := make(map[string]struct{}, len(jobs))
	for _, job := range jobs {
		if job.presentationID == "" || job.relativeSVG == "" {
			t.Fatalf("анонимная карточка не получила путь или идентификатор: %+v", job)
		}
		if _, exists := seen[job.presentationID]; exists {
			t.Fatalf("повторён анонимный идентификатор %q", job.presentationID)
		}
		seen[job.presentationID] = struct{}{}
	}
}

func TestSelectExpertLevelsRejectsUnconfirmedBoundary(t *testing.T) {
	_, err := selectExpertLevels([]adaptivemodel.AdaptiveComparisonLevelReport{{Level: adaptivemodel.TargetLevel{ID: "bad"}}}, "")
	if err == nil {
		t.Fatal("ожидался отказ для уровня без подтверждённой общей границы")
	}
}
