package cobra

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"text/tabwriter"
	"time"

	adaptivemodel "coastal-geometry/internal/domain/adaptive"
	"coastal-geometry/internal/domain/coastline"
	"coastal-geometry/internal/domain/geometry"
	mesh2d "coastal-geometry/internal/domain/mesh"
	"coastal-geometry/internal/domain/seabed"
	renderadaptive "coastal-geometry/internal/render/adaptive"

	"github.com/spf13/cobra"
)

var (
	seabedExpertSetComparison string
	seabedExpertSetOutput     string
	seabedExpertSetLevels     string
	seabedExpertSetSeed       string
)

var seabedExpertSetCmd = &cobra.Command{
	Use:   "expert-set",
	Short: "Подготовить экспертный набор контрольных фрагментов",
	Long: `Создаёт слепой набор одинаковых фрагментов для успешных генераторов
ADAPT-03. Каждая карточка показывает один и тот же прямоугольник LAEA,
исходный берег и изобаты фиксированной модели дна, но не раскрывает генератор
или уровень детализации. Набор включает косы, заливы, острова и изобаты, а
географические районы обучения и независимой проверки не пересекаются.

Команда не строит нейросеть и не меняет рейтинг ADAPT-03. Экспертная оценка
сохраняется как отдельный будущий источник данных AI-02.`,
	RunE: runSeabedExpertSet,
}

func init() {
	seabedCmd.AddCommand(seabedExpertSetCmd)
	seabedExpertSetCmd.Flags().StringVar(&seabedExpertSetComparison, "comparison", "output/seabed/adaptive/comparison/adaptive-generator-comparison.json", "JSON-отчёт ADAPT-03")
	seabedExpertSetCmd.Flags().StringVar(&seabedExpertSetOutput, "output", "output", "корневой каталог результатов")
	seabedExpertSetCmd.Flags().StringVar(&seabedExpertSetLevels, "levels", "", "идентификаторы уровней ADAPT-03 через запятую; пусто — все принятые уровни")
	seabedExpertSetCmd.Flags().StringVar(&seabedExpertSetSeed, "seed", "ai-01-black-sea-2026", "фиксированная строка анонимизации порядка карточек")
}

type expertSetOptions struct {
	ComparisonPath string
	OutputRoot     string
	Levels         string
	Seed           string
	Fragments      []adaptivemodel.ExpertFragment
}

type expertSetJob struct {
	fragment        adaptivemodel.ExpertFragment
	level           adaptivemodel.AdaptiveComparisonLevelReport
	run             adaptivemodel.GeneratorComparisonRun
	presentationID  string
	relativeSVG     string
	meshSHA256      string
	visibleCells    int
	visibleContours int
}

func runSeabedExpertSet(_ *cobra.Command, _ []string) error {
	result, err := executeSeabedExpertSet(expertSetOptions{
		ComparisonPath: seabedExpertSetComparison,
		OutputRoot:     seabedExpertSetOutput,
		Levels:         seabedExpertSetLevels,
		Seed:           seabedExpertSetSeed,
		Fragments:      adaptivemodel.DefaultExpertFragments(),
	})
	if err != nil {
		return err
	}
	if quiet {
		return nil
	}
	fmt.Printf("Экспертный набор AI-01: %d карточек, %d исключённых запусков\n", len(result.manifest.Cards), len(result.manifest.ExcludedRuns))
	writer := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(writer, "Артефакт\tПуть")
	fmt.Fprintln(writer, "--------\t----")
	fmt.Fprintf(writer, "Карточки SVG\t%s\n", filepath.Join(result.directory, "cards"))
	fmt.Fprintf(writer, "Лист оценивания\t%s\n", result.ratingTemplate)
	fmt.Fprintf(writer, "Инструкция участнику\t%s\n", result.instructions)
	fmt.Fprintf(writer, "Закрытый ключ\t%s\n", result.organizerKey)
	fmt.Fprintf(writer, "Журнал\t%s\n", result.log)
	return writer.Flush()
}

type expertSetResult struct {
	directory      string
	ratingTemplate string
	instructions   string
	organizerKey   string
	log            string
	manifest       adaptivemodel.ExpertSetManifest
}

