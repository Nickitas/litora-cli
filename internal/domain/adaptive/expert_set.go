package adaptive

import (
	"bufio"
	"encoding/json"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

const (
	// ExpertSetSchemaVersion задаёт версию машинного контракта AI-01.
	ExpertSetSchemaVersion = "lito-expert-fragment-set/v1"

	// ExpertSplitTraining содержит карточки, допустимые для настройки будущей
	// модели только после независимой экспертной разметки.
	ExpertSplitTraining ExpertSplit = "training"
	// ExpertSplitEvaluation содержит географически изолированные карточки для
	// итоговой проверки и не должен использоваться при обучении.
	ExpertSplitEvaluation ExpertSplit = "evaluation"
)

// ExpertSplit разделяет фрагменты по географическим районам, а не случайно по
// отдельным изображениям: так соседние участки одного берега не попадают в
// обучение и проверку одновременно.
type ExpertSplit string

// ExpertFragment описывает фиксированное окно экспертного набора AI-01.
// Координаты центра заданы в WGS 84, а размеры окна — в метрах LAEA.
type ExpertFragment struct {
	ID                 string      `json:"id"`
	Name               string      `json:"name"`
	Feature            string      `json:"feature"`
	RegionID           string      `json:"region_id"`
	Split              ExpertSplit `json:"split"`
	LatitudeDeg        float64     `json:"latitude_deg"`
	LongitudeDeg       float64     `json:"longitude_deg"`
	WidthM             float64     `json:"width_m"`
	HeightM            float64     `json:"height_m"`
	ReferenceIsobathsM []float64   `json:"reference_isobaths_m"`
}

// ExpertSetCard представляет одну анонимную карточку. Поля Algorithm и
// LevelID хранятся только в закрытом ключе, а не в материалах эксперта.
type ExpertSetCard struct {
	PresentationID  string      `json:"presentation_id"`
	FragmentID      string      `json:"fragment_id"`
	Feature         string      `json:"feature"`
	RegionID        string      `json:"region_id"`
	Split           ExpertSplit `json:"split"`
	LevelID         string      `json:"level_id"`
	Algorithm       string      `json:"algorithm"`
	MeshSHA256      string      `json:"mesh_sha256"`
	SVGPath         string      `json:"svg_path"`
	VisibleCells    int         `json:"visible_cells"`
	VisibleContours int         `json:"visible_contours"`
}

// ExpertSetExcludedRun фиксирует генератор, который не дал MSH и потому не
// может быть подменён пустой карточкой.
type ExpertSetExcludedRun struct {
	LevelID   string `json:"level_id"`
	Algorithm string `json:"algorithm"`
	Reason    string `json:"reason"`
}

// ExpertSetManifest связывает анонимные карточки с воспроизводимым отчётом
// ADAPT-03. Этот файл является закрытым ключом для организатора исследования.
type ExpertSetManifest struct {
	SchemaVersion          string                 `json:"schema_version"`
	GeneratedAt            string                 `json:"generated_at"`
	ComparisonReport       string                 `json:"comparison_report"`
	ComparisonReportSHA256 string                 `json:"comparison_report_sha256"`
	Seed                   string                 `json:"seed"`
	Fragments              []ExpertFragment       `json:"fragments"`
	Cards                  []ExpertSetCard        `json:"cards"`
	ExcludedRuns           []ExpertSetExcludedRun `json:"excluded_runs"`
	ParticipantInstruction string                 `json:"participant_instruction"`
	Limitation             string                 `json:"limitation"`
}

// ExpertAssignment — строка открытого листа оценивания без названия
// генератора, масштаба и принадлежности к обучающей/контрольной части.
type ExpertAssignment struct {
	PresentationID string
	Feature        string
}

// DefaultExpertFragments возвращает географически разнесённый набор из
// восьми фрагментов. В каждой части есть коса, залив, остров и изобаты.
func DefaultExpertFragments() []ExpertFragment {
	return []ExpertFragment{
		{ID: "train-tendra-spit", Name: "Тендровская коса", Feature: "коса", RegionID: "northwest-shelf", Split: ExpertSplitTraining, LatitudeDeg: 46.20, LongitudeDeg: 31.85, WidthM: 28000, HeightM: 28000, ReferenceIsobathsM: []float64{5, 10, 20, 50}},
		{ID: "train-burgas-bay", Name: "Бургасский залив", Feature: "залив", RegionID: "bulgarian-coast", Split: ExpertSplitTraining, LatitudeDeg: 42.50, LongitudeDeg: 27.50, WidthM: 28000, HeightM: 28000, ReferenceIsobathsM: []float64{10, 20, 50, 100}},
		{ID: "train-zmiinyi-island", Name: "Остров Змеиный", Feature: "остров", RegionID: "danube-shelf", Split: ExpertSplitTraining, LatitudeDeg: 45.25, LongitudeDeg: 30.21, WidthM: 24000, HeightM: 24000, ReferenceIsobathsM: []float64{10, 20, 50, 100}},
		{ID: "train-crimean-isobaths", Name: "Крымский шельф", Feature: "изобаты", RegionID: "crimean-coast", Split: ExpertSplitTraining, LatitudeDeg: 44.60, LongitudeDeg: 33.70, WidthM: 30000, HeightM: 30000, ReferenceIsobathsM: []float64{20, 50, 100, 200, 500}},
		{ID: "eval-kizilirmak-spit", Name: "Кызылырмакская коса", Feature: "коса", RegionID: "kizilirmak-delta", Split: ExpertSplitEvaluation, LatitudeDeg: 41.75, LongitudeDeg: 35.90, WidthM: 28000, HeightM: 28000, ReferenceIsobathsM: []float64{5, 10, 20, 50}},
		{ID: "eval-batumi-bay", Name: "Батумский залив", Feature: "залив", RegionID: "adjara-coast", Split: ExpertSplitEvaluation, LatitudeDeg: 41.64, LongitudeDeg: 41.65, WidthM: 30000, HeightM: 30000, ReferenceIsobathsM: []float64{20, 50, 100, 200}},
		{ID: "eval-giresun-island", Name: "Остров Гиресун", Feature: "остров", RegionID: "pontic-coast", Split: ExpertSplitEvaluation, LatitudeDeg: 40.91, LongitudeDeg: 38.43, WidthM: 24000, HeightM: 24000, ReferenceIsobathsM: []float64{20, 50, 100, 200}},
		{ID: "eval-anatolian-isobaths", Name: "Анатолийский склон", Feature: "изобаты", RegionID: "southern-basin", Split: ExpertSplitEvaluation, LatitudeDeg: 42.10, LongitudeDeg: 34.70, WidthM: 30000, HeightM: 30000, ReferenceIsobathsM: []float64{100, 200, 500, 1000, 1500}},
	}
}

// ValidateExpertFragments проверяет, что экспертный набор содержит все
// требуемые типы особенностей в обеих географически изолированных частях.
func ValidateExpertFragments(fragments []ExpertFragment) error {
	if len(fragments) == 0 {
		return fmt.Errorf("набор AI-01 не содержит контрольных фрагментов")
	}
	features := map[string]struct{}{"коса": {}, "залив": {}, "остров": {}, "изобаты": {}}
	seenIDs := make(map[string]struct{}, len(fragments))
	regionSplits := make(map[string]ExpertSplit, len(fragments))
	seenFeatures := map[ExpertSplit]map[string]struct{}{
		ExpertSplitTraining:   {},
		ExpertSplitEvaluation: {},
	}
	for _, fragment := range fragments {
		if !comparisonLevelIDPattern.MatchString(fragment.ID) {
			return fmt.Errorf("идентификатор фрагмента AI-01 %q должен состоять из строчных латинских букв, цифр и дефисов", fragment.ID)
		}
		if _, exists := seenIDs[fragment.ID]; exists {
			return fmt.Errorf("идентификатор фрагмента AI-01 %q повторяется", fragment.ID)
		}
		seenIDs[fragment.ID] = struct{}{}
		if strings.TrimSpace(fragment.Name) == "" || strings.TrimSpace(fragment.RegionID) == "" {
			return fmt.Errorf("фрагмент AI-01 %q должен иметь название и географический район", fragment.ID)
		}
		if _, exists := features[fragment.Feature]; !exists {
			return fmt.Errorf("фрагмент AI-01 %q имеет неподдерживаемый тип особенности %q", fragment.ID, fragment.Feature)
		}
		if fragment.Split != ExpertSplitTraining && fragment.Split != ExpertSplitEvaluation {
			return fmt.Errorf("фрагмент AI-01 %q имеет неподдерживаемую часть %q", fragment.ID, fragment.Split)
		}
		if split, exists := regionSplits[fragment.RegionID]; exists && split != fragment.Split {
			return fmt.Errorf("географический район %q одновременно попал в обучение и проверку", fragment.RegionID)
		}
		regionSplits[fragment.RegionID] = fragment.Split
		if !finiteExpertCoordinate(fragment.LatitudeDeg, -90, 90) || !finiteExpertCoordinate(fragment.LongitudeDeg, -180, 180) {
			return fmt.Errorf("фрагмент AI-01 %q имеет некорректный центр WGS 84", fragment.ID)
		}
		if !finiteExpertCoordinate(fragment.WidthM, 1000, 100000) || !finiteExpertCoordinate(fragment.HeightM, 1000, 100000) {
			return fmt.Errorf("размер окна фрагмента AI-01 %q должен быть от 1 до 100 км", fragment.ID)
		}
		if err := validateExpertIsobaths(fragment.ID, fragment.ReferenceIsobathsM); err != nil {
			return err
		}
		seenFeatures[fragment.Split][fragment.Feature] = struct{}{}
	}
	for _, split := range []ExpertSplit{ExpertSplitTraining, ExpertSplitEvaluation} {
		for feature := range features {
			if _, exists := seenFeatures[split][feature]; !exists {
				return fmt.Errorf("часть %q AI-01 не содержит особенности %q", split, feature)
			}
		}
	}
	return nil
}

// WriteExpertSetManifest сохраняет закрытый ключ карточек AI-01 в JSON.
func WriteExpertSetManifest(path string, manifest ExpertSetManifest) error {
	if manifest.SchemaVersion != ExpertSetSchemaVersion {
		return fmt.Errorf("набор AI-01 имеет неподдерживаемую схему %q", manifest.SchemaVersion)
	}
	if err := ValidateExpertFragments(manifest.Fragments); err != nil {
		return err
	}
	if len(manifest.Cards) == 0 {
		return fmt.Errorf("набор AI-01 не содержит карточек")
	}
	if err := validateExpertCards(manifest.Cards); err != nil {
		return err
	}
	manifest.Fragments = append([]ExpertFragment(nil), manifest.Fragments...)
	manifest.Cards = append([]ExpertSetCard(nil), manifest.Cards...)
	if manifest.ExcludedRuns == nil {
		manifest.ExcludedRuns = []ExpertSetExcludedRun{}
	} else {
		manifest.ExcludedRuns = append([]ExpertSetExcludedRun(nil), manifest.ExcludedRuns...)
	}
	sort.Slice(manifest.Cards, func(left, right int) bool {
		return manifest.Cards[left].PresentationID < manifest.Cards[right].PresentationID
	})
	data, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return fmt.Errorf("кодирование ключа карточек AI-01: %w", err)
	}
	return writeExpertSetFile(path, append(data, '\n'))
}

