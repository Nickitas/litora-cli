package cobra

import "testing"

func TestSelectedWaterbodyRejectsRiverForCERC(t *testing.T) {
	if _, err := selectedWaterbody("volga"); err == nil {
		t.Fatal("Волга не должна допускаться к волновой CERC-модели")
	}
}

func TestSelectedWaterbodyAcceptsLakeWithInputData(t *testing.T) {
	body, err := selectedWaterbody("lake-baikal")
	if err != nil {
		t.Fatalf("Байкал должен допускать расчёт при реальных входных данных: %v", err)
	}
	if body.ID != "lake-baikal" {
		t.Fatalf("получен другой водоём: %q", body.ID)
	}
}