func executeSeabedExpertSet(options expertSetOptions) (expertSetResult, error) {
	if strings.TrimSpace(options.ComparisonPath) == "" {
		return expertSetResult{}, fmt.Errorf("для AI-01 обязателен путь к JSON ADAPT-03")
	}
	if strings.TrimSpace(options.OutputRoot) == "" {
		return expertSetResult{}, fmt.Errorf("для AI-01 обязателен корневой каталог output")
	}
	if strings.TrimSpace(options.Seed) == "" {
		return expertSetResult{}, fmt.Errorf("строка анонимизации AI-01 не должна быть пустой")
	}
	fragments := options.Fragments
	if len(fragments) == 0 {
		fragments = adaptivemodel.DefaultExpertFragments()
	}
	if err := adaptivemodel.ValidateExpertFragments(fragments); err != nil {
		return expertSetResult{}, err
	}
	report, err := readExpertComparison(options.ComparisonPath)
	if err != nil {
		return expertSetResult{}, err
	}
	if !report.Accepted {
		return expertSetResult{}, fmt.Errorf("AI-01 принимает только отчёт ADAPT-03 со статусом accepted=true")
	}
	if err := verifyExpertComparisonInputs(report); err != nil {
		return expertSetResult{}, err
	}
	levels, err := selectExpertLevels(report.Levels, options.Levels)
	if err != nil {
		return expertSetResult{}, err
	}
	referenceDocument, err := seabed.ReadMSH2(report.Inputs.InputMSH)
	if err != nil {
		return expertSetResult{}, fmt.Errorf("чтение опорной модели дна AI-01: %w", err)
	}
	if referenceDocument.Metadata.ModelKind != seabed.MSHModelSeabed || referenceDocument.Metadata.SchemaVersion != seabed.SeabedMSHSchemaVersion || !referenceDocument.Model.Accepted {
		return expertSetResult{}, fmt.Errorf("входной MSH AI-01 не является принятой моделью дна %s", seabed.SeabedMSHSchemaVersion)
	}
	metadata, err := seabed.ReadExportMetadataJSON(report.Inputs.ExportMetadata)
	if err != nil {
		return expertSetResult{}, fmt.Errorf("чтение паспорта EXPORT-02 для AI-01: %w", err)
	}
	polygon, err := coastline.LoadPolygon(coastline.LoadOptions{LocalPath: report.Inputs.Coastline})
	if err != nil {
		return expertSetResult{}, fmt.Errorf("загрузка исходного берега AI-01: %w", err)
	}
	projection := mesh2d.EqualAreaProjection{ReferenceLat: metadata.ProjectionReferenceLatitudeDeg, ReferenceLon: metadata.ProjectionReferenceLongitudeDeg}
	sourceRings := expertSourceRings(polygon, projection)
	if len(sourceRings) == 0 {
		return expertSetResult{}, fmt.Errorf("исходный берег AI-01 не содержит колец")
	}

	jobs, excluded, err := prepareExpertSetJobs(levels, fragments, options.Seed)
	if err != nil {
		return expertSetResult{}, err
	}
	directory := filepath.Join(options.OutputRoot, "seabed", "expert-set")
	for levelIndex := range levels {
		for _, run := range levels[levelIndex].Runs {
			if !run.Success {
				continue
			}
			meshSHA, checksumErr := expertFileSHA256(run.Artifacts.MSH)
			if checksumErr != nil {
				return expertSetResult{}, checksumErr
			}
			candidate, readErr := mesh2d.ReadMSH2(run.Artifacts.MSH)
			if readErr != nil {
				return expertSetResult{}, fmt.Errorf("чтение MSH генератора %q уровня %q для AI-01: %w", run.Algorithm, levels[levelIndex].Level.ID, readErr)
			}
			for jobIndex := range jobs {
				job := &jobs[jobIndex]
				if job.level.Level.ID != levels[levelIndex].Level.ID || job.run.Algorithm != run.Algorithm {
					continue
				}
				job.meshSHA256 = meshSHA
				center := projection.Project(geometry.LatLon{Lat: job.fragment.LatitudeDeg, Lon: job.fragment.LongitudeDeg})
				card, renderErr := renderadaptive.WriteExpertFragmentSVG(filepath.Join(directory, job.relativeSVG), referenceDocument.Model, candidate, renderadaptive.ExpertFragmentSVGConfig{
					PresentationID: job.presentationID,
					FragmentID:     job.fragment.ID,
					Feature:        job.fragment.Feature,
					CenterX:        center.X,
					CenterY:        center.Y,
					WidthM:         job.fragment.WidthM,
					HeightM:        job.fragment.HeightM,
					IsobathsM:      job.fragment.ReferenceIsobathsM,
					SourceRings:    sourceRings,
				})
				if renderErr != nil {
					return expertSetResult{}, fmt.Errorf("создание карточки %s AI-01: %w", job.presentationID, renderErr)
				}
				job.visibleCells, job.visibleContours = card.VisibleCellCount, card.VisibleContourCount
			}
			runtime.GC()
		}
	}

	comparisonSHA, err := expertFileSHA256(options.ComparisonPath)
	if err != nil {
		return expertSetResult{}, err
	}
	manifest := adaptivemodel.ExpertSetManifest{
		SchemaVersion:          adaptivemodel.ExpertSetSchemaVersion,
		GeneratedAt:            time.Now().UTC().Format(time.RFC3339),
		ComparisonReport:       options.ComparisonPath,
		ComparisonReportSHA256: comparisonSHA,
		Seed:                   options.Seed,
		Fragments:              fragments,
		Cards:                  expertSetCards(jobs),
		ExcludedRuns:           excluded,
		ParticipantInstruction: "Оцените каждую карточку по форме берега, читаемости изобат, достаточности сетки и визуальным артефактам. Название генератора и уровень скрыты.",
		Limitation:             "Карточки дополняют, но не заменяют детерминированный рейтинг ADAPT-03; общая граница входных сеток уже проверена отдельно.",
	}
	organizerKey := filepath.Join(directory, "organizer-key.json")
	if err := adaptivemodel.WriteExpertSetManifest(organizerKey, manifest); err != nil {
		return expertSetResult{}, err
	}
	ratingTemplate := filepath.Join(directory, "expert-rating-template.tsv")
	if err := adaptivemodel.WriteExpertAssignmentsTSV(ratingTemplate, manifest.Cards); err != nil {
		return expertSetResult{}, err
	}
	if err := adaptivemodel.WriteExpertAssignmentsTSV(filepath.Join(directory, "training-assignments.tsv"), filterExpertCards(manifest.Cards, adaptivemodel.ExpertSplitTraining)); err != nil {
		return expertSetResult{}, err
	}
	if err := adaptivemodel.WriteExpertAssignmentsTSV(filepath.Join(directory, "evaluation-assignments.tsv"), filterExpertCards(manifest.Cards, adaptivemodel.ExpertSplitEvaluation)); err != nil {
		return expertSetResult{}, err
	}
	instructions := filepath.Join(directory, "participant-instructions.md")
	if err := writeExpertParticipantInstructions(instructions, manifest); err != nil {
		return expertSetResult{}, err
	}
	logPath := filepath.Join(directory, "expert-set.log")
	if err := writeExpertSetLog(logPath, manifest); err != nil {
		return expertSetResult{}, err
	}
	return expertSetResult{directory: directory, ratingTemplate: ratingTemplate, instructions: instructions, organizerKey: organizerKey, log: logPath, manifest: manifest}, nil
}

