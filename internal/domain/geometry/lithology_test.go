package geometry

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"testing"
)

// TestLoadLithologyProfile тест загрузки профиля
func TestLoadLithologyProfile(t *testing.T) {
	// Создаём тестовый профиль
	testProfile := LithologyProfile{
		Metadata: LithologyMetadata{
			Name:       "Test Profile",
			Version:    "1.0",
			Resolution: 0.5,
			Bounds: Bounds{
				MinLat: 40.0,
				MaxLat: 47.0,
				MinLon: 27.0,
				MaxLon: 42.0,
			},
		},
		Points: []LithologyPoint{
			{
				Lat:         45.0,
				Lon:         34.0,
				Region:      "crimea",
				Lithology:   "limestone",
				Resistance:  4.5,
				Color:       "#6b6b6b",
				Description: "Test limestone",
				Confidence:  "high",
			},
		},
		Classes: map[string]LithologyClass{
			"limestone": {
				Resistance:  4.5,
				Color:       "#6b6b6b",
				Description: "Limestone test",
			},
		},
	}

	// Конвертируем в JSON
	data, err := json.Marshal(testProfile)
	if err != nil {
		t.Fatalf("Не удалось сериализовать тестовый профиль: %v", err)
	}

	// Загружаем обратно
	loaded, err := LoadLithologyProfile(data)
	if err != nil {
		t.Fatalf("Не удалось загрузить профиль: %v", err)
	}

	// Проверки
	if loaded.Metadata.Name != testProfile.Metadata.Name {
		t.Errorf("Ожидалось имя %s, получено %s", testProfile.Metadata.Name, loaded.Metadata.Name)
	}

	if len(loaded.Points) != len(testProfile.Points) {
		t.Errorf("Ожидалось %d точек, получено %d", len(testProfile.Points), len(loaded.Points))
	}

	if len(loaded.Classes) != len(testProfile.Classes) {
		t.Errorf("Ожидалось %d классов, получено %d", len(testProfile.Classes), len(loaded.Classes))
	}

	t.Logf("✓ Профиль загружен успешно: %s (%d точек, %d классов)",
		loaded.Metadata.Name, len(loaded.Points), len(loaded.Classes))
}

// TestLoadLithologyProfileValidation тест валидации
func TestLoadLithologyProfileValidation(t *testing.T) {
	testCases := []struct {
		name    string
		profile LithologyProfile
		wantErr bool
	}{
		{
			name: "валидный профиль",
			profile: LithologyProfile{
				Metadata: LithologyMetadata{
					Name:   "Valid",
					Bounds: Bounds{MinLat: 40, MaxLat: 47, MinLon: 27, MaxLon: 42},
				},
				Points: []LithologyPoint{
					{Lat: 45, Lon: 34, Lithology: "limestone", Resistance: 4.0, Color: "#6b6b6b"},
				},
				Classes: map[string]LithologyClass{
					"limestone": {Resistance: 4.0, Color: "#6b6b6b"},
				},
			},
			wantErr: false,
		},
		{
			name: "неверные границы широты",
			profile: LithologyProfile{
				Metadata: LithologyMetadata{
					Name:   "Invalid",
					Bounds: Bounds{MinLat: 47, MaxLat: 40, MinLon: 27, MaxLon: 42},
				},
				Points: []LithologyPoint{},
			},
			wantErr: true,
		},
		{
			name: "неверные координаты точки",
			profile: LithologyProfile{
				Metadata: LithologyMetadata{
					Name:   "Invalid",
					Bounds: Bounds{MinLat: 40, MaxLat: 47, MinLon: 27, MaxLon: 42},
				},
				Points: []LithologyPoint{
					{Lat: 100, Lon: 34, Lithology: "limestone", Resistance: 4.0, Color: "#6b6b6b"},
				},
			},
			wantErr: true,
		},
		{
			name: "неверное сопротивление",
			profile: LithologyProfile{
				Metadata: LithologyMetadata{
					Name:   "Invalid",
					Bounds: Bounds{MinLat: 40, MaxLat: 47, MinLon: 27, MaxLon: 42},
				},
				Points: []LithologyPoint{
					{Lat: 45, Lon: 34, Lithology: "limestone", Resistance: -1.0, Color: "#6b6b6b"},
				},
			},
			wantErr: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			data, _ := json.Marshal(tc.profile)
			_, err := LoadLithologyProfile(data)

			if tc.wantErr && err == nil {
				t.Error("Ожидалась ошибка, но не получено")
			}
			if !tc.wantErr && err != nil {
				t.Errorf("Ожидалось отсутствие ошибки, но получено: %v", err)
			}
		})
	}
}

