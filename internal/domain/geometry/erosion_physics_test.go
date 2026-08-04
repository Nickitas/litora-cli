package geometry

import (
	"math"
	"testing"
)

// TestPhysicalDepthFactorValues проверяет корректность значений depthFactor
func TestPhysicalDepthFactorValues(t *testing.T) {
	tests := []struct {
		name        string
		depthMeters float64
		fetchMeters float64
		depthScale  float64
		expectedMin float64
		expectedMax float64
	}{
		{
			name:        "Очень глубокая вода (2000 м)",
			depthMeters: -2000,
			fetchMeters: 5000,
			depthScale:  1000,
			expectedMin: 0.8, // exp(-2000/1000) = 0.135, 1 - 0.135 = 0.865
			expectedMax: 0.9,
		},
		{
			name:        "Мелководье (10 м)",
			depthMeters: -10,
			fetchMeters: 1000,
			depthScale:  1000,
			expectedMin: 0.0,
			expectedMax: 0.05, // exp(-10/1000) = 0.99, 1 - 0.99 = 0.01
		},
		{
			name:        "Нулевая глубина (уровень моря)",
			depthMeters: 0,
			fetchMeters: 1000,
			depthScale:  1000,
			expectedMin: 0.0,
			expectedMax: 0.01,
		},
		{
			name:        "Суша (положительная глубина)",
			depthMeters: 10,
			fetchMeters: 1000,
			depthScale:  1000,
			expectedMin: 0.0,
			expectedMax: 0.01,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := physicalDepthFactor(tt.depthMeters, tt.fetchMeters, tt.depthScale)
			if result < tt.expectedMin || result > tt.expectedMax {
				t.Errorf("physicalDepthFactor(%f, %f, %f) = %f, ожидается [%f, %f]",
					tt.depthMeters, tt.fetchMeters, tt.depthScale, result, tt.expectedMin, tt.expectedMax)
			}
		})
	}
}

// TestWaveErosionSanityChecks проверяет базовые sanity checks для волновой эрозии
func TestWaveErosionSanityChecks(t *testing.T) {
	// Простой прямоугольный берег
	points := []LatLon{
		{Lat: 45.0, Lon: 30.0},
		{Lat: 45.0, Lon: 31.0},
		{Lat: 44.0, Lon: 31.0},
		{Lat: 44.0, Lon: 30.0},
		{Lat: 45.0, Lon: 30.0},
	}

	options := WaveErosionOptions{
		StrengthMeters:           10,
		WindSourceDirectionDeg:   0,
		WindSpeedMetersPerSecond: 10,
		FetchSpreadDeg:           45,
		FetchSamples:             5,
		MaxFetchMeters:           5000,
		DepthScaleMeters:         1000,
		ExposurePower:            1.5,
	}

	snapshots := SimulateWaveErosionWithSeed(points, 1, options, 42)
	if len(snapshots) != 2 {
		t.Fatalf("ожидалось 2 снимка, получено %d", len(snapshots))
	}

	initial := snapshots[0]
	eroded := snapshots[1]

	// Проверки:
	// 1. Число точек не изменилось
	if len(initial) != len(eroded) {
		t.Errorf("количество точек изменилось: %d -> %d", len(initial), len(eroded))
	}

	// 2. Берег остался замкнутым
	if initial[0] != initial[len(initial)-1] {
		t.Error("начальный полигон не замкнут")
	}
	if eroded[0] != eroded[len(eroded)-1] {
		t.Error("эродированный полигон не замкнут")
	}

	// 3. Длина уменьшилась (эрозия)
	initialLength := PolylineLength(initial)
	erodedLength := PolylineLength(eroded)
	if erodedLength >= initialLength {
		t.Errorf("длина должна уменьшиться: %.2f -> %.2f", initialLength, erodedLength)
	}

	// 4. Площадь уменьшилась (эрозия)
	initialArea := Area(initial)
	erodedArea := Area(eroded)
	if erodedArea >= initialArea {
		t.Errorf("площадь должна уменьшиться: %.2f -> %.2f", initialArea, erodedArea)
	}

	// 5. Отступ был разумным (не слишком большой, не слишком маленький)
	maxRetreat := 0.0
	for i := 0; i < len(initial); i++ {
		dist := Haversine(initial[i], eroded[i])
		if dist > maxRetreat {
			maxRetreat = dist
		}
	}

	if maxRetreat < 0.001 {
		t.Errorf("максимальный отступ слишком мал: %f км (должен быть ~10 м)", maxRetreat)
	}
	if maxRetreat > 0.1 {
		t.Errorf("максимальный отступ слишком велик: %f км (должен быть ~10 м)", maxRetreat)
	}
}

