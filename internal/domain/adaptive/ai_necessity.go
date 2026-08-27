package adaptive

import (
	"bufio"
	"encoding/json"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
)

const (
	// AINecessitySchemaVersion задаёт машинный контракт решения AI-02.
	AINecessitySchemaVersion = "lito-ai-necessity/v1"
	// AIRidgeModelVersion задаёт версию прозрачного ML-кандидата AI-02.
	AIRidgeModelVersion = "lito-ai-ridge/v1"
	// AIBaselineModelVersion задаёт версию прозрачной калибровки итогового
	// детерминированного балла ADAPT-03.
	AIBaselineModelVersion = "lito-deterministic-baseline/v1"
)

// ExpertRating — одна заполненная строка экспертной оценки AI-01.
type ExpertRating struct {
	PresentationID string  `json:"presentation_id"`
	ExpertID       string  `json:"expert_id"`
	Coastline      float64 `json:"coastline"`
	Isobaths       float64 `json:"isobaths"`
	Mesh           float64 `json:"mesh"`
	Artifacts      float64 `json:"artifacts"`
	Comment        string  `json:"comment,omitempty"`
}

// Overall возвращает средний экспертный балл по четырём фиксированным шкалам.
func (rating ExpertRating) Overall() float64 {
	return (rating.Coastline + rating.Isobaths + rating.Mesh + rating.Artifacts) / 4
}

// ExpertRatingAggregate объединяет оценки разных экспертов для одной карточки.
type ExpertRatingAggregate struct {
	PresentationID string  `json:"presentation_id"`
	ExpertCount    int     `json:"expert_count"`
	MeanScore      float64 `json:"mean_score"`
	StdDev         float64 `json:"stddev"`
}

// AIDeterministicFeatures содержит компоненты воспроизводимого рейтинга
// ADAPT-03, сопоставляемые с экспертным баллом.
type AIDeterministicFeatures struct {
	Overall              float64 `json:"overall"`
	Coastline            float64 `json:"coastline"`
	Bathymetry           float64 `json:"bathymetry"`
	CellGeometry         float64 `json:"cell_geometry"`
	TargetSizeCompliance float64 `json:"target_size_compliance"`
	Efficiency           float64 `json:"efficiency"`
}

// AICalibrationRecord связывает закрытый ключ карточки с агрегированной
// оценкой экспертов и известными до разметки детерминированными признаками.
type AICalibrationRecord struct {
	PresentationID string                  `json:"presentation_id"`
	Split          ExpertSplit             `json:"split"`
	ExpertScore    float64                 `json:"expert_score"`
	ExpertCount    int                     `json:"expert_count"`
	Features       AIDeterministicFeatures `json:"features"`
}

// AICorrelation описывает связь одного детерминированного признака с
// экспертным баллом. Available=false означает постоянный или слишком малый ряд.
type AICorrelation struct {
	Feature   string  `json:"feature"`
	Pearson   float64 `json:"pearson"`
	Spearman  float64 `json:"spearman"`
	Available bool    `json:"available"`
}

// AILinearModel хранит версию, признаки и параметры базовой либо ridge-модели.
type AILinearModel struct {
	Version        string    `json:"version"`
	Kind           string    `json:"kind"`
	Trained        bool      `json:"trained"`
	FeatureNames   []string  `json:"feature_names"`
	FeatureMeans   []float64 `json:"feature_means"`
	FeatureStdDevs []float64 `json:"feature_stddevs"`
	Intercept      float64   `json:"intercept"`
	Coefficients   []float64 `json:"coefficients"`
	RidgeLambda    float64   `json:"ridge_lambda"`
	TrainingCount  int       `json:"training_count"`
	OutputMinimum  float64   `json:"output_minimum"`
	OutputMaximum  float64   `json:"output_maximum"`
	Limitation     string    `json:"limitation"`
}

// AIEvaluationMetrics — ошибка и корреляция только на указанной части набора.
type AIEvaluationMetrics struct {
	Available bool    `json:"available"`
	Count     int     `json:"count"`
	MAE       float64 `json:"mae"`
	RMSE      float64 `json:"rmse"`
	Pearson   float64 `json:"pearson"`
	Spearman  float64 `json:"spearman"`
}

