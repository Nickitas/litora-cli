// Package fractal предоставляет функциональность для анализа фрактальной размерности береговых линий.
//
// Основные возможности:
//   - Вычисление фрактальной размерности методом box-counting
//   - Анализ стабильности размерности на разных масштабах
//   - Параллельная обработка для повышения производительности
//   - Линейная регрессия для определения размерности
//
// Основные типы:
//   - Point2D: точка в декартовой системе координат
//   - BoxCountingSample: результат box-counting для одного масштаба
//   - BoxCountingAnalysis: полный анализ фрактальной размерности
//
// Пример использования:
//
//	analysis := fractal.AnalyzeBoxCounting(coastlinePoints)
//	if analysis.Valid {
//	    fmt.Printf("Фрактальная размерность: %.2f\n", analysis.Dimension)
//	    fmt.Printf("R²: %.3f\n", analysis.RegressionRSquared)
//	}
//
// Для параллельной обработки:
//
//	ctx := context.Background()
//	analysis := fractal.AnalyzeBoxCountingParallel(ctx, coastlinePoints)
package fractal
