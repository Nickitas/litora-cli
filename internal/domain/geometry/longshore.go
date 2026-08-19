package geometry

import (
	"fmt"
	"math"
	"time"
)

const (
	accelerationGravity   = 9.80665
	seaWaterDensity       = 1025.0
	quartzSedimentDensity = 2650.0
	secondsPerHour        = 3600.0

	// ScenarioStatusDemo обозначает технический демонстрационный расчёт,
	// который нельзя интерпретировать как исследовательский результат.
	ScenarioStatusDemo = "demo"
	// ScenarioStatusUnclassified обозначает расчёт, исследовательская
	// пригодность которого не подтверждена автоматическими проверками Lito.
	ScenarioStatusUnclassified = "unclassified"
)

// ScenarioClassification явно отделяет успешность вычисления от пригодности
// результата для исследования, калибровки или публикации.
type ScenarioClassification struct {
	ScenarioStatus   string   `json:"scenario_status"`
	UsageLimitations []string `json:"usage_limitations,omitempty"`
}

// LongshoreModelConfig задаёт параметры инженерной одномерной модели CERC.
// Сетка глубин обязательна: модель не подменяет преобразование волн
// эвристическим множителем глубины.
type LongshoreModelConfig struct {
	Scenario                  ScenarioClassification
	Bathymetry                *BathymetryGrid
	BathymetrySource          string                    // источник или путь к использованной батиметрии
	BathymetrySHA256          string                    // SHA-256 фактически загруженного JSON
	BathymetryPassport        string                    // путь к проверенному паспорту происхождения
	BathymetryStatus          string                    // статус набора из паспорта
	WaterbodyID               string                    // идентификатор выбранного водоёма из каталога Lito
	SedimentSources           []LongshoreSedimentSource // внешние источники и стоки наносов по ячейкам
	Structures                []LongshoreStructure      // сооружения, меняющие пропуск потока между ячейками
	LeftBoundaryTransportM3S  float64                   // поток через левую границу, положительный внутрь сегмента
	RightBoundaryTransportM3S float64                   // поток через правую границу, положительный из сегмента
	BreakingIndex             float64                   // γ = H_b / h_b, обычно 0,78
	BermHeightMeters          float64                   // высота активного профиля над уровнем моря
	ClosureDepthMeters        float64                   // глубина замыкания активного профиля
	Porosity                  float64                   // пористость наносов [0; 1)
	CERCCoefficient           float64                   // безразмерный коэффициент CERC
	OffshoreSampleDistanceM   float64                   // расстояние отбора глубины от береговой линии
	MaxBathymetryGapMeters    float64                   // максимальное расстояние до реальной точки глубины
	MaxShorelineChangeMeters  float64                   // численная защита для одного состояния волн
}

// LongshoreCellResult хранит пространственно определённый баланс одной ячейки.
// Объёмы относятся к сегменту между соседними узлами и измеряются в м³.
type LongshoreCellResult struct {
	PointIndex               int     `json:"point_index"`
	BathymetrySampled        bool    `json:"bathymetry_sampled"`
	BreakingWaveHeightM      float64 `json:"breaking_wave_height_m"`
	BreakingDepthM           float64 `json:"breaking_depth_m"`
	BreakingAngleDeg         float64 `json:"breaking_angle_deg"`
	BathymetryDistanceM      float64 `json:"bathymetry_distance_m"`
	LongshoreTransportM3S    float64 `json:"longshore_transport_m3_s"`
	LeftFaceTransportM3S     float64 `json:"left_face_transport_m3_s"`
	RightFaceTransportM3S    float64 `json:"right_face_transport_m3_s"`
	VolumeChangeM3           float64 `json:"volume_change_m3"`
	ShorelineChangeM         float64 `json:"shoreline_change_m"`
	ExternalSedimentVolumeM3 float64 `json:"external_sediment_volume_m3"`
}

// LongshoreStepResult описывает результат одного реального состояния волн.
type LongshoreStepResult struct {
	Condition                 WaveCondition         `json:"condition"`
	Cells                     []LongshoreCellResult `json:"cells"`
	ErodedVolumeM3            float64               `json:"eroded_volume_m3"`
	DepositedVolumeM3         float64               `json:"deposited_volume_m3"`
	MassBalanceM3             float64               `json:"mass_balance_m3"` // суммарное изменение объёма ячеек
	LeftBoundaryTransportM3S  float64               `json:"left_boundary_transport_m3_s"`
	RightBoundaryTransportM3S float64               `json:"right_boundary_transport_m3_s"`
	BoundaryNetVolumeM3       float64               `json:"boundary_net_volume_m3"`
	ExternalSedimentVolumeM3  float64               `json:"external_sediment_volume_m3"`
	BalanceClosureResidualM3  float64               `json:"balance_closure_residual_m3"`
}

