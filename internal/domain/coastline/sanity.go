package coastline

import "fmt"

// Допустимое отклонение от эталонной длины береговой линии (40%)
const sanityTolerance = 0.40

// coastlineEstimate представляет диапазон ожидаемой длины береговой линии
type coastlineEstimate struct {
	MinKM float64
	MaxKM float64
}

// SanityCheckResult представляет результат проверки длины береговой линии
type SanityCheckResult struct {
	Checked bool
	Valid   bool
	Warning string
}

// Известные оценки длины береговых линий для различных наборов данных
var knownCoastlineEstimates = map[string]coastlineEstimate{
	"black-sea.json": {
		MinKM: 4000,
		MaxKM: 4987,
	},
}

// SanityCheck проверяет, что вычисленная длина береговой линии соответствует ожидаемому диапазону
func SanityCheck(dataset string, lengthKM float64) SanityCheckResult {
	estimate, ok := knownCoastlineEstimates[dataset]
	if !ok {
		return SanityCheckResult{}
	}

	minAllowed := estimate.MinKM * (1 - sanityTolerance)
	maxAllowed := estimate.MaxKM * (1 + sanityTolerance)
	if lengthKM >= minAllowed && lengthKM <= maxAllowed {
		return SanityCheckResult{
			Checked: true,
			Valid:   true,
		}
	}

	return SanityCheckResult{
		Checked: true,
		Valid:   false,
		Warning: fmt.Sprintf(
			"ПРЕДУПРЕЖДЕНИЕ: длина береговой линии, вероятно, неверна\nВозможные причины:\n- неправильный порядок точек\n- отсутствуют участки береговой линии\n- сегменты пересекают море\nЭталонный диапазон для %s: %.0f-%.0f км",
			dataset,
			estimate.MinKM,
			estimate.MaxKM,
		),
	}
}
