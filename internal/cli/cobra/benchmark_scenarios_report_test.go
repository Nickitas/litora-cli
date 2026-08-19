package cobra

import (
	"bytes"
	"encoding/json"
	"math"
	"os"
	"path/filepath"
	"testing"

	"coastal-geometry/internal/cli"
	"coastal-geometry/internal/domain/benchmark"
	"coastal-geometry/internal/domain/geometry"
)

// TestParametricScenarioReportJSONContract защищает методологические
// метаданные и имена полей от возврата к климатической интерпретации.
func TestParametricScenarioReportJSONContract(t *testing.T) {
	results := scenarioReportTestResults()
	report, err := newParametricScenarioReport("test-site", results)
	if err != nil {
		t.Fatalf("формирование отчёта: %v", err)
	}
	data, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		t.Fatalf("кодирование отчёта: %v", err)
	}

	if report.SchemaVersion != parametricScenarioReportSchemaVersion {
		t.Errorf("версия схемы = %q, требуется %q", report.SchemaVersion, parametricScenarioReportSchemaVersion)
	}
	if report.ScenarioType != parametricScenarioReportType {
		t.Errorf("тип сценария = %q, требуется %q", report.ScenarioType, parametricScenarioReportType)
	}
	if report.SchemaRef != parametricScenarioReportSchemaRef {
		t.Errorf("ссылка на схему = %q, требуется %q", report.SchemaRef, parametricScenarioReportSchemaRef)
	}
	for _, result := range report.Results {
		if result.Config.ParameterBasis != benchmark.ScenarioParameterBasisManual {
			t.Errorf("parameter_basis сценария %q = %q, требуется manual", result.Config.Name, result.Config.ParameterBasis)
		}
		if result.Config.Source != nil {
			t.Errorf("source сценария %q = %q, требуется null", result.Config.Name, *result.Config.Source)
		}
	}

	requiredFragments := [][]byte{
		[]byte(`"schema_version": "2.0"`),
		[]byte(`"schema_ref": "docs/schemas/parametric-scenarios-v2.schema.json"`),
		[]byte(`"scenario_type": "parametric_impact_amplification"`),
		[]byte(`"parameter_basis": "manual"`),
		[]byte(`"source": null`),
		[]byte(`"sea_level_rise_proxy_m_per_year"`),
		[]byte(`"storm_weight"`),
	}
	for _, fragment := range requiredFragments {
		if !bytes.Contains(data, fragment) {
			t.Errorf("JSON не содержит обязательный фрагмент %s", fragment)
		}
	}

	forbiddenFragments := [][]byte{
		[]byte(`"sea_level_rise_m_per_year"`),
		[]byte(`"storm_probability_per_step"`),
		[]byte(`"rcp45_2050"`),
		[]byte(`"rcp85_2050"`),
		[]byte(`"rcp85_2100"`),
	}
	for _, fragment := range forbiddenFragments {
		if bytes.Contains(data, fragment) {
			t.Errorf("JSON содержит устаревший фрагмент %s", fragment)
		}
	}
}

// TestParametricScenarioJSONSchema проверяет, что машинно-читаемая схема
// существует и согласована с версией и типом формируемого отчёта.
func TestParametricScenarioJSONSchema(t *testing.T) {
	path := filepath.Join("..", "..", "..", parametricScenarioReportSchemaRef)
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("чтение JSON Schema %q: %v", path, err)
	}
	var schema struct {
		MetaSchema string `json:"$schema"`
		Properties struct {
			SchemaVersion struct {
				Const string `json:"const"`
			} `json:"schema_version"`
			SchemaRef struct {
				Const string `json:"const"`
			} `json:"schema_ref"`
			ScenarioType struct {
				Const string `json:"const"`
			} `json:"scenario_type"`
		} `json:"properties"`
	}
	if err := json.Unmarshal(data, &schema); err != nil {
		t.Fatalf("разбор JSON Schema: %v", err)
	}
	if schema.MetaSchema != "https://json-schema.org/draft/2020-12/schema" {
		t.Errorf("неподдерживаемая метасхема %q", schema.MetaSchema)
	}
	if schema.Properties.SchemaVersion.Const != parametricScenarioReportSchemaVersion {
		t.Errorf("версия в JSON Schema = %q, требуется %q", schema.Properties.SchemaVersion.Const, parametricScenarioReportSchemaVersion)
	}
	if schema.Properties.SchemaRef.Const != parametricScenarioReportSchemaRef {
		t.Errorf("schema_ref в JSON Schema = %q, требуется %q", schema.Properties.SchemaRef.Const, parametricScenarioReportSchemaRef)
	}
	if schema.Properties.ScenarioType.Const != parametricScenarioReportType {
		t.Errorf("scenario_type в JSON Schema = %q, требуется %q", schema.Properties.ScenarioType.Const, parametricScenarioReportType)
	}
}

// TestWriteParametricScenarioReport проверяет обязательное сохранение
// относительного отчёта в подкаталог output/benchmark.
func TestWriteParametricScenarioReport(t *testing.T) {
	baseDir := t.TempDir()
	manager := cli.NewOutputPathManager(baseDir)
	path, err := writeParametricScenarioReport(
		manager,
		"custom.json",
		"test-site",
		scenarioReportTestResults(),
	)
	if err != nil {
		t.Fatalf("сохранение отчёта: %v", err)
	}
	want := filepath.Join(baseDir, "benchmark", "custom.json")
	if path != want {
		t.Errorf("путь отчёта = %q, требуется %q", path, want)
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("отчёт не сохранён: %v", err)
	}

	defaultPath, err := writeParametricScenarioReport(manager, "", "test-site", scenarioReportTestResults())
	if err != nil {
		t.Fatalf("сохранение отчёта по умолчанию: %v", err)
	}
	wantDefault := filepath.Join(baseDir, "benchmark", defaultParametricScenarioReportName)
	if defaultPath != wantDefault {
		t.Errorf("путь отчёта по умолчанию = %q, требуется %q", defaultPath, wantDefault)
	}
}