// WaveClimateQuality описывает временное покрытие использованного ряда волн.
// Отсутствие временных меток не делает ряд вымышленным, но не позволяет
// проверить пропуски или наложения интервалов.
type WaveClimateQuality struct {
	ConditionCount      int     `json:"condition_count"`
	TotalDurationHours  float64 `json:"total_duration_hours"`
	HasCompleteTimes    bool    `json:"has_complete_times"`
	FirstTime           string  `json:"first_time,omitempty"`
	LastTime            string  `json:"last_time,omitempty"`
	MaxTemporalGapHours float64 `json:"max_temporal_gap_hours,omitempty"`
}

// BathymetryQuality описывает фактическую пространственную опору расчёта.
// Ненулевое расстояние означает использование ближайшего измерения вместо
// билинейной интерполяции и должно учитываться при интерпретации результата.
type BathymetryQuality struct {
	PointCount                int     `json:"point_count"`
	ResolutionDegrees         float64 `json:"resolution_degrees"`
	SampleCount               int     `json:"sample_count"`
	InterpolatedSampleCount   int     `json:"interpolated_sample_count"`
	NearestSampleCount        int     `json:"nearest_sample_count"`
	MeanNearestDistanceMeters float64 `json:"mean_nearest_distance_meters"`
	MaxNearestDistanceMeters  float64 `json:"max_nearest_distance_meters"`
}

// ModelInputQuality объединяет проверяемые характеристики входов, реально
// использованных в расчёте, а не только параметры командной строки.
type ModelInputQuality struct {
	WaveClimate WaveClimateQuality `json:"wave_climate"`
	Bathymetry  BathymetryQuality  `json:"bathymetry"`
}

// LongshoreModelResult содержит положения береговой линии и баланс для всего
// волнового ряда. Первый снимок — исходное положение береговой линии.
type LongshoreModelResult struct {
	ScenarioClassification
	Model              string                    `json:"model"`
	WaterbodyID        string                    `json:"waterbody_id,omitempty"`
	BathymetrySource   string                    `json:"bathymetry_source"`
	BathymetrySHA256   string                    `json:"bathymetry_sha256,omitempty"`
	BathymetryPassport string                    `json:"bathymetry_passport,omitempty"`
	BathymetryStatus   string                    `json:"bathymetry_status,omitempty"`
	Climate            WaveClimate               `json:"climate"`
	Snapshots          [][]LatLon                `json:"snapshots"`
	Steps              []LongshoreStepResult     `json:"steps"`
	InputQuality       ModelInputQuality         `json:"input_quality"`
	SedimentSources    []LongshoreSedimentSource `json:"sediment_sources,omitempty"`
	Structures         []LongshoreStructure      `json:"structures,omitempty"`
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
	if err := validateLongshoreSedimentSources(config.SedimentSources, len(points)); err != nil {
		return LongshoreModelResult{}, err
	}
	if err := validateLongshoreStructures(config.Structures, len(points)); err != nil {
		return LongshoreModelResult{}, err
	}

	result := LongshoreModelResult{
		ScenarioClassification: config.Scenario,
		Model:                  "Одномерная CERC: дисперсия, рефракция, shoaling, разрушение волн и баланс наносов",
		WaterbodyID:            config.WaterbodyID,
		BathymetrySource:       config.BathymetrySource,
		BathymetrySHA256:       config.BathymetrySHA256,
		BathymetryPassport:     config.BathymetryPassport,
		BathymetryStatus:       config.BathymetryStatus,
		Climate:                climate,
		SedimentSources:        append([]LongshoreSedimentSource(nil), config.SedimentSources...),
		Structures:             append([]LongshoreStructure(nil), config.Structures...),
		Snapshots:              make([][]LatLon, 1, len(climate.Conditions)+1),
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
	result.InputQuality = summarizeModelInputQuality(climate, config.Bathymetry, result.Steps)
	return result, nil
}

func normalizeLongshoreModelConfig(config LongshoreModelConfig) LongshoreModelConfig {
	if config.Scenario.ScenarioStatus == "" {
		config.Scenario.ScenarioStatus = ScenarioStatusUnclassified
	}
	config.Scenario.UsageLimitations = append([]string(nil), config.Scenario.UsageLimitations...)
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
	sedimentSources := longshoreSedimentSourceRates(config.SedimentSources, len(points))
	faceTransmission := longshoreFaceTransmission(config.Structures, len(points))
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
		cells[i] = LongshoreCellResult{PointIndex: i, BathymetrySampled: true, BreakingWaveHeightM: height, BreakingDepthM: breakingDepth, BreakingAngleDeg: angle * 180 / math.Pi, BathymetryDistanceM: sampleDistance, LongshoreTransportM3S: transport[i]}
	}

	durationSeconds := condition.DurationHours * secondsPerHour
	profileHeight := config.BermHeightMeters + config.ClosureDepthMeters
	faceTransport := make([]float64, len(points)-1)
	for i := range faceTransport {
		faceTransport[i] = (transport[i] + transport[i+1]) / 2 * faceTransmission[i]
	}
	updated := make([]pointXY, len(projected))
	copy(updated, projected)
	step := LongshoreStepResult{
		Condition:                 condition,
		Cells:                     cells,
		LeftBoundaryTransportM3S:  config.LeftBoundaryTransportM3S,
		RightBoundaryTransportM3S: config.RightBoundaryTransportM3S,
	}
	for i := range projected {
		leftFlux, rightFlux := 0.0, 0.0
		if i > 0 {
			leftFlux = faceTransport[i-1]
		} else {
			leftFlux = config.LeftBoundaryTransportM3S
		}
		if i < len(projected)-1 {
			rightFlux = faceTransport[i]
		} else {
			rightFlux = config.RightBoundaryTransportM3S
		}
		externalVolume := sedimentSources[i] * durationSeconds
		volume := (leftFlux-rightFlux)*durationSeconds + externalVolume
		length := controlCellLength(projected, i)
		change := volume / (length * profileHeight)
		change = clamp(change, -config.MaxShorelineChangeMeters, config.MaxShorelineChangeMeters)
		cells[i].VolumeChangeM3 = change * length * profileHeight
		cells[i].ExternalSedimentVolumeM3 = externalVolume
		cells[i].LeftFaceTransportM3S = leftFlux
		cells[i].RightFaceTransportM3S = rightFlux
		cells[i].ShorelineChangeM = change
		updated[i] = pointXY{X: projected[i].X + seaward[i].X*change, Y: projected[i].Y + seaward[i].Y*change}
		if cells[i].VolumeChangeM3 < 0 {
			step.ErodedVolumeM3 -= cells[i].VolumeChangeM3
		} else {
			step.DepositedVolumeM3 += cells[i].VolumeChangeM3
		}
		step.MassBalanceM3 += cells[i].VolumeChangeM3
		step.ExternalSedimentVolumeM3 += externalVolume
	}
	step.BoundaryNetVolumeM3 = (config.LeftBoundaryTransportM3S - config.RightBoundaryTransportM3S) * durationSeconds
	step.BalanceClosureResidualM3 = step.MassBalanceM3 - step.BoundaryNetVolumeM3 - step.ExternalSedimentVolumeM3
	step.Cells = cells
	output := make([]LatLon, len(updated))
	for i := range updated {
		output[i] = projectFromMeters(updated[i], reference)
	}
	return output, step, nil
}