// ReadExpertSetManifest читает и проверяет закрытый ключ AI-01 перед
// последующей оценкой AI-02.
func ReadExpertSetManifest(path string) (ExpertSetManifest, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return ExpertSetManifest{}, fmt.Errorf("чтение закрытого ключа AI-01 %q: %w", path, err)
	}
	var manifest ExpertSetManifest
	if err := json.Unmarshal(data, &manifest); err != nil {
		return ExpertSetManifest{}, fmt.Errorf("разбор закрытого ключа AI-01 %q: %w", path, err)
	}
	if manifest.SchemaVersion != ExpertSetSchemaVersion {
		return ExpertSetManifest{}, fmt.Errorf("закрытый ключ AI-01 имеет неподдерживаемую схему %q", manifest.SchemaVersion)
	}
	if err := ValidateExpertFragments(manifest.Fragments); err != nil {
		return ExpertSetManifest{}, err
	}
	if len(manifest.Cards) == 0 {
		return ExpertSetManifest{}, fmt.Errorf("закрытый ключ AI-01 не содержит карточек")
	}
	if err := validateExpertCards(manifest.Cards); err != nil {
		return ExpertSetManifest{}, err
	}
	return manifest, nil
}

// WriteExpertAssignmentsTSV сохраняет открытый лист оценивания. В нём
// намеренно отсутствуют генератор, уровень и географическая часть набора.
func WriteExpertAssignmentsTSV(path string, cards []ExpertSetCard) error {
	if len(cards) == 0 {
		return fmt.Errorf("для листа оценивания AI-01 нужны карточки")
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("создание каталога листа оценивания AI-01: %w", err)
	}
	file, err := os.Create(path)
	if err != nil {
		return fmt.Errorf("создание листа оценивания AI-01 %q: %w", path, err)
	}
	defer func() { _ = file.Close() }()
	writer := bufio.NewWriter(file)
	if _, err := fmt.Fprintln(writer, "Идентификатор карточки\tОсобенность\tЭксперт\tБерег 1–5\tИзобаты 1–5\tСетка 1–5\tАртефакты 1–5\tКомментарий"); err != nil {
		return err
	}
	if _, err := fmt.Fprintln(writer, "-----------------------\t-----------\t-------\t----------\t------------\t----------\t---------------\t-----------"); err != nil {
		return err
	}
	ordered := append([]ExpertSetCard(nil), cards...)
	sort.Slice(ordered, func(left, right int) bool { return ordered[left].PresentationID < ordered[right].PresentationID })
	for _, card := range ordered {
		if _, err := fmt.Fprintf(writer, "%s\t%s\t\t\t\t\t\t\n", card.PresentationID, card.Feature); err != nil {
			return err
		}
	}
	if err := writer.Flush(); err != nil {
		return fmt.Errorf("запись листа оценивания AI-01: %w", err)
	}
	return nil
}

