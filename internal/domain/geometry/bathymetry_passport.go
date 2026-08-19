package geometry

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// BathymetryStatusVerifiedDerived обозначает производный набор с заполненным
// паспортом, проверяемым исходным файлом и описанной процедурой обработки.
const BathymetryStatusVerifiedDerived = "verified_derived"

// BathymetryPassport описывает происхождение и обработку файла батиметрии.
// Паспорт хранится рядом с данными в файле с суффиксом .metadata.json.
type BathymetryPassport struct {
	SchemaVersion                string            `json:"schema_version"`                      // версия контракта паспорта
	Title                        string            `json:"title"`                               // полное название производного набора
	Status                       string            `json:"status"`                              // уровень подтверждения происхождения
	DatasetFile                  string            `json:"dataset_file"`                        // имя связанного JSON с точками
	DatasetSHA256                string            `json:"dataset_sha256"`                      // SHA-256 связанного JSON
	CreatedAt                    string            `json:"created_at"`                          // время создания производного набора
	PointCount                   int               `json:"point_count"`                         // число точек в связанном JSON
	Bounds                       Bounds            `json:"bounds"`                              // географические границы выборки
	TargetResolutionDegrees      float64           `json:"target_resolution_degrees"`           // шаг производной сетки в градусах
	TargetResolutionArcSeconds   float64           `json:"target_resolution_arc_seconds"`       // шаг производной сетки в угловых секундах
	SourceProduct                string            `json:"source_product"`                      // продукт и версия источника
	SourceProductDOI             string            `json:"source_product_doi,omitempty"`        // DOI версии источника
	SourceURL                    string            `json:"source_url,omitempty"`                // точный URL загрузки исходника
	SourceDownloadedAt           *string           `json:"source_downloaded_at"`                // время загрузки исходника
	SourceNetCDF                 *string           `json:"source_netcdf"`                       // путь или имя сохранённого NetCDF
	SourceNetCDFSHA256           *string           `json:"source_netcdf_sha256"`                // SHA-256 исходного NetCDF
	SourceGridIntervalArcSeconds *float64          `json:"source_grid_interval_arc_seconds"`    // шаг исходной сетки
	HorizontalReference          string            `json:"horizontal_reference"`                // горизонтальная система отсчёта
	VerticalReference            string            `json:"vertical_reference"`                  // вертикальная система отсчёта
	VerticalReferenceCaveat      string            `json:"vertical_reference_caveat,omitempty"` // оговорка источника о вертикальной системе
	ResamplingMethod             string            `json:"resampling_method"`                   // алгоритм перехода на целевую сетку
	LandFilter                   string            `json:"land_filter,omitempty"`               // правило исключения суши
	ProcessingScript             string            `json:"processing_script"`                   // программа получения производного файла
	ProcessingSoftware           map[string]string `json:"processing_software,omitempty"`       // версии использованного ПО
	License                      string            `json:"license"`                             // лицензия и обязательные условия
	LicenseURL                   string            `json:"license_url,omitempty"`               // ссылка на полные условия
	Attribution                  string            `json:"attribution,omitempty"`               // рекомендуемая атрибуция
	Limitations                  []string          `json:"limitations,omitempty"`               // ограничения научного применения
}

// BathymetryPassportPath возвращает стандартный путь к паспорту файла данных.
func BathymetryPassportPath(dataPath string) string {
	ext := filepath.Ext(dataPath)
	if strings.EqualFold(ext, ".json") {
		return strings.TrimSuffix(dataPath, ext) + ".metadata.json"
	}
	return dataPath + ".metadata.json"
}

// LoadBathymetryPassportFromFile загружает и проверяет структуру паспорта.
func LoadBathymetryPassportFromFile(path string) (*BathymetryPassport, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("чтение паспорта батиметрии %q: %w", path, err)
	}

	var passport BathymetryPassport
	if err := json.Unmarshal(data, &passport); err != nil {
		return nil, fmt.Errorf("разбор паспорта батиметрии %q: %w", path, err)
	}
	if err := validateBathymetryPassport(&passport); err != nil {
		return nil, fmt.Errorf("паспорт батиметрии %q: %w", path, err)
	}
	return &passport, nil
}

// VerifyDataset проверяет контрольную сумму файла данных по паспорту.
func (p *BathymetryPassport) VerifyDataset(data []byte) error {
	if p == nil {
		return fmt.Errorf("паспорт батиметрии не задан")
	}
	actual := fmt.Sprintf("%x", sha256.Sum256(data))
	if !strings.EqualFold(actual, p.DatasetSHA256) {
		return fmt.Errorf("контрольная сумма набора не совпадает: в паспорте %s, фактически %s", p.DatasetSHA256, actual)
	}
	return nil
}

// ReproducibilityWarnings возвращает пробелы паспорта, которые мешают считать
// набор подтверждённым воспроизводимым производным продуктом.
func (p *BathymetryPassport) ReproducibilityWarnings() []string {
	if p == nil {
		return []string{"паспорт батиметрии отсутствует"}
	}

	warnings := make([]string, 0, 8)
	if p.Status != BathymetryStatusVerifiedDerived {
		warnings = append(warnings, fmt.Sprintf("набор батиметрии имеет статус %q и не подтверждён как воспроизводимый производный продукт", p.Status))
	}
	if p.SourceProduct == "" {
		warnings = append(warnings, "не указаны продукт и версия источника батиметрии")
	}
	if p.SourceDownloadedAt == nil || strings.TrimSpace(*p.SourceDownloadedAt) == "" {
		warnings = append(warnings, "не указана дата загрузки источника батиметрии")
	}
	if p.SourceNetCDF == nil || strings.TrimSpace(*p.SourceNetCDF) == "" {
		warnings = append(warnings, "не указан исходный NetCDF")
	}
	if p.SourceNetCDFSHA256 == nil || strings.TrimSpace(*p.SourceNetCDFSHA256) == "" {
		warnings = append(warnings, "не указан SHA-256 исходного NetCDF")
	}
	if p.VerticalReference == "" || strings.Contains(strings.ToLower(p.VerticalReference), "не подтверж") {
		warnings = append(warnings, "вертикальная система отсчёта не подтверждена")
	}
	if p.ResamplingMethod == "" {
		warnings = append(warnings, "не описан метод ресэмплинга")
	}
	if p.License == "" {
		warnings = append(warnings, "не указаны лицензия и условия использования")
	}
	return warnings
}

func validateBathymetryPassport(passport *BathymetryPassport) error {
	if passport.SchemaVersion == "" {
		return fmt.Errorf("не указана версия схемы")
	}
	if passport.Title == "" {
		return fmt.Errorf("не указано название набора")
	}
	if passport.Status == "" {
		return fmt.Errorf("не указан статус набора")
	}
	if passport.DatasetFile == "" {
		return fmt.Errorf("не указано имя файла данных")
	}
	checksum, err := hex.DecodeString(passport.DatasetSHA256)
	if err != nil || len(checksum) != sha256.Size {
		return fmt.Errorf("dataset_sha256 должен содержать SHA-256 в шестнадцатеричном виде")
	}
	if passport.PointCount <= 0 {
		return fmt.Errorf("число точек должно быть положительным")
	}
	if passport.TargetResolutionDegrees <= 0 {
		return fmt.Errorf("целевой шаг сетки должен быть положительным")
	}
	return nil
}
