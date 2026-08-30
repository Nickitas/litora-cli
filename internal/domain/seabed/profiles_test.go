package seabed

import "testing"

func TestSelectCoastToDeepProfilesBuildsThreeDeterministicMeshPaths(t *testing.T) {
	model, err := Build(threeByThreeMesh(), constantSampler(-80, SamplingBilinear, 25), BuildConfig{CoastTransitionWidthM: 20})
	if err != nil {
		t.Fatal(err)
	}
	profiles, reports, err := SelectCoastToDeepProfiles(model)
	if err != nil {
		t.Fatal(err)
	}
	if len(profiles) != 3 || len(reports) != 3 {
		t.Fatalf("ожидались три профиля, получено profiles=%d reports=%d", len(profiles), len(reports))
	}
	wantedIDs := []string{"profile-west", "profile-east", "profile-north"}
	for index, profile := range profiles {
		if profile.ID != wantedIDs[index] || len(profile.NodeIDs) < 2 {
			t.Fatalf("некорректный профиль %d: %+v", index, profile)
		}
		start := model.Nodes[profile.NodeIDs[0]]
		end := model.Nodes[profile.NodeIDs[len(profile.NodeIDs)-1]]
		if start.BoundaryKind != BoundaryCoastline || start.WaterDepthM == nil || *start.WaterDepthM != 0 {
			t.Fatalf("профиль должен начинаться на нулевом внешнем берегу: %+v", start)
		}
		if end.WaterDepthM == nil || *end.WaterDepthM < 0.9*80 {
			t.Fatalf("профиль должен завершаться в глубоководном ядре: %+v", end)
		}
		if reports[index].LengthM <= 0 || reports[index].PointCount != len(profile.NodeIDs) {
			t.Fatalf("некорректный отчёт профиля: %+v", reports[index])
		}
	}

	repeated, repeatedReports, err := SelectCoastToDeepProfiles(model)
	if err != nil {
		t.Fatal(err)
	}
	for index := range profiles {
		if len(repeated[index].NodeIDs) != len(profiles[index].NodeIDs) || repeatedReports[index] != reports[index] {
			t.Fatalf("повторный выбор профиля %d не воспроизводим", index)
		}
		for point := range profiles[index].NodeIDs {
			if repeated[index].NodeIDs[point] != profiles[index].NodeIDs[point] {
				t.Fatalf("повторный путь профиля %d отличается в точке %d", index, point)
			}
		}
	}
}
