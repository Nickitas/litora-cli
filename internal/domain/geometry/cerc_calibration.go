package geometry

import (
	"encoding/json"
	"fmt"
	"math"
	"os"
	"sort"
)

const hoursPerJulianYear = 365.25 * 24

// ShorelineRateObservation представляет наблюдаемую скорость смещения линии
// берега. Положительное значение означает размыв, отрицательное — аккумуляцию.
type ShorelineRateObservation struct {
	Lat                         float64 `json:"lat"`
	Lon                         float64 `json:"lon"`
	ShorelineChangeRateMPerYear float64 `json:"shoreline_change_rate_m_per_year"`
	UncertaintyMPerYear         float64 `json:"uncertainty_m_per_year,omitempty"`
}

// LoadShorelineRateObservations загружает независимые наблюдения из JSON.
// Допустим либо массив наблюдений, либо объект с полем observations.
func LoadShorelineRateObservations(path string) ([]ShorelineRateObservation, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("чтение наблюдений %q: %w", path, err)
	}
	var observations []ShorelineRateObservation
	if err := json.Unmarshal(data, &observations); err != nil {
		var wrapped struct {
			Observations []ShorelineRateObservation `json:"observations"`
		}
		if wrappedErr := json.Unmarshal(data, &wrapped); wrappedErr != nil {
			return nil, fmt.Errorf("разбор наблюдений %q: %w", path, err)
		}
		observations = wrapped.Observations
	}
	if err := validateCalibrationObservations(observations); err != nil {
		return nil, err
	}
	return observations, nil
}

// CERCCalibrationConfig задаёт калибровку коэффициента CERC по независимым
// наблюдениям. Волновой ряд должен покрывать хотя бы репрезентативный год,
// иначе краткий прогноз нельзя корректно сопоставлять с годовой скоростью.
type CERCCalibrationConfig struct {
	Model                LongshoreModelConfig
	CERCCoefficients     []float64
	MaxDistanceMeters    float64
	MinWaveDurationHours float64
}

// CERCComparisonPoint хранит сопоставление наблюдаемой и рассчитанной
// годовой скорости в одной точке.
type CERCComparisonPoint struct {
	Observation           ShorelineRateObservation `json:"observation"`
	CoastlinePointIndex   int                      `json:"coastline_point_index"`
	DistanceToCoastMeters float64                  `json:"distance_to_coast_meters"`
	ModeledRateMPerYear   float64                  `json:"modeled_rate_m_per_year"`
	ResidualMPerYear      float64                  `json:"residual_m_per_year"`
}

// CERCCalibrationMetrics содержит ошибки подгонки по независимым точкам.
type CERCCalibrationMetrics struct {
	RMSEMPerYear float64 `json:"rmse_m_per_year"`
	MAEMPerYear  float64 `json:"mae_m_per_year"`
	MBEMPerYear  float64 `json:"mbe_m_per_year"`
	RSquared     float64 `json:"r_squared"`
	Count        int     `json:"count"`
}

// CERCCalibrationResult описывает одну проверенную величину коэффициента CERC.
type CERCCalibrationResult struct {
	CERCCoefficient float64                `json:"cerc_coefficient"`
	Metrics         CERCCalibrationMetrics `json:"metrics"`
	Comparisons     []CERCComparisonPoint  `json:"comparisons"`
}

// CalibrateCERCCoefficient подбирает коэффициент CERC по скоростям изменения
// береговой линии. Калибровка не меняет других физических параметров модели:
// берма, глубина замыкания, пористость и граничные потоки остаются явными.
func CalibrateCERCCoefficient(points []LatLon, climate WaveClimate, observations []ShorelineRateObservation, config CERCCalibrationConfig) ([]CERCCalibrationResult, error) {
	if len(observations) == 0 {
		return nil, fmt.Errorf("для калибровки нужны наблюдения скорости береговой линии")
	}
	if len(config.CERCCoefficients) == 0 {
		return nil, fmt.Errorf("задайте хотя бы один коэффициент CERC")
	}
	if config.MaxDistanceMeters <= 0 {
		config.MaxDistanceMeters = 500
	}
	if config.MinWaveDurationHours <= 0 {
		config.MinWaveDurationHours = hoursPerJulianYear
	}
	waveDuration := totalWaveDurationHours(climate)
	if waveDuration < config.MinWaveDurationHours {
		return nil, fmt.Errorf("волновой ряд покрывает только %.1f ч; для годовой калибровки требуется минимум %.1f ч", waveDuration, config.MinWaveDurationHours)
	}
	if err := validateCalibrationObservations(observations); err != nil {
		return nil, err
	}

	result := make([]CERCCalibrationResult, 0, len(config.CERCCoefficients))
	for _, coefficient := range config.CERCCoefficients {
		if coefficient <= 0 || coefficient > 1 {
			return nil, fmt.Errorf("коэффициент CERC %.6f должен быть в диапазоне (0; 1]", coefficient)
		}
		modelConfig := config.Model
		modelConfig.CERCCoefficient = coefficient
		model, err := RunLongshoreCERC(points, climate, modelConfig)
		if err != nil {
			return nil, fmt.Errorf("расчёт для коэффициента %.6f: %w", coefficient, err)
		}
		comparisons, err := compareCERCRates(points, model.Steps, observations, waveDuration, config.MaxDistanceMeters)
		if err != nil {
			return nil, err
		}
		result = append(result, CERCCalibrationResult{CERCCoefficient: coefficient, Comparisons: comparisons, Metrics: calculateCERCCalibrationMetrics(comparisons)})
	}
	sort.Slice(result, func(i, j int) bool { return result[i].Metrics.RMSEMPerYear < result[j].Metrics.RMSEMPerYear })
	return result, nil
}

