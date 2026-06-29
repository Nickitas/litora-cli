package geometry

import "testing"

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
		t.Fatalf("expected open headland retreat %.2f m to exceed sheltered bay retreat %.2f m", headlandMove, bayMove)
	}
}

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
		t.Fatalf("expected closed ring to remain closed, got first=%+v last=%+v", last[0], last[len(last)-1])
	}
}