func readExpertComparison(path string) (adaptivemodel.AdaptiveComparisonReport, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return adaptivemodel.AdaptiveComparisonReport{}, fmt.Errorf("чтение JSON ADAPT-03 %q: %w", path, err)
	}
	var report adaptivemodel.AdaptiveComparisonReport
	if err := json.Unmarshal(data, &report); err != nil {
		return adaptivemodel.AdaptiveComparisonReport{}, fmt.Errorf("разбор JSON ADAPT-03 %q: %w", path, err)
	}
	if report.SchemaVersion != adaptivemodel.ComparisonSchemaVersion {
		return adaptivemodel.AdaptiveComparisonReport{}, fmt.Errorf("AI-01 поддерживает только отчёт %s, получен %q", adaptivemodel.ComparisonSchemaVersion, report.SchemaVersion)
	}
	if strings.TrimSpace(report.Inputs.InputMSH) == "" || strings.TrimSpace(report.Inputs.ExportMetadata) == "" || strings.TrimSpace(report.Inputs.Coastline) == "" {
		return adaptivemodel.AdaptiveComparisonReport{}, fmt.Errorf("JSON ADAPT-03 не содержит пути к модели дна, паспорту EXPORT-02 или берегу")
	}
	return report, nil
}

func verifyExpertComparisonInputs(report adaptivemodel.AdaptiveComparisonReport) error {
	inputs := []struct {
		path     string
		expected string
		name     string
	}{
		{path: report.Inputs.InputMSH, expected: report.Inputs.InputMSHSHA256, name: "модель дна"},
		{path: report.Inputs.ExportMetadata, expected: report.Inputs.ExportMetadataSHA256, name: "паспорт EXPORT-02"},
		{path: report.Inputs.Coastline, expected: report.Inputs.CoastlineSHA256, name: "исходный берег"},
	}
	for _, input := range inputs {
		if !adaptivemodelSHA256(input.expected) {
			return fmt.Errorf("отчёт ADAPT-03 не содержит корректный SHA-256 для %s", input.name)
		}
		actual, err := expertFileSHA256(input.path)
		if err != nil {
			return err
		}
		if actual != input.expected {
			return fmt.Errorf("SHA-256 %s не совпадает с отчётом ADAPT-03; повторите сравнение или верните исходный файл", input.name)
		}
	}
	return nil
}

