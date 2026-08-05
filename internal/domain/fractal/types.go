package fractal

// Point2D представляет точку в декартовой системе координат (в метрах)
type Point2D struct {
	X float64 // Координата X (долгота в метрах)
	Y float64 // Координата Y (широта в метрах)
}

// BoxCountingSample представляет результат box-counting для одного масштаба
type BoxCountingSample struct {
	ScaleFactor   float64 // Коэффициент масштаба относительно размера области
	RelativeScale float64 // Относительный размер ячейки (0-1)
	BoxSizeMeters float64 // Размер ячейки в метрах
	BoxesCovered  int     // Количество покрытых ячеек
	LogInvScale   float64 // Логарифм обратного масштаба
	LogBoxes      float64 // Логарифм количества ячеек
}

// GridOffset представляет нормированное смещение расчётной сетки.
type GridOffset struct {
	X float64 // Смещение по оси X в долях размера ячейки
	Y float64 // Смещение по оси Y в долях размера ячейки
}

// BoxCountingAnalysis представляет полный анализ фрактальной размерности
type BoxCountingAnalysis struct {
	Dimension          float64             // Вычисленная фрактальная размерность
	RegressionRSquared float64             // Коэффициент детерминации R²
	StableAcrossScales bool                // Стабильность на разных масштабах
	StabilitySpread    float64             // Разброс локальных размерностей
	Samples            []BoxCountingSample // Все замеры на разных масштабах
	LocalDimensions    []float64           // Локальные фрактальные размерности
	GridOffsets        []GridOffset        // Смещения, использованные для усреднения
	RegressionStart    int                 // Первый индекс sample в регрессионном окне
	RegressionEnd      int                 // Последний индекс sample в регрессионном окне
	Valid              bool                // Валидность результата
}
