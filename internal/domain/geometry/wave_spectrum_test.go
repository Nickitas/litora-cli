package geometry

import (
	"math"
	"testing"
)

// TestWaveSpectrumNormalize проверяет нормализацию весов спектра
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
		t.Errorf("Нормализованные веса суммируются до %f, ожидалось 1.0", sum)
	}
	if s.Bins[0].Weight != 0.25 {
		t.Errorf("Вес первой корзины = %f, ожидалось 0.25", s.Bins[0].Weight)
	}
	if s.Bins[2].Weight != 0.5 {
		t.Errorf("Вес третьей корзины = %f, ожидалось 0.5", s.Bins[2].Weight)
	}
}

// TestSingleDirectionSpectrum проверяет спектр с одним направлением
func TestSingleDirectionSpectrum(t *testing.T) {
	s := NewSingleDirectionSpectrum(90)
	if got := s.DominantDirection(); got != 90 {
		t.Errorf("DominantDirection = %v, ожидалось 90", got)
	}
	if got := s.MeanDirection(); got != 90 {
		t.Errorf("MeanDirection = %v, ожидалось 90", got)
	}
	if got := s.Spread(); got > 1e-9 {
		t.Errorf("Spread = %v, ожидалось 0", got)
	}
}

// TestBimodalSpectrumMean проверяет бимодальный спектр
func TestBimodalSpectrumMean(t *testing.T) {
	// 0° и 180° с равными весами должны дать круговое среднее ~90 или ~270
	// (математически не определено, но наша реализация вернёт некоторое значение)
	s := NewBimodalSpectrum(0, 180, 1, 1)
	spread := s.Spread()
	if spread < 50 {
		t.Errorf("Разброс противоположных направлений = %v, ожидалось > 50", spread)
	}
}

// TestGaussianSpectrum проверяет гауссов спектр
func TestGaussianSpectrum(t *testing.T) {
	s := NewGaussianSpectrum(90, 30, 16)
	if got := s.MeanDirection(); math.Abs(got-90) > 5 {
		t.Errorf("MeanDirection = %v, ожидалось ~90", got)
	}
	if got := s.DominantDirection(); got != 90 {
		t.Errorf("DominantDirection = %v, ожидалось 90", got)
	}
	spread := s.Spread()
	if math.Abs(spread-30) > 10 {
		t.Errorf("Spread = %v, ожидалось ~30", spread)
	}
}

// TestGaussianSpectrumDegenerate проверяет вырожденный случай гауссова спектра
func TestGaussianSpectrumDegenerate(t *testing.T) {
	// stdDev = 0 должен свестись к одному направлению
	s := NewGaussianSpectrum(45, 0, 8)
	if len(s.Bins) != 1 {
		t.Errorf("Ожидалась 1 корзина, получено %d", len(s.Bins))
	}
	if s.Bins[0].Direction != 45 {
		t.Errorf("Направление = %v, ожидалось 45", s.Bins[0].Direction)
	}
}

// TestSpectrumWeightsSumToOne проверяет, что веса спектра суммируются до 1
func TestSpectrumWeightsSumToOne(t *testing.T) {
	s := NewGaussianSpectrum(180, 45, 8)
	var sum float64
	for _, b := range s.Bins {
		sum += b.Weight
	}
	if math.Abs(sum-1.0) > 1e-9 {
		t.Errorf("Веса суммируются до %f, ожидалось 1.0", sum)
	}
}

func TestNewDirectionalSpectrumFromWeights(t *testing.T) {
	tests := []struct {
		name    string
		dirs    []float64
		weights []float64
		wantOK  bool
	}{
		{
			name:   "валидные данные",
			dirs:   []float64{45, 90, 135, 180},
			weights: []float64{0.25, 0.25, 0.25, 0.25},
			wantOK: true,
		},
		{
			name:   "разные длины массивов",
			dirs:   []float64{45, 90, 135},
			weights: []float64{0.25, 0.25},
			wantOK: false,
		},
		{
			name:   "пустые массивы",
			dirs:   []float64{},
			weights: []float64{},
			wantOK: true, // пустой спектр валиден
		},
		{
			name:   "одно направление",
			dirs:   []float64{90},
			weights: []float64{1.0},
			wantOK: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			spectrum := NewDirectionalSpectrumFromWeights(tt.dirs, tt.weights)

			if tt.wantOK {
				if len(tt.dirs) > 0 && len(spectrum.Bins) == 0 {
					t.Errorf("NewDirectionalSpectrumFromWeights() returned empty spectrum for valid input")
				}
				// Проверяем, что веса нормализованы (сумма = 1)
				totalWeight := 0.0
				for _, bin := range spectrum.Bins {
					totalWeight += bin.Weight
				}
				if len(spectrum.Bins) > 0 && (totalWeight < 0.99 || totalWeight > 1.01) {
					t.Errorf("Weights not normalized: total = %f, want ~1.0", totalWeight)
				}
			} else {
				if len(spectrum.Bins) != 0 {
					t.Errorf("NewDirectionalSpectrumFromWeights() returned non-empty spectrum for invalid input")
				}
			}
		})
	}
}