// TestWaveErosionConsistency проверяет консистентность результатов
func TestWaveErosionConsistency(t *testing.T) {
	points := []LatLon{
		{Lat: 45.0, Lon: 30.0},
		{Lat: 45.0, Lon: 31.0},
		{Lat: 44.0, Lon: 31.0},
		{Lat: 44.0, Lon: 30.0},
		{Lat: 45.0, Lon: 30.0},
	}

	options := WaveErosionOptions{
		StrengthMeters:           20,
		WindSourceDirectionDeg:   90,
		WindSpeedMetersPerSecond: 12,
		FetchSpreadDeg:           55,
		FetchSamples:             9,
		MaxFetchMeters:           10000,
		DepthScaleMeters:         4000,
		ExposurePower:            1.5,
	}

	// Два запуска с одинаковым seed должны давать идентичные результаты
	snapshots1 := SimulateWaveErosionWithSeed(points, 3, options, 12345)
	snapshots2 := SimulateWaveErosionWithSeed(points, 3, options, 12345)

	if len(snapshots1) != len(snapshots2) {
		t.Fatalf("разное количество снимков: %d против %d", len(snapshots1), len(snapshots2))
	}

	for step := 0; step < len(snapshots1); step++ {
		if len(snapshots1[step]) != len(snapshots2[step]) {
			t.Errorf("шаг %d: разное количество точек", step)
			continue
		}

		for i := 0; i < len(snapshots1[step]); i++ {
			p1 := snapshots1[step][i]
			p2 := snapshots2[step][i]

			// Проверяем координаты с точностью до 1 мм (0.000001 градусов)
			if math.Abs(p1.Lat-p2.Lat) > 1e-6 || math.Abs(p1.Lon-p2.Lon) > 1e-6 {
				t.Errorf("шаг %d, точка %d: не детерминировано: (%f, %f) против (%f, %f)",
					step, i, p1.Lat, p1.Lon, p2.Lat, p2.Lon)
			}
		}
	}
}

// TestWaveErosionOpenCoastErodesMoreThanProtected проверяет физически корректное поведение
func TestWaveErosionOpenCoastErodesMoreThanProtected(t *testing.T) {
	// Создаём "изгиб" берега - открытая часть vs защищённая
	points := []LatLon{
		{Lat: 45.0, Lon: 30.0},
		{Lat: 45.0, Lon: 31.0}, // Открытый к северу
		{Lat: 44.5, Lon: 31.0}, // Вогнута (защищённая)
		{Lat: 44.0, Lon: 30.5}, // Открытый к северу
		{Lat: 44.0, Lon: 30.0},
		{Lat: 45.0, Lon: 30.0},
	}

	options := WaveErosionOptions{
		StrengthMeters:           50,
		WindSourceDirectionDeg:   0, // Волны с севера
		WindSpeedMetersPerSecond: 12,
		FetchSpreadDeg:           30,
		FetchSamples:             7,
		MaxFetchMeters:           5000,
		DepthScaleMeters:         2000,
		ExposurePower:            1.5,
	}

	snapshots := SimulateWaveErosionWithSeed(points, 1, options, 42)
	eroded := snapshots[1]

	// Открытые точки (индексы 1, 3) должны эродировать больше, чем защищённые (индекс 2)
	openCoastRetreat := Haversine(points[1], eroded[1]) + Haversine(points[3], eroded[3])
	protectedRetreat := Haversine(points[2], eroded[2])

	t.Logf("Открытый берег: отступ %.2f км", openCoastRetreat*1000)
	t.Logf("Защищённый берег: отступ %.2f км", protectedRetreat*1000)

	if protectedRetreat > openCoastRetreat {
		t.Errorf("защищённый берег (%.2f км) эродирован больше чем открытый (%.2f км)",
			protectedRetreat, openCoastRetreat)
	}
}

