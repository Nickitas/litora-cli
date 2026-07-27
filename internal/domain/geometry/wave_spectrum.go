package geometry

import "math"

// WaveSpectrumBin represents one direction component of a wave spectrum
type WaveSpectrumBin struct {
	// Direction the waves come FROM, in degrees clockwise from north
	Direction float64 `json:"direction"`
	// Weight is the relative energy contribution (0-1); typically normalized across bins
	Weight float64 `json:"weight"`
	// SignificantWaveHeightM is the Hs for this direction (m), optional
	// If 0, uses WindSpeedMetersPerSecond to derive energy
	SignificantWaveHeightM float64 `json:"hs_m,omitempty"`
}

// WaveSpectrum represents a directional wave spectrum
// Replaces single WindSourceDirectionDeg with multiple components
type WaveSpectrum struct {
	Bins []WaveSpectrumBin `json:"bins"`
}

// Normalize ensures weights sum to 1
func (s *WaveSpectrum) Normalize() {
	if s == nil || len(s.Bins) == 0 {
		return
	}
	var sum float64
	for _, b := range s.Bins {
		sum += b.Weight
	}
	if sum == 0 {
		return
	}
	for i := range s.Bins {
		s.Bins[i].Weight /= sum
	}
}

// DominantDirection returns the direction with the highest weight
func (s *WaveSpectrum) DominantDirection() float64 {
	if s == nil || len(s.Bins) == 0 {
		return 0
	}
	best := s.Bins[0]
	for _, b := range s.Bins[1:] {
		if b.Weight > best.Weight {
			best = b
		}
	}
	return best.Direction
}

// MeanDirection returns the energy-weighted circular mean direction (deg from N)
func (s *WaveSpectrum) MeanDirection() float64 {
	if s == nil || len(s.Bins) == 0 {
		return 0
	}
	var sumX, sumY float64
	for _, b := range s.Bins {
		rad := b.Direction * math.Pi / 180
		// Direction waves come FROM, so use it directly
		sumX += b.Weight * math.Sin(rad)
		sumY += b.Weight * math.Cos(rad)
	}
	// atan2 returns angle of (sin, cos) which matches our convention
	deg := math.Atan2(sumX, sumY) * 180 / math.Pi
	if deg < 0 {
		deg += 360
	}
	return deg
}

// Spread returns the circular standard deviation in degrees
// (high = widely spread, low = unimodal)
func (s *WaveSpectrum) Spread() float64 {
	if s == nil || len(s.Bins) == 0 {
		return 0
	}
	meanRad := s.MeanDirection() * math.Pi / 180
	var sumSin, sumCos float64
	var totalWeight float64
	for _, b := range s.Bins {
		rad := b.Direction * math.Pi / 180
		diff := rad - meanRad
		sumSin += b.Weight * math.Sin(diff)
		sumCos += b.Weight * math.Cos(diff)
		totalWeight += b.Weight
	}
	if totalWeight == 0 {
		return 0
	}
	R := math.Sqrt(sumSin*sumSin+sumCos*sumCos) / totalWeight
	if R >= 1 {
		return 0
	}
	return math.Sqrt(-2*math.Log(R)) * 180 / math.Pi
}

// NewSingleDirectionSpectrum creates a spectrum with a single direction
// This is the legacy mode (equivalent to WindSourceDirectionDeg)
func NewSingleDirectionSpectrum(direction float64) WaveSpectrum {
	return WaveSpectrum{
		Bins: []WaveSpectrumBin{
			{Direction: direction, Weight: 1.0},
		},
	}
}

// NewBimodalSpectrum creates a spectrum with two dominant directions
// Example: NE and SW winds for Black Sea
func NewBimodalSpectrum(dir1, dir2, weight1, weight2 float64) WaveSpectrum {
	s := WaveSpectrum{
		Bins: []WaveSpectrumBin{
			{Direction: dir1, Weight: weight1},
			{Direction: dir2, Weight: weight2},
		},
	}
	s.Normalize()
	return s
}

// NewGaussianSpectrum creates a spectrum centered at meanDir with given std dev
// and n bins distributed evenly around the compass
func NewGaussianSpectrum(meanDir, stdDevDeg float64, n int) WaveSpectrum {
	if n < 1 {
		n = 8
	}
	if stdDevDeg <= 0 {
		return NewSingleDirectionSpectrum(meanDir)
	}

	bins := make([]WaveSpectrumBin, n)
	stepDeg := 360.0 / float64(n)
	for i := 0; i < n; i++ {
		dir := float64(i) * stepDeg
		// Circular distance
		diff := math.Mod(dir-meanDir+180, 360) - 180
		if diff < -180 {
			diff += 360
		}
		weight := math.Exp(-0.5 * diff * diff / (stdDevDeg * stdDevDeg))
		bins[i] = WaveSpectrumBin{Direction: dir, Weight: weight}
	}

	s := WaveSpectrum{Bins: bins}
	s.Normalize()
	return s
}

// NewDirectionalSpectrumFromWeights creates a spectrum from explicit weights
// dirs and weights must have the same length
func NewDirectionalSpectrumFromWeights(dirs, weights []float64) WaveSpectrum {
	if len(dirs) != len(weights) {
		return WaveSpectrum{}
	}
	bins := make([]WaveSpectrumBin, len(dirs))
	for i := range dirs {
		bins[i] = WaveSpectrumBin{Direction: dirs[i], Weight: weights[i]}
	}
	s := WaveSpectrum{Bins: bins}
	s.Normalize()
	return s
}
