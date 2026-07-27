package coastline

import "fmt"

const sanityTolerance = 0.40

type coastlineEstimate struct {
	MinKM float64
	MaxKM float64
}

type SanityCheckResult struct {
	Checked bool
	Valid   bool
	Warning string
}

var knownCoastlineEstimates = map[string]coastlineEstimate{
	"black-sea.json": {
		MinKM: 4000,
		MaxKM: 4987,
	},
}

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
			"WARNING: coastline length likely incorrect\nPossible causes:\n- wrong order of points\n- missing coastline sections\n- segments crossing sea\nReference range for %s: %.0f-%.0f km",
			dataset,
			estimate.MinKM,
			estimate.MaxKM,
		),
	}
}
