package geometry

import (
	"encoding/json"
	"math"
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
		t.Fatalf("Failed to marshal test profile: %v", err)
	}

	// Загружаем обратно
	loaded, err := LoadLithologyProfile(data)
	if err != nil {
		t.Fatalf("Failed to load profile: %v", err)
	}

	// Проверки
	if loaded.Metadata.Name != testProfile.Metadata.Name {
		t.Errorf("Expected name %s, got %s", testProfile.Metadata.Name, loaded.Metadata.Name)
	}

	if len(loaded.Points) != len(testProfile.Points) {
		t.Errorf("Expected %d points, got %d", len(testProfile.Points), len(loaded.Points))
	}

	if len(loaded.Classes) != len(testProfile.Classes) {
		t.Errorf("Expected %d classes, got %d", len(testProfile.Classes), len(loaded.Classes))
	}

	t.Logf("✓ Profile loaded successfully: %s (%d points, %d classes)",
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
			name: "valid profile",
			profile: LithologyProfile{
				Metadata: LithologyMetadata{
					Name: "Valid",
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
			name: "invalid latitude bounds",
			profile: LithologyProfile{
				Metadata: LithologyMetadata{
					Name: "Invalid",
					Bounds: Bounds{MinLat: 47, MaxLat: 40, MinLon: 27, MaxLon: 42},
				},
				Points: []LithologyPoint{},
			},
			wantErr: true,
		},
		{
			name: "invalid point coordinates",
			profile: LithologyProfile{
				Metadata: LithologyMetadata{
					Name: "Invalid",
					Bounds: Bounds{MinLat: 40, MaxLat: 47, MinLon: 27, MaxLon: 42},
				},
				Points: []LithologyPoint{
					{Lat: 100, Lon: 34, Lithology: "limestone", Resistance: 4.0, Color: "#6b6b6b"},
				},
			},
			wantErr: true,
		},
		{
			name: "invalid resistance",
			profile: LithologyProfile{
				Metadata: LithologyMetadata{
					Name: "Invalid",
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
				t.Error("Expected error but got none")
			}
			if !tc.wantErr && err != nil {
				t.Errorf("Expected no error but got: %v", err)
			}
		})
	}
}

// TestGetLithologyAt тест получения литологии
func TestGetLithologyAt(t *testing.T) {
	// Создаём профиль с известными точками
	profile := &LithologyProfile{
		Metadata: LithologyMetadata{
			Name: "Test",
			Bounds: Bounds{MinLat: 44, MaxLat: 46, MinLon: 33, MaxLon: 35},
		},
		Points: []LithologyPoint{
			{Lat: 45.0, Lon: 34.0, Lithology: "limestone", Resistance: 4.5, Color: "#6b6b6b"},
			{Lat: 44.5, Lon: 33.5, Lithology: "clay", Resistance: 1.2, Color: "#c4a484"},
		},
		Classes: map[string]LithologyClass{
			"limestone": {Resistance: 4.5, Color: "#6b6b6b", Description: "Limestone"},
			"clay":     {Resistance: 1.2, Color: "#c4a484", Description: "Clay"},
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
			name:      "close to limestone point",
			lat:       45.0,
			lon:       34.0,
			wantClass: "limestone",
			wantR:     4.5,
		},
		{
			name:      "close to clay point",
			lat:       44.5,
			lon:       33.5,
			wantClass: "clay",
			wantR:     1.2,
		},
		{
			name:      "midpoint between",
			lat:       44.75,
			lon:       33.75,
			wantClass: "limestone", // ближайшая точка (доминирующий вес)
			wantR:     2.85, // IDW интерполяция: (4.5 + 1.2) / 2 ≈ 2.85
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			state := profile.GetLithologyAt(tc.lat, tc.lon)

			if state.Class != tc.wantClass {
				t.Errorf("Expected class %s, got %s", tc.wantClass, state.Class)
			}

			if state.Resistance != tc.wantR {
				t.Errorf("Expected resistance %.2f, got %.2f", tc.wantR, state.Resistance)
			}

			t.Logf("✓ (%.2f, %.2f) → %s (R=%.2f)", tc.lat, tc.lon, state.Class, state.Resistance)
		})
	}
}