// TestWriteParametricScenarioReportReturnsErrors проверяет, что отсутствие
// результатов и ошибка файловой системы не игнорируются.
func TestWriteParametricScenarioReportReturnsErrors(t *testing.T) {
	manager := cli.NewOutputPathManager(t.TempDir())
	if _, err := writeParametricScenarioReport(manager, "", "test-site", nil); err == nil {
		t.Fatal("ожидалась ошибка при отсутствии результатов")
	}
	invalidResults := scenarioReportTestResults()
	invalidResults[0].Config.ParameterBasis = ""
	if _, err := writeParametricScenarioReport(manager, "", "test-site", invalidResults); err == nil {
		t.Fatal("ожидалась ошибка при отсутствии parameter_basis=manual")
	}
	invalidEncoding := scenarioReportTestResults()
	invalidEncoding[0].MeanRetreatRate = math.NaN()
	if _, err := writeParametricScenarioReport(manager, "", "test-site", invalidEncoding); err == nil {
		t.Fatal("ожидалась ошибка сериализации NaN")
	}

	blockedBase := filepath.Join(t.TempDir(), "blocked")
	if err := os.WriteFile(blockedBase, []byte("не каталог"), 0o644); err != nil {
		t.Fatalf("подготовка блокирующего файла: %v", err)
	}
	if _, err := writeParametricScenarioReport(
		cli.NewOutputPathManager(blockedBase),
		"report.json",
		"test-site",
		scenarioReportTestResults(),
	); err == nil {
		t.Fatal("ожидалась ошибка записи отчёта")
	}
}

// TestBenchmarkScenariosCommandE2E выполняет настоящую Cobra-команду с
// временным benchmark-репозиторием и проверяет итоговый JSON в output/benchmark.
func TestBenchmarkScenariosCommandE2E(t *testing.T) {
	workspace := t.TempDir()
	previousBenchDir := benchDir
	previousSiteID := benchSiteID
	previousOutput := benchOutput
	previousBathymetry := benchBathymetry
	previousStrength := benchStrength
	previousWaveDirection := benchWaveDir
	previousDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("получение рабочего каталога: %v", err)
	}
	if err := os.Chdir(workspace); err != nil {
		t.Fatalf("переход во временный каталог: %v", err)
	}
	defer func() {
		_ = os.Chdir(previousDir)
		benchDir = previousBenchDir
		benchSiteID = previousSiteID
		benchOutput = previousOutput
		benchBathymetry = previousBathymetry
		benchStrength = previousStrength
		benchWaveDir = previousWaveDirection
		rootCmd.SetArgs(nil)
		for _, flagName := range []string{"dir", "site", "erosion-strength", "wave-direction", "output"} {
			if flag := benchScenariosCmd.Flags().Lookup(flagName); flag != nil {
				flag.Changed = false
			}
			if flag := benchmarkCmd.PersistentFlags().Lookup(flagName); flag != nil {
				flag.Changed = false
			}
		}
	}()

	repositoryDir := filepath.Join(workspace, "benchmarks")
	site := benchmark.BenchmarkSite{
		ID:                "e2e-site",
		Name:              "Проверочный участок",
		MeanWaveDirection: 90,
		Coastline: []geometry.LatLon{
			{Lat: 44.00, Lon: 34.00},
			{Lat: 44.01, Lon: 34.01},
			{Lat: 44.02, Lon: 34.015},
			{Lat: 44.03, Lon: 34.01},
			{Lat: 44.04, Lon: 34.00},
		},
	}
	if err := benchmark.NewRepository(repositoryDir).Save(site); err != nil {
		t.Fatalf("сохранение тестового участка: %v", err)
	}

	rootCmd.SetArgs([]string{
		"benchmark", "scenarios",
		"--dir", repositoryDir,
		"--site", site.ID,
		"--erosion-strength", "10",
		"--wave-direction", "90",
		"--output", "e2e-report.json",
	})
	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("выполнение benchmark scenarios: %v", err)
	}

	reportPath := filepath.Join(workspace, "output", "benchmark", "e2e-report.json")
	data, err := os.ReadFile(reportPath)
	if err != nil {
		t.Fatalf("чтение E2E-отчёта %q: %v", reportPath, err)
	}
	var report parametricScenarioReport
	if err := json.Unmarshal(data, &report); err != nil {
		t.Fatalf("разбор E2E-отчёта: %v", err)
	}
	if report.SchemaVersion != parametricScenarioReportSchemaVersion || report.SchemaRef != parametricScenarioReportSchemaRef {
		t.Errorf("неверные метаданные схемы: version=%q ref=%q", report.SchemaVersion, report.SchemaRef)
	}
	if len(report.Results) != len(benchmark.DefaultScenarios(10, 90)) {
		t.Errorf("число сценариев = %d, требуется %d", len(report.Results), len(benchmark.DefaultScenarios(10, 90)))
	}
}

func scenarioReportTestResults() []benchmark.ScenarioResult {
	configs := benchmark.DefaultScenarios(10, 90)
	results := make([]benchmark.ScenarioResult, len(configs))
	for i, config := range configs {
		results[i] = benchmark.ScenarioResult{Config: config, SegmentRetreats: []float64{}}
	}
	return results
}
