package geometry

import (
	"fmt"
	"math"
)

const (
	accelerationGravity   = 9.80665
	seaWaterDensity       = 1025.0
	quartzSedimentDensity = 2650.0
	secondsPerHour        = 3600.0
)

// LongshoreModelConfig задаёт параметры инженерной одномерной модели CERC.
// Сетка глубин обязательна: модель не подменяет преобразование волн
// эвристическим множителем глубины.
type LongshoreModelConfig struct {
	Bathymetry               *BathymetryGrid
	BathymetrySource         string  // источник или путь к использованной батиметрии
	WaterbodyID              string  // идентификатор выбранного водоёма из каталога Lito
	BreakingIndex            float64 // γ = H_b / h_b, обычно 0,78
	BermHeightMeters         float64 // высота активного профиля над уровнем моря
	ClosureDepthMeters       float64 // глубина замыкания активного профиля
	Porosity                 float64 // пористость наносов [0; 1)
	CERCCoefficient          float64 // безразмерный коэффициент CERC
	OffshoreSampleDistanceM  float64 // расстояние отбора глубины от береговой линии
	MaxBathymetryGapMeters   float64 // максимальное расстояние до реальной точки глубины
	MaxShorelineChangeMeters float64 // численная защита для одного состояния волн
}

// LongshoreCellResult хранит пространственно определённый баланс одной ячейки.
// Объёмы относятся к сегменту между соседними узлами и измеряются в м³.
type LongshoreCellResult struct {
	PointIndex            int     `json:"point_index"`
	BreakingWaveHeightM   float64 `json:"breaking_wave_height_m"`
	BreakingDepthM        float64 `json:"breaking_depth_m"`
	BreakingAngleDeg      float64 `json:"breaking_angle_deg"`
	BathymetryDistanceM   float64 `json:"bathymetry_distance_m"`
	LongshoreTransportM3S float64 `json:"longshore_transport_m3_s"`
	VolumeChangeM3        float64 `json:"volume_change_m3"`
	ShorelineChangeM      float64 `json:"shoreline_change_m"`
}

// LongshoreStepResult описывает результат одного реального состояния волн.
type LongshoreStepResult struct {
	Condition         WaveCondition         `json:"condition"`
	Cells             []LongshoreCellResult `json:"cells"`
	ErodedVolumeM3    float64               `json:"eroded_volume_m3"`
	DepositedVolumeM3 float64               `json:"deposited_volume_m3"`
	MassBalanceM3     float64               `json:"mass_balance_m3"`
}

// LongshoreModelResult содержит положения береговой линии и баланс для всего
// волнового ряда. Первый снимок — исходное положение береговой линии.
type LongshoreModelResult struct {
	Model            string                `json:"model"`
	WaterbodyID      string                `json:"waterbody_id,omitempty"`
	BathymetrySource string                `json:"bathymetry_source"`
	Climate          WaveClimate           `json:"climate"`
	Snapshots        [][]LatLon            `json:"snapshots"`
	Steps            []LongshoreStepResult `json:"steps"`
}

// RunLongshoreCERC запускает инженерную one-line модель: дисперсия и
// групповая скорость, рефракция по закону Снеллиуса, shoaling, ограничение
// разрушением и уравнение неразрывности вдольберегового транспорта CERC.
func RunLongshoreCERC(points []LatLon, climate WaveClimate, config LongshoreModelConfig) (LongshoreModelResult, error) {
	if len(points) < 3 {
		return LongshoreModelResult{}, fmt.Errorf("для вдольбереговой модели нужно минимум три точки")
	}
	if isClosedPolyline(points) {
		return LongshoreModelResult{}, fmt.Errorf("одномерная вдольбереговая модель принимает незамкнутый сегмент берега")
	}
	if err := climate.Validate(); err != nil {
		return LongshoreModelResult{}, err
	}
	config = normalizeLongshoreModelConfig(config)
	if err := validateLongshoreModelConfig(config); err != nil {
		return LongshoreModelResult{}, err
	}

	result := LongshoreModelResult{
		Model:            "CERC-one-line: dispersion, refraction, shoaling, breaking and sediment continuity",
		WaterbodyID:      config.WaterbodyID,
		BathymetrySource: config.BathymetrySource,
		Climate:          climate,
		Snapshots:        make([][]LatLon, 1, len(climate.Conditions)+1),
	}
	current := clonePoints(points)
	result.Snapshots[0] = current
	for _, condition := range climate.Conditions {
		next, step, err := longshoreStep(current, condition, config)
		if err != nil {
			return LongshoreModelResult{}, err
		}
		result.Steps = append(result.Steps, step)
		result.Snapshots = append(result.Snapshots, next)
		current = next
	}
	return result, nil
}