// TestGetLithologyAt тест получения литологии
func TestGetLithologyAt(t *testing.T) {
	// Создаём профиль с известными точками
	profile := &LithologyProfile{
		Metadata: LithologyMetadata{
			Name:   "Test",
			Bounds: Bounds{MinLat: 44, MaxLat: 46, MinLon: 33, MaxLon: 35},
		},
		Points: []LithologyPoint{
			{Lat: 45.0, Lon: 34.0, Lithology: "limestone", Resistance: 4.5, Color: "#6b6b6b"},
			{Lat: 44.5, Lon: 33.5, Lithology: "clay", Resistance: 1.2, Color: "#c4a484"},
		},
		Classes: map[string]LithologyClass{
			"limestone": {Resistance: 4.5, Color: "#6b6b6b", Description: "Limestone"},
			"clay":      {Resistance: 1.2, Color: "#c4a484", Description: "Clay"},
		},
	}

	testCases := []struct {
		name      string
		lat       float64
		lon       float64
		wantClass string
		wantR     float64
	}{
		{
			name:      "близко к точке известняка",
			lat:       45.0,
			lon:       34.0,
			wantClass: "limestone",
			wantR:     4.5,
		},
		{
			name:      "близко к точке глины",
			lat:       44.5,
			lon:       33.5,
			wantClass: "clay",
			wantR:     1.2,
		},
		{
			name:      "средняя точка между",
			lat:       44.75,
			lon:       33.75,
			wantClass: "limestone", // ближайшая точка (доминирующий вес)
			wantR:     2.85,        // IDW интерполяция: (4.5 + 1.2) / 2 ≈ 2.85
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			state := profile.GetLithologyAt(tc.lat, tc.lon)

			if state.Class != tc.wantClass {
				t.Errorf("Ожидался класс %s, получено %s", tc.wantClass, state.Class)
			}

			if state.Resistance != tc.wantR {
				t.Errorf("Ожидалось сопротивление %.2f, получено %.2f", tc.wantR, state.Resistance)
			}

			t.Logf("✓ (%.2f, %.2f) → %s (R=%.2f)", tc.lat, tc.lon, state.Class, state.Resistance)
		})
	}
}

// TestGetLithologyAtOutOfBounds тест граничных условий
func TestGetLithologyAtOutOfBounds(t *testing.T) {
	profile := &LithologyProfile{
		Metadata: LithologyMetadata{
			Name:   "Test",
			Bounds: Bounds{MinLat: 44, MaxLat: 46, MinLon: 33, MaxLon: 35},
		},
		Points:  []LithologyPoint{},
		Classes: map[string]LithologyClass{},
	}

	testCases := []struct {
		name  string
		lat   float64
		lon   float64
		valid bool
	}{
		{name: "в пределах границ", lat: 45.0, lon: 34.0, valid: true},
		{name: "вне границ - юг", lat: 43.0, lon: 34.0, valid: false},
		{name: "вне границ - север", lat: 47.0, lon: 34.0, valid: false},
		{name: "вне границ - запад", lat: 45.0, lon: 32.0, valid: false},
		{name: "вне границ - восток", lat: 45.0, lon: 36.0, valid: false},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			state := profile.GetLithologyAt(tc.lat, tc.lon)

			// Для out of bounds должен возвращаться default
			if !tc.valid {
				if state.Class == "" {
					t.Error("Ожидалась литология по умолчанию для точек вне границ, получен пустой класс")
				}
				if state.Resistance <= 0 {
					t.Error("Ожидалось положительное сопротивление для литологии по умолчанию")
				}
				t.Logf("✓ Вне границ (%.2f, %.2f) → дефолт %s (R=%.2f)",
					tc.lat, tc.lon, state.Class, state.Resistance)
			} else {
				if state.Class == "" {
					t.Error("Ожидалась какая-то литология в пределах границ")
				}
				t.Logf("✓ В пределах границ (%.2f, %.2f) → %s (R=%.2f)",
					tc.lat, tc.lon, state.Class, state.Resistance)
			}
		})
	}
}

