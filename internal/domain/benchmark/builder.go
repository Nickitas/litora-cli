package benchmark

import (
	"fmt"
	"strings"

	"coastal-geometry/internal/domain/geometry"
)

// SiteSpec определяет параметры для создания нового сайта калибровки
type SiteSpec struct {
	Name              string    // название сайта
	ID                string    // идентификатор
	Region            string    // регион
	Country           string    // страна
	Description       string    // описание
	Bounds            Bounds    // географические границы
	CoastType         CoastType // тип берега
	DominantLithology string    // доминирующая литология
	MeanWaveHeight    float64   // средняя высота волны (м)
	MeanWavePeriod    float64   // средний период волны (с)
	MeanWaveDirection float64   // среднее направление волны (град)
	DataQuality       Quality   // качество данных
	ObservationYears  Range     // годы наблюдений
	DataSource        string    // источник данных
	References        []string  // ссылки на источники
}

// Validate проверяет корректность спецификации сайта и возвращает ошибку при некорректных данных
func (s *SiteSpec) Validate() error {
	if strings.TrimSpace(s.Name) == "" {
		return fmt.Errorf("необходимо указать название")
	}
	if strings.TrimSpace(s.ID) == "" {
		// Автоматическая генерация ID из названия
		s.ID = slugifyID(s.Name)
	}
	if s.Bounds.MinLat >= s.Bounds.MaxLat {
		return fmt.Errorf("границы: минимальная широта должна быть меньше максимальной")
	}
	if s.Bounds.MinLon >= s.Bounds.MaxLon {
		return fmt.Errorf("границы: минимальная долгота должна быть меньше максимальной")
	}
	if s.Bounds.MinLat < -90 || s.Bounds.MaxLat > 90 {
		return fmt.Errorf("границы: широта вне диапазона [-90, 90]")
	}
	if s.Bounds.MinLon < -180 || s.Bounds.MaxLon > 180 {
		return fmt.Errorf("границы: долгота вне диапазона [-180, 180]")
	}
	if s.ObservationYears.Min == 0 && s.ObservationYears.Max == 0 {
		// Значение по умолчанию - последнее десятилетие
		s.ObservationYears = Range{Min: 2000, Max: 2024}
	}
	if s.ObservationYears.Min > s.ObservationYears.Max {
		return fmt.Errorf("годы_наблюдений: минимум должен быть меньше или равен максимуму")
	}
	if s.CoastType == "" {
		s.CoastType = CoastTypeMixed
	}
	if s.DataQuality == "" {
		s.DataQuality = QualityMedium
	}
	return nil
}

// Build создаёт BenchmarkSite из спецификации, извлекая береговую линию из
// предоставленной полной береговой линии
func (s *SiteSpec) Build(fullCoastline []geometry.LatLon) BenchmarkSite {
	site := BenchmarkSite{
		ID:                s.ID,
		Name:              s.Name,
		Region:            s.Region,
		Country:           s.Country,
		Description:       s.Description,
		Bounds:            s.Bounds,
		CoastType:         s.CoastType,
		DominantLithology: s.DominantLithology,
		MeanWaveHeight:    s.MeanWaveHeight,
		MeanWavePeriod:    s.MeanWavePeriod,
		MeanWaveDirection: s.MeanWaveDirection,
		DataSource:        s.DataSource,
		References:        s.References,
		DataQuality:       s.DataQuality,
		ObservationYears:  s.ObservationYears,
		ObservedErosion:   []ErosionObservation{},
	}

	if len(fullCoastline) > 0 {
		site.Coastline = ExtractCoastline(fullCoastline, s.Bounds.ToGeoBounds())
	}

	return site
}

// slugifyID преобразует название в валидный ID
// Пример: "San Francisco Bay" -> "san-francisco-bay"
func slugifyID(name string) string {
	var b strings.Builder
	prevDash := true // allow trimming leading dashes
	for _, r := range strings.TrimSpace(strings.ToLower(name)) {
		switch {
		case r >= 'a' && r <= 'z':
			b.WriteRune(r)
			prevDash = false
		case r >= '0' && r <= '9':
			b.WriteRune(r)
			prevDash = false
		case !prevDash:
			b.WriteByte('-')
			prevDash = true
		}
	}
	return strings.Trim(b.String(), "-")
}

// ParseBounds разбирает строку "мин_шир,макс_шир,мин_долг,макс_долг" в структуру Bounds
func ParseBounds(s string) (Bounds, error) {
	parts := strings.Split(s, ",")
	if len(parts) != 4 {
		return Bounds{}, fmt.Errorf("границы должны содержать 4 значения через запятую, получено %d", len(parts))
	}
	var values [4]float64
	for i, p := range parts {
		p = strings.TrimSpace(p)
		_, err := fmt.Sscanf(p, "%f", &values[i])
		if err != nil {
			return Bounds{}, fmt.Errorf("некорректное значение границы %q: %w", p, err)
		}
	}
	return Bounds{
		MinLat: values[0],
		MaxLat: values[1],
		MinLon: values[2],
		MaxLon: values[3],
	}, nil
}

// PresetCoastType возвращает CoastType из строкового представления
func PresetCoastType(s string) (CoastType, error) {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "sandy":
		return CoastTypeSandy, nil
	case "cliff":
		return CoastTypeCliff, nil
	case "rocky":
		return CoastTypeRocky, nil
	case "muddy":
		return CoastTypeMuddy, nil
	case "mixed":
		return CoastTypeMixed, nil
	case "artificial":
		return CoastTypeArtificial, nil
	case "":
		return CoastTypeMixed, nil
	default:
		return "", fmt.Errorf("неизвестный тип берега %q (допустимые: sandy, cliff, rocky, muddy, mixed, artificial)", s)
	}
}

// PresetQuality возвращает Quality из строкового представления
func PresetQuality(s string) (Quality, error) {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "high":
		return QualityHigh, nil
	case "medium":
		return QualityMedium, nil
	case "low":
		return QualityLow, nil
	case "":
		return QualityMedium, nil
	default:
		return "", fmt.Errorf("неизвестное качество %q (допустимые: high, medium, low)", s)
	}
}