// AINecessityConfig задаёт заранее зафиксированные шлюзы решения AI-02.
type AINecessityConfig struct {
	MinimumExpertsPerCard      int     `json:"minimum_experts_per_card"`
	MinimumTrainingCards       int     `json:"minimum_training_cards"`
	MinimumEvaluationCards     int     `json:"minimum_evaluation_cards"`
	RidgeLambda                float64 `json:"ridge_lambda"`
	MinimumRMSEImprovement     float64 `json:"minimum_rmse_improvement"`
	MaximumDeterministicAbsRho float64 `json:"maximum_deterministic_abs_rho"`
}

// DefaultAINecessityConfig возвращает консервативные пороги AI-02: одной
// заполненной укрупнённой серии недостаточно, а ML-кандидат обязан улучшить
// независимую ошибку минимум на десять процентов.
func DefaultAINecessityConfig() AINecessityConfig {
	return AINecessityConfig{
		MinimumExpertsPerCard:      2,
		MinimumTrainingCards:       12,
		MinimumEvaluationCards:     12,
		RidgeLambda:                1,
		MinimumRMSEImprovement:     0.10,
		MaximumDeterministicAbsRho: 0.80,
	}
}

// AINecessityDecision хранит сравнение прозрачной базы с ridge-кандидатом.
type AINecessityDecision struct {
	TrainingRecordCount   int                 `json:"training_record_count"`
	EvaluationRecordCount int                 `json:"evaluation_record_count"`
	Correlations          []AICorrelation     `json:"correlations"`
	BaselineModel         AILinearModel       `json:"baseline_model"`
	CandidateModel        AILinearModel       `json:"candidate_model"`
	BaselineEvaluation    AIEvaluationMetrics `json:"baseline_evaluation"`
	CandidateEvaluation   AIEvaluationMetrics `json:"candidate_evaluation"`
	RMSEImprovement       float64             `json:"rmse_improvement"`
	MLRecommended         bool                `json:"ml_recommended"`
	Reasons               []string            `json:"reasons"`
}

// AINecessityInputs фиксирует проверяемые входы решения AI-02.
type AINecessityInputs struct {
	OrganizerKey       string `json:"organizer_key"`
	OrganizerKeySHA256 string `json:"organizer_key_sha256"`
	ComparisonReport   string `json:"comparison_report"`
	ComparisonSHA256   string `json:"comparison_sha256"`
	Ratings            string `json:"ratings"`
	RatingsSHA256      string `json:"ratings_sha256"`
}

// AINecessityReport объединяет исходные файлы, параметры, число оценок и
// решение AI-02 в одном воспроизводимом отчёте.
type AINecessityReport struct {
	SchemaVersion       string              `json:"schema_version"`
	GeneratedAt         string              `json:"generated_at"`
	Inputs              AINecessityInputs   `json:"inputs"`
	Config              AINecessityConfig   `json:"config"`
	RatingRowCount      int                 `json:"rating_row_count"`
	UniqueExpertCount   int                 `json:"unique_expert_count"`
	AggregatedCardCount int                 `json:"aggregated_card_count"`
	Decision            AINecessityDecision `json:"decision"`
}