// TestGetLithologyAtOutOfBounds тест граничных условий
func TestGetLithologyAtOutOfBounds(t *testing.T) {
	profile := &LithologyProfile{
		Metadata: LithologyMetadata{
			Name: "Test",
			Bounds: Bounds{MinLat: 44, MaxLat: 46, MinLon: 33, MaxLon: 35},
		},
		Points:     []LithologyPoint{},
		Classes:    map[string]LithologyClass{},
	}

	testCases := []struct {
		name  string
		lat   float64
		lon   float64
		valid bool
	}{
		{name: "within bounds", lat: 45.0, lon: 34.0, valid: true},
		{name: "outside bounds - south", lat: 43.0, lon: 34.0, valid: false},
		{name: "outside bounds - north", lat: 47.0, lon: 34.0, valid: false},
		{name: "outside bounds - west", lat: 45.0, lon: 32.0, valid: false},
		{name: "outside bounds - east", lat: 45.0, lon: 36.0, valid: false},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			state := profile.GetLithologyAt(tc.lat, tc.lon)

			// Для out of bounds должен возвращаться default
			if !tc.valid {
				if state.Class == "" {
					t.Error("Expected default lithology for out of bounds, got empty class")
				}
				if state.Resistance <= 0 {
					t.Error("Expected positive resistance for default lithology")
				}
				t.Logf("✓ Out of bounds (%.2f, %.2f) → default %s (R=%.2f)",
					tc.lat, tc.lon, state.Class, state.Resistance)
			} else {
				if state.Class == "" {
					t.Error("Expected some lithology within bounds")
				}
				t.Logf("✓ Within bounds (%.2f, %.2f) → %s (R=%.2f)",
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
			Bounds: Bounds{MinLat: 40, MaxLat: 47, MinLon: 27, MaxLon: 42},
			Regions:    []string{"test1", "test2"},
		},
		Points: []LithologyPoint{
			{Lat: 45.0, Lon: 34.0, Lithology: "limestone", Resistance: 4.5, Color: "#6b6b6b", Confidence: "high"},
			{Lat: 44.0, Lon: 33.0, Lithology: "clay", Resistance: 1.2, Color: "#c4a484", Confidence: "medium"},
			{Lat: 46.0, Lon: 35.0, Lithology: "limestone", Resistance: 4.8, Color: "#6b6b6b", Confidence: "high"},
		},
		Classes: map[string]LithologyClass{
			"limestone": {Resistance: 4.5, Color: "#6b6b6b"},
			"clay":     {Resistance: 1.2, Color: "#c4a484"},
		},
	}

	stats := profile.GetStatistics()

	// Проверки
	if stats["name"] != "Test Stats" {
		t.Errorf("Expected name 'Test Stats', got %v", stats["name"])
	}

	if stats["num_points"] != 3 {
		t.Errorf("Expected 3 points, got %v", stats["num_points"])
	}

	if stats["num_classes"] != 2 {
		t.Errorf("Expected 2 classes, got %v", stats["num_classes"])
	}

	// Resistance statistics
	minR, ok1 := stats["resistance_min"].(float64)
	maxR, ok2 := stats["resistance_max"].(float64)
	meanR, ok3 := stats["resistance_mean"].(float64)

	if !ok1 || !ok2 || !ok3 {
		t.Error("Missing resistance statistics")
	} else {
		if minR != 1.2 {
			t.Errorf("Expected min resistance 1.2, got %.2f", minR)
		}
		if maxR != 4.8 {
			t.Errorf("Expected max resistance 4.8, got %.2f", maxR)
		}
		expectedMean := (4.5 + 1.2 + 4.8) / 3.0
		if math.Abs(meanR-expectedMean) > 0.01 {
			t.Errorf("Expected mean resistance %.2f, got %.2f", expectedMean, meanR)
		}
	}

	// Confidence distribution
	confidenceDist, ok := stats["confidence_distribution"].(map[string]int)
	if !ok {
		t.Error("Missing confidence distribution")
	} else {
		if confidenceDist["high"] != 2 {
			t.Errorf("Expected 2 high confidence points, got %d", confidenceDist["high"])
		}
		if confidenceDist["medium"] != 1 {
			t.Errorf("Expected 1 medium confidence point, got %d", confidenceDist["medium"])
		}
	}

	t.Logf("✓ Statistics: %v", stats)
}

// TestCreateDefaultBlackSeaProfile тест создания дефолтного профиля
func TestCreateDefaultBlackSeaProfile(t *testing.T) {
	profile := CreateDefaultBlackSeaProfile()

	if profile == nil {
		t.Fatal("Expected non-nil profile")
	}

	// Проверки
	if profile.Metadata.Name != "Default Black Sea Lithology" {
		t.Errorf("Expected name 'Default Black Sea Lithology', got %s", profile.Metadata.Name)
	}

	if len(profile.Points) != 5 {
		t.Errorf("Expected 5 points, got %d", len(profile.Points))
	}

	if len(profile.Classes) != 4 {
		t.Errorf("Expected 4 classes, got %d", len(profile.Classes))
	}

	if len(profile.Baselines) != 6 {
		t.Errorf("Expected 6 baselines, got %d", len(profile.Baselines))
	}

	// Проверка coverage по регионам
	coveredRegions := make(map[string]bool)
	for _, point := range profile.Points {
		coveredRegions[point.Region] = true
	}

	expectedRegions := []string{"crimea", "turkey", "bulgaria", "romania", "georgia"}
	for _, region := range expectedRegions {
		if !coveredRegions[region] {
			t.Errorf("Missing coverage for region: %s", region)
		}
	}

	// Проверка range resistance
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

	t.Logf("✓ Default profile created: %d points, resistance range [%.1f, %.1f]",
		len(profile.Points), minR, maxR)
}

// BenchmarkGetLithologyAt бенчмарк для производительности
func BenchmarkGetLithologyAt(b *testing.B) {
	profile := &LithologyProfile{
		Metadata: LithologyMetadata{
			Name: "Benchmark",
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
		lat := 40.0 + (float64(i%70)*0.1)
		lon := 27.0 + (float64(i/70)*0.2)
		profile.GetLithologyAt(lat, lon)
	}
}