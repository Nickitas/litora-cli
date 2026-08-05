package svg

import (
	"fmt"
	"html"
	"math"
	"os"
)

// LogLogPoint представляет одну точку box-counting на log-log графике.
type LogLogPoint struct {
	LogInverseScale float64
	LogBoxes        float64
	BoxSizeMeters   float64
	InRegression    bool
}

// LogLogPlotOptions содержит данные и подписи отдельного графика аудита.
type LogLogPlotOptions struct {
	Title          string
	Subtitle       string
	Points         []LogLogPoint
	Dimension      float64
	RSquared       float64
	RegressionFrom int
	RegressionTo   int
}

// DrawLogLogSVG сохраняет график log(1/ε) — log(N(ε)) отдельным SVG-файлом.
func DrawLogLogSVG(options LogLogPlotOptions, filename string) error {
	if len(options.Points) < 2 {
		return fmt.Errorf("для log-log графика требуется минимум две точки")
	}

	const (
		width  = 1100
		height = 720
		left   = 100.0
		right  = 42.0
		top    = 100.0
		bottom = 90.0
	)
	plotWidth := float64(width) - left - right
	plotHeight := float64(height) - top - bottom

	minX, maxX := options.Points[0].LogInverseScale, options.Points[0].LogInverseScale
	minY, maxY := options.Points[0].LogBoxes, options.Points[0].LogBoxes
	for _, point := range options.Points[1:] {
		minX = math.Min(minX, point.LogInverseScale)
		maxX = math.Max(maxX, point.LogInverseScale)
		minY = math.Min(minY, point.LogBoxes)
		maxY = math.Max(maxY, point.LogBoxes)
	}
	minX, maxX = paddedRange(minX, maxX)
	minY, maxY = paddedRange(minY, maxY)
	projectX := func(value float64) float64 { return left + (value-minX)/(maxX-minX)*plotWidth }
	projectY := func(value float64) float64 { return top + plotHeight - (value-minY)/(maxY-minY)*plotHeight }

	var pointsSVG, labelsSVG string
	for index, point := range options.Points {
		color := "#6b7a87"
		radius := 5.0
		if point.InRegression {
			color = "#c2410c"
			radius = 6.5
		}
		pointsSVG += fmt.Sprintf(`  <circle cx="%.2f" cy="%.2f" r="%.1f" fill="%s"><title>ε=%.0f м, N(ε)=%.0f</title></circle>`+"\n",
			projectX(point.LogInverseScale), projectY(point.LogBoxes), radius, color, point.BoxSizeMeters, math.Exp(point.LogBoxes))
		labelsSVG += fmt.Sprintf(`  <text x="%.2f" y="%.2f" font-family="Helvetica, Arial, sans-serif" font-size="9" fill="#6b7a87" text-anchor="middle">%d</text>`+"\n",
			projectX(point.LogInverseScale), projectY(point.LogBoxes)-12, index+1)
	}

	regressionSVG := ""
	regressionMinX, regressionMaxX := minX, maxX
	if options.RegressionFrom >= 0 && options.RegressionTo < len(options.Points) && options.RegressionFrom < options.RegressionTo {
		from := options.Points[options.RegressionFrom]
		to := options.Points[options.RegressionTo]
		regressionMinX = math.Min(from.LogInverseScale, to.LogInverseScale)
		regressionMaxX = math.Max(from.LogInverseScale, to.LogInverseScale)
		intercept := from.LogBoxes - options.Dimension*from.LogInverseScale
		regressionSVG = fmt.Sprintf(`  <line x1="%.2f" y1="%.2f" x2="%.2f" y2="%.2f" stroke="#16324f" stroke-width="2.4" stroke-dasharray="8 5"/>`+"\n",
			projectX(regressionMinX), projectY(options.Dimension*regressionMinX+intercept),
			projectX(regressionMaxX), projectY(options.Dimension*regressionMaxX+intercept))
	}

	gridSVG := ""
	for index := 0; index <= 5; index++ {
		ratio := float64(index) / 5
		x := left + ratio*plotWidth
		y := top + plotHeight - ratio*plotHeight
		gridSVG += fmt.Sprintf(`  <line x1="%.2f" y1="%.2f" x2="%.2f" y2="%.2f" stroke="#d6d0c4" stroke-width="1"/>`+"\n", x, top, x, top+plotHeight)
		gridSVG += fmt.Sprintf(`  <line x1="%.2f" y1="%.2f" x2="%.2f" y2="%.2f" stroke="#d6d0c4" stroke-width="1"/>`+"\n", left, y, left+plotWidth, y)
		gridSVG += fmt.Sprintf(`  <text x="%.2f" y="%.2f" font-family="Helvetica, Arial, sans-serif" font-size="10" fill="#4f6d7a" text-anchor="middle">%.2f</text>`+"\n", x, top+plotHeight+22, minX+ratio*(maxX-minX))
		gridSVG += fmt.Sprintf(`  <text x="%.2f" y="%.2f" font-family="Helvetica, Arial, sans-serif" font-size="10" fill="#4f6d7a" text-anchor="end">%.2f</text>`+"\n", left-10, y+4, minY+ratio*(maxY-minY))
	}

	svg := fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8"?>
<svg xmlns="http://www.w3.org/2000/svg" width="%d" height="%d" viewBox="0 0 %d %d">
  <rect width="100%%" height="100%%" fill="#f7f4ea"/>
  <rect x="20" y="20" width="%d" height="%d" rx="24" fill="#fcfbf7" stroke="#d6d0c4"/>
  <text x="%.0f" y="58" font-family="Helvetica, Arial, sans-serif" font-size="24" font-weight="700" fill="#16324f">%s</text>
  <text x="%.0f" y="82" font-family="Helvetica, Arial, sans-serif" font-size="12" fill="#4f6d7a">%s</text>
  <rect x="%.2f" y="%.2f" width="%.2f" height="%.2f" fill="#fff7ed" fill-opacity="0.65"/>
%s%s%s%s
  <line x1="%.2f" y1="%.2f" x2="%.2f" y2="%.2f" stroke="#16324f" stroke-width="1.5"/>
  <line x1="%.2f" y1="%.2f" x2="%.2f" y2="%.2f" stroke="#16324f" stroke-width="1.5"/>
  <text x="%.2f" y="%.2f" font-family="Helvetica, Arial, sans-serif" font-size="12" font-weight="700" fill="#16324f" text-anchor="middle">log(1/ε)</text>
  <text x="%.2f" y="%.2f" font-family="Helvetica, Arial, sans-serif" font-size="12" font-weight="700" fill="#16324f" text-anchor="middle" transform="rotate(-90, %.2f, %.2f)">log(N(ε))</text>
  <text x="%.0f" y="%.0f" font-family="Helvetica, Arial, sans-serif" font-size="11" fill="#c2410c">оранжевые точки — регрессионное окно</text>
  <text x="%.0f" y="%.0f" font-family="Helvetica, Arial, sans-serif" font-size="11" fill="#16324f">D=%.5f · R²=%.4f</text>
</svg>
`, width, height, width, height, width-40, height-40,
		left, html.EscapeString(options.Title), left, html.EscapeString(options.Subtitle),
		projectX(regressionMinX), projectY(maxY), projectX(regressionMaxX)-projectX(regressionMinX), plotHeight,
		gridSVG, regressionSVG, pointsSVG, labelsSVG,
		left, top+plotHeight, left+plotWidth, top+plotHeight,
		left, top, left, top+plotHeight,
		left+plotWidth/2, float64(height)-30,
		left-64, top+plotHeight/2, left-64, top+plotHeight/2,
		left+plotWidth-260, float64(height)-28, left+plotWidth-170, float64(height)-28, options.Dimension, options.RSquared)

	return os.WriteFile(filename, []byte(svg), 0o644)
}

func paddedRange(minimum, maximum float64) (float64, float64) {
	if minimum == maximum {
		return minimum - 1, maximum + 1
	}
	padding := (maximum - minimum) * 0.08
	return minimum - padding, maximum + padding
}