// TestGetStatistics тест статистики профиля
func TestGetStatistics(t *testing.T) {
	profile := &LithologyProfile{
		Metadata: LithologyMetadata{
			Name:       "Test Stats",
			Version:    "1.0",
			Resolution: 0.5,
			Bounds:     Bounds{MinLat: 40, MaxLat: 47, MinLon: 27, MaxLon: 42},
			Regions:    []string{"test1", "test2"},
		},
		Points: []LithologyPoint{
			{Lat: 45.0, Lon: 34.0, Lithology: "limestone", Resistance: 4.5, Color: "#6b6b6b", Confidence: "high"},
			{Lat: 44.0, Lon: 33.0, Lithology: "clay", Resistance: 1.2, Color: "#c4a484", Confidence: "medium"},
			{Lat: 46.0, Lon: 35.0, Lithology: "limestone", Resistance: 4.8, Color: "#6b6b6b", Confidence: "high"},
		},
		Classes: map[string]LithologyClass{
			"limestone": {Resistance: 4.5, Color: "#6b6b6b"},
			"clay":      {Resistance: 1.2, Color: "#c4a484"},
		},
	}

	stats := profile.GetStatistics()

	// Проверки
	if stats["name"] != "Test Stats" {
		t.Errorf("Ожидалось имя 'Test Stats', получено %v", stats["name"])
	}

	if stats["num_points"] != 3 {
		t.Errorf("Ожидалось 3 точки, получено %v", stats["num_points"])
	}

	if stats["num_classes"] != 2 {
		t.Errorf("Ожидалось 2 класса, получено %v", stats["num_classes"])
	}

	// Статистика сопротивления
	minR, ok1 := stats["resistance_min"].(float64)
	maxR, ok2 := stats["resistance_max"].(float64)
	meanR, ok3 := stats["resistance_mean"].(float64)

	if !ok1 || !ok2 || !ok3 {
		t.Error("Отсутствует статистика сопротивления")
	} else {
		if minR != 1.2 {
			t.Errorf("Ожидалось мин. сопротивление 1.2, получено %.2f", minR)
		}
		if maxR != 4.8 {
			t.Errorf("Ожидалось макс. сопротивление 4.8, получено %.2f", maxR)
		}
		expectedMean := (4.5 + 1.2 + 4.8) / 3.0
		if math.Abs(meanR-expectedMean) > 0.01 {
			t.Errorf("Ожидалось среднее сопротивление %.2f, получено %.2f", expectedMean, meanR)
		}
	}

	// Распределение уверенности
	confidenceDist, ok := stats["confidence_distribution"].(map[string]int)
	if !ok {
		t.Error("Отсутствует распределение уверенности")
	} else {
		if confidenceDist["high"] != 2 {
			t.Errorf("Ожидалось 2 точки с высокой уверенностью, получено %d", confidenceDist["high"])
		}
		if confidenceDist["medium"] != 1 {
			t.Errorf("Ожидалась 1 точка со средней уверенностью, получено %d", confidenceDist["medium"])
		}
	}

	t.Logf("✓ Статистика: %v", stats)
}

// TestCreateDefaultBlackSeaProfile тест создания дефолтного профиля
func TestCreateDefaultBlackSeaProfile(t *testing.T) {
	profile := CreateDefaultBlackSeaProfile()

	if profile == nil {
		t.Fatal("Ожидалось ненулевой профиль")
	}

	// Проверки
	if profile.Metadata.Name != "Литология Чёрного моря по умолчанию" {
		t.Errorf("Ожидалось имя 'Литология Чёрного моря по умолчанию', получено %s", profile.Metadata.Name)
	}

	if len(profile.Points) != 5 {
		t.Errorf("Ожидалось 5 точек, получено %d", len(profile.Points))
	}

	if len(profile.Classes) != 4 {
		t.Errorf("Ожидалось 4 класса, получено %d", len(profile.Classes))
	}

	if len(profile.Baselines) != 6 {
		t.Errorf("Ожидалось 6 базовых линий, получено %d", len(profile.Baselines))
	}

	// Проверка покрытия по регионам
	coveredRegions := make(map[string]bool)
	for _, point := range profile.Points {
		coveredRegions[point.Region] = true
	}

	expectedRegions := []string{"crimea", "turkey", "bulgaria", "romania", "georgia"}
	for _, region := range expectedRegions {
		if !coveredRegions[region] {
			t.Errorf("Отсутствует покрытие для региона: %s", region)
		}
	}

	// Проверка диапазона сопротивления
	minR := profile.Points[0].Resistance
	maxR := profile.Points[0].Resistance
	for _, point := range profile.Points {
		if point.Resistance < minR {
			minR = point.Resistance
		}
		if point.Resistance > maxR {
			maxR = point.Resistance
		}
	}

	t.Logf("✓ Дефолтный профиль создан: %d точек, диапазон сопротивления [%.1f, %.1f]",
		len(profile.Points), minR, maxR)
}

