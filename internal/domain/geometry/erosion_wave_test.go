package geometry

import "testing"

// TestSimulateWaveErosionRetreatsOpenHeadlandMoreThanShelteredBay проверяет физически корректное поведение
func TestSimulateWaveErosionRetreatsOpenHeadlandMoreThanShelteredBay(t *testing.T) {
	points := []LatLon{
		{Lat: -300 / metersPerDegLat, Lon: -300 / metersPerDegLat},
		{Lat: -300 / metersPerDegLat, Lon: 300 / metersPerDegLat},
		{Lat: 300 / metersPerDegLat, Lon: 300 / metersPerDegLat},
		{Lat: 300 / metersPerDegLat, Lon: 220 / metersPerDegLat},
		{Lat: 360 / metersPerDegLat, Lon: 220 / metersPerDegLat},
		{Lat: 360 / metersPerDegLat, Lon: 180 / metersPerDegLat},
		{Lat: 360 / metersPerDegLat, Lon: 140 / metersPerDegLat},
		{Lat: 300 / metersPerDegLat, Lon: 140 / metersPerDegLat},
		{Lat: 300 / metersPerDegLat, Lon: 80 / metersPerDegLat},
		{Lat: 80 / metersPerDegLat, Lon: 80 / metersPerDegLat},
		{Lat: 80 / metersPerDegLat, Lon: 0},
		{Lat: 80 / metersPerDegLat, Lon: -80 / metersPerDegLat},
		{Lat: 300 / metersPerDegLat, Lon: -80 / metersPerDegLat},
		{Lat: 300 / metersPerDegLat, Lon: -300 / metersPerDegLat},
		{Lat: -300 / metersPerDegLat, Lon: -300 / metersPerDegLat},
	}

	options := WaveErosionOptions{
		StrengthMeters:           45,
		WindSourceDirectionDeg:   0,
		WindSpeedMetersPerSecond: 14,
		FetchSpreadDeg:           55,
		FetchSamples:             9,
		MaxFetchMeters:           1200,
		DepthScaleMeters:         250,
		ExposurePower:            1.5,
	}

	snapshots := SimulateWaveErosionWithSeed(points, 1, options, 42)
	eroded := snapshots[1]

	headlandIndex := 5
	bayIndex := 10

	headlandMove := Haversine(points[headlandIndex], eroded[headlandIndex]) * 1000
	bayMove := Haversine(points[bayIndex], eroded[bayIndex]) * 1000

	if headlandMove <= bayMove {
		t.Fatalf("ожидалось, что отступ открытого мыса (%.2f м) превысит отступ защищённой бухты (%.2f м)", headlandMove, bayMove)
	}
}

// TestSimulateWaveErosionPreservesClosedRing проверяет сохранение замкнутого контура
func TestSimulateWaveErosionPreservesClosedRing(t *testing.T) {
	points := []LatLon{
		{Lat: 0, Lon: 0},
		{Lat: 0, Lon: 0.01},
		{Lat: 0.01, Lon: 0.01},
		{Lat: 0.01, Lon: 0},
		{Lat: 0, Lon: 0},
	}

	options := WaveErosionOptions{
		StrengthMeters:           20,
		WindSourceDirectionDeg:   90,
		WindSpeedMetersPerSecond: 10,
		FetchSpreadDeg:           45,
		FetchSamples:             7,
		MaxFetchMeters:           5000,
		DepthScaleMeters:         1000,
		ExposurePower:            1.2,
	}

	snapshots := SimulateWaveErosionWithSeed(points, 2, options, 7)
	last := snapshots[len(snapshots)-1]
	if last[0] != last[len(last)-1] {
		t.Fatalf("ожидалось, что замкнутый контур останется замкнутым, получено: первая=%+v последняя=%+v", last[0], last[len(last)-1])
	}
}

func TestSimulateWaveErosion(t *testing.T) {
	points := []LatLon{
		{Lat: 45, Lon: 30},
		{Lat: 45, Lon: 31},
		{Lat: 46, Lon: 31},
		{Lat: 46, Lon: 30},
	}

	options := WaveErosionOptions{
		StrengthMeters:           1000.0,
		WindSourceDirectionDeg:   45.0,
		WindSpeedMetersPerSecond: 10.0,
		FetchSpreadDeg:           45.0,
		FetchSamples:             12,
		MaxFetchMeters:           50000.0,
		DepthScaleMeters:         20.0,
		ExposurePower:            2.0,
		MaxRetreatMeters:         100.0,
		ProbeDistanceMeters:      5000.0,
		Irregularity:             0.1,
	}

	tests := []struct {
		name    string
		points  []LatLon
		steps   int
		options WaveErosionOptions
		wantLen int
	}{
		{
			name:    "один шаг волновой эрозии",
			points:  points,
			steps:   1,
			options: options,
			wantLen: 2, // начальное состояние + 1 шаг
		},
		{
			name:    "несколько шагов",
			points:  points,
			steps:   3,
			options: options,
			wantLen: 4, // начальное состояние + 3 шага
		},
		{
			name:    "ноль шагов",
			points:  points,
			steps:   0,
			options: options,
			wantLen: 1, // только начальное состояние
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			snapshots := SimulateWaveErosion(tt.points, tt.steps, tt.options)

			if len(snapshots) != tt.wantLen {
				t.Errorf("SimulateWaveErosion() returned %d snapshots, want %d", len(snapshots), tt.wantLen)
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
