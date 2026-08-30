package adaptive

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestReadExpertRatingsTSVAndAggregate(t *testing.T) {
	path := filepath.Join(t.TempDir(), "ratings.tsv")
	content := strings.Join([]string{
		"Идентификатор карточки\tОсобенность\tЭксперт\tБерег 1–5\tИзобаты 1–5\tСетка 1–5\tАртефакты 1–5\tКомментарий",
		"-----------------------\t-----------\t-------\t----------\t------------\t----------\t---------------\t-----------",
		"E-001\tкоса\texpert-a\t5\t4\t3\t4\tхорошо",
		"E-001\tкоса\texpert-b\t3\t4\t5\t4\tпроверено",
		"E-002\tзалив\t\t\t\t\t\t",
	}, "\n") + "\n"
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("запись листа: %v", err)
	}
	ratings, err := ReadExpertRatingsTSV(path)
	if err != nil {
		t.Fatalf("чтение листа: %v", err)
	}
	if len(ratings) != 2 {
		t.Fatalf("число оценок = %d, требуется 2", len(ratings))
	}
	aggregates := AggregateExpertRatings(ratings)
	if len(aggregates) != 1 || aggregates[0].PresentationID != "E-001" || aggregates[0].ExpertCount != 2 {
		t.Fatalf("неверная агрегация: %#v", aggregates)
	}
	if aggregates[0].MeanScore != 4 {
		t.Fatalf("средний балл = %.3f, требуется 4", aggregates[0].MeanScore)
	}
}

func TestAssessMLNecessityRejectsInsufficientIndependentRatings(t *testing.T) {
	decision, err := AssessMLNecessity([]AICalibrationRecord{
		{PresentationID: "E-001", Split: ExpertSplitTraining, ExpertScore: 4, ExpertCount: 1},
		{PresentationID: "E-002", Split: ExpertSplitEvaluation, ExpertScore: 4, ExpertCount: 1},
	}, DefaultAINecessityConfig())
	if err != nil {
		t.Fatalf("проверка AI-02: %v", err)
	}
	if decision.MLRecommended {
		t.Fatal("ML не должен рекомендоваться по одиночным оценкам")
	}
	if decision.TrainingRecordCount != 0 || decision.EvaluationRecordCount != 0 {
		t.Fatalf("одиночные оценки не были исключены: %#v", decision)
	}
	if len(decision.Reasons) == 0 || !strings.Contains(decision.Reasons[0], "минимум 2") {
		t.Fatalf("не указана причина недостаточной независимой разметки: %#v", decision.Reasons)
	}
}

func TestAssessMLNecessityRecommendsOnlyAfterIndependentImprovement(t *testing.T) {
	records := make([]AICalibrationRecord, 0, 24)
	for index := 0; index < 24; index++ {
		x := float64(index) / 23
		split := ExpertSplitTraining
		if index >= 12 {
			split = ExpertSplitEvaluation
		}
		records = append(records, AICalibrationRecord{
			PresentationID: "E-" + string(rune('A'+index)),
			Split:          split,
			ExpertScore:    1 + 4*x,
			ExpertCount:    2,
			Features: AIDeterministicFeatures{
				Coastline: x,
				Overall:   float64((index*11)%23) / 22,
			},
		})
	}
	config := DefaultAINecessityConfig()
	config.RidgeLambda = 0.001
	decision, err := AssessMLNecessity(records, config)
	if err != nil {
		t.Fatalf("проверка AI-02: %v", err)
	}
	if !decision.MLRecommended {
		t.Fatalf("ML должен быть рекомендован при доказанном улучшении: %#v", decision)
	}
	if decision.RMSEImprovement < config.MinimumRMSEImprovement {
		t.Fatalf("улучшение RMSE = %.4f, требуется минимум %.4f", decision.RMSEImprovement, config.MinimumRMSEImprovement)
	}
}
