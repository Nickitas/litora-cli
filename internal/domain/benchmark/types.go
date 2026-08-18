package benchmark

import "coastal-geometry/internal/domain/geometry"

// BenchmarkSite представляет откалиброванный сегмент побережья с известными данными об эрозии
type BenchmarkSite struct {
	// Идентификация
	ID          string `json:"id"`
	Name        string `json:"name"`
	Region      string `json:"region"`
	Country     string `json:"country"`
	Description string `json:"description"`

	// Геометрия
	Coastline []geometry.LatLon `json:"coastline"`
	Bounds    Bounds            `json:"bounds"`

	// Наблюдения эрозии
	ObservedErosion []ErosionObservation `json:"observed_erosion"`

	// Характеристики побережья
	CoastType         CoastType `json:"coast_type"`
	DominantLithology string    `json:"dominant_lithology"`
	MeanWaveHeight    float64   `json:"mean_wave_height_m"`
	MeanWavePeriod    float64   `json:"mean_wave_period_s"`
	MeanWaveDirection float64   `json:"mean_wave_direction_deg"`

	// Метаданные
	DataSource       string   `json:"data_source"`
	References       []string `json:"references"`
	DataQuality      Quality  `json:"data_quality"`
	ObservationYears Range    `json:"observation_years"`
}

// Bounds представляет географические границы эталонного участка
type Bounds struct {
	MinLat float64 `json:"min_lat"`
	MaxLat float64 `json:"max_lat"`
	MinLon float64 `json:"min_lon"`
	MaxLon float64 `json:"max_lon"`
}

// ErosionObservation представляет измеренную эрозию в определённом месте и времени
type ErosionObservation struct {
	// Местоположение
	LatLon geometry.LatLon `json:"lat_lon"`

	// Измерение
	ShorelineChangeRate float64 `json:"shoreline_change_rate_m_per_year"` // положительное = эрозия, отрицательное = аккумуляция
	Uncertainty         float64 `json:"uncertainty_m_per_year"`

	// Время
	StartDate string `json:"start_date"` // ГГГГ-ММ-ДД
	EndDate   string `json:"end_date"`   // ГГГГ-ММ-ДД

	// Метод
	MeasurementMethod string `json:"measurement_method"` // "aerial_photography", "satellite", "field_survey"
	DataResolution    string `json:"data_resolution"`    // например "10m"
}

// CoastType описывает доминирующие прибрежные характеристики
type CoastType string

const (
	CoastTypeSandy      CoastType = "sandy"      // песчаные пляжи
	CoastTypeCliff      CoastType = "cliff"      // разываемые скалы
	CoastTypeRocky      CoastType = "rocky"      // твёрдые породы
	CoastTypeMuddy      CoastType = "muddy"      // илистые отмели
	CoastTypeMixed      CoastType = "mixed"      // смешанные характеристики
	CoastTypeArtificial CoastType = "artificial" // инженерные сооружения/молы
)

// Quality представляет рейтинг качества данных
type Quality string

const (
	QualityHigh   Quality = "high"   // прямые измерения, <10% неопределённости
	QualityMedium Quality = "medium" // косвенные измерения, 10-30% неопределённости
	QualityLow    Quality = "low"    // оценочные, >30% неопределённости
)

// Range представляет числовой диапазон
type Range struct {
	Min float64 `json:"min"`
	Max float64 `json:"max"`
}

// CalibrationResult хранит откалиброванные параметры для участка
type CalibrationResult struct {
	SiteID string `json:"site_id"`

	// Калиброванные параметры
	ErosionStrength       float64 `json:"erosion_strength"`
	ErosionStrengthUncert float64 `json:"erosion_strength_uncertainty"`
	WaveHeightMultiplier  float64 `json:"wave_height_multiplier"`
	LithologyFactor       float64 `json:"lithology_factor"`

	// Метрики валидации
	ValidationMetrics ValidationMetrics `json:"validation_metrics"`

	// Метаданные калибровки
	CalibrationDate string `json:"calibration_date"`
	Calibrator      string `json:"calibrator"`
	Notes           string `json:"notes"`
}

// ValidationMetrics сравнивает модельные и наблюдаемые значения эрозии
type ValidationMetrics struct {
	RMSE             float64 `json:"rmse_m_per_year"`              // среднеквадратичная ошибка
	WeightedRMSE     float64 `json:"weighted_rmse_m_per_year"`     // RMSE с весами 1/σ² наблюдений
	MAE              float64 `json:"mae_m_per_year"`               // средняя абсолютная ошибка
	MBE              float64 `json:"mbe_m_per_year"`               // средняя смещённая ошибка
	RSquared         float64 `json:"r_squared"`                    // коэффициент детерминации
	N                int     `json:"n_observations"`               // количество наблюдений
	PValue           float64 `json:"p_value_raw"`                  // сырое p-значение точного t-критерия Пирсона
	AdjustedPValue   float64 `json:"p_value_bonferroni"`           // p-значение с поправкой Бонферрони на поиск параметров
	Significant      bool    `json:"significant_after_correction"` // значима ли корреляция после поправки
	InferenceAllowed bool    `json:"inference_allowed"`            // допустима ли проверка значимости при данном размере выборки
}

// CalibrationParameters представляет пространство параметров для калибровки
type CalibrationParameters struct {
	// Диапазоны параметров для поиска
	ErosionStrengthRange      Range `json:"erosion_strength_range"`
	WaveHeightMultiplierRange Range `json:"wave_height_multiplier_range"`
	LithologyFactorRange      Range `json:"lithology_factor_range"`

	// Разрешение поиска
	Steps int `json:"steps"` // количество шагов в каждом измерении

	// Оптимизация
	OptimizationTarget string `json:"optimization_target"` // "rmse", "mae", "rsquared"
	MaxIterations      int    `json:"max_iterations"`
}