// ReadExpertRatingsTSV читает заполненный лист AI-01. Пустые строки шаблона
// допускаются и не считаются оценками; частично заполненная оценка отклоняется.
func ReadExpertRatingsTSV(path string) ([]ExpertRating, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("открытие листа экспертных оценок %q: %w", path, err)
	}
	defer file.Close()
	scanner := bufio.NewScanner(file)
	scanner.Buffer(make([]byte, 64*1024), 4*1024*1024)
	ratings := make([]ExpertRating, 0)
	seen := make(map[string]struct{})
	lineNumber := 0
	for scanner.Scan() {
		lineNumber++
		line := strings.TrimRight(scanner.Text(), "\r")
		if lineNumber <= 2 || strings.TrimSpace(line) == "" {
			continue
		}
		fields := strings.Split(line, "\t")
		if len(fields) < 8 {
			legacy := strings.Fields(line)
			if len(legacy) == 2 && strings.HasPrefix(legacy[0], "E-") {
				continue
			}
			return nil, fmt.Errorf("строка %d листа оценок должна содержать восемь TSV-полей", lineNumber)
		}
		if len(fields) > 8 {
			fields = append(fields[:7], strings.Join(fields[7:], "\t"))
		}
		for index := range fields {
			fields[index] = strings.TrimSpace(fields[index])
		}
		scoreFields := fields[3:7]
		nonEmpty := 0
		for _, value := range scoreFields {
			if value != "" {
				nonEmpty++
			}
		}
		if nonEmpty == 0 {
			continue
		}
		if nonEmpty != len(scoreFields) || fields[0] == "" || fields[2] == "" {
			return nil, fmt.Errorf("строка %d листа оценок должна содержать карточку, эксперта и все четыре балла", lineNumber)
		}
		scores := [4]float64{}
		for index, value := range scoreFields {
			score, parseErr := strconv.ParseFloat(strings.ReplaceAll(value, ",", "."), 64)
			if parseErr != nil || math.Trunc(score) != score || score < 1 || score > 5 {
				return nil, fmt.Errorf("строка %d листа оценок содержит недопустимый балл %q; разрешены целые 1–5", lineNumber, value)
			}
			scores[index] = score
		}
		key := fields[0] + "\x00" + fields[2]
		if _, exists := seen[key]; exists {
			return nil, fmt.Errorf("эксперт %q повторно оценил карточку %q", fields[2], fields[0])
		}
		seen[key] = struct{}{}
		ratings = append(ratings, ExpertRating{PresentationID: fields[0], ExpertID: fields[2], Coastline: scores[0], Isobaths: scores[1], Mesh: scores[2], Artifacts: scores[3], Comment: fields[7]})
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("чтение листа оценок %q: %w", path, err)
	}
	return ratings, nil
}

// AggregateExpertRatings усредняет независимые оценки одной карточки.
func AggregateExpertRatings(ratings []ExpertRating) []ExpertRatingAggregate {
	values := make(map[string][]float64)
	for _, rating := range ratings {
		values[rating.PresentationID] = append(values[rating.PresentationID], rating.Overall())
	}
	result := make([]ExpertRatingAggregate, 0, len(values))
	for presentationID, scores := range values {
		mean := 0.0
		for _, score := range scores {
			mean += score
		}
		mean /= float64(len(scores))
		variance := 0.0
		for _, score := range scores {
			variance += (score - mean) * (score - mean)
		}
		if len(scores) > 1 {
			variance /= float64(len(scores) - 1)
		}
		result = append(result, ExpertRatingAggregate{PresentationID: presentationID, ExpertCount: len(scores), MeanScore: mean, StdDev: math.Sqrt(variance)})
	}
	sort.Slice(result, func(left, right int) bool { return result[left].PresentationID < result[right].PresentationID })
	return result
}

