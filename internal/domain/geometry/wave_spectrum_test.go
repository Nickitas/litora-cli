package geometry

import (
	"math"
	"testing"
)

func TestWaveSpectrumNormalize(t *testing.T) {
	s := WaveSpectrum{
		Bins: []WaveSpectrumBin{
			{Direction: 0, Weight: 2.0},
			{Direction: 90, Weight: 2.0},
			{Direction: 180, Weight: 4.0},
		},
	}
	s.Normalize()

	var sum float64
	for _, b := range s.Bins {
		sum += b.Weight
	}
	if math.Abs(sum-1.0) > 1e-9 {
		t.Errorf("normalized weights sum to %f, want 1.0", sum)
	}
	if s.Bins[0].Weight != 0.25 {
		t.Errorf("first bin weight = %f, want 0.25", s.Bins[0].Weight)
	}
	if s.Bins[2].Weight != 0.5 {
		t.Errorf("third bin weight = %f, want 0.5", s.Bins[2].Weight)
	}
}

func TestSingleDirectionSpectrum(t *testing.T) {
	s := NewSingleDirectionSpectrum(90)
	if got := s.DominantDirection(); got != 90 {
		t.Errorf("DominantDirection = %v, want 90", got)
	}
	if got := s.MeanDirection(); got != 90 {
		t.Errorf("MeanDirection = %v, want 90", got)
	}
	if got := s.Spread(); got > 1e-9 {
		t.Errorf("Spread = %v, want 0", got)
	}
}

func TestBimodalSpectrumMean(t *testing.T) {
	// 0° and 180° with equal weights should give circular mean ~90 or ~270
	// (mathematically undefined, but our implementation will return some value)
	s := NewBimodalSpectrum(0, 180, 1, 1)
	spread := s.Spread()
	if spread < 50 {
		t.Errorf("Spread of opposite directions = %v, expected > 50", spread)
	}
}

func TestGaussianSpectrum(t *testing.T) {
	s := NewGaussianSpectrum(90, 30, 16)
	if got := s.MeanDirection(); math.Abs(got-90) > 5 {
		t.Errorf("MeanDirection = %v, want ~90", got)
	}
	if got := s.DominantDirection(); got != 90 {
		t.Errorf("DominantDirection = %v, want 90", got)
	}
	spread := s.Spread()
	if math.Abs(spread-30) > 10 {
		t.Errorf("Spread = %v, want ~30", spread)
	}
}

func TestGaussianSpectrumDegenerate(t *testing.T) {
	// stdDev = 0 should reduce to single direction
	s := NewGaussianSpectrum(45, 0, 8)
	if len(s.Bins) != 1 {
		t.Errorf("expected 1 bin, got %d", len(s.Bins))
	}
	if s.Bins[0].Direction != 45 {
		t.Errorf("direction = %v, want 45", s.Bins[0].Direction)
	}
}

func TestSpectrumWeightsSumToOne(t *testing.T) {
	s := NewGaussianSpectrum(180, 45, 8)
	var sum float64
	for _, b := range s.Bins {
		sum += b.Weight
	}
	if math.Abs(sum-1.0) > 1e-9 {
		t.Errorf("weights sum to %f, want 1.0", sum)
	}
}