func totalWaveDurationHours(climate WaveClimate) float64 {
	total := 0.0
	for _, condition := range climate.Conditions {
		total += condition.DurationHours
	}
	return total
}

func validateCalibrationObservations(observations []ShorelineRateObservation) error {
	for i, observation := range observations {
		if observation.Lat < -90 || observation.Lat > 90 || observation.Lon < -180 || observation.Lon > 180 {
			return fmt.Errorf("наблюдение %d: координаты вне допустимых границ", i)
		}
		if math.IsNaN(observation.ShorelineChangeRateMPerYear) || math.IsInf(observation.ShorelineChangeRateMPerYear, 0) {
			return fmt.Errorf("наблюдение %d: скорость равна NaN/Inf", i)
		}
		if observation.UncertaintyMPerYear < 0 || math.IsNaN(observation.UncertaintyMPerYear) || math.IsInf(observation.UncertaintyMPerYear, 0) {
			return fmt.Errorf("наблюдение %d: неопределённость должна быть неотрицательной", i)
		}
	}
	return nil
}

func compareCERCRates(points []LatLon, steps []LongshoreStepResult, observations []ShorelineRateObservation, durationHours, maxDistanceMeters float64) ([]CERCComparisonPoint, error) {
	rates := make([]float64, len(points))
	for _, step := range steps {
		for index, cell := range step.Cells {
			if index < len(rates) {
				rates[index] += cell.ShorelineChangeM
			}
		}
	}
	for i := range rates {
		// В one-line схеме положительное смещение направлено в море и означает
		// аккумуляцию. Контракт наблюдений, напротив, задаёт положительным
		// размыв, поэтому знак переводится перед сравнением.
		rates[i] = -rates[i] / durationHours * hoursPerJulianYear
	}
	comparisons := make([]CERCComparisonPoint, 0, len(observations))
	for _, observation := range observations {
		index, distance := nearestCoastlinePoint(points, LatLon{Lat: observation.Lat, Lon: observation.Lon})
		if distance > maxDistanceMeters {
			return nil, fmt.Errorf("наблюдение %.6f, %.6f находится в %.0f м от сегмента, что превышает %.0f м", observation.Lat, observation.Lon, distance, maxDistanceMeters)
		}
		modeled := rates[index]
		comparisons = append(comparisons, CERCComparisonPoint{
			Observation:           observation,
			CoastlinePointIndex:   index,
			DistanceToCoastMeters: distance,
			ModeledRateMPerYear:   modeled,
			ResidualMPerYear:      modeled - observation.ShorelineChangeRateMPerYear,
		})
	}
	return comparisons, nil
}

func nearestCoastlinePoint(points []LatLon, target LatLon) (int, float64) {
	index := 0
	best := math.Inf(1)
	for i, point := range points {
		distance := Haversine(point, target) * 1000
		if distance < best {
			index, best = i, distance
		}
	}
	return index, best
}

func calculateCERCCalibrationMetrics(comparisons []CERCComparisonPoint) CERCCalibrationMetrics {
	metrics := CERCCalibrationMetrics{Count: len(comparisons)}
	if len(comparisons) == 0 {
		return metrics
	}
	meanObserved := 0.0
	for _, comparison := range comparisons {
		meanObserved += comparison.Observation.ShorelineChangeRateMPerYear
		metrics.MAEMPerYear += math.Abs(comparison.ResidualMPerYear)
		metrics.MBEMPerYear += comparison.ResidualMPerYear
		metrics.RMSEMPerYear += comparison.ResidualMPerYear * comparison.ResidualMPerYear
	}
	count := float64(len(comparisons))
	meanObserved /= count
	metrics.MAEMPerYear /= count
	metrics.MBEMPerYear /= count
	metrics.RMSEMPerYear = math.Sqrt(metrics.RMSEMPerYear / count)
	variance := 0.0
	for _, comparison := range comparisons {
		delta := comparison.Observation.ShorelineChangeRateMPerYear - meanObserved
		variance += delta * delta
	}
	if variance > 0 {
		metrics.RSquared = 1 - (metrics.RMSEMPerYear*metrics.RMSEMPerYear*count)/variance
	}
	return metrics
}