func selectExpertLevels(all []adaptivemodel.AdaptiveComparisonLevelReport, raw string) ([]adaptivemodel.AdaptiveComparisonLevelReport, error) {
	required := make(map[string]struct{})
	for _, value := range strings.Split(raw, ",") {
		value = strings.TrimSpace(value)
		if value != "" {
			required[value] = struct{}{}
		}
	}
	selected := make([]adaptivemodel.AdaptiveComparisonLevelReport, 0, len(all))
	for _, level := range all {
		if len(required) > 0 {
			if _, exists := required[level.Level.ID]; !exists {
				continue
			}
			delete(required, level.Level.ID)
		}
		if !level.CommonBoundaryConfirmed {
			return nil, fmt.Errorf("уровень ADAPT-03 %q не подтвердил общую границу и не пригоден для слепой оценки", level.Level.ID)
		}
		successful := 0
		for _, run := range level.Runs {
			if run.Success {
				successful++
			}
		}
		if successful < 2 {
			return nil, fmt.Errorf("уровень ADAPT-03 %q имеет менее двух успешных генераторов", level.Level.ID)
		}
		selected = append(selected, level)
	}
	if len(required) > 0 {
		ids := make([]string, 0, len(required))
		for id := range required {
			ids = append(ids, id)
		}
		sort.Strings(ids)
		return nil, fmt.Errorf("в отчёте ADAPT-03 отсутствуют запрошенные уровни: %s", strings.Join(ids, ", "))
	}
	if len(selected) == 0 {
		return nil, fmt.Errorf("AI-01 не получил пригодных уровней ADAPT-03")
	}
	return selected, nil
}

func prepareExpertSetJobs(levels []adaptivemodel.AdaptiveComparisonLevelReport, fragments []adaptivemodel.ExpertFragment, seed string) ([]expertSetJob, []adaptivemodel.ExpertSetExcludedRun, error) {
	jobs := make([]expertSetJob, 0)
	excluded := make([]adaptivemodel.ExpertSetExcludedRun, 0)
	for _, level := range levels {
		for _, run := range level.Runs {
			if !run.Success {
				reason := strings.TrimSpace(run.Error)
				if reason == "" {
					reason = "генератор не создал принятую MSH"
				}
				excluded = append(excluded, adaptivemodel.ExpertSetExcludedRun{LevelID: level.Level.ID, Algorithm: string(run.Algorithm), Reason: reason})
				continue
			}
			if strings.TrimSpace(run.Artifacts.MSH) == "" {
				return nil, nil, fmt.Errorf("успешный генератор %q уровня %q не содержит путь к MSH", run.Algorithm, level.Level.ID)
			}
			for _, fragment := range fragments {
				jobs = append(jobs, expertSetJob{fragment: fragment, level: level, run: run})
			}
		}
	}
	sort.Slice(jobs, func(left, right int) bool {
		return expertPresentationHash(seed, jobs[left]) < expertPresentationHash(seed, jobs[right])
	})
	for index := range jobs {
		jobs[index].presentationID = fmt.Sprintf("E-%03d", index+1)
		jobs[index].relativeSVG = filepath.Join("cards", jobs[index].presentationID+".svg")
	}
	if len(jobs) == 0 {
		return nil, nil, fmt.Errorf("AI-01 не получил успешных MSH для карточек")
	}
	return jobs, excluded, nil
}

func expertPresentationHash(seed string, job expertSetJob) string {
	sum := sha256.Sum256([]byte(strings.Join([]string{seed, job.fragment.ID, job.level.Level.ID, string(job.run.Algorithm)}, "\x00")))
	return hex.EncodeToString(sum[:])
}

