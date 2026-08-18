package cobra

import (
	"fmt"

	"coastal-geometry/internal/domain/geometry"
)

// printLongshoreInputQuality выводит свойства входов, фактически
// использованных CERC-моделью, чтобы пользователь видел ограничение данных.
func printLongshoreInputQuality(quality geometry.ModelInputQuality) {
	waves := quality.WaveClimate
	bathymetry := quality.Bathymetry
	fmt.Printf("Качество волн: %d состояний, %.1f ч", waves.ConditionCount, waves.TotalDurationHours)
	if waves.HasCompleteTimes {
		fmt.Printf(", %s — %s", waves.FirstTime, waves.LastTime)
		if waves.MaxTemporalGapHours > 0 {
			fmt.Printf(", максимальный пропуск %.1f ч", waves.MaxTemporalGapHours)
		}
	} else {
		fmt.Print(", временные метки не заданы")
	}
	fmt.Println()
	fmt.Printf("Качество батиметрии: %d точек, шаг %.6f°, выборок %d (интерполяция %d, ближайшая точка %d, максимум %.0f м)\n",
		bathymetry.PointCount, bathymetry.ResolutionDegrees, bathymetry.SampleCount,
		bathymetry.InterpolatedSampleCount, bathymetry.NearestSampleCount, bathymetry.MaxNearestDistanceMeters)
}