// AssessMLNecessity проверяет необходимость ML только на независимой
// evaluation-части. Кандидат не рекомендуют, если прозрачный итоговый балл
// ADAPT-03 уже достаточно согласован с экспертами или улучшение ошибки мало.
func AssessMLNecessity(records []AICalibrationRecord, config AINecessityConfig) (AINecessityDecision, error) {
	config, err := normalizeAINecessityConfig(config)
	if err != nil {
		return AINecessityDecision{}, err
	}
	qualified := aiQualifiedRecords(records, config.MinimumExpertsPerCard)
	training, evaluation := splitAIRecords(qualified)
	decision := AINecessityDecision{
		TrainingRecordCount:   len(training),
		EvaluationRecordCount: len(evaluation),
		Correlations:          aiCorrelations(qualified),
		BaselineModel:         emptyAIModel(AIBaselineModelVersion, "детерминированная калибровка", []string{"overall"}, 0),
		CandidateModel:        emptyAIModel(AIRidgeModelVersion, "ridge-кандидат", aiComponentFeatureNames(), config.RidgeLambda),
		Reasons:               []string{},
	}
	if len(training) < config.MinimumTrainingCards || len(evaluation) < config.MinimumEvaluationCards {
		decision.Reasons = append(decision.Reasons, fmt.Sprintf("недостаточно карточек с минимум %d независимыми экспертами: training=%d из %d, evaluation=%d из %d", config.MinimumExpertsPerCard, len(training), config.MinimumTrainingCards, len(evaluation), config.MinimumEvaluationCards))
		return decision, nil
	}
	baseline, err := fitAIModel(training, []string{"overall"}, 0, AIBaselineModelVersion, "детерминированная калибровка")
	if err != nil {
		return AINecessityDecision{}, fmt.Errorf("обучение прозрачной базы AI-02: %w", err)
	}
	candidate, err := fitAIModel(training, aiComponentFeatureNames(), config.RidgeLambda, AIRidgeModelVersion, "ridge-кандидат")
	if err != nil {
		return AINecessityDecision{}, fmt.Errorf("обучение ridge-кандидата AI-02: %w", err)
	}
	decision.BaselineModel, decision.CandidateModel = baseline, candidate
	decision.BaselineEvaluation = evaluateAIModel(baseline, evaluation)
	decision.CandidateEvaluation = evaluateAIModel(candidate, evaluation)
	if !decision.BaselineEvaluation.Available || !decision.CandidateEvaluation.Available || decision.BaselineEvaluation.RMSE <= 0 {
		decision.Reasons = append(decision.Reasons, "независимая ошибка базы или кандидата не определена")
		return decision, nil
	}
	decision.RMSEImprovement = (decision.BaselineEvaluation.RMSE - decision.CandidateEvaluation.RMSE) / decision.BaselineEvaluation.RMSE
	overallCorrelation := correlationByFeature(decision.Correlations, "overall")
	if overallCorrelation.Available && math.Abs(overallCorrelation.Spearman) >= config.MaximumDeterministicAbsRho {
		decision.Reasons = append(decision.Reasons, fmt.Sprintf("итоговый детерминированный балл уже сильно согласован с экспертами: |ρ|=%.3f ≥ %.3f", math.Abs(overallCorrelation.Spearman), config.MaximumDeterministicAbsRho))
	}
	if decision.RMSEImprovement < config.MinimumRMSEImprovement {
		decision.Reasons = append(decision.Reasons, fmt.Sprintf("ridge-кандидат улучшил независимый RMSE только на %.1f%% при требовании %.1f%%", 100*decision.RMSEImprovement, 100*config.MinimumRMSEImprovement))
	}
	if decision.CandidateEvaluation.Spearman+1e-12 < decision.BaselineEvaluation.Spearman {
		decision.Reasons = append(decision.Reasons, "ridge-кандидат ухудшил ранговое согласие на независимой выборке")
	}
	decision.MLRecommended = len(decision.Reasons) == 0
	if decision.MLRecommended {
		decision.Reasons = append(decision.Reasons, fmt.Sprintf("ridge-кандидат улучшил независимый RMSE на %.1f%% и не уступил прозрачной базе по ранговому согласию", 100*decision.RMSEImprovement))
	}
	return decision, nil
}

func normalizeAINecessityConfig(config AINecessityConfig) (AINecessityConfig, error) {
	defaults := DefaultAINecessityConfig()
	if config.MinimumExpertsPerCard == 0 {
		config.MinimumExpertsPerCard = defaults.MinimumExpertsPerCard
	}
	if config.MinimumTrainingCards == 0 {
		config.MinimumTrainingCards = defaults.MinimumTrainingCards
	}
	if config.MinimumEvaluationCards == 0 {
		config.MinimumEvaluationCards = defaults.MinimumEvaluationCards
	}
	if config.RidgeLambda == 0 {
		config.RidgeLambda = defaults.RidgeLambda
	}
	if config.MinimumRMSEImprovement == 0 {
		config.MinimumRMSEImprovement = defaults.MinimumRMSEImprovement
	}
	if config.MaximumDeterministicAbsRho == 0 {
		config.MaximumDeterministicAbsRho = defaults.MaximumDeterministicAbsRho
	}
	if config.MinimumExpertsPerCard < 2 || config.MinimumTrainingCards < 2 || config.MinimumEvaluationCards < 2 || config.RidgeLambda <= 0 || config.MinimumRMSEImprovement <= 0 || config.MinimumRMSEImprovement >= 1 || config.MaximumDeterministicAbsRho <= 0 || config.MaximumDeterministicAbsRho > 1 {
		return AINecessityConfig{}, fmt.Errorf("некорректные пороги AI-02")
	}
	return config, nil
}