// BenchmarkGetLithologyAt бенчмарк для производительности
func BenchmarkGetLithologyAt(b *testing.B) {
	profile := &LithologyProfile{
		Metadata: LithologyMetadata{
			Name:   "Benchmark",
			Bounds: Bounds{MinLat: 40, MaxLat: 47, MinLon: 27, MaxLon: 42},
		},
		Points: make([]LithologyPoint, 100),
		Classes: map[string]LithologyClass{
			"limestone": {Resistance: 4.0, Color: "#6b6b6b"},
		},
	}

	// Заполняем точки
	for i := range profile.Points {
		profile.Points[i] = LithologyPoint{
			Lat:        40.0 + float64(i)*0.07,
			Lon:        27.0 + float64(i)*0.15,
			Lithology:  "limestone",
			Resistance: 4.0,
			Color:      "#6b6b6b",
		}
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		// Случайные координаты в пределах bounds
		lat := 40.0 + (float64(i%70) * 0.1)
		lon := 27.0 + (float64(i/70) * 0.2)
		profile.GetLithologyAt(lat, lon)
	}
}

// TestApplyWeathering тест функции выветривания
func TestApplyWeathering(t *testing.T) {
	weathering := CreateDefaultWeatheringProfile()

	testCases := []struct {
		name            string
		resistance      float64
		years           float64
		wantWeathered   bool
		wantMinProgress float64
	}{
		{
			name:            "нет времени - нет выветривания",
			resistance:      4.0,
			years:           0,
			wantWeathered:   false,
			wantMinProgress: 0,
		},
		{
			name:            "короткое время - небольшое выветривание",
			resistance:      4.0,
			years:           10,
			wantWeathered:   false,
			wantMinProgress: 0,
		},
		{
			name:            "длительное время - значительное выветривание",
			resistance:      4.0,
			years:           500,
			wantWeathered:   true,
			wantMinProgress: 0.3,
		},
		{
			name:            "очень длительное время - сильное выветривание",
			resistance:      4.0,
			years:           2000,
			wantWeathered:   true,
			wantMinProgress: 0.8,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			state := LithologyState{
				Class:       "limestone",
				Resistance:  tc.resistance,
				Color:       "#6b6b6b",
				Description: "Test limestone",
			}

			dynamic := ApplyWeathering(state, tc.years, weathering, 1.0)

			if tc.wantWeathered && !dynamic.IsWeathered {
				t.Errorf("Ожидалось выветрелое состояние после %.1f лет", tc.years)
			}

			if dynamic.WeatheringProgress < tc.wantMinProgress {
				t.Errorf("Ожидался прогресс >= %.2f, получено %.2f",
					tc.wantMinProgress, dynamic.WeatheringProgress)
			}

			// Сопротивление должно уменьшиться
			if tc.years > 0 && dynamic.CurrentResistance >= state.Resistance {
				t.Errorf("Сопротивление должно уменьшаться: было %.2f, теперь %.2f",
					state.Resistance, dynamic.CurrentResistance)
			}

			t.Logf("✓ %s: R %.2f → %.2f (прогресс: %.2f)",
				tc.name, state.Resistance, dynamic.CurrentResistance, dynamic.WeatheringProgress)
		})
	}
}