func normalizeLongshoreModelConfig(config LongshoreModelConfig) LongshoreModelConfig {
	if config.BreakingIndex == 0 {
		config.BreakingIndex = 0.78
	}
	if config.BermHeightMeters == 0 {
		config.BermHeightMeters = 2
	}
	if config.ClosureDepthMeters == 0 {
		config.ClosureDepthMeters = 8
	}
	if config.Porosity == 0 {
		config.Porosity = 0.4
	}
	if config.CERCCoefficient == 0 {
		config.CERCCoefficient = 0.39
	}
	if config.OffshoreSampleDistanceM == 0 {
		config.OffshoreSampleDistanceM = 300
	}
	if config.MaxShorelineChangeMeters == 0 {
		config.MaxShorelineChangeMeters = 25
	}
	if config.MaxBathymetryGapMeters == 0 {
		config.MaxBathymetryGapMeters = 1500
	}
	return config
}

func validateLongshoreModelConfig(config LongshoreModelConfig) error {
	if config.Bathymetry == nil {
		return fmt.Errorf("для модели CERC обязательна батиметрия")
	}
	if config.BathymetrySource == "" {
		return fmt.Errorf("для батиметрии укажите источник или путь")
	}
	if config.BreakingIndex < 0.55 || config.BreakingIndex > 1.2 {
		return fmt.Errorf("breaking index должен быть в диапазоне [0,55; 1,2]")
	}
	if config.BermHeightMeters <= 0 || config.ClosureDepthMeters <= 0 {
		return fmt.Errorf("высота бермы и глубина замыкания должны быть положительными")
	}
	if config.Porosity < 0 || config.Porosity >= 0.7 {
		return fmt.Errorf("пористость должна быть в диапазоне [0; 0,7)")
	}
	if config.CERCCoefficient <= 0 || config.CERCCoefficient > 1 {
		return fmt.Errorf("коэффициент CERC должен быть в диапазоне (0; 1]")
	}
	if config.MaxBathymetryGapMeters <= 0 {
		return fmt.Errorf("максимальный радиус поиска батиметрии должен быть положительным")
	}
	return nil
}

func longshoreStep(points []LatLon, condition WaveCondition, config LongshoreModelConfig) ([]LatLon, LongshoreStepResult, error) {
	projected, reference := projectToMetersWithReference(points)
	cells := make([]LongshoreCellResult, len(points))
	seaward := make([]pointXY, len(points))
	transport := make([]float64, len(points))
	fromDirection := directionFromNorthClockwise(condition.DirectionFromDeg)

	for i := range points {
		previous, next := waveNeighborIndexesOpen(i, len(points))
		tangent := normalizeXY(pointXY{X: projected[next].X - projected[previous].X, Y: projected[next].Y - projected[previous].Y})
		left := pointXY{X: -tangent.Y, Y: tangent.X}
		right := pointXY{X: tangent.Y, Y: -tangent.X}
		seaward[i] = left
		if dotXY(right, fromDirection) > dotXY(left, fromDirection) {
			seaward[i] = right
		}
		alternative := left
		if seaward[i] == left {
			alternative = right
		}
		incidence := dotXY(seaward[i], fromDirection)
		if incidence <= 0 {
			continue
		}
		depth, sampleDistance, err := sampleOffshoreDepth(projected[i], seaward[i], reference, config)
		if err != nil {
			// У контура всего бассейна ориентировка линии может меняться. Второй
			// нормалью выбирается только реальная водная ячейка, а не синтетическая глубина.
			depth, sampleDistance, err = sampleOffshoreDepth(projected[i], alternative, reference, config)
			if err == nil {
				seaward[i] = alternative
				incidence = dotXY(seaward[i], fromDirection)
				if incidence <= 0 {
					continue
				}
			}
		}
		if err != nil {
			return nil, LongshoreStepResult{}, fmt.Errorf("батиметрия для узла %d: %w", i, err)
		}
		availableDepth := math.Abs(depth)
		height, breakingDepth, angle, ok := transformToBreaking(condition, availableDepth, seaward[i], config.BreakingIndex)
		if !ok {
			continue
		}
		energy := seaWaterDensity * accelerationGravity * height * height / 8
		celerity, group := waveCelerityAndGroup(condition.PeakPeriodSeconds, breakingDepth)
		_ = celerity
		power := energy * group
		transport[i] = config.CERCCoefficient * power * math.Sin(angle) * math.Cos(angle) /
			((quartzSedimentDensity - seaWaterDensity) * accelerationGravity * (1 - config.Porosity))
		cells[i] = LongshoreCellResult{PointIndex: i, BreakingWaveHeightM: height, BreakingDepthM: breakingDepth, BreakingAngleDeg: angle * 180 / math.Pi, BathymetryDistanceM: sampleDistance, LongshoreTransportM3S: transport[i]}
	}

	durationSeconds := condition.DurationHours * secondsPerHour
	profileHeight := config.BermHeightMeters + config.ClosureDepthMeters
	updated := make([]pointXY, len(projected))
	copy(updated, projected)
	step := LongshoreStepResult{Condition: condition, Cells: cells}
	for i := range projected {
		leftFlux, rightFlux := 0.0, 0.0 // Граничные условия: непроницаемые торцы сегмента.
		if i > 0 {
			leftFlux = (transport[i-1] + transport[i]) / 2
		}
		if i < len(projected)-1 {
			rightFlux = (transport[i] + transport[i+1]) / 2
		}
		volume := (leftFlux - rightFlux) * durationSeconds
		length := controlCellLength(projected, i)
		change := volume / (length * profileHeight)
		change = clamp(change, -config.MaxShorelineChangeMeters, config.MaxShorelineChangeMeters)
		cells[i].VolumeChangeM3 = change * length * profileHeight
		cells[i].ShorelineChangeM = change
		updated[i] = pointXY{X: projected[i].X + seaward[i].X*change, Y: projected[i].Y + seaward[i].Y*change}
		if cells[i].VolumeChangeM3 < 0 {
			step.ErodedVolumeM3 -= cells[i].VolumeChangeM3
		} else {
			step.DepositedVolumeM3 += cells[i].VolumeChangeM3
		}
		step.MassBalanceM3 += cells[i].VolumeChangeM3
	}
	step.Cells = cells
	output := make([]LatLon, len(updated))
	for i := range updated {
		output[i] = projectFromMeters(updated[i], reference)
	}
	return output, step, nil
}