// aiQualifiedRecords исключает одиночные оценки: решение AI-02 должно
// опираться минимум на две независимые оценки каждой карточки.
func aiQualifiedRecords(records []AICalibrationRecord, minimumExperts int) []AICalibrationRecord {
	qualified := make([]AICalibrationRecord, 0, len(records))
	for _, record := range records {
		if record.ExpertCount >= minimumExperts {
			qualified = append(qualified, record)
		}
	}
	return qualified
}

func splitAIRecords(records []AICalibrationRecord) ([]AICalibrationRecord, []AICalibrationRecord) {
	training, evaluation := make([]AICalibrationRecord, 0), make([]AICalibrationRecord, 0)
	for _, record := range records {
		switch record.Split {
		case ExpertSplitTraining:
			training = append(training, record)
		case ExpertSplitEvaluation:
			evaluation = append(evaluation, record)
		}
	}
	return training, evaluation
}

func aiCorrelations(records []AICalibrationRecord) []AICorrelation {
	features := append([]string{"overall"}, aiComponentFeatureNames()...)
	result := make([]AICorrelation, 0, len(features))
	for _, feature := range features {
		values, targets := make([]float64, 0, len(records)), make([]float64, 0, len(records))
		for _, record := range records {
			values, targets = append(values, aiFeatureValue(record.Features, feature)), append(targets, record.ExpertScore)
		}
		pearson, available := aiPearson(values, targets)
		spearman, rankAvailable := aiSpearman(values, targets)
		result = append(result, AICorrelation{Feature: feature, Pearson: pearson, Spearman: spearman, Available: available && rankAvailable})
	}
	return result
}

func aiComponentFeatureNames() []string {
	return []string{"coastline", "bathymetry", "cell_geometry", "target_size_compliance", "efficiency"}
}

func correlationByFeature(values []AICorrelation, feature string) AICorrelation {
	for _, value := range values {
		if value.Feature == feature {
			return value
		}
	}
	return AICorrelation{Feature: feature}
}

func emptyAIModel(version, kind string, features []string, lambda float64) AILinearModel {
	return AILinearModel{Version: version, Kind: kind, Trained: false, FeatureNames: append([]string(nil), features...), FeatureMeans: []float64{}, FeatureStdDevs: []float64{}, Coefficients: []float64{}, RidgeLambda: lambda, OutputMinimum: 1, OutputMaximum: 5, Limitation: "Модель не применяется без достаточной разметки и независимой проверки."}
}

func fitAIModel(records []AICalibrationRecord, features []string, lambda float64, version, kind string) (AILinearModel, error) {
	if len(records) < 2 || len(features) == 0 {
		return AILinearModel{}, fmt.Errorf("для обучения AI-02 нужны минимум две записи и один признак")
	}
	model := emptyAIModel(version, kind, features, lambda)
	model.Trained, model.TrainingCount = true, len(records)
	model.FeatureMeans, model.FeatureStdDevs = make([]float64, len(features)), make([]float64, len(features))
	for index, feature := range features {
		for _, record := range records {
			model.FeatureMeans[index] += aiFeatureValue(record.Features, feature)
		}
		model.FeatureMeans[index] /= float64(len(records))
		for _, record := range records {
			delta := aiFeatureValue(record.Features, feature) - model.FeatureMeans[index]
			model.FeatureStdDevs[index] += delta * delta
		}
		model.FeatureStdDevs[index] = math.Sqrt(model.FeatureStdDevs[index] / float64(len(records)))
		if model.FeatureStdDevs[index] < 1e-9 {
			model.FeatureStdDevs[index] = 1
		}
	}
	dimension := len(features) + 1
	matrix := make([][]float64, dimension)
	for row := range matrix {
		matrix[row] = make([]float64, dimension+1)
	}
	for _, record := range records {
		values := aiModelValues(model, record.Features)
		values = append([]float64{1}, values...)
		for row := 0; row < dimension; row++ {
			for column := 0; column < dimension; column++ {
				matrix[row][column] += values[row] * values[column]
			}
			matrix[row][dimension] += values[row] * record.ExpertScore
		}
	}
	for index := 1; index < dimension; index++ {
		matrix[index][index] += lambda
	}
	solution, err := aiSolveLinearSystem(matrix)
	if err != nil {
		return AILinearModel{}, err
	}
	model.Intercept, model.Coefficients = solution[0], append([]float64(nil), solution[1:]...)
	return model, nil
}