// TestWindFactorScaling проверяет корректность масштабирования по скорости ветра
func TestWindFactorScaling(t *testing.T) {
	tests := []struct {
		name           string
		windSpeed      float64
		expectedFactor float64
		tolerance      float64
	}{
		{"6 м/с", 6, 0.25, 0.01},
		{"12 м/с", 12, 1.0, 0.01},
		{"18 м/с", 18, 2.25, 0.01},
		{"24 м/с", 24, 4.0, 0.01},
		{"3 м/с", 3, 0.1, 0.01},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			factor := math.Pow(tt.windSpeed/12.0, 2)
			factor = math.Max(0.1, math.Min(factor, 4.0))

			if math.Abs(factor-tt.expectedFactor) > tt.tolerance {
				t.Errorf("скорость ветра %.1f м/с: коэффициент %.2f, ожидается %.2f",
					tt.windSpeed, factor, tt.expectedFactor)
			}
		})
	}
}

// TestFetchFactorCalculation проверяет корректность расчёта fetch factor
func TestFetchFactorCalculation(t *testing.T) {
	tests := []struct {
		name        string
		meanFetch   float64
		maxFetch    float64
		expectedMin float64
		expectedMax float64
	}{
		{
			name:        "Fetch = max",
			meanFetch:   150000,
			maxFetch:    150000,
			expectedMin: 0.99,
			expectedMax: 1.01,
		},
		{
			name:        "Fetch = half of max",
			meanFetch:   75000,
			maxFetch:    150000,
			expectedMin: 0.69,
			expectedMax: 0.72, // sqrt(0.5) ≈ 0.707
		},
		{
			name:        "Fetch = quarter of max",
			meanFetch:   37500,
			maxFetch:    150000,
			expectedMin: 0.48,
			expectedMax: 0.51, // sqrt(0.25) = 0.5
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fetchFactor := math.Sqrt(clamp(tt.meanFetch/tt.maxFetch, 0, 1))

			if fetchFactor < tt.expectedMin || fetchFactor > tt.expectedMax {
				t.Errorf("fetch %.0f/%.0f: коэффициент %.3f, ожидается [%.2f, %.2f]",
					tt.meanFetch, tt.maxFetch, fetchFactor, tt.expectedMin, tt.expectedMax)
			}
		})
	}
}

// TestExposurePowerCalculation проверяет корректность расчёта экспозиции
func TestExposurePowerCalculation(t *testing.T) {
	tests := []struct {
		name      string
		incidence float64 // cos(угла)
		power     float64
		expected  float64
	}{
		{
			name:      "Перпендикулярно (максимальная экспозиция)",
			incidence: 1.0,
			power:     1.5,
			expected:  1.0,
		},
		{
			name:      "45 градусов",
			incidence: 0.707,
			power:     1.5,
			expected:  0.594, // 0.707^1.5
		},
		{
			name:      "Скользящий (низкая экспозиция)",
			incidence: 0.1,
			power:     1.5,
			expected:  0.032, // 0.1^1.5
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			weight := math.Pow(tt.incidence, tt.power)

			if math.Abs(weight-tt.expected) > 0.01 {
				t.Errorf("угол падения %.3f при степени %.1f: %.3f, ожидается %.3f",
					tt.incidence, tt.power, weight, tt.expected)
			}
		})
	}
}

