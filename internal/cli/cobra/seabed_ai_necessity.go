package cobra

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"text/tabwriter"
	"time"

	adaptivemodel "coastal-geometry/internal/domain/adaptive"

	"github.com/spf13/cobra"
)

var (
	seabedAINecessityOrganizerKey   string
	seabedAINecessityRatings        string
	seabedAINecessityOutput         string
	seabedAINecessityMinExperts     int
	seabedAINecessityMinTraining    int
	seabedAINecessityMinEvaluation  int
	seabedAINecessityRidgeLambda    float64
	seabedAINecessityMinImprovement float64
)

var seabedAINecessityCmd = &cobra.Command{
	Use:   "assess-ml",
	Short: "Проверить необходимость ML по экспертным оценкам",
	Long: `Сопоставляет заполненные экспертные оценки AI-01 с детерминированными
компонентами рейтинга ADAPT-03. Прозрачная база использует только итоговый
балл ADAPT-03; ridge-кандидат обучается лишь на geographic training-карточках
и проверяется только на evaluation-карточках.

Команда рекомендует ML только при достаточном числе оценок, улучшении
независимого RMSE и отсутствии уже достаточного согласия с детерминированной
метрикой. Без разметки она сохраняет отчёт «ML не рекомендуется».`,
	RunE: runSeabedAINecessity,
}

func init() {
	seabedCmd.AddCommand(seabedAINecessityCmd)
	seabedAINecessityCmd.Flags().StringVar(&seabedAINecessityOrganizerKey, "organizer-key", "output/seabed/expert-set/organizer-key.json", "закрытый ключ набора AI-01")
	seabedAINecessityCmd.Flags().StringVar(&seabedAINecessityRatings, "ratings", "output/seabed/expert-set/expert-rating-template.tsv", "заполненный TSV-лист экспертных оценок")
	seabedAINecessityCmd.Flags().StringVar(&seabedAINecessityOutput, "output", "output", "корневой каталог результатов")
	seabedAINecessityCmd.Flags().IntVar(&seabedAINecessityMinExperts, "min-experts-per-card", 2, "минимум независимых экспертов на карточку")
	seabedAINecessityCmd.Flags().IntVar(&seabedAINecessityMinTraining, "min-training", 12, "минимум размеченных карточек training")
	seabedAINecessityCmd.Flags().IntVar(&seabedAINecessityMinEvaluation, "min-evaluation", 12, "минимум размеченных карточек evaluation")
	seabedAINecessityCmd.Flags().Float64Var(&seabedAINecessityRidgeLambda, "ridge-lambda", 1, "регуляризация ridge-кандидата")
	seabedAINecessityCmd.Flags().Float64Var(&seabedAINecessityMinImprovement, "min-rmse-improvement", 0.10, "минимальное относительное улучшение независимого RMSE")
}

type aiNecessityOptions struct {
	OrganizerKeyPath string
	RatingsPath      string
	OutputRoot       string
	Config           adaptivemodel.AINecessityConfig
}

type aiNecessityResult struct {
	directory  string
	reportPath string
	tsvPath    string
	modelPath  string
	logPath    string
	report     adaptivemodel.AINecessityReport
}

func runSeabedAINecessity(_ *cobra.Command, _ []string) error {
	result, err := executeSeabedAINecessity(aiNecessityOptions{
		OrganizerKeyPath: seabedAINecessityOrganizerKey,
		RatingsPath:      seabedAINecessityRatings,
		OutputRoot:       seabedAINecessityOutput,
		Config: adaptivemodel.AINecessityConfig{
			MinimumExpertsPerCard:  seabedAINecessityMinExperts,
			MinimumTrainingCards:   seabedAINecessityMinTraining,
			MinimumEvaluationCards: seabedAINecessityMinEvaluation,
			RidgeLambda:            seabedAINecessityRidgeLambda,
			MinimumRMSEImprovement: seabedAINecessityMinImprovement,
		},
	})
	if err != nil {
		return err
	}
	if quiet {
		return nil
	}
	decision := "ML не рекомендуется"
	if result.report.Decision.MLRecommended {
		decision = "ML-кандидат рекомендуется для следующей проверяемой итерации"
	}
	writer := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(writer, "Показатель\tЗначение")
	fmt.Fprintln(writer, "----------\t--------")
	fmt.Fprintf(writer, "Оценок экспертов\t%d\n", result.report.RatingRowCount)
	fmt.Fprintf(writer, "Карточек training/evaluation\t%d / %d\n", result.report.Decision.TrainingRecordCount, result.report.Decision.EvaluationRecordCount)
	fmt.Fprintf(writer, "Минимум экспертов на карточку\t%d\n", result.report.Config.MinimumExpertsPerCard)
	fmt.Fprintf(writer, "Решение\t%s\n", decision)
	fmt.Fprintf(writer, "Отчёт\t%s\n", result.reportPath)
	fmt.Fprintf(writer, "Модель\t%s\n", result.modelPath)
	fmt.Fprintf(writer, "Журнал\t%s\n", result.logPath)
	return writer.Flush()
}