func aiModelValues(model AILinearModel, features AIDeterministicFeatures) []float64 {
	values := make([]float64, len(model.FeatureNames))
	for index, name := range model.FeatureNames {
		values[index] = (aiFeatureValue(features, name) - model.FeatureMeans[index]) / model.FeatureStdDevs[index]
	}
	return values
}

func aiFeatureValue(features AIDeterministicFeatures, name string) float64 {
	switch name {
	case "overall":
		return features.Overall
	case "coastline":
		return features.Coastline
	case "bathymetry":
		return features.Bathymetry
	case "cell_geometry":
		return features.CellGeometry
	case "target_size_compliance":
		return features.TargetSizeCompliance
	case "efficiency":
		return features.Efficiency
	default:
		return 0
	}
}

func aiSolveLinearSystem(matrix [][]float64) ([]float64, error) {
	dimension := len(matrix)
	for pivot := 0; pivot < dimension; pivot++ {
		best := pivot
		for row := pivot + 1; row < dimension; row++ {
			if math.Abs(matrix[row][pivot]) > math.Abs(matrix[best][pivot]) {
				best = row
			}
		}
		if math.Abs(matrix[best][pivot]) < 1e-12 {
			return nil, fmt.Errorf("матрица обучения AI-02 вырождена")
		}
		matrix[pivot], matrix[best] = matrix[best], matrix[pivot]
		factor := matrix[pivot][pivot]
		for column := pivot; column <= dimension; column++ {
			matrix[pivot][column] /= factor
		}
		for row := 0; row < dimension; row++ {
			if row == pivot {
				continue
			}
			factor = matrix[row][pivot]
			for column := pivot; column <= dimension; column++ {
				matrix[row][column] -= factor * matrix[pivot][column]
			}
		}
	}
	result := make([]float64, dimension)
	for index := range result {
		result[index] = matrix[index][dimension]
	}
	return result, nil
}

func evaluateAIModel(model AILinearModel, records []AICalibrationRecord) AIEvaluationMetrics {
	if !model.Trained || len(records) == 0 {
		return AIEvaluationMetrics{}
	}
	predicted, actual := make([]float64, 0, len(records)), make([]float64, 0, len(records))
	mae, squared := 0.0, 0.0
	for _, record := range records {
		value := model.Intercept
		for index, feature := range aiModelValues(model, record.Features) {
			value += model.Coefficients[index] * feature
		}
		value = math.Max(model.OutputMinimum, math.Min(model.OutputMaximum, value))
		delta := value - record.ExpertScore
		mae, squared = mae+math.Abs(delta), squared+delta*delta
		predicted, actual = append(predicted, value), append(actual, record.ExpertScore)
	}
	pearson, pearsonOK := aiPearson(predicted, actual)
	spearman, spearmanOK := aiSpearman(predicted, actual)
	return AIEvaluationMetrics{Available: pearsonOK && spearmanOK, Count: len(records), MAE: mae / float64(len(records)), RMSE: math.Sqrt(squared / float64(len(records))), Pearson: pearson, Spearman: spearman}
}

func aiPearson(left, right []float64) (float64, bool) {
	if len(left) != len(right) || len(left) < 2 {
		return 0, false
	}
	leftMean, rightMean := 0.0, 0.0
	for index := range left {
		leftMean, rightMean = leftMean+left[index], rightMean+right[index]
	}
	leftMean, rightMean = leftMean/float64(len(left)), rightMean/float64(len(right))
	covariance, leftVariance, rightVariance := 0.0, 0.0, 0.0
	for index := range left {
		leftDelta, rightDelta := left[index]-leftMean, right[index]-rightMean
		covariance, leftVariance, rightVariance = covariance+leftDelta*rightDelta, leftVariance+leftDelta*leftDelta, rightVariance+rightDelta*rightDelta
	}
	if leftVariance <= 1e-12 || rightVariance <= 1e-12 {
		return 0, false
	}
	return covariance / math.Sqrt(leftVariance*rightVariance), true
}