// sampleOffshoreDepth выбирает только измеренную глубину на морской стороне
// узла; ограничение расстояния исключает неявную экстраполяцию через сушу.
func sampleOffshoreDepth(point, normal pointXY, reference projectionReference, config LongshoreModelConfig) (float64, float64, error) {
	offshore := pointXY{X: point.X + normal.X*config.OffshoreSampleDistanceM, Y: point.Y + normal.Y*config.OffshoreSampleDistanceM}
	coordinate := projectFromMeters(offshore, reference)
	return config.Bathymetry.SampleDepth(coordinate.Lat, coordinate.Lon, config.MaxBathymetryGapMeters)
}

func waveNeighborIndexesOpen(index, count int) (int, int) {
	if index == 0 {
		return 0, 1
	}
	if index == count-1 {
		return count - 2, count - 1
	}
	return index - 1, index + 1
}

func controlCellLength(points []pointXY, index int) float64 {
	if index == 0 {
		return distanceXY(points[0], points[1]) / 2
	}
	if index == len(points)-1 {
		return distanceXY(points[index-1], points[index]) / 2
	}
	return (distanceXY(points[index-1], points[index]) + distanceXY(points[index], points[index+1])) / 2
}

func transformToBreaking(condition WaveCondition, depth float64, normal pointXY, breakingIndex float64) (float64, float64, float64, bool) {
	if depth <= 0.05 {
		return 0, 0, 0, false
	}
	from := directionFromNorthClockwise(condition.DirectionFromDeg)
	alpha0 := math.Atan2(crossXY(normal, from), dotXY(normal, from))
	if math.Abs(alpha0) >= math.Pi/2 {
		return 0, 0, 0, false
	}
	_, group0 := waveCelerityAndGroup(condition.PeakPeriodSeconds, 10000)
	low, high := 0.05, math.Min(depth, 100)
	for iteration := 0; iteration < 50; iteration++ {
		middle := (low + high) / 2
		_, group := waveCelerityAndGroup(condition.PeakPeriodSeconds, middle)
		sinAlpha := clamp(group/group0*math.Sin(alpha0), -0.999, 0.999)
		alpha := math.Asin(sinAlpha)
		height := condition.SignificantWaveHeightM * math.Sqrt(group0*math.Cos(alpha0)/(group*math.Cos(alpha)))
		if height > breakingIndex*middle {
			low = middle
		} else {
			high = middle
		}
	}
	depthBreak := high
	_, group := waveCelerityAndGroup(condition.PeakPeriodSeconds, depthBreak)
	alpha := math.Asin(clamp(group/group0*math.Sin(alpha0), -0.999, 0.999))
	height := math.Min(condition.SignificantWaveHeightM*math.Sqrt(group0*math.Cos(alpha0)/(group*math.Cos(alpha))), breakingIndex*depthBreak)
	return height, depthBreak, alpha, height > 0
}

func waveCelerityAndGroup(period, depth float64) (float64, float64) {
	omega := 2 * math.Pi / period
	k := omega * omega / accelerationGravity
	for iteration := 0; iteration < 30; iteration++ {
		kh := k * depth
		f := accelerationGravity*k*math.Tanh(kh) - omega*omega
		derivative := accelerationGravity * (math.Tanh(kh) + kh/(math.Cosh(kh)*math.Cosh(kh)))
		if math.Abs(derivative) < 1e-12 {
			break
		}
		next := k - f/derivative
		if math.Abs(next-k) < 1e-12 {
			k = next
			break
		}
		k = math.Max(next, 1e-9)
	}
	celerity := omega / k
	groupFactor := 0.5 * (1 + 2*k*depth/math.Sinh(2*k*depth))
	return celerity, celerity * groupFactor
}