func executeSeabedAINecessity(options aiNecessityOptions) (aiNecessityResult, error) {
	if strings.TrimSpace(options.OrganizerKeyPath) == "" || strings.TrimSpace(options.RatingsPath) == "" || strings.TrimSpace(options.OutputRoot) == "" {
		return aiNecessityResult{}, fmt.Errorf("AI-02 требует закрытый ключ, лист оценок и каталог output")
	}
	config, err := normalizeCLIConfig(options.Config)
	if err != nil {
		return aiNecessityResult{}, err
	}
	manifest, err := adaptivemodel.ReadExpertSetManifest(options.OrganizerKeyPath)
	if err != nil {
		return aiNecessityResult{}, err
	}
	organizerSHA, err := expertFileSHA256(options.OrganizerKeyPath)
	if err != nil {
		return aiNecessityResult{}, err
	}
	comparisonSHA, err := expertFileSHA256(manifest.ComparisonReport)
	if err != nil {
		return aiNecessityResult{}, err
	}
	if comparisonSHA != manifest.ComparisonReportSHA256 {
		return aiNecessityResult{}, fmt.Errorf("SHA-256 отчёта ADAPT-03 не совпадает с закрытым ключом AI-01")
	}
	comparison, err := readExpertComparison(manifest.ComparisonReport)
	if err != nil {
		return aiNecessityResult{}, err
	}
	if err := verifyExpertComparisonInputs(comparison); err != nil {
		return aiNecessityResult{}, err
	}
	ratingsSHA, err := expertFileSHA256(options.RatingsPath)
	if err != nil {
		return aiNecessityResult{}, err
	}
	ratings, err := adaptivemodel.ReadExpertRatingsTSV(options.RatingsPath)
	if err != nil {
		return aiNecessityResult{}, err
	}
	records, err := aiRecordsFromRatings(manifest, comparison, ratings)
	if err != nil {
		return aiNecessityResult{}, err
	}
	decision, err := adaptivemodel.AssessMLNecessity(records, config)
	if err != nil {
		return aiNecessityResult{}, err
	}
	aggregates := adaptivemodel.AggregateExpertRatings(ratings)
	expertIDs := make(map[string]struct{})
	for _, rating := range ratings {
		expertIDs[rating.ExpertID] = struct{}{}
	}
	report := adaptivemodel.AINecessityReport{
		SchemaVersion: adaptivemodel.AINecessitySchemaVersion,
		GeneratedAt:   time.Now().UTC().Format(time.RFC3339),
		Inputs: adaptivemodel.AINecessityInputs{
			OrganizerKey: options.OrganizerKeyPath, OrganizerKeySHA256: organizerSHA,
			ComparisonReport: manifest.ComparisonReport, ComparisonSHA256: comparisonSHA,
			Ratings: options.RatingsPath, RatingsSHA256: ratingsSHA,
		},
		Config:              config,
		RatingRowCount:      len(ratings),
		UniqueExpertCount:   len(expertIDs),
		AggregatedCardCount: len(aggregates),
		Decision:            decision,
	}
	directory := filepath.Join(options.OutputRoot, "seabed", "ai-necessity")
	reportPath := filepath.Join(directory, "ai-necessity.json")
	tsvPath := filepath.Join(directory, "ai-necessity.tsv")
	modelPath := filepath.Join(directory, "model-assessment.json")
	logPath := filepath.Join(directory, "ai-necessity.log")
	if err := adaptivemodel.WriteAINecessityJSON(reportPath, report); err != nil {
		return aiNecessityResult{}, err
	}
	if err := adaptivemodel.WriteAINecessityTSV(tsvPath, decision); err != nil {
		return aiNecessityResult{}, err
	}
	if err := adaptivemodel.WriteAIModelJSON(modelPath, decision); err != nil {
		return aiNecessityResult{}, err
	}
	if err := writeAINecessityLog(logPath, report); err != nil {
		return aiNecessityResult{}, err
	}
	return aiNecessityResult{directory: directory, reportPath: reportPath, tsvPath: tsvPath, modelPath: modelPath, logPath: logPath, report: report}, nil
}