func expertSourceRings(polygon coastline.PolygonLoadResult, projection mesh2d.EqualAreaProjection) [][]mesh2d.Point {
	rings := make([][]mesh2d.Point, 0, len(polygon.Holes)+1)
	appendRing := func(source []geometry.LatLon) {
		if len(source) < 2 {
			return
		}
		ring := make([]mesh2d.Point, 0, len(source))
		for _, point := range source {
			ring = append(ring, projection.Project(point))
		}
		rings = append(rings, ring)
	}
	appendRing(polygon.Outer)
	for _, hole := range polygon.Holes {
		appendRing(hole)
	}
	return rings
}

func expertSetCards(jobs []expertSetJob) []adaptivemodel.ExpertSetCard {
	cards := make([]adaptivemodel.ExpertSetCard, 0, len(jobs))
	for _, job := range jobs {
		cards = append(cards, adaptivemodel.ExpertSetCard{
			PresentationID:  job.presentationID,
			FragmentID:      job.fragment.ID,
			Feature:         job.fragment.Feature,
			RegionID:        job.fragment.RegionID,
			Split:           job.fragment.Split,
			LevelID:         job.level.Level.ID,
			Algorithm:       string(job.run.Algorithm),
			MeshSHA256:      job.meshSHA256,
			SVGPath:         job.relativeSVG,
			VisibleCells:    job.visibleCells,
			VisibleContours: job.visibleContours,
		})
	}
	return cards
}

func filterExpertCards(cards []adaptivemodel.ExpertSetCard, split adaptivemodel.ExpertSplit) []adaptivemodel.ExpertSetCard {
	filtered := make([]adaptivemodel.ExpertSetCard, 0)
	for _, card := range cards {
		if card.Split == split {
			filtered = append(filtered, card)
		}
	}
	return filtered
}

func adaptivemodelSHA256(value string) bool {
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

func expertFileSHA256(path string) (string, error) {
	file, err := os.Open(path)
	if err != nil {
		return "", fmt.Errorf("открытие файла AI-01 %q: %w", path, err)
	}
	defer file.Close()
	hash := sha256.New()
	if _, err := io.Copy(hash, file); err != nil {
		return "", fmt.Errorf("расчёт SHA-256 AI-01 %q: %w", path, err)
	}
	return hex.EncodeToString(hash.Sum(nil)), nil
}

func writeExpertParticipantInstructions(path string, manifest adaptivemodel.ExpertSetManifest) error {
	data := fmt.Sprintf(`# Инструкция для экспертной оценки AI-01

Перед вами анонимные SVG-карточки фрагментов Чёрного моря. Каждая карточка
содержит исходный берег, изобаты фиксированной модели BATHY-03 и одну
кандидатную четырёхугольную сетку. Генератор и диапазон размера намеренно
скрыты.

Заполните «expert-rating-template.tsv» для каждой карточки по шкале 1–5:

1. **Берег** — достаточно ли сетка передаёт форму косы, залива или острова.
2. **Изобаты** — читаются ли опорные формы рельефа без ложных изломов.
3. **Сетка** — соответствует ли плотность и ориентация ячеек особенности.
4. **Артефакты** — 5 означает отсутствие заметных геометрических артефактов.

Не пытайтесь угадать генератор. Сначала разметьте карточки из
«training-assignments.tsv»; «evaluation-assignments.tsv» предназначен только
для окончательной независимой проверки будущей модели AI-02.

Набор содержит %d карточек. Закрытый файл «organizer-key.json» не передаётся
разметчикам: он раскрывает соответствие карточек генераторам и уровням.
`, len(manifest.Cards))
	return writeExpertText(path, []byte(data))
}

func writeExpertSetLog(path string, manifest adaptivemodel.ExpertSetManifest) error {
	data := fmt.Sprintf("AI-01 — экспертный набор контрольных фрагментов\nКарточек: %d\nИсключённых запусков: %d\nСхема: %s\nSHA-256 ADAPT-03: %s\nРезультат: успешно.\n", len(manifest.Cards), len(manifest.ExcludedRuns), manifest.SchemaVersion, manifest.ComparisonReportSHA256)
	return writeExpertText(path, []byte(data))
}

func writeExpertText(path string, data []byte) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("создание каталога AI-01: %w", err)
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		return fmt.Errorf("запись файла AI-01 %q: %w", path, err)
	}
	return nil
}