// TestCalculateLithologyErosionInteraction тест взаимодействия литологии и эрозии
func TestCalculateLithologyErosionInteraction(t *testing.T) {
	params := CreateDefaultLithologyInteractionParams()

	testCases := []struct {
		name           string
		baseErosion    float64
		resistance     float64
		weathered      bool
		weatheringProg float64
		isStorm        bool
	}{
		{
			name:           "высокое сопротивление, нормальные условия",
			baseErosion:    5.0,
			resistance:     8.0,
			weathered:      false,
			weatheringProg: 0,
			isStorm:        false,
		},
		{
			name:           "низкое сопротивление, выветрелая",
			baseErosion:    5.0,
			resistance:     1.0,
			weathered:      true,
			weatheringProg: 0.7,
			isStorm:        false,
		},
		{
			name:           "штормовые условия",
			baseErosion:    5.0,
			resistance:     4.0,
			weathered:      false,
			weatheringProg: 0,
			isStorm:        true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			state := DynamicLithologyState{
				Static: LithologyState{
					Class:       "limestone",
					Resistance:  tc.resistance,
					Color:       "#6b6b6b",
					Description: "Test",
				},
				CurrentResistance:  tc.resistance,
				WeatheringProgress: tc.weatheringProg,
				IsWeathered:        tc.weathered,
				FractureDensity:    0.2,
				Porosity:           0.1,
			}

			modified := CalculateLithologyErosionInteraction(
				tc.baseErosion,
				state,
				params,
				tc.isStorm,
			)

			// Шторм должен увеличивать эрозию
			if tc.isStorm && modified <= tc.baseErosion {
				t.Errorf("Шторм должен увеличивать эрозию: база=%.2f, модифицировано=%.2f",
					tc.baseErosion, modified)
			}

			// Выветривание должно увеличивать эрозию
			if tc.weathered && modified <= tc.baseErosion*0.9 {
				t.Errorf("Выветрелая порода должна эродировать быстрее: база=%.2f, модифицировано=%.2f",
					tc.baseErosion, modified)
			}

			t.Logf("✓ %s: эрозия %.2f → %.2f", tc.name, tc.baseErosion, modified)
		})
	}
}

// TestCalculateLithologyDepositionInteraction тест взаимодействия литологии и аккумуляции
func TestCalculateLithologyDepositionInteraction(t *testing.T) {
	params := CreateDefaultLithologyInteractionParams()

	testCases := []struct {
		name        string
		baseDep     float64
		porosity    float64
		fractureDen float64
		isWeathered bool
	}{
		{
			name:        "низкая пористость",
			baseDep:     1.0,
			porosity:    0.05,
			fractureDen: 0.1,
			isWeathered: false,
		},
		{
			name:        "высокая пористость, выветрелая",
			baseDep:     1.0,
			porosity:    0.3,
			fractureDen: 0.5,
			isWeathered: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			state := DynamicLithologyState{
				Static: LithologyState{
					Class:       "limestone",
					Resistance:  4.0,
					Color:       "#6b6b6b",
					Description: "Test",
				},
				CurrentResistance: 4.0,
				Porosity:          tc.porosity,
				FractureDensity:   tc.fractureDen,
				IsWeathered:       tc.isWeathered,
			}

			modified := CalculateLithologyDepositionInteraction(
				tc.baseDep,
				state,
				params,
				2.0, // поставка наносов
			)

			// Результат должен быть положительным
			if modified < 0 {
				t.Errorf("Аккумуляция не может быть отрицательной: %.2f", modified)
			}

			t.Logf("✓ %s: аккумуляция %.2f → %.2f", tc.name, tc.baseDep, modified)
		})
	}
}

// TestGenerateSpatialLithologyVariability тест пространственной вариабельности
func TestGenerateSpatialLithologyVariability(t *testing.T) {
	basePoints := []LithologyState{
		{Class: "limestone", Resistance: 4.0, Color: "#6b6b6b", Description: "Limestone"},
		{Class: "clay", Resistance: 1.5, Color: "#c4a484", Description: "Clay"},
		{Class: "granite", Resistance: 7.0, Color: "#4a4a4a", Description: "Granite"},
		{Class: "sandstone", Resistance: 3.0, Color: "#8b8b8b", Description: "Sandstone"},
	}

	bounds := Bounds{MinLat: 44, MaxLat: 46, MinLon: 33, MaxLon: 35}
	params := CreateDefaultLithologyInteractionParams()
	weathering := CreateDefaultWeatheringProfile()

	result := GenerateSpatialLithologyVariability(basePoints, bounds, params, weathering)

	if len(result.Points) != len(basePoints) {
		t.Errorf("Ожидалось %d точек, получено %d", len(basePoints), len(result.Points))
	}

	// Проверяем вариабельность
	variations := 0
	for i, point := range result.Points {
		if point.CurrentResistance != basePoints[i].Resistance {
			variations++
		}
	}

	if variations == 0 {
		t.Error("Ожидалась некоторая пространственная вариабельность")
	}

	t.Logf("✓ Сгенерировано %d точек с %d вариациями", len(result.Points), variations)
}

