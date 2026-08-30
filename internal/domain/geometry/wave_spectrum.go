package geometry

import "math"

// WaveSpectrumBin представляет одну направляющую компоненту волнового спектра
type WaveSpectrumBin struct {
	// Direction — направление, откуда приходят волны, в градусах по часовой стрелке от севера
	Direction float64 `json:"direction"`
	// Weight — относительный вклад энергии (0-1); обычно нормализуется по корзинам
	Weight float64 `json:"weight"`
	// SignificantWaveHeightM — значительная высота волны Hs для этого направления (м), опционально
	// Если 0, используется WindSpeedMetersPerSecond для вывода энергии
	SignificantWaveHeightM float64 `json:"hs_m,omitempty"`
}

// WaveSpectrum представляет направленный волновой спектр
// Заменяет единственный WindSourceDirectionDeg на несколько компонентов
type WaveSpectrum struct {
	Bins []WaveSpectrumBin `json:"bins"`
}

// Normalize гарантирует, что веса суммируются до 1
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

// DominantDirection возвращает направление с наивысшим весом
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

// MeanDirection возвращает энергетически-взвешенное круговое среднее направление (град от С)
func (s *WaveSpectrum) MeanDirection() float64 {
	if s == nil || len(s.Bins) == 0 {
		return 0
	}
	var sumX, sumY float64
	for _, b := range s.Bins {
		rad := b.Direction * math.Pi / 180
		// Direction — направление, откуда приходят волны, поэтому используем напрямую
		sumX += b.Weight * math.Sin(rad)
		sumY += b.Weight * math.Cos(rad)
	}
	// atan2 возвращает угол (sin, cos), что соответствует нашей конвенции
	deg := math.Atan2(sumX, sumY) * 180 / math.Pi
	if deg < 0 {
		deg += 360
	}
	return deg
}

// Spread возвращает круговое стандартное отклонение в градусах
// (высокое значение = широко распространено, низкое = унимодальное)
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

// NewSingleDirectionSpectrum создаёт спектр с единственным направлением
// Это режим совместимости (эквивалентно WindSourceDirectionDeg)
func NewSingleDirectionSpectrum(direction float64) WaveSpectrum {
	return WaveSpectrum{
		Bins: []WaveSpectrumBin{
			{Direction: direction, Weight: 1.0},
		},
	}
}

// NewBimodalSpectrum создаёт спектр с двумя доминирующими направлениями
// Пример: NE и SW ветра для Чёрного моря
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

// NewGaussianSpectrum создаёт гауссов спектр с центром в meanDir и заданным stddev
// и n корзинами, равномерно распределёнными вокруг компаса
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
		// Круговое расстояние
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

// NewDirectionalSpectrumFromWeights создаёт спектр из явных весов
// dirs и weights должны иметь одинаковую длину
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