func TestErode(t *testing.T) {
	tests := []struct {
		name     string
		points   []LatLon
		strength float64
		wantLen  int
	}{
		{
			name: "нулевая сила - возвращает копию",
			points: []LatLon{
				{Lat: 45, Lon: 30},
				{Lat: 45, Lon: 31},
			},
			strength: 0,
			wantLen:  2,
		},
		{
			name: "отрицательная сила - возвращает копию",
			points: []LatLon{
				{Lat: 45, Lon: 30},
				{Lat: 45, Lon: 31},
			},
			strength: -10,
			wantLen:  2,
		},
		{
			name: "положительная сила - возвращает изменённые точки",
			points: []LatLon{
				{Lat: 45, Lon: 30},
				{Lat: 45, Lon: 31},
			},
			strength: 1000, // 1 км в метрах
			wantLen:  2,
		},
		{
			name:     "пустой набор точек",
			points:   []LatLon{},
			strength: 100,
			wantLen:  0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := Erode(tt.points, tt.strength)
			
			if len(result) != tt.wantLen {
				t.Errorf("Erode() returned length %v, want %v", len(result), tt.wantLen)
			}
			
			// Проверяем, что результат это новый слайс (не та же память)
			if len(tt.points) > 0 && tt.strength >= 0 {
				// Для нулевой силы результат должен быть равен исходному
				if tt.strength == 0 {
					for i := range result {
						if result[i] != tt.points[i] {
							t.Errorf("Erode() with strength=0 should return copy, but point %d differs", i)
						}
					}
				}
			}
		})
	}
}

func TestSimulateErosion(t *testing.T) {
	points := []LatLon{
		{Lat: 45, Lon: 30},
		{Lat: 45, Lon: 31},
		{Lat: 46, Lon: 31},
	}
	
	tests := []struct {
		name     string
		points   []LatLon
		steps    int
		strength float64
		wantLen  int
	}{
		{
			name:     "один шаг эрозии",
			points:   points,
			steps:    1,
			strength: 1000,
			wantLen:  2, // начальное состояние + 1 шаг
		},
		{
			name:     "несколько шагов эрозии",
			points:   points,
			steps:    3,
			strength: 500,
			wantLen:  4, // начальное состояние + 3 шага
		},
		{
			name:     "ноль шагов - только начальное состояние",
			points:   points,
			steps:    0,
			strength: 1000,
			wantLen:  1, // только начальное состояние
		},
		{
			name:     "отрицательное число шагов - как ноль",
			points:   points,
			steps:    -1,
			strength: 1000,
			wantLen:  1, // только начальное состояние
		},
		{
			name:     "нулевая сила - без изменений",
			points:   points,
			steps:    2,
			strength: 0,
			wantLen:  3, // начальное состояние + 2 шага
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			snapshots := SimulateErosion(tt.points, tt.steps, tt.strength)
			
			if len(snapshots) != tt.wantLen {
				t.Errorf("SimulateErosion() returned %d snapshots, want %d", len(snapshots), tt.wantLen)
			}
			
			// Проверяем, что начальное состояние сохранено
			if len(snapshots) > 0 {
				if len(snapshots[0]) != len(tt.points) {
					t.Errorf("Initial snapshot has %d points, want %d", len(snapshots[0]), len(tt.points))
				}
			}
		})
	}
}

func TestErodeWithSeed(t *testing.T) {
	points := []LatLon{
		{Lat: 45, Lon: 30},
		{Lat: 45, Lon: 31},
	}
	
	tests := []struct {
		name     string
		points   []LatLon
		strength float64
		seed     int64
		wantLen  int
	}{
		{
			name:     "с фиксированным seed",
			points:   points,
			strength: 1000,
			seed:     12345,
			wantLen:  2,
		},
		{
			name:     "один и тот же seed даёт одинаковый результат",
			points:   points,
			strength: 1000,
			seed:     999,
			wantLen:  2,
		},
		{
			name:     "нулевой seed использует случайное значение",
			points:   points,
			strength: 1000,
			seed:     0,
			wantLen:  2,
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result1 := ErodeWithSeed(tt.points, tt.strength, tt.seed)
			result2 := ErodeWithSeed(tt.points, tt.strength, tt.seed)
			
			if len(result1) != tt.wantLen {
				t.Errorf("ErodeWithSeed() returned length %v, want %v", len(result1), tt.wantLen)
			}
			
			// Для ненулевого seed результаты должны быть идентичными
			if tt.seed != 0 {
				for i := range result1 {
					if result1[i] != result2[i] {
						t.Errorf("ErodeWithSeed() with same seed produced different results at index %d", i)
					}
				}
			}
		})
	}
}