// TestUpdateLithologyAfterErosion тест обратной связи от эрозии
func TestUpdateLithologyAfterErosion(t *testing.T) {
	params := CreateDefaultLithologyInteractionParams()

	initialStates := []DynamicLithologyState{
		{
			Static: LithologyState{
				Class:       "limestone",
				Resistance:  4.0,
				Color:       "#6b6b6b",
				Description: "Test",
			},
			CurrentResistance: 4.0,
			FractureDensity:   0.1,
			Porosity:          0.05,
		},
	}

	erosionAmounts := []float64{5.0} // 5 метров эрозии

	updated := UpdateLithologyAfterErosion(initialStates, erosionAmounts, params)

	// Проверяем изменения
	if updated[0].FractureDensity <= initialStates[0].FractureDensity {
		t.Errorf("Плотность трещин должна увеличиваться: было %.2f, теперь %.2f",
			initialStates[0].FractureDensity, updated[0].FractureDensity)
	}

	if updated[0].Porosity <= initialStates[0].Porosity {
		t.Errorf("Пористость должна увеличиваться: было %.2f, теперь %.2f",
			initialStates[0].Porosity, updated[0].Porosity)
	}

	if updated[0].CurrentResistance >= initialStates[0].CurrentResistance {
		t.Errorf("Сопротивление должно уменьшаться: было %.2f, теперь %.2f",
			initialStates[0].CurrentResistance, updated[0].CurrentResistance)
	}

	if len(updated[0].ModificationHistory) == 0 {
		t.Error("Ожидалась запись в истории модификаций")
	}

	t.Logf("✓ Обратная связь от эрозии: R %.2f → %.2f, F %.2f → %.2f",
		initialStates[0].CurrentResistance, updated[0].CurrentResistance,
		initialStates[0].FractureDensity, updated[0].FractureDensity)
}

// TestUpdateLithologyAfterDeposition тест обратной связи от аккумуляции
func TestUpdateLithologyAfterDeposition(t *testing.T) {
	params := CreateDefaultLithologyInteractionParams()

	initialStates := []DynamicLithologyState{
		{
			Static: LithologyState{
				Class:       "limestone",
				Resistance:  4.0,
				Color:       "#6b6b6b",
				Description: "Test",
			},
			CurrentResistance: 4.0,
			FractureDensity:   0.5,
			Porosity:          0.3,
			Thickness:         2.0,
		},
	}

	depositionAmounts := []float64{2.0} // 2 метра аккумуляции

	updated := UpdateLithologyAfterDeposition(initialStates, depositionAmounts, params)

	// Толщина должна увеличиться
	if updated[0].Thickness <= initialStates[0].Thickness {
		t.Errorf("Толщина должна увеличиваться: было %.2f, теперь %.2f",
			initialStates[0].Thickness, updated[0].Thickness)
	}

	// Залечивание трещин при значительной аккумуляции
	if updated[0].FractureDensity >= initialStates[0].FractureDensity {
		t.Logf("Примечание: Плотность трещин не залечена: %.2f → %.2f",
			initialStates[0].FractureDensity, updated[0].FractureDensity)
	}

	t.Logf("✓ Обратная связь от аккумуляции: толщина %.2f → %.2f",
		initialStates[0].Thickness, updated[0].Thickness)
}