func aiSpearman(left, right []float64) (float64, bool) {
	return aiPearson(aiRanks(left), aiRanks(right))
}

func aiRanks(values []float64) []float64 {
	type item struct {
		value float64
		index int
	}
	items := make([]item, len(values))
	for index, value := range values {
		items[index] = item{value: value, index: index}
	}
	sort.SliceStable(items, func(left, right int) bool { return items[left].value < items[right].value })
	ranks := make([]float64, len(values))
	for start := 0; start < len(items); {
		end := start + 1
		for end < len(items) && math.Abs(items[end].value-items[start].value) <= 1e-12 {
			end++
		}
		rank := (float64(start+1) + float64(end)) / 2
		for index := start; index < end; index++ {
			ranks[items[index].index] = rank
		}
		start = end
	}
	return ranks
}

// WriteAINecessityJSON сохраняет полный отчёт AI-02.
func WriteAINecessityJSON(path string, report AINecessityReport) error {
	if report.SchemaVersion != AINecessitySchemaVersion {
		return fmt.Errorf("отчёт AI-02 имеет неподдерживаемую схему %q", report.SchemaVersion)
	}
	report.Decision.Correlations = append([]AICorrelation(nil), report.Decision.Correlations...)
	report.Decision.Reasons = append([]string(nil), report.Decision.Reasons...)
	data, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		return fmt.Errorf("кодирование отчёта AI-02: %w", err)
	}
	return writeAIFile(path, append(data, '\n'))
}

// WriteAIModelJSON сохраняет версии, признаки, параметры и независимые ошибки
// базы и ridge-кандидата, в том числе когда ML не рекомендован.
func WriteAIModelJSON(path string, decision AINecessityDecision) error {
	data, err := json.MarshalIndent(decision, "", "  ")
	if err != nil {
		return fmt.Errorf("кодирование модели AI-02: %w", err)
	}
	return writeAIFile(path, append(data, '\n'))
}

// WriteAINecessityTSV сохраняет компактное сравнение для просмотра без JSON.
func WriteAINecessityTSV(path string, decision AINecessityDecision) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("создание каталога TSV AI-02: %w", err)
	}
	file, err := os.Create(path)
	if err != nil {
		return fmt.Errorf("создание TSV AI-02 %q: %w", path, err)
	}
	defer file.Close()
	writer := bufio.NewWriter(file)
	if _, err := fmt.Fprintln(writer, "Показатель\tЗначение"); err != nil {
		return err
	}
	if _, err := fmt.Fprintln(writer, "----------\t--------"); err != nil {
		return err
	}
	rows := []struct{ name, value string }{
		{"Карточек training", strconv.Itoa(decision.TrainingRecordCount)},
		{"Карточек evaluation", strconv.Itoa(decision.EvaluationRecordCount)},
		{"RMSE базы", fmt.Sprintf("%.4f", decision.BaselineEvaluation.RMSE)},
		{"RMSE ridge", fmt.Sprintf("%.4f", decision.CandidateEvaluation.RMSE)},
		{"Улучшение RMSE, %", fmt.Sprintf("%.2f", 100*decision.RMSEImprovement)},
		{"ML рекомендуется", strconv.FormatBool(decision.MLRecommended)},
	}
	for _, row := range rows {
		if _, err := fmt.Fprintf(writer, "%s\t%s\n", row.name, row.value); err != nil {
			return err
		}
	}
	for _, correlation := range decision.Correlations {
		if _, err := fmt.Fprintf(writer, "ρ %s\t%.4f\n", correlation.Feature, correlation.Spearman); err != nil {
			return err
		}
	}
	if err := writer.Flush(); err != nil {
		return fmt.Errorf("запись TSV AI-02: %w", err)
	}
	return nil
}

func writeAIFile(path string, data []byte) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("создание каталога AI-02: %w", err)
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		return fmt.Errorf("запись файла AI-02 %q: %w", path, err)
	}
	return nil
}
