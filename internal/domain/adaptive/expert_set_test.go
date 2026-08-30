package adaptive

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDefaultExpertFragmentsSeparateRegionsAndFeatures(t *testing.T) {
	fragments := DefaultExpertFragments()
	if err := ValidateExpertFragments(fragments); err != nil {
		t.Fatal(err)
	}
	if len(fragments) != 8 {
		t.Fatalf("ожидалось восемь фрагментов AI-01, получено %d", len(fragments))
	}
	regions := map[string]ExpertSplit{}
	for _, fragment := range fragments {
		if split, exists := regions[fragment.RegionID]; exists && split != fragment.Split {
			t.Fatalf("район %q попал в обе части: %q и %q", fragment.RegionID, split, fragment.Split)
		}
		regions[fragment.RegionID] = fragment.Split
	}
}

func TestValidateExpertFragmentsRejectsGeographicLeakage(t *testing.T) {
	fragments := DefaultExpertFragments()
	fragments[len(fragments)-1].RegionID = fragments[0].RegionID
	if err := ValidateExpertFragments(fragments); err == nil || !strings.Contains(err.Error(), "одновременно попал") {
		t.Fatalf("ожидался отказ при географической утечке, получено: %v", err)
	}
}

func TestExpertSetWritersKeepParticipantSheetBlind(t *testing.T) {
	directory := t.TempDir()
	manifest := ExpertSetManifest{
		SchemaVersion:          ExpertSetSchemaVersion,
		GeneratedAt:            "2026-08-27T00:00:00Z",
		ComparisonReport:       "comparison.json",
		ComparisonReportSHA256: strings.Repeat("a", 64),
		Seed:                   "test",
		Fragments:              DefaultExpertFragments(),
		Cards: []ExpertSetCard{{
			PresentationID: "E-001", FragmentID: "train-tendra-spit", Feature: "коса", RegionID: "northwest-shelf",
			Split: ExpertSplitTraining, LevelID: "detailed", Algorithm: "delaunay", MeshSHA256: strings.Repeat("b", 64), SVGPath: "cards/E-001.svg", VisibleCells: 5,
		}},
	}
	manifestPath := filepath.Join(directory, "organizer-key.json")
	if err := WriteExpertSetManifest(manifestPath, manifest); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(manifestPath)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(data), ": null") || !strings.Contains(string(data), "delaunay") {
		t.Fatalf("закрытый ключ записан неполно: %s", data)
	}
	sheetPath := filepath.Join(directory, "expert-rating-template.tsv")
	if err := WriteExpertAssignmentsTSV(sheetPath, manifest.Cards); err != nil {
		t.Fatal(err)
	}
	sheet, err := os.ReadFile(sheetPath)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(sheet), "E-001") || strings.Contains(string(sheet), "delaunay") || strings.Contains(string(sheet), "detailed") {
		t.Fatalf("лист участника раскрывает генератор или уровень: %s", sheet)
	}
}