// TestSimulateErosionWithLithologyFeedback тест полной симуляции
func TestSimulateErosionWithLithologyFeedback(t *testing.T) {
	points := []LatLon{
		{Lat: 45.0, Lon: 34.0},
		{Lat: 45.1, Lon: 34.1},
		{Lat: 45.2, Lon: 34.2},
	}

	baseErosion := []float64{3.0, 2.5, 4.0}

	initialStates := []DynamicLithologyState{
		{Static: LithologyState{Class: "limestone", Resistance: 4.0, Color: "#6b6b6b", Description: "Test"}, CurrentResistance: 4.0, FractureDensity: 0.1, Porosity: 0.05},
		{Static: LithologyState{Class: "clay", Resistance: 1.5, Color: "#c4a484", Description: "Test"}, CurrentResistance: 1.5, FractureDensity: 0.2, Porosity: 0.1},
		{Static: LithologyState{Class: "granite", Resistance: 7.0, Color: "#4a4a4a", Description: "Test"}, CurrentResistance: 7.0, FractureDensity: 0.05, Porosity: 0.02},
	}

	params := CreateDefaultLithologyInteractionParams()

	result := SimulateErosionWithLithologyFeedback(
		points,
		baseErosion,
		initialStates,
		params,
		false, // не шторм
		0,     // интенсивность шторма
	)

	if len(result.ModifiedErosion) != len(baseErosion) {
		t.Errorf("Ожидалось %d значений эрозии, получено %d", len(baseErosion), len(result.ModifiedErosion))
	}

	if !result.FeedbackApplied {
		t.Error("Ожидалось применение обратной связи")
	}

	// Проверяем, что сопротивление изменилось
	changed := false
	for i := range result.ResistanceBefore {
		if result.ResistanceBefore[i] != result.ResistanceAfter[i] {
			changed = true
			break
		}
	}

	if !changed {
		t.Error("Ожидалось изменение сопротивления после эрозии")
	}

	t.Logf("✓ Полная симуляция: %d точек, обратная связь применена",
		len(result.ModifiedErosion))
}

// TestGetLithologyStatistics тест статистики динамической литологии
func TestGetLithologyStatistics(t *testing.T) {
	states := []DynamicLithologyState{
		{
			Static:             LithologyState{Class: "limestone", Resistance: 4.0, Color: "#6b6b6b", Description: "Test"},
			CurrentResistance:  4.0,
			IsWeathered:        false,
			WeatheringProgress: 0.1,
			FractureDensity:    0.1,
			Porosity:           0.05,
		},
		{
			Static:             LithologyState{Class: "clay", Resistance: 1.5, Color: "#c4a484", Description: "Test"},
			CurrentResistance:  1.2,
			IsWeathered:        true,
			WeatheringProgress: 0.6,
			FractureDensity:    0.4,
			Porosity:           0.2,
		},
		{
			Static:             LithologyState{Class: "granite", Resistance: 7.0, Color: "#4a4a4a", Description: "Test"},
			CurrentResistance:  6.5,
			IsWeathered:        true,
			WeatheringProgress: 0.3,
			FractureDensity:    0.2,
			Porosity:           0.1,
		},
	}

	stats := GetLithologyStatistics(states)

	totalPoints, ok := stats["total_points"].(int)
	if !ok || totalPoints != 3 {
		t.Errorf("Ожидалось 3 точки всего, получено %v", totalPoints)
	}

	weatheredFrac, ok := stats["weathered_fraction"].(float64)
	if !ok || weatheredFrac != 2.0/3.0 {
		t.Errorf("Ожидалась доля выветрелых 0.67, получено %.2f", weatheredFrac)
	}

	minR, ok := stats["resistance_min"].(float64)
	if !ok || minR != 1.2 {
		t.Errorf("Ожидалось мин. сопротивление 1.2, получено %.2f", minR)
	}

	t.Logf("✓ Статистика: %v", stats)
}

// TestLithologyInteractionParamsValidation тест валидации параметров
func TestLithologyInteractionParamsValidation(t *testing.T) {
	testCases := []struct {
		name    string
		params  LithologyInteractionParams
		wantErr bool
	}{
		{
			name:    "валидные параметры",
			params:  CreateDefaultLithologyInteractionParams(),
			wantErr: false,
		},
		{
			name: "неверный коэффициент эрозии",
			params: LithologyInteractionParams{
				ErosionResistanceFactor: 1.5, // > 1
			},
			wantErr: true,
		},
		{
			name: "неверный множитель шторма",
			params: LithologyInteractionParams{
				StormFractureMultiplier: 15, // > 10
			},
			wantErr: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.params.Validate()
			if tc.wantErr && err == nil {
				t.Error("Ожидалась ошибка, но не получено")
			}
			if !tc.wantErr && err != nil {
				t.Errorf("Ожидалось отсутствие ошибки, но получено: %v", err)
			}
		})
	}
}