func summarizeModelInputQuality(climate WaveClimate, bathymetry *BathymetryGrid, steps []LongshoreStepResult) ModelInputQuality {
	quality := ModelInputQuality{WaveClimate: summarizeWaveClimateQuality(climate)}
	if bathymetry == nil {
		return quality
	}
	bathQuality := BathymetryQuality{PointCount: len(bathymetry.Points), ResolutionDegrees: bathymetry.Resolution}
	nearestDistanceSum := 0.0
	for _, step := range steps {
		for _, cell := range step.Cells {
			if !cell.BathymetrySampled {
				continue
			}
			bathQuality.SampleCount++
			if cell.BathymetryDistanceM == 0 {
				bathQuality.InterpolatedSampleCount++
				continue
			}
			bathQuality.NearestSampleCount++
			nearestDistanceSum += cell.BathymetryDistanceM
			if cell.BathymetryDistanceM > bathQuality.MaxNearestDistanceMeters {
				bathQuality.MaxNearestDistanceMeters = cell.BathymetryDistanceM
			}
		}
	}
	if bathQuality.NearestSampleCount > 0 {
		bathQuality.MeanNearestDistanceMeters = nearestDistanceSum / float64(bathQuality.NearestSampleCount)
	}
	quality.Bathymetry = bathQuality
	return quality
}

func summarizeWaveClimateQuality(climate WaveClimate) WaveClimateQuality {
	quality := WaveClimateQuality{ConditionCount: len(climate.Conditions)}
	if len(climate.Conditions) == 0 {
		return quality
	}
	quality.HasCompleteTimes = true
	for i, condition := range climate.Conditions {
		quality.TotalDurationHours += condition.DurationHours
		if condition.Time.IsZero() {
			quality.HasCompleteTimes = false
			continue
		}
		if i == 0 {
			quality.FirstTime = condition.Time.UTC().Format(time.RFC3339)
			continue
		}
		previous := climate.Conditions[i-1]
		if !previous.Time.IsZero() {
			gap := condition.Time.Sub(previous.Time).Hours() - previous.DurationHours
			if gap > quality.MaxTemporalGapHours {
				quality.MaxTemporalGapHours = gap
			}
		}
	}
	if quality.HasCompleteTimes {
		last := climate.Conditions[len(climate.Conditions)-1]
		quality.LastTime = last.Time.Add(time.Duration(last.DurationHours * float64(time.Hour))).UTC().Format(time.RFC3339)
	}
	return quality
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