func finiteExpertCoordinate(value, lower, upper float64) bool {
	return !math.IsNaN(value) && !math.IsInf(value, 0) && value >= lower && value <= upper
}

func validateExpertIsobaths(id string, values []float64) error {
	if len(values) == 0 {
		return fmt.Errorf("фрагмент AI-01 %q не содержит опорных изобат", id)
	}
	previous := 0.0
	for _, value := range values {
		if !finiteExpertCoordinate(value, 0.001, 12000) || value <= previous {
			return fmt.Errorf("изобаты фрагмента AI-01 %q должны быть конечными, положительными и строго возрастающими", id)
		}
		previous = value
	}
	return nil
}

func validateExpertCards(cards []ExpertSetCard) error {
	seen := make(map[string]struct{}, len(cards))
	for _, card := range cards {
		if !strings.HasPrefix(card.PresentationID, "E-") || strings.TrimSpace(card.FragmentID) == "" || strings.TrimSpace(card.LevelID) == "" || strings.TrimSpace(card.Algorithm) == "" || strings.TrimSpace(card.SVGPath) == "" {
			return fmt.Errorf("карточка AI-01 должна иметь идентификатор, фрагмент, уровень, генератор и путь SVG")
		}
		if _, exists := seen[card.PresentationID]; exists {
			return fmt.Errorf("анонимный идентификатор карточки AI-01 %q повторяется", card.PresentationID)
		}
		seen[card.PresentationID] = struct{}{}
		if !expertSHA256(card.MeshSHA256) {
			return fmt.Errorf("карточка AI-01 %q не содержит SHA-256 кандидатного MSH", card.PresentationID)
		}
		if card.VisibleCells <= 0 || card.VisibleContours < 0 {
			return fmt.Errorf("карточка AI-01 %q не содержит проверяемой статистики видимых слоёв", card.PresentationID)
		}
	}
	return nil
}

func expertSHA256(value string) bool {
	if len(value) != 64 {
		return false
	}
	for _, symbol := range value {
		if !(symbol >= '0' && symbol <= '9') && !(symbol >= 'a' && symbol <= 'f') {
			return false
		}
	}
	return true
}

func writeExpertSetFile(path string, data []byte) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("создание каталога AI-01: %w", err)
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		return fmt.Errorf("запись файла AI-01 %q: %w", path, err)
	}
	return nil
}