func TestLoadLithologyProfileFromFile(t *testing.T) {
	// Создаем временный файл с валидными данными
	tempDir := t.TempDir()
	tempFile := filepath.Join(tempDir, "test_lithology.json")

	validJSON := `{
		"metadata": {
			"name": "Test Profile",
			"version": "1.0",
			"resolution": 0.5,
			"bounds": {
				"min_lat": 44.0,
				"max_lat": 46.0,
				"min_lon": 33.0,
				"max_lon": 35.0
			}
		},
		"points": [
			{
				"lat": 45.0,
				"lon": 34.0,
				"region": "crimea",
				"lithology_class": "limestone",
				"resistance": 4.5,
				"color": "#6b6b6b",
				"description": "Test limestone",
				"confidence": "high"
			}
		],
		"classes": {
			"limestone": {
				"resistance": 4.5,
				"color": "#6b6b6b",
				"description": "Limestone test"
			}
		}
	}`

	err := os.WriteFile(tempFile, []byte(validJSON), 0644)
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}

	// Тест успешной загрузки
	profile, err := LoadLithologyProfileFromFile(tempFile)
	if err != nil {
		t.Errorf("LoadLithologyProfileFromFile() error = %v", err)
		return
	}

	if profile == nil {
		t.Error("LoadLithologyProfileFromFile() returned nil profile")
		return
	}

	if profile.Metadata.Name != "Test Profile" {
		t.Errorf("Expected name 'Test Profile', got '%s'", profile.Metadata.Name)
	}

	// Тест несуществующего файла
	_, err = LoadLithologyProfileFromFile("/nonexistent/file.json")
	if err == nil {
		t.Error("LoadLithologyProfileFromFile() expected error for nonexistent file, got nil")
	}
}

func TestBatchGetLithologyAt(t *testing.T) {
	profile := CreateDefaultBlackSeaProfile()

	// Подготавливаем координаты в правильном формате
	points := []struct {
		Lat, Lon float64
	}{
		{Lat: 45.0, Lon: 34.0}, // Crimea area - limestone
		{Lat: 44.5, Lon: 33.5}, // Different area - clay
		{Lat: 45.5, Lon: 35.0}, // Another area
	}

	ctx := context.Background()

	// Тест с пакетным запросом
	results := profile.BatchGetLithologyAt(ctx, points)

	if len(results) != len(points) {
		t.Errorf("BatchGetLithologyAt() returned %d results, want %d", len(results), len(points))
	}

	// Проверяем, что все результаты валидны
	for i, result := range results {
		if result.Class == "" {
			t.Errorf("Result %d has empty lithology class", i)
		}
		if result.Resistance <= 0 {
			t.Errorf("Result %d has invalid resistance %f", i, result.Resistance)
		}
	}

	// Проверяем, что результаты совпадают с индивидуальными запросами
	for i, point := range points {
		individual := profile.GetLithologyAt(point.Lat, point.Lon)
		if individual.Class != results[i].Class {
			t.Errorf("Batch result differs from individual at index %d: batch=%s, individual=%s",
				i, results[i].Class, individual.Class)
		}
	}
}

func TestGetLithologyAtParallel(t *testing.T) {
	profile := CreateDefaultBlackSeaProfile()

	ctx := context.Background()

	// Тестируем несколько точек параллельно
	testCases := []struct {
		lat       float64
		lon       float64
		wantClass string
	}{
		{lat: 45.0, lon: 34.0, wantClass: "limestone"}, // Crimea area
		{lat: 44.5, lon: 33.5, wantClass: "clay"},      // Different area
		{lat: 45.5, lon: 35.0, wantClass: "limestone"}, // Another area
	}

	for _, tc := range testCases {
		t.Run(fmt.Sprintf("(%.2f, %.2f)", tc.lat, tc.lon), func(t *testing.T) {
			state := profile.GetLithologyAtParallel(ctx, tc.lat, tc.lon)

			if state.Class == "" {
				t.Error("GetLithologyAtParallel() returned empty class")
			}
			if state.Resistance <= 0 {
				t.Error("GetLithologyAtParallel() returned invalid resistance")
			}
		})
	}
}