func normalizeCLIConfig(config adaptivemodel.AINecessityConfig) (adaptivemodel.AINecessityConfig, error) {
	if config.MaximumDeterministicAbsRho == 0 {
		config.MaximumDeterministicAbsRho = adaptivemodel.DefaultAINecessityConfig().MaximumDeterministicAbsRho
	}
	if config.MinimumExpertsPerCard < 2 || config.MinimumTrainingCards < 2 || config.MinimumEvaluationCards < 2 || config.RidgeLambda <= 0 || config.MinimumRMSEImprovement <= 0 || config.MinimumRMSEImprovement >= 1 {
		return adaptivemodel.AINecessityConfig{}, fmt.Errorf("нужно минимум два независимых эксперта на карточку; остальные пороги должны быть положительными, а улучшение RMSE — в интервале (0, 1)")
	}
	return config, nil
}

func aiRecordsFromRatings(manifest adaptivemodel.ExpertSetManifest, comparison adaptivemodel.AdaptiveComparisonReport, ratings []adaptivemodel.ExpertRating) ([]adaptivemodel.AICalibrationRecord, error) {
	cards := make(map[string]adaptivemodel.ExpertSetCard, len(manifest.Cards))
	for _, card := range manifest.Cards {
		cards[card.PresentationID] = card
	}
	features := make(map[string]adaptivemodel.AIDeterministicFeatures)
	for _, level := range comparison.Levels {
		for _, run := range level.Runs {
			if !run.Success {
				continue
			}
			features[aiFeatureKey(level.Level.ID, string(run.Algorithm))] = adaptivemodel.AIDeterministicFeatures{
				Overall: run.Score.Overall, Coastline: run.Score.Coastline, Bathymetry: run.Score.Bathymetry,
				CellGeometry: run.Score.CellGeometry, TargetSizeCompliance: run.Score.TargetSizeCompliance, Efficiency: run.Score.Efficiency,
			}
		}
	}
	records := make([]adaptivemodel.AICalibrationRecord, 0)
	for _, aggregate := range adaptivemodel.AggregateExpertRatings(ratings) {
		card, exists := cards[aggregate.PresentationID]
		if !exists {
			return nil, fmt.Errorf("лист оценок содержит карточку %q, отсутствующую в закрытом ключе", aggregate.PresentationID)
		}
		feature, exists := features[aiFeatureKey(card.LevelID, card.Algorithm)]
		if !exists {
			return nil, fmt.Errorf("для карточки %q отсутствует успешный запуск ADAPT-03", card.PresentationID)
		}
		records = append(records, adaptivemodel.AICalibrationRecord{PresentationID: card.PresentationID, Split: card.Split, ExpertScore: aggregate.MeanScore, ExpertCount: aggregate.ExpertCount, Features: feature})
	}
	return records, nil
}

func aiFeatureKey(levelID, algorithm string) string {
	return levelID + "\x00" + algorithm
}

func writeAINecessityLog(path string, report adaptivemodel.AINecessityReport) error {
	decision := "ML не рекомендуется"
	if report.Decision.MLRecommended {
		decision = "ML-кандидат рекомендуется"
	}
	data := fmt.Sprintf("AI-02 — проверка необходимости ML\nОценок: %d\nКарточек training/evaluation: %d/%d\nРешение: %s\nRMSE базы: %.4f\nRMSE ridge: %.4f\nУлучшение: %.2f%%\n", report.RatingRowCount, report.Decision.TrainingRecordCount, report.Decision.EvaluationRecordCount, decision, report.Decision.BaselineEvaluation.RMSE, report.Decision.CandidateEvaluation.RMSE, 100*report.Decision.RMSEImprovement)
	return writeExpertText(path, []byte(data))
}
