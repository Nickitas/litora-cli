package paradox

import (
	"fmt"
	"time"

	"coastal-geometry/internal/domain/generators/koch"
	"coastal-geometry/internal/domain/geometry"
)

func Demonstrate(base []geometry.LatLon, maxIterations int, erosionStrength float64, seed int64) {
	fmt.Println("\n  ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	fmt.Println("  ПАРАДОКС БЕРЕГОВОЙ ЛИНИИ")
	fmt.Println("  ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━\n")

	fmt.Println("  ┌──────────┬───────────┬───────────┬──────────────┬───────────┬───────────┐")
	fmt.Println("  │ Уровень  │ Точек     │ Сегментов │ Сред. шаг км │ Длина км  │ Прирост   │")
	fmt.Println("  ├──────────┼───────────┼───────────┼──────────────┼───────────┼───────────┤")

	prevLength := 0.0
	for level := 0; level <= maxIterations; level++ {
		curve := koch.KochCurve(base, level)
		if erosionStrength > 0 {
			if seed == 0 {
				seed = time.Now().UnixNano()
			}
			curve = geometry.ErodeWithSeed(curve, erosionStrength, seed+int64(level))
		}
		length := geometry.PolylineLength(curve)
		segments := max(len(curve)-1, 0)
		avgStep := 0.0
		if segments > 0 {
			avgStep = length / float64(segments)
		}

		var ratio string
		if level > 0 {
			ratio = fmt.Sprintf("×%.3f", length/prevLength)
		} else {
			ratio = "—"
		}

		fmt.Printf("  │ %-8d │ %-9d │ %-9d │ %-12.2f │ %-9.0f │ %-9s │\n", level, len(curve), segments, avgStep, length, ratio)
		prevLength = length
	}

	fmt.Println("  └──────────┴───────────┴───────────┴──────────────┴───────────┴───────────┘")
	fmt.Println()
}
