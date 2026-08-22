package bathymetry

import (
	"fmt"
	"math"
)

type colourStop struct {
	depthM float64
	red    int
	green  int
	blue   int
}

func bathymetryStops(maxDepthM float64) []colourStop {
	base := []colourStop{
		{depthM: 0, red: 225, green: 246, blue: 245},
		{depthM: 20, red: 164, green: 221, blue: 222},
		{depthM: 200, red: 84, green: 166, blue: 200},
		{depthM: 500, red: 42, green: 126, blue: 176},
		{depthM: 1000, red: 40, green: 86, blue: 153},
		{depthM: 1500, red: 31, green: 58, blue: 122},
		{depthM: 2000, red: 17, green: 37, blue: 84},
		{depthM: math.Max(2200, maxDepthM), red: 6, green: 20, blue: 48},
	}
	result := make([]colourStop, 0, len(base)+1)
	for _, stop := range base {
		if stop.depthM < maxDepthM {
			result = append(result, stop)
		}
	}
	last := base[len(base)-1]
	last.depthM = maxDepthM
	if len(result) == 0 || math.Abs(result[len(result)-1].depthM-maxDepthM) > 1e-9 {
		result = append(result, last)
	} else {
		result[len(result)-1] = last
	}
	return result
}

func depthColour(depthM, maxDepthM float64) string {
	depthM = math.Max(0, math.Min(depthM, maxDepthM))
	stops := bathymetryStops(maxDepthM)
	for index := 1; index < len(stops); index++ {
		left, right := stops[index-1], stops[index]
		if depthM > right.depthM {
			continue
		}
		ratio := 0.0
		if right.depthM > left.depthM {
			ratio = (depthM - left.depthM) / (right.depthM - left.depthM)
		}
		return rgbHex(
			interpolateChannel(left.red, right.red, ratio),
			interpolateChannel(left.green, right.green, ratio),
			interpolateChannel(left.blue, right.blue, ratio),
		)
	}
	last := stops[len(stops)-1]
	return rgbHex(last.red, last.green, last.blue)
}

func interpolateChannel(left, right int, ratio float64) int {
	return int(math.Round(float64(left) + (float64(right)-float64(left))*ratio))
}

func rgbHex(red, green, blue int) string {
	return fmt.Sprintf("#%02x%02x%02x", red, green, blue)
}
