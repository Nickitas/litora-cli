package svg

import (
	"coastal-geometry/internal/domain/geometry"
	"fmt"
	"html"
	"math"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// EnhancedDocument extends Document with additional map elements
type EnhancedDocument struct {
	Document
	// MinimalMap оставляет на карте только научно значимые элементы.
	MinimalMap               bool
	GridOptions              *GridOptions
	CompassOptions           *CompassOptions
	MarkerOptions            *MarkerOptions
	SedimentTransportOptions *SedimentTransportOptions
	BoxCountingGridOptions   *BoxCountingGridOptions
	ErosionGridOptions       *ErosionGridOptions
	ErosionChangeOptions     *ErosionChangeOptions
}

// GridOptions configures coordinate grid display
type GridOptions struct {
	Show          bool
	ShowLatLabels bool
	ShowLonLabels bool
	LatStep       float64 // degrees between latitude lines
	LonStep       float64 // degrees between longitude lines
	LineColor     string
	LabelColor    string
	FontSize      float64
	Opacity       float64
	DashArray     string
}

// CompassOptions configures compass/wind rose display
type CompassOptions struct {
	Show          bool
	X             float64 // SVG x position (0 = auto position)
	Y             float64 // SVG y position (0 = auto position)
	Size          float64 // pixels
	WindDirection float64 // degrees from north (for wave erosion)
	ShowWindArrow bool
	Label         string // optional label text
	Style         string // "modern", "classic", "minimal"
}

// MarkerOptions configures key point markers
type MarkerOptions struct {
	Show         bool
	Markers      []Marker
	DefaultSize  float64
	DefaultColor string
	ShowLabels   bool
}

// Marker represents a labeled point on the map
type Marker struct {
	Lat     float64
	Lon     float64
	Label   string
	Color   string
	Size    float64
	Shape   string // "circle", "square", "diamond", "triangle"
	Tooltip string
}

// SedimentTransportOptions configures sediment transport visualization
type SedimentTransportOptions struct {
	Show                 bool
	Points               []geometry.LatLon
	SedimentStates       []geometry.SedimentState
	ShowAccumulation     bool // show accumulation points
	ShowErosion          bool // show erosion points
	ShowTransportVectors bool // show longshore drift vectors
	AccumulationColor    string
	ErosionColor         string
	VectorColor          string
	VectorScale          float64 // scale factor for vector length
	MarkerSize           float64
}

// BoxCountingGridOptions configures box-counting grid visualization for fractal dimension analysis
type BoxCountingGridOptions struct {
	Show               bool
	Points             []geometry.LatLon // coastline points for box counting
	BoxSize            float64           // size of grid boxes in meters
	MinLat             float64           // bounding box parameters
	MaxLat             float64
	MinLon             float64
	MaxLon             float64
	ShowCoveredBoxes   bool   // highlight boxes that intersect with coastline
	ShowAllBoxes       bool   // show all boxes in grid
	CoveredColor       string // color for covered boxes
	UncoveredColor     string // color for uncovered boxes
	LineColor          string // grid line color
	LineWidth          float64
	Opacity            float64
	LabelScaleFactors  []float64 // specific scale factors to label (e.g., [16, 64, 256])
	ShowCoverageDegree bool      // show different colors based on coverage degree
	BufferZoneKM       float64   // buffer zone around coastline for detailed grid (km)
	ContextGrid        bool      // also show larger context grid
	ContextCellSize    float64   // cell size for context grid (meters)
	RegressionWindow   bool      // отображаемый масштаб входит в окно регрессии
	RegressionMinBox   float64   // минимальный размер ячейки в окне регрессии
	RegressionMaxBox   float64   // максимальный размер ячейки в окне регрессии
	LogLogSVGFile      string    // связанный отчёт log-log
}

// ErosionGridOptions configures erosion cell grid visualization for wave erosion modeling
type ErosionGridOptions struct {
	Show            bool
	Points          []geometry.LatLon // coastline points
	CellSize        float64           // size of erosion cells in meters
	MinLat          float64           // bounding box parameters
	MaxLat          float64
	MinLon          float64
	MaxLon          float64
	ShowCells       bool    // show grid cells
	ShowWaveVectors bool    // show wave direction vectors
	WaveDirection   float64 // primary wave direction in degrees
	LineColor       string  // grid line color
	LineWidth       float64
	Opacity         float64
	VectorColor     string  // wave vector color
	VectorLength    float64 // length of wave vectors
	BufferZoneKM    float64 // buffer zone around coastline for grid (km)
}

// ErosionChangePoint содержит локальное изменение положения берега.
type ErosionChangePoint struct {
	Point         geometry.LatLon
	ChangePerUnit float64
}

// ErosionChangeOptions задаёт научную цветовую шкалу изменения берега.
type ErosionChangeOptions struct {
	Show             bool
	Points           []ErosionChangePoint
	MaxAbsChange     float64
	NeutralThreshold float64
	UnitLabel        string
}

// DrawEnhancedSVG creates an SVG with additional map elements
func DrawEnhancedSVG(doc EnhancedDocument, filename string) error {
	if len(doc.Layers) == 0 {
		return fmt.Errorf("need at least 1 layer to draw svg")
	}

	allPoints := flattenLayers(doc.Layers)
	if len(allPoints) < 2 {
		return fmt.Errorf("need at least 2 points to draw svg")
	}

	minLat, maxLat, minLon, maxLon := bounds(allPoints)
	lonSpan := maxLon - minLon
	latSpan := maxLat - minLat
	if lonSpan == 0 {
		lonSpan = 1
	}
	if latSpan == 0 {
		latSpan = 1
	}

	minimalMap := doc.MinimalMap
	plotWidth := float64(canvasWidth) - sidebarWidth - 2*padding
	if minimalMap {
		plotWidth = float64(canvasWidth) - 2*padding
	}
	header, headerBottom := buildHeader(doc.Title, doc.Subtitle, padding, plotWidth)
	if minimalMap {
		header, headerBottom = buildScientificHeader(doc.Title, doc.Subtitle, padding)
	}
	annotationTopY := headerBottom + 8
	annotationHeight := 0.0
	if doc.BoxCountingGridOptions != nil && doc.BoxCountingGridOptions.Show {
		annotationHeight = 58
	} else if doc.ErosionGridOptions != nil && doc.ErosionGridOptions.Show {
		annotationHeight = 20
		if doc.ErosionChangeOptions != nil && doc.ErosionChangeOptions.Show {
			annotationHeight = 52
		}
	}
	plotTopY := headerBottom + 24 + annotationHeight
	plotHeight := float64(canvasHeight) - plotTopY - padding
	scale := math.Min(plotWidth/lonSpan, plotHeight/latSpan)
	contentWidth := lonSpan * scale
	contentHeight := latSpan * scale
	originX := padding + (plotWidth-contentWidth)/2
	originY := plotTopY + (plotHeight-contentHeight)/2

	// Build layers
	var layers strings.Builder
	for _, layer := range doc.Layers {
		polyline := projectPolyline(layer.Points, minLat, minLon, originX, originY, contentHeight, scale)
		layers.WriteString(fmt.Sprintf(
			`    <polyline fill="none" stroke="%s" stroke-width="%.2f" stroke-opacity="%.2f" stroke-linejoin="round" stroke-linecap="round"%s points="%s"/>`+"\n",
			escapeText(layerStroke(layer)),
			layerWidth(layer),
			layerOpacity(layer),
			layerDashAttribute(layer),
			polyline,
		))
	}

	// Build highlights
	var highlights strings.Builder
	for _, highlight := range doc.Highlights {
		x1 := originX + (highlight.Start.Lon-minLon)*scale
		y1 := originY + contentHeight - (highlight.Start.Lat-minLat)*scale
		x2 := originX + (highlight.End.Lon-minLon)*scale
		y2 := originY + contentHeight - (highlight.End.Lat-minLat)*scale

		highlights.WriteString(fmt.Sprintf(
			`    <line x1="%.2f" y1="%.2f" x2="%.2f" y2="%.2f" stroke="%s" stroke-width="%.2f" stroke-opacity="%.2f" stroke-linecap="round"/>`+"\n",
			x1, y1, x2, y2,
			escapeText(highlightStroke(highlight.Stroke)),
			highlightWidth(highlight.StrokeWidth),
			highlightOpacity(highlight.Opacity),
		))
		highlights.WriteString(fmt.Sprintf(
			`    <circle cx="%.2f" cy="%.2f" r="3.2" fill="%s" fill-opacity="%.2f"/>`+"\n",
			x1, y1,
			escapeText(highlightStroke(highlight.Stroke)),
			highlightOpacity(highlight.Opacity),
		))
		highlights.WriteString(fmt.Sprintf(
			`    <circle cx="%.2f" cy="%.2f" r="3.2" fill="%s" fill-opacity="%.2f"/>`+"\n",
			x2, y2,
			escapeText(highlightStroke(highlight.Stroke)),
			highlightOpacity(highlight.Opacity),
		))
	}

	// Build additional elements
	var gridElements, compassElements, markerElements, sedimentElements, boxCountingGridElements, erosionGridElements, erosionChangeElements, axisElements, gridAnnotationElements string

	if !minimalMap && doc.GridOptions != nil && doc.GridOptions.Show {
		gridElements = buildCoordinateGrid(*doc.GridOptions, minLat, maxLat, minLon, maxLon, originX, originY, contentWidth, contentHeight, scale)
	}

	if doc.CompassOptions != nil && doc.CompassOptions.Show {
		compassElements = buildCompass(*doc.CompassOptions, originX, originY, contentWidth, contentHeight, plotWidth, plotHeight)
	}

	if doc.MarkerOptions != nil && doc.MarkerOptions.Show {
		markerElements = buildMarkers(*doc.MarkerOptions, doc.MarkerOptions.Markers, minLat, minLon, originX, originY, contentHeight, scale)
	}

	if doc.SedimentTransportOptions != nil && doc.SedimentTransportOptions.Show {
		sedimentElements = buildSedimentTransportVisualization(*doc.SedimentTransportOptions, minLat, minLon, originX, originY, contentHeight, scale)
	}

	if doc.ErosionChangeOptions != nil && doc.ErosionChangeOptions.Show {
		erosionChangeElements = buildErosionChangeVisualization(*doc.ErosionChangeOptions, minLat, minLon, originX, originY, contentHeight, scale)
		gridAnnotationElements += buildErosionChangeAnnotation(*doc.ErosionChangeOptions, annotationTopY+18)
	}
	if doc.BoxCountingGridOptions != nil && doc.BoxCountingGridOptions.Show {
		fmt.Printf("   🔧 Отрисовка сетки box-counting: включена=%v, точек=%d\n", doc.BoxCountingGridOptions.Show, len(doc.BoxCountingGridOptions.Points))
		boxCountingGridElements = buildBoxCountingGrid(*doc.BoxCountingGridOptions, originX, originY, contentWidth, contentHeight, scale)
		gridAnnotationElements = buildBoxCountingGridAnnotation(*doc.BoxCountingGridOptions, padding, annotationTopY)
		if len(boxCountingGridElements) > 0 {
			fmt.Printf("   ✅ Сетка box-counting отрисована: %d символов SVG\n", len(boxCountingGridElements))
		} else {
			fmt.Printf("   ⚠️  Сетка box-counting пуста\n")
		}
	}

	if doc.ErosionGridOptions != nil && doc.ErosionGridOptions.Show {
		fmt.Printf("   🔧 Отрисовка эрозионной сетки: включена=%v, точек=%d, размер ячейки=%.0f м\n",
			doc.ErosionGridOptions.Show, len(doc.ErosionGridOptions.Points), doc.ErosionGridOptions.CellSize)
		erosionGridElements = buildErosionGrid(*doc.ErosionGridOptions, originX, originY, contentWidth, contentHeight, scale)
		gridAnnotationElements += buildErosionGridAnnotation(*doc.ErosionGridOptions, padding, annotationTopY)
		if len(erosionGridElements) > 0 {
			fmt.Printf("   ✅ Эрозионная сетка отрисована: %d символов SVG\n", len(erosionGridElements))
		} else {
			fmt.Printf("   ⚠️  Эрозионная сетка пуста\n")
		}
	}

	if minimalMap {
		axisElements = buildScientificAxes(minLat, maxLat, minLon, maxLon, originX, originY, contentWidth, contentHeight)
	}

	sidebarX := padding + plotWidth + 28
	legend, legendBottom := buildLegend(doc.Layers, sidebarX, plotTopY+10, sidebarWidth-56)
	statCards, statCardsBottom := buildStatCards(doc.StatCards, sidebarX, legendBottom+20, sidebarWidth-56)
	charts, chartsBottom := buildCharts(doc.Charts, sidebarX, statCardsBottom+18, sidebarWidth-56)
	alerts, alertsBottom := buildAlerts(doc.Alerts, sidebarX, chartsBottom+18, sidebarWidth-56)
	metaStartY := math.Max(608.0, alertsBottom+26)
	meta, metaBottom := buildMetaCard(doc.Meta, sidebarX, metaStartY, sidebarWidth-56)
	if minimalMap {
		legend, statCards, charts, alerts, meta = "", "", "", "", ""
	}
	highlights.WriteString(erosionChangeElements)

	documentHeight := canvasHeight
	if minimalMap {
		documentHeight = canvasHeight
	}
	sidebarBottom := max(max(legendBottom, statCardsBottom), max(chartsBottom, alertsBottom))
	sidebarBottom = max(sidebarBottom, metaBottom)
	requiredHeight := int(math.Ceil(sidebarBottom + padding))
	if requiredHeight > documentHeight {
		documentHeight = requiredHeight
	}

	scaleBarY := math.Max(float64(documentHeight)-padding-scaleBarYGap, originY+contentHeight+68)
	scaleBar := buildScaleBar(minLat, maxLat, minLon, maxLon, plotWidth, scale, padding, scaleBarY)

	sidebarBackground := fmt.Sprintf(`<rect x="%.0f" y="20" width="%.0f" height="%d" rx="24" fill="#f0ece2" stroke="#d6d0c4"/>`, padding+plotWidth+8, sidebarWidth-16, documentHeight-40)
	if minimalMap {
		sidebarBackground = ""
	}

	svg := fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8"?>
<svg xmlns="http://www.w3.org/2000/svg" width="%d" height="%d" viewBox="0 0 %d %d">
  <rect width="100%%" height="100%%" fill="#f7f4ea"/>
  <rect x="20" y="20" width="%d" height="%d" rx="28" fill="#fcfbf7" stroke="#d6d0c4"/>
  %s
  <defs>
    <clipPath id="map-clip"><rect x="%.2f" y="%.2f" width="%.2f" height="%.2f"/></clipPath>
  </defs>
  <g>
%s  </g>
  <g>
%s  </g>
  <g clip-path="url(#map-clip)">
%s  </g>
  <g>
%s  </g>
  <g clip-path="url(#map-clip)">
%s  </g>
  <g clip-path="url(#map-clip)">
%s  </g>
  <g>
%s  </g>
  <g>
%s  </g>
  <g clip-path="url(#map-clip)">
%s  </g>
  <g>
%s  </g>
  <g>
%s  </g>
  <g>
%s  </g>
  <g>
%s  </g>
  <g>
%s  </g>
  <g>
%s  </g>
  <g>
%s  </g>
  <g>
%s  </g>
</svg>
`, canvasWidth, documentHeight, canvasWidth, documentHeight,
		canvasWidth-40, documentHeight-40,
		sidebarBackground,
		originX, originY, contentWidth, contentHeight,
		header,
		boxCountingGridElements,
		erosionGridElements,
		axisElements,
		gridAnnotationElements,
		gridElements,
		layers.String(),
		highlights.String(),
		markerElements,
		sedimentElements,
		legend,
		statCards,
		charts,
		alerts,
		meta,
		compassElements,
		scaleBar,
	)

	if err := writeToFile(filename, []byte(svg)); err != nil {
		return fmt.Errorf("write svg %q: %w", filename, err)
	}

	return nil
}

// buildScientificHeader создаёт компактную шапку публикационной карты.
func buildScientificHeader(title, subtitle string, x float64) (string, float64) {
	var out strings.Builder
	out.WriteString(fmt.Sprintf(
		`  <text x="%.0f" y="58" font-family="Helvetica, Arial, sans-serif" font-size="24" font-weight="700" fill="#16324f">%s</text>`+"\n",
		x, escapeText(title),
	))
	if subtitle != "" {
		lines := strings.Split(subtitle, "\n")
		for index, line := range lines {
			out.WriteString(fmt.Sprintf(
				`  <text x="%.0f" y="%.0f" font-family="Helvetica, Arial, sans-serif" font-size="12" fill="#4f6d7a">%s</text>`+"\n",
				x, 84+float64(index)*18, escapeText(line),
			))
		}
		return out.String(), 96 + float64(len(lines)-1)*18
	}
	return out.String(), 70
}

// buildScientificAxes добавляет значения координат на осях OX и OY научной карты.
func buildScientificAxes(minLat, maxLat, minLon, maxLon, originX, originY, contentWidth, contentHeight float64) string {
	var out strings.Builder
	const tickCount = 5
	for i := 0; i <= tickCount; i++ {
		ratio := float64(i) / tickCount
		x := originX + contentWidth*ratio
		lon := minLon + (maxLon-minLon)*ratio
		out.WriteString(fmt.Sprintf(
			`    <line x1="%.1f" y1="%.1f" x2="%.1f" y2="%.1f" stroke="#687783" stroke-width="0.8"/>`+"\n",
			x, originY+contentHeight, x, originY+contentHeight+5,
		))
		out.WriteString(fmt.Sprintf(
			`    <text x="%.1f" y="%.1f" font-family="Helvetica, Arial, sans-serif" font-size="9" fill="#4f6d7a" text-anchor="middle">%.2f°</text>`+"\n",
			x, originY+contentHeight+18, lon,
		))

		y := originY + contentHeight*(1-ratio)
		lat := minLat + (maxLat-minLat)*ratio
		out.WriteString(fmt.Sprintf(
			`    <line x1="%.1f" y1="%.1f" x2="%.1f" y2="%.1f" stroke="#687783" stroke-width="0.8"/>`+"\n",
			originX-5, y, originX, y,
		))
		out.WriteString(fmt.Sprintf(
			`    <text x="%.1f" y="%.1f" font-family="Helvetica, Arial, sans-serif" font-size="9" fill="#4f6d7a" text-anchor="end" dominant-baseline="middle">%.2f°</text>`+"\n",
			originX-9, y, lat,
		))
	}
	out.WriteString(fmt.Sprintf(
		`    <text x="%.1f" y="%.1f" font-family="Helvetica, Arial, sans-serif" font-size="10" font-weight="700" fill="#16324f" text-anchor="middle">OX — долгота, °</text>`+"\n",
		originX+contentWidth/2, originY+contentHeight+34,
	))
	out.WriteString(fmt.Sprintf(
		`    <text x="%.1f" y="%.1f" font-family="Helvetica, Arial, sans-serif" font-size="10" font-weight="700" fill="#16324f" text-anchor="middle" transform="rotate(-90, %.1f, %.1f)">OY — широта, °</text>`+"\n",
		originX-24, originY+contentHeight/2, originX-24, originY+contentHeight/2,
	))
	return out.String()
}

// buildCoordinateGrid creates latitude and longitude grid lines
func buildCoordinateGrid(opts GridOptions, minLat, maxLat, minLon, maxLon, originX, originY, contentWidth, contentHeight, scale float64) string {
	var out strings.Builder

	// Set defaults
	latStep := opts.LatStep
	if latStep <= 0 {
		latStep = 0.5 // 30 minutes
	}
	lonStep := opts.LonStep
	if lonStep <= 0 {
		lonStep = 0.5
	}

	lineColor := opts.LineColor
	if lineColor == "" {
		lineColor = "#d6d0c4"
	}

	labelColor := opts.LabelColor
	if labelColor == "" {
		labelColor = "#8a9aa6"
	}

	fontSize := opts.FontSize
	if fontSize <= 0 {
		fontSize = 10
	}

	opacity := opts.Opacity
	if opacity <= 0 {
		opacity = 0.6
	}

	dashArray := opts.DashArray
	if dashArray == "" {
		dashArray = "4 4"
	}

	// Latitude lines (horizontal)
	startLat := math.Ceil(minLat/latStep) * latStep
	for lat := startLat; lat <= maxLat; lat += latStep {
		y := originY + contentHeight - (lat-minLat)*scale
		out.WriteString(fmt.Sprintf(
			`    <line x1="%.2f" y1="%.2f" x2="%.2f" y2="%.2f" stroke="%s" stroke-width="0.8" stroke-opacity="%.2f" stroke-dasharray="%s"/>`+"\n",
			originX, y, originX+contentWidth, y, lineColor, opacity, dashArray,
		))

		if opts.ShowLatLabels {
			label := formatCoordinate(lat, "lat")
			out.WriteString(fmt.Sprintf(
				`    <text x="%.0f" y="%.0f" font-family="Helvetica, Arial, sans-serif" font-size="%.1f" fill="%s" text-anchor="end">%s</text>`+"\n",
				originX-8, y+3, fontSize, labelColor, label,
			))
		}
	}

	// Longitude lines (vertical)
	startLon := math.Ceil(minLon/lonStep) * lonStep
	for lon := startLon; lon <= maxLon; lon += lonStep {
		x := originX + (lon-minLon)*scale
		out.WriteString(fmt.Sprintf(
			`    <line x1="%.2f" y1="%.2f" x2="%.2f" y2="%.2f" stroke="%s" stroke-width="0.8" stroke-opacity="%.2f" stroke-dasharray="%s"/>`+"\n",
			x, originY, x, originY+contentHeight, lineColor, opacity, dashArray,
		))

		if opts.ShowLonLabels {
			label := formatCoordinate(lon, "lon")
			out.WriteString(fmt.Sprintf(
				`    <text x="%.0f" y="%.0f" font-family="Helvetica, Arial, sans-serif" font-size="%.1f" fill="%s" text-anchor="middle" transform="rotate(-45, %.0f, %.0f)">%s</text>`+"\n",
				x, originY+contentHeight+14, fontSize, labelColor, x, originY+contentHeight+14, label,
			))
		}
	}

	return out.String()
}

// buildCompass creates a compass/wind rose
func buildCompass(opts CompassOptions, originX, originY, contentWidth, contentHeight, plotWidth, plotHeight float64) string {
	var out strings.Builder

	// Set defaults - smaller size for bottom-right placement
	size := opts.Size
	if size <= 0 {
		size = 45 // Slightly larger for better proportions
	}

	// Auto position: bottom-right corner, below the plot area to avoid overlap
	x := opts.X
	if x <= 0 {
		x = originX + contentWidth - size - 10 // Right side with minimal padding
	}
	y := opts.Y
	if y <= 0 {
		// Position below the plot area, well below to avoid overlap
		y = originY + contentHeight + size + 70
	}

	style := opts.Style
	if style == "" {
		style = "modern"
	}

	out.WriteString(fmt.Sprintf(`    <g transform="translate(%.0f, %.0f)">`+"\n", x, y))

	switch style {
	case "modern":
		out.WriteString(buildModernCompass(size, opts.WindDirection, opts.ShowWindArrow, opts.Label))
	case "classic":
		out.WriteString(buildClassicCompass(size, opts.WindDirection, opts.ShowWindArrow, opts.Label))
	case "minimal":
		out.WriteString(buildMinimalCompass(size, opts.WindDirection, opts.ShowWindArrow, opts.Label))
	default:
		out.WriteString(buildModernCompass(size, opts.WindDirection, opts.ShowWindArrow, opts.Label))
	}

	out.WriteString(`    </g>` + "\n")

	return out.String()
}

// buildModernCompass creates a modern styled compass
func buildModernCompass(size float64, windDirection float64, showWindArrow bool, label string) string {
	var out strings.Builder
	radius := size / 2

	// Outer circle
	out.WriteString(fmt.Sprintf(
		`      <circle cx="0" cy="0" r="%.1f" fill="none" stroke="#16324f" stroke-width="2" opacity="0.8"/>`+"\n",
		radius,
	))

	// Cardinal direction markers
	cardinals := []struct {
		angle float64
		label string
	}{
		{0, "N"}, {90, "E"}, {180, "S"}, {270, "W"},
	}

	for _, c := range cardinals {
		rad := (c.angle - 90) * math.Pi / 180 // Convert to SVG coordinate system
		x := math.Cos(rad) * (radius - 12)
		y := math.Sin(rad) * (radius - 12)

		out.WriteString(fmt.Sprintf(
			`      <text x="%.1f" y="%.1f" font-family="Helvetica, Arial, sans-serif" font-size="9" font-weight="700" fill="#16324f" text-anchor="middle" dominant-baseline="middle">%s</text>`+"\n",
			x, y, c.label,
		))
	}

	// Wind arrow if specified
	if showWindArrow && windDirection >= 0 {
		windRad := (windDirection - 90) * math.Pi / 180
		arrowLength := radius - 10

		// Wind direction arrow (pointing toward wind source)
		ax := math.Cos(windRad) * arrowLength
		ay := math.Sin(windRad) * arrowLength

		out.WriteString(fmt.Sprintf(
			`      <line x1="0" y1="0" x2="%.1f" y2="%.1f" stroke="#c2410c" stroke-width="1.8" stroke-linecap="round"/>`+"\n",
			ax, ay,
		))

		// Arrow head
		arrowHeadSize := 5.0
		arrowAngle := 0.5 // radians

		leftWingX := ax - arrowHeadSize*math.Cos(windRad-arrowAngle)
		leftWingY := ay - arrowHeadSize*math.Sin(windRad-arrowAngle)
		rightWingX := ax - arrowHeadSize*math.Cos(windRad+arrowAngle)
		rightWingY := ay - arrowHeadSize*math.Sin(windRad+arrowAngle)

		out.WriteString(fmt.Sprintf(
			`      <polygon points="%.1f,%.1f %.1f,%.1f %.1f,%.1f" fill="#c2410c"/>`+"\n",
			ax, ay, leftWingX, leftWingY, rightWingX, rightWingY,
		))
	}

	// Optional label
	if label != "" {
		out.WriteString(fmt.Sprintf(
			`      <text x="0" y="%.1f" font-family="Helvetica, Arial, sans-serif" font-size="10" fill="#4f6d7a" text-anchor="middle">%s</text>`+"\n",
			radius+14, escapeText(label),
		))
	}

	return out.String()
}

// buildClassicCompass creates a classic styled compass with star
func buildClassicCompass(size float64, windDirection float64, showWindArrow bool, label string) string {
	var out strings.Builder
	radius := size / 2

	// Outer ring
	out.WriteString(fmt.Sprintf(
		`      <circle cx="0" cy="0" r="%.1f" fill="none" stroke="#4f6d7a" stroke-width="1.5"/>`+"\n",
		radius,
	))
	out.WriteString(fmt.Sprintf(
		`      <circle cx="0" cy="0" r="%.1f" fill="none" stroke="#4f6d7a" stroke-width="1"/>`+"\n",
		radius-5,
	))

	// 4-point star
	for i := 0; i < 4; i++ {
		angle := float64(i)*90 - 90
		rad := angle * math.Pi / 180

		// Main star points
		outerX := math.Cos(rad) * (radius - 8)
		outerY := math.Sin(rad) * (radius - 8)

		innerAngle := angle + 45
		innerRad := innerAngle * math.Pi / 180
		innerX := math.Cos(innerRad) * (radius - 18)
		innerY := math.Sin(innerRad) * (radius - 18)

		out.WriteString(fmt.Sprintf(
			`      <polygon points="0,0 %.1f,%.1f %.1f,%.1f" fill="%s" opacity="0.7"/>`+"\n",
			outerX, outerY, innerX, innerY,
			map[string]string{"0": "#16324f", "1": "#6b7a87", "2": "#4f6d7a", "3": "#8a9aa6"}[fmt.Sprint(i)],
		))
	}

	// Cardinal labels
	cardinals := []struct {
		angle float64
		label string
	}{
		{0, "N"}, {90, "E"}, {180, "S"}, {270, "W"},
	}

	for _, c := range cardinals {
		rad := (c.angle - 90) * math.Pi / 180
		x := math.Cos(rad) * (radius - 25)
		y := math.Sin(rad) * (radius - 25)

		out.WriteString(fmt.Sprintf(
			`      <text x="%.1f" y="%.1f" font-family="Georgia, serif" font-size="10" font-weight="700" fill="#16324f" text-anchor="middle" dominant-baseline="middle">%s</text>`+"\n",
			x, y, c.label,
		))
	}

	// Wind arrow
	if showWindArrow && windDirection >= 0 {
		windRad := (windDirection - 90) * math.Pi / 180
		arrowLength := radius - 28
		ax := math.Cos(windRad) * arrowLength
		ay := math.Sin(windRad) * arrowLength

		out.WriteString(fmt.Sprintf(
			`      <line x1="0" y1="0" x2="%.1f" y2="%.1f" stroke="#c2410c" stroke-width="2"/>`+"\n",
			ax, ay,
		))
		out.WriteString(fmt.Sprintf(
			`      <circle cx="%.1f" cy="%.1f" r="3" fill="#c2410c"/>`+"\n",
			ax, ay,
		))
	}

	if label != "" {
		out.WriteString(fmt.Sprintf(
			`      <text x="0" y="%.1f" font-family="Georgia, serif" font-size="10" fill="#4f6d7a" text-anchor="middle" font-style="italic">%s</text>`+"\n",
			radius+14, escapeText(label),
		))
	}

	return out.String()
}

// buildMinimalCompass creates a minimal compass
func buildMinimalCompass(size float64, windDirection float64, showWindArrow bool, label string) string {
	var out strings.Builder
	radius := size / 2

	// Simple circle
	out.WriteString(fmt.Sprintf(
		`      <circle cx="0" cy="0" r="%.1f" fill="none" stroke="#8a9aa6" stroke-width="1.5" opacity="0.6"/>`+"\n",
		radius,
	))

	// Обозначения сторон света.
	for _, cardinal := range []struct {
		label string
		x     float64
		y     float64
	}{
		{label: "С", x: 0, y: -radius + 12},
		{label: "В", x: radius - 12, y: 0},
		{label: "Ю", x: 0, y: radius - 12},
		{label: "З", x: -radius + 12, y: 0},
	} {
		out.WriteString(fmt.Sprintf(
			`      <text x="%.1f" y="%.1f" font-family="Helvetica, Arial, sans-serif" font-size="11" font-weight="700" fill="#16324f" text-anchor="middle" dominant-baseline="middle">%s</text>`+"\n",
			cardinal.x, cardinal.y, cardinal.label,
		))
	}

	// Small tick marks
	for i := 0; i < 8; i++ {
		if i == 0 {
			continue // Skip N
		}
		angle := float64(i)*45 - 90
		rad := angle * math.Pi / 180

		tickOuter := radius - 4
		tickInner := radius - 10

		x1 := math.Cos(rad) * tickOuter
		y1 := math.Sin(rad) * tickOuter
		x2 := math.Cos(rad) * tickInner
		y2 := math.Sin(rad) * tickInner

		out.WriteString(fmt.Sprintf(
			`      <line x1="%.1f" y1="%.1f" x2="%.1f" y2="%.1f" stroke="#8a9aa6" stroke-width="1.5"/>`+"\n",
			x1, y1, x2, y2,
		))
	}

	// Wind direction
	if showWindArrow && windDirection >= 0 {
		windRad := (windDirection - 90) * math.Pi / 180
		arrowLength := radius - 14
		ax := math.Cos(windRad) * arrowLength
		ay := math.Sin(windRad) * arrowLength

		out.WriteString(fmt.Sprintf(
			`      <line x1="0" y1="0" x2="%.1f" y2="%.1f" stroke="#c2410c" stroke-width="2" stroke-linecap="round"/>`+"\n",
			ax, ay,
		))
	}

	if label != "" {
		out.WriteString(fmt.Sprintf(
			`      <text x="0" y="%.1f" font-family="Helvetica, Arial, sans-serif" font-size="9" fill="#6b7a87" text-anchor="middle" transform="rotate(-45, %.0f, %.0f)">%s</text>`+"\n",
			radius+12, 0.0, 0.0, label,
		))
	}

	return out.String()
}

// buildMarkers creates point markers on the map
func buildMarkers(opts MarkerOptions, markers []Marker, minLat, minLon, originX, originY, contentHeight, scale float64) string {
	var out strings.Builder

	if len(markers) == 0 {
		return ""
	}

	defaultSize := opts.DefaultSize
	if defaultSize <= 0 {
		defaultSize = 8
	}

	defaultColor := opts.DefaultColor
	if defaultColor == "" {
		defaultColor = "#c2410c"
	}

	for _, m := range markers {
		size := m.Size
		if size <= 0 {
			size = defaultSize
		}

		color := m.Color
		if color == "" {
			color = defaultColor
		}

		x := originX + (m.Lon-minLon)*scale
		y := originY + contentHeight - (m.Lat-minLat)*scale

		shape := m.Shape
		if shape == "" {
			shape = "circle"
		}

		// Draw shape
		switch shape {
		case "circle":
			out.WriteString(fmt.Sprintf(
				`    <circle cx="%.2f" cy="%.2f" r="%.1f" fill="%s" stroke="#fff" stroke-width="2"/>`+"\n",
				x, y, size, color,
			))
		case "square":
			halfSize := size
			out.WriteString(fmt.Sprintf(
				`    <rect x="%.2f" y="%.2f" width="%.1f" height="%.1f" fill="%s" stroke="#fff" stroke-width="2" transform="translate(%.1f,%.1f)"/>`+"\n",
				x-halfSize, y-halfSize, size*2, size*2, color, -halfSize, -halfSize,
			))
		case "diamond":
			points := fmt.Sprintf("%.2f,%.2f %.2f,%.2f %.2f,%.2f %.2f,%.2f",
				x, y-size, x+size, y, x, y+size, x-size, y)
			out.WriteString(fmt.Sprintf(
				`    <polygon points="%s" fill="%s" stroke="#fff" stroke-width="2"/>`+"\n",
				points, color,
			))
		case "triangle":
			points := fmt.Sprintf("%.2f,%.2f %.2f,%.2f %.2f,%.2f",
				x, y-size, x+size, y+size, x-size, y+size)
			out.WriteString(fmt.Sprintf(
				`    <polygon points="%s" fill="%s" stroke="#fff" stroke-width="2"/>`+"\n",
				points, color,
			))
		default:
			out.WriteString(fmt.Sprintf(
				`    <circle cx="%.2f" cy="%.2f" r="%.1f" fill="%s" stroke="#fff" stroke-width="2"/>`+"\n",
				x, y, size, color,
			))
		}

		// Label
		if opts.ShowLabels && m.Label != "" {
			labelX := x
			labelY := y - size - 6
			out.WriteString(fmt.Sprintf(
				`    <text x="%.2f" y="%.2f" font-family="Helvetica, Arial, sans-serif" font-size="11" font-weight="600" fill="#16324f" text-anchor="middle">%s</text>`+"\n",
				labelX, labelY, escapeText(m.Label),
			))
		}
	}

	return out.String()
}

// formatCoordinate formats latitude or longitude for display
func formatCoordinate(value float64, coordType string) string {
	degrees := int(math.Abs(value))
	minutes := (math.Abs(value) - float64(degrees)) * 60

	var direction string
	if coordType == "lat" {
		if value >= 0 {
			direction = "N"
		} else {
			direction = "S"
		}
	} else {
		if value >= 0 {
			direction = "E"
		} else {
			direction = "W"
		}
	}

	return fmt.Sprintf("%d°%.0f'%s", degrees, minutes, direction)
}

// Helper function to write SVG to file
func writeToFile(filename string, data []byte) error {
	return os.WriteFile(filename, data, 0o644)
}

// buildBoxCountingGrid creates box-counting grid visualization for fractal dimension analysis
func buildBoxCountingGrid(opts BoxCountingGridOptions, originX, originY, contentWidth, contentHeight, scale float64) string {
	var out strings.Builder

	fmt.Printf("      🔧 Сетка box-counting: точек=%d, ячейка=%.0f м, буфер=%.0f км, контекст=%v\n",
		len(opts.Points), opts.BoxSize, opts.BufferZoneKM, opts.ContextGrid)

	if len(opts.Points) == 0 {
		fmt.Printf("      ⚠️  Нет точек для построения сетки\n")
		return ""
	}

	// Set defaults
	lineColor := opts.LineColor
	if lineColor == "" {
		lineColor = "#8a9aa6"
	}

	lineWidth := opts.LineWidth
	if lineWidth <= 0 {
		lineWidth = 1.0
	}

	opacity := opts.Opacity
	if opacity <= 0 {
		opacity = 0.3
	}

	coveredColor := opts.CoveredColor
	if coveredColor == "" {
		coveredColor = "rgba(193, 65, 12, 0.3)" // orange with transparency
	}

	uncoveredColor := opts.UncoveredColor
	if uncoveredColor == "" {
		uncoveredColor = "rgba(138, 154, 166, 0.1)" // gray with low transparency
	}

	// Calculate grid parameters
	boxSizeMeters := opts.BoxSize
	if boxSizeMeters <= 0 {
		boxSizeMeters = 1000 // default 1km boxes
	}

	// Convert bounding box to meters
	minLat, maxLat := opts.MinLat, opts.MaxLat
	minLon, maxLon := opts.MinLon, opts.MaxLon
	mapMinLat, mapMinLon := minLat, minLon
	gridAnchorLat, gridAnchorLon := minLat, minLon

	// If buffer zone specified, use smaller area around coastline for better visualization
	if opts.BufferZoneKM > 0 {
		projection := geometry.NewLocalMetricProjection(opts.Points)
		metersPerDegLat := projection.MetersPerDegreeLatitude
		metersPerDegLon := projection.MetersPerDegreeLongitude

		fmt.Printf("      🎯 Буферная зона: %.0f км вокруг береговой линии (вместо всего моря)\n", opts.BufferZoneKM)

		minLat, maxLat, minLon, maxLon = calculateBoundingBoxWithBuffer(
			opts.Points, opts.BufferZoneKM, metersPerDegLat, metersPerDegLon)
	}

	// Convert to meters using approximate conversion
	projection := geometry.NewLocalMetricProjection(opts.Points)
	metersPerDegLat := projection.MetersPerDegreeLatitude
	metersPerDegLon := projection.MetersPerDegreeLongitude
	minLat = gridAnchorLat + math.Floor((minLat-gridAnchorLat)*metersPerDegLat/boxSizeMeters)*boxSizeMeters/metersPerDegLat
	minLon = gridAnchorLon + math.Floor((minLon-gridAnchorLon)*metersPerDegLon/boxSizeMeters)*boxSizeMeters/metersPerDegLon

	latSpan := (maxLat - minLat) * metersPerDegLat
	lonSpan := (maxLon - minLon) * metersPerDegLon

	// Calculate grid dimensions
	rows := int(math.Ceil(latSpan / boxSizeMeters))
	cols := int(math.Ceil(lonSpan / boxSizeMeters))

	fmt.Printf("      📐 Размер области: широта=%.0f км, долгота=%.0f км, ячейка=%.0f м, строк=%d, столбцов=%d\n",
		latSpan/1000, lonSpan/1000, boxSizeMeters, rows, cols)

	if rows <= 0 || cols <= 0 {
		fmt.Printf("      ⚠️  Некорректные размеры сетки: строк=%d, столбцов=%d\n", rows, cols)
		return ""
	}

	// Mark covered boxes using simplified algorithm
	coverageCount := make(map[[2]int]int)
	if opts.ShowCoveredBoxes {
		coverageCount = markCoveredBoxes(opts.Points, minLat, minLon, boxSizeMeters, metersPerDegLat, metersPerDegLon)
	}

	// Draw grid boxes with optimization limit
	maxBoxes := 100000 // increased limit for better visualization of large areas
	boxCount := 0

	// Calculate adaptive minimum size threshold
	// For very large areas, allow smaller boxes to be drawn
	totalBoxes := rows * cols
	minSizeThreshold := 1.0   // default 1 pixel
	if totalBoxes > 1000000 { // very large grid
		minSizeThreshold = 0.1 // allow 0.1 pixel boxes for large grids
	} else if totalBoxes > 100000 { // large grid
		minSizeThreshold = 0.3 // allow 0.3 pixel boxes
	}

	fmt.Printf("      🔢 Начало цикла отрисовки: rows=%d, cols=%d, maxBoxes=%d, minSizeThreshold=%.2f\n",
		rows, cols, maxBoxes, minSizeThreshold)

	for row := 0; row < rows && boxCount < maxBoxes; row++ {
		for col := 0; col < cols && boxCount < maxBoxes; col++ {
			// Calculate box boundaries in degrees
			boxLatMin := minLat + float64(row)*boxSizeMeters/metersPerDegLat
			boxLatMax := boxLatMin + boxSizeMeters/metersPerDegLat
			boxLonMin := minLon + float64(col)*boxSizeMeters/metersPerDegLon
			boxLonMax := boxLonMin + boxSizeMeters/metersPerDegLon

			// Project to SVG coordinates
			x1 := originX + (boxLonMin-mapMinLon)*scale
			y1 := originY + contentHeight - (boxLatMax-mapMinLat)*scale
			x2 := originX + (boxLonMax-mapMinLon)*scale
			y2 := originY + contentHeight - (boxLatMin-mapMinLat)*scale

			width := x2 - x1
			height := y2 - y1

			// Skip boxes that are too small (using adaptive threshold)
			if width < minSizeThreshold || height < minSizeThreshold {
				continue
			}

			// Determine coverage degree
			coverage := coverageCount[[2]int{row, col}]
			isCovered := coverage > 0

			// Select color based on coverage degree
			fillColor := uncoveredColor
			if opts.ShowCoverageDegree && isCovered {
				if coverage >= 5 {
					fillColor = "rgba(193, 65, 12, 0.7)" // темно-оранжевый: сильное покрытие
				} else if coverage >= 2 {
					fillColor = "rgba(193, 65, 12, 0.5)" // средне-оранжевый: среднее покрытие
				} else {
					fillColor = "rgba(193, 65, 12, 0.3)" // светло-оранжевый: слабое покрытие
				}
			} else if isCovered {
				fillColor = coveredColor
			}

			if opts.ShowAllBoxes || (opts.ShowCoveredBoxes && isCovered) {
				stroke := lineColor
				strokeWidth := lineWidth
				if opts.RegressionWindow {
					stroke = "#7c2d12"
					strokeWidth = math.Max(lineWidth, 1.1)
				}
				out.WriteString(fmt.Sprintf(
					`    <rect x="%.2f" y="%.2f" width="%.2f" height="%.2f" fill="%s" stroke="%s" stroke-width="%.2f" stroke-opacity="%.2f" fill-opacity="%.2f"/>`+"\n",
					x1, y1, width, height, fillColor, stroke, strokeWidth, opacity, opacity,
				))
				boxCount++ // increment counter only when actually drawing
			}
		}
	}

	fmt.Printf("      📊 Отрисовано ячеек: %d из %d возможных (ограничение: %d)\n",
		boxCount, rows*cols, maxBoxes)

	return out.String()
}

// buildBoxCountingGridAnnotation создаёт пояснение сетки вне клиппинга карты.
func buildBoxCountingGridAnnotation(opts BoxCountingGridOptions, originX, labelY float64) string {
	boxSize := opts.BoxSize
	if boxSize <= 0 {
		boxSize = 1000
	}
	annotation := fmt.Sprintf(
		`    <text x="%.0f" y="%.0f" font-family="Helvetica, Arial, sans-serif" font-size="10" fill="#6b7a87">Сетка box-counting: ячейка %.0f м</text>`+"\n"+
			`    <text x="%.0f" y="%.0f" font-family="Helvetica, Arial, sans-serif" font-size="9" fill="#6b7a87">Оранжевые ячейки — покрытые N(ε); насыщенность показывает плотность прохождения линии</text>`+"\n",
		padding, labelY, boxSize,
		padding, labelY+15,
	)
	if opts.RegressionWindow {
		annotation += fmt.Sprintf(
			`    <rect x="%.0f" y="%.0f" width="10" height="10" fill="#c2410c" fill-opacity="0.45" stroke="#7c2d12" stroke-width="1.2"/><text x="%.0f" y="%.0f" font-family="Helvetica, Arial, sans-serif" font-size="9" fill="#6b7a87">тёмная рамка — ячейки на масштабе ε в регрессионном окне (%.0f–%.0f м)</text>`+"\n",
			padding, labelY+32, padding+15, labelY+41, opts.RegressionMinBox, opts.RegressionMaxBox,
		)
	}
	if opts.LogLogSVGFile != "" {
		annotation += fmt.Sprintf(
			`    <a href="%s"><text x="%.0f" y="%.0f" font-family="Helvetica, Arial, sans-serif" font-size="9" fill="#1f6f8b" text-decoration="underline">Открыть отдельный график: %s</text></a>`+"\n",
			html.EscapeString(filepath.Base(opts.LogLogSVGFile)), padding, labelY+48, html.EscapeString(filepath.Base(opts.LogLogSVGFile)),
		)
	}
	return annotation
}

// markCoveredBoxes marks which grid boxes are intersected by the coastline
// Returns map of box -> coverage count (number of segments passing through box)
func markCoveredBoxes(points []geometry.LatLon, minLat, minLon, boxSizeMeters, metersPerDegLat, metersPerDegLon float64) map[[2]int]int {
	coverage := make(map[[2]int]int)

	if len(points) < 2 {
		return coverage
	}

	// Mark boxes for each segment
	for i := 1; i < len(points); i++ {
		p1 := points[i-1]
		p2 := points[i]

		// Convert to meters relative to min bounds
		x1 := (p1.Lon - minLon) * metersPerDegLon
		y1 := (p1.Lat - minLat) * metersPerDegLat
		x2 := (p2.Lon - minLon) * metersPerDegLon
		y2 := (p2.Lat - minLat) * metersPerDegLat

		// Sample points along segment
		dx := x2 - x1
		dy := y2 - y1
		distance := math.Hypot(dx, dy)
		steps := 1
		if boxSizeMeters > 0 {
			steps = int(math.Ceil(distance/(boxSizeMeters/2))) + 1
		}
		if steps < 2 {
			steps = 2
		}

		for step := 0; step <= steps; step++ {
			t := float64(step) / float64(steps)
			x := x1 + dx*t
			y := y1 + dy*t

			row := int(math.Floor(y / boxSizeMeters))
			col := int(math.Floor(x / boxSizeMeters))

			coverage[[2]int{row, col}]++
		}
	}

	return coverage
}

// calculateBoundingBoxWithBuffer calculates extended bounding box with buffer zone around coastline
func calculateBoundingBoxWithBuffer(points []geometry.LatLon, bufferKM float64, metersPerDegLat, metersPerDegLon float64) (minLat, maxLat, minLon, maxLon float64) {
	if len(points) == 0 {
		return 0, 0, 0, 0
	}

	minLat, maxLat = points[0].Lat, points[0].Lat
	minLon, maxLon = points[0].Lon, points[0].Lon

	for _, p := range points[1:] {
		if p.Lat < minLat {
			minLat = p.Lat
		}
		if p.Lat > maxLat {
			maxLat = p.Lat
		}
		if p.Lon < minLon {
			minLon = p.Lon
		}
		if p.Lon > maxLon {
			maxLon = p.Lon
		}
	}

	// Add buffer zone
	bufferLat := bufferKM * 1000 / metersPerDegLat
	bufferLon := bufferKM * 1000 / metersPerDegLon

	minLat -= bufferLat
	maxLat += bufferLat
	minLon -= bufferLon
	maxLon += bufferLon

	return minLat, maxLat, minLon, maxLon
}

// buildErosionGrid creates erosion cell grid visualization for wave erosion modeling
func buildErosionGrid(opts ErosionGridOptions, originX, originY, contentWidth, contentHeight, scale float64) string {
	var out strings.Builder

	if len(opts.Points) == 0 {
		return ""
	}

	// Set defaults
	lineColor := opts.LineColor
	if lineColor == "" {
		lineColor = "#4a90b8"
	}

	lineWidth := opts.LineWidth
	if lineWidth <= 0 {
		lineWidth = 0.6
	}

	opacity := opts.Opacity
	if opacity <= 0 {
		opacity = 0.32
	}

	vectorColor := opts.VectorColor
	if vectorColor == "" {
		vectorColor = "#1f6f8b"
	}

	vectorLength := opts.VectorLength
	if vectorLength <= 0 {
		vectorLength = 15.0
	}

	// Calculate grid parameters
	cellSizeMeters := opts.CellSize
	if cellSizeMeters <= 0 {
		cellSizeMeters = 500 // default 500m cells
	}

	// Convert bounding box to meters
	minLat, maxLat := opts.MinLat, opts.MaxLat
	minLon, maxLon := opts.MinLon, opts.MaxLon
	mapMinLat, mapMinLon := minLat, minLon
	gridAnchorLat, gridAnchorLon := minLat, minLon

	// If buffer zone specified, use smaller area around coastline for better visualization
	if opts.BufferZoneKM > 0 {
		projection := geometry.NewLocalMetricProjection(opts.Points)
		metersPerDegLat := projection.MetersPerDegreeLatitude
		metersPerDegLon := projection.MetersPerDegreeLongitude

		fmt.Printf("      🎯 Буферная зона для эрозии: %.0f км вокруг береговой линии\n", opts.BufferZoneKM)

		minLat, maxLat, minLon, maxLon = calculateBoundingBoxWithBuffer(
			opts.Points, opts.BufferZoneKM, metersPerDegLat, metersPerDegLon)
	}

	// Convert to meters using approximate conversion
	projection := geometry.NewLocalMetricProjection(opts.Points)
	metersPerDegLat := projection.MetersPerDegreeLatitude
	metersPerDegLon := projection.MetersPerDegreeLongitude
	minLat = gridAnchorLat + math.Floor((minLat-gridAnchorLat)*metersPerDegLat/cellSizeMeters)*cellSizeMeters/metersPerDegLat
	minLon = gridAnchorLon + math.Floor((minLon-gridAnchorLon)*metersPerDegLon/cellSizeMeters)*cellSizeMeters/metersPerDegLon

	latSpan := (maxLat - minLat) * metersPerDegLat
	lonSpan := (maxLon - minLon) * metersPerDegLon

	// Calculate grid dimensions
	rows := int(math.Ceil(latSpan / cellSizeMeters))
	cols := int(math.Ceil(lonSpan / cellSizeMeters))

	if rows <= 0 || cols <= 0 {
		return ""
	}

	// Draw grid cells
	if opts.ShowCells {
		maxCells := 100000
		drawnCells := 0
		totalCells := rows * cols
		minSizeThreshold := 1.0
		if totalCells > 1000000 {
			minSizeThreshold = 0.1
		} else if totalCells > 100000 {
			minSizeThreshold = 0.3
		}
		for row := 0; row < rows; row++ {
			for col := 0; col < cols && drawnCells < maxCells; col++ {
				// Calculate cell boundaries in degrees
				cellLatMin := minLat + float64(row)*cellSizeMeters/metersPerDegLat
				cellLatMax := cellLatMin + cellSizeMeters/metersPerDegLat
				cellLonMin := minLon + float64(col)*cellSizeMeters/metersPerDegLon
				cellLonMax := cellLonMin + cellSizeMeters/metersPerDegLon

				// Project to SVG coordinates
				x1 := originX + (cellLonMin-mapMinLon)*scale
				y1 := originY + contentHeight - (cellLatMax-mapMinLat)*scale
				x2 := originX + (cellLonMax-mapMinLon)*scale
				y2 := originY + contentHeight - (cellLatMin-mapMinLat)*scale

				width := x2 - x1
				height := y2 - y1

				// Применяем тот же адаптивный порог, что и для box-counting.
				if width < minSizeThreshold || height < minSizeThreshold {
					continue
				}

				out.WriteString(fmt.Sprintf(
					`    <rect x="%.2f" y="%.2f" width="%.2f" height="%.2f" fill="none" stroke="%s" stroke-width="%.2f" stroke-opacity="%.2f"/>`+"\n",
					x1, y1, width, height, lineColor, lineWidth, opacity,
				))
				drawnCells++
			}
		}
	}

	// Draw wave direction vectors
	if opts.ShowWaveVectors && opts.WaveDirection >= 0 {
		// Calculate vector direction
		waveRad := (opts.WaveDirection - 90) * math.Pi / 180

		// Рисуем разреженное поле направления, чтобы не перекрывать береговую линию.
		vectorInterval := 10
		for row := 0; row < rows; row += vectorInterval {
			for col := 0; col < cols; col += vectorInterval {
				// Calculate cell center in degrees
				cellLatMin := minLat + float64(row)*cellSizeMeters/metersPerDegLat
				cellLatMax := cellLatMin + cellSizeMeters/metersPerDegLat
				cellLonMin := minLon + float64(col)*cellSizeMeters/metersPerDegLon
				cellLonMax := cellLonMin + cellSizeMeters/metersPerDegLon

				centerLat := (cellLatMin + cellLatMax) / 2
				centerLon := (cellLonMin + cellLonMax) / 2

				// Project to SVG coordinates
				centerX := originX + (centerLon-mapMinLon)*scale
				centerY := originY + contentHeight - (centerLat-mapMinLat)*scale

				// Calculate vector end point
				vx := math.Cos(waveRad) * vectorLength
				vy := math.Sin(waveRad) * vectorLength

				endX := centerX + vx
				endY := centerY + vy

				// Draw vector arrow
				out.WriteString(fmt.Sprintf(
					`    <line x1="%.2f" y1="%.2f" x2="%.2f" y2="%.2f" stroke="%s" stroke-width="1.4" stroke-opacity="0.8" stroke-linecap="round"/>`+"\n",
					centerX, centerY, endX, endY, vectorColor,
				))

				// Arrow head
				arrowSize := 4.0
				leftWingX := endX - arrowSize*math.Cos(waveRad-0.5)
				leftWingY := endY - arrowSize*math.Sin(waveRad-0.5)
				rightWingX := endX - arrowSize*math.Cos(waveRad+0.5)
				rightWingY := endY - arrowSize*math.Sin(waveRad+0.5)

				out.WriteString(fmt.Sprintf(
					`    <polygon points="%.2f,%.2f %.2f,%.2f %.2f,%.2f" fill="%s" fill-opacity="0.7"/>`+"\n",
					endX, endY, leftWingX, leftWingY, rightWingX, rightWingY, vectorColor,
				))
			}
		}
	}

	return out.String()
}

// buildErosionGridAnnotation создаёт подпись сетки вне клиппинга карты.
func buildErosionGridAnnotation(opts ErosionGridOptions, originX, labelY float64) string {
	cellSize := opts.CellSize
	if cellSize <= 0 {
		cellSize = 500
	}
	return fmt.Sprintf(
		`    <text x="%.0f" y="%.0f" font-family="Helvetica, Arial, sans-serif" font-size="10" fill="#6b7a87">Эрозионная сетка: ячейка %.0f м; стрелки — направление волн %.0f°</text>`+"\n",
		padding, labelY, cellSize, opts.WaveDirection,
	)
}

func buildErosionChangeVisualization(opts ErosionChangeOptions, minLat, minLon, originX, originY, contentHeight, scale float64) string {
	if len(opts.Points) < 2 {
		return ""
	}

	maxAbs := opts.MaxAbsChange
	if maxAbs <= 0 {
		for _, point := range opts.Points {
			maxAbs = math.Max(maxAbs, math.Abs(point.ChangePerUnit))
		}
	}
	if maxAbs <= 0 {
		return ""
	}
	neutralThreshold := opts.NeutralThreshold
	if neutralThreshold <= 0 {
		neutralThreshold = math.Max(0.1, maxAbs*0.03)
	}

	var out strings.Builder
	for index := 1; index < len(opts.Points); index++ {
		previous := opts.Points[index-1]
		current := opts.Points[index]
		value := (previous.ChangePerUnit + current.ChangePerUnit) / 2
		color := erosionChangeColor(value, maxAbs, neutralThreshold)
		x1 := originX + (previous.Point.Lon-minLon)*scale
		y1 := originY + contentHeight - (previous.Point.Lat-minLat)*scale
		x2 := originX + (current.Point.Lon-minLon)*scale
		y2 := originY + contentHeight - (current.Point.Lat-minLat)*scale
		out.WriteString(fmt.Sprintf(
			`    <line x1="%.2f" y1="%.2f" x2="%.2f" y2="%.2f" stroke="%s" stroke-width="4.6" stroke-opacity="0.9" stroke-linecap="round"/>`+"\n",
			x1, y1, x2, y2, color,
		))
	}
	return out.String()
}

func erosionChangeColor(value, maxAbs, neutralThreshold float64) string {
	if math.Abs(value) <= neutralThreshold {
		return "#9aa3ab"
	}
	ratio := math.Min(1, math.Abs(value)/maxAbs)
	if value > 0 {
		return blendHexColor("#c2410c", ratio)
	}
	return blendHexColor("#1f6f8b", ratio)
}

func blendHexColor(base string, intensity float64) string {
	red, errRed := strconv.ParseUint(base[1:3], 16, 8)
	green, errGreen := strconv.ParseUint(base[3:5], 16, 8)
	blue, errBlue := strconv.ParseUint(base[5:7], 16, 8)
	if errRed != nil || errGreen != nil || errBlue != nil {
		return base
	}
	whiteShare := 0.72 * (1 - intensity)
	return fmt.Sprintf("#%02x%02x%02x",
		uint8(float64(red)*(1-whiteShare)+255*whiteShare),
		uint8(float64(green)*(1-whiteShare)+255*whiteShare),
		uint8(float64(blue)*(1-whiteShare)+255*whiteShare))
}

func buildErosionChangeAnnotation(opts ErosionChangeOptions, labelY float64) string {
	if !opts.Show || len(opts.Points) == 0 {
		return ""
	}
	maxAbs := opts.MaxAbsChange
	if maxAbs <= 0 {
		for _, point := range opts.Points {
			maxAbs = math.Max(maxAbs, math.Abs(point.ChangePerUnit))
		}
	}
	unit := opts.UnitLabel
	if unit == "" {
		unit = "м/шаг"
	}
	return fmt.Sprintf(
		`    <rect x="%.0f" y="%.0f" width="10" height="10" fill="#c2410c"/><text x="%.0f" y="%.0f" font-family="Helvetica, Arial, sans-serif" font-size="9" fill="#6b7a87">размыв</text>`+"\n"+`    <rect x="%.0f" y="%.0f" width="10" height="10" fill="#9aa3ab"/><text x="%.0f" y="%.0f" font-family="Helvetica, Arial, sans-serif" font-size="9" fill="#6b7a87">нейтральная зона — низкая устойчивость</text>`+"\n"+`    <rect x="%.0f" y="%.0f" width="10" height="10" fill="#1f6f8b"/><text x="%.0f" y="%.0f" font-family="Helvetica, Arial, sans-serif" font-size="9" fill="#6b7a87">накопление</text>`+"\n"+`    <text x="%.0f" y="%.0f" font-family="Helvetica, Arial, sans-serif" font-size="9" fill="#6b7a87">шкала: 0–%.1f %s</text>`+"\n",
		padding, labelY+17, padding+15, labelY+26,
		padding+75, labelY+17, padding+90, labelY+26,
		padding+190, labelY+17, padding+205, labelY+26,
		padding+300, labelY+26, maxAbs, unit,
	)
}

// buildSedimentTransportVisualization creates sediment transport visualization
func buildSedimentTransportVisualization(opts SedimentTransportOptions, minLat, minLon, originX, originY, contentHeight, scale float64) string {
	var out strings.Builder

	// Set defaults
	accumulationColor := opts.AccumulationColor
	if accumulationColor == "" {
		accumulationColor = "#2d6a4f" // green for accumulation
	}

	erosionColor := opts.ErosionColor
	if erosionColor == "" {
		erosionColor = "#c2410c" // red for erosion
	}

	vectorColor := opts.VectorColor
	if vectorColor == "" {
		vectorColor = "#1f6f8b" // blue for transport vectors
	}

	markerSize := opts.MarkerSize
	if markerSize <= 0 {
		markerSize = 6
	}

	vectorScale := opts.VectorScale
	if vectorScale <= 0 {
		vectorScale = 1000 // sensible default
	}

	n := len(opts.Points)
	if n == 0 || len(opts.SedimentStates) == 0 {
		fmt.Printf("❌ buildSedimentTransportVisualization: n=%d, states=%d -> return empty\n",
			n, len(opts.SedimentStates))
		return ""
	}

	fmt.Printf("🔧 buildSedimentTransportVisualization: n=%d points, %d states\n",
		n, len(opts.SedimentStates))

	// Draw accumulation and erosion points
	for i, state := range opts.SedimentStates {
		if i >= n {
			break
		}

		point := opts.Points[i]
		x := originX + (point.Lon-minLon)*scale
		y := originY + contentHeight - (point.Lat-minLat)*scale

		// Accumulation point
		if opts.ShowAccumulation && state.IsAccumulating {
			out.WriteString(fmt.Sprintf(
				`    <circle cx="%.2f" cy="%.2f" r="%.1f" fill="%s" fill-opacity="0.7" stroke="#fff" stroke-width="1.5"/>`+"\n",
				x, y, markerSize, accumulationColor,
			))
		}

		// Erosion point
		if opts.ShowErosion && state.IsEroding {
			out.WriteString(fmt.Sprintf(
				`    <circle cx="%.2f" cy="%.2f" r="%.1f" fill="%s" fill-opacity="0.7" stroke="#fff" stroke-width="1.5"/>`+"\n",
				x, y, markerSize, erosionColor,
			))
		}

		// Transport vectors (longshore drift)
		if opts.ShowTransportVectors && len(state.InTransitTo) >= 2 {
			// Draw vectors to neighbors
			// state.InTransitTo[0] = to prev, [1] = to next
			toPrev := state.InTransitTo[0]
			toNext := state.InTransitTo[1]

			// Find neighbor points
			prevIdx := (i - 1 + n) % n
			nextIdx := (i + 1) % n

			if prevIdx < n && toPrev > 1e-6 { // only significant flows
				prevPoint := opts.Points[prevIdx]
				prevX := originX + (prevPoint.Lon-minLon)*scale
				prevY := originY + contentHeight - (prevPoint.Lat-minLat)*scale

				// Calculate vector length based on sediment volume
				vectorLen := math.Sqrt(toPrev) * vectorScale * 0.05

				// Direction towards previous point
				dirX := prevX - x
				dirY := prevY - y
				length := math.Hypot(dirX, dirY)

				if length > 1e-6 {
					// Normalize and scale
					dirX /= length
					dirY /= length

					endX := x + dirX*vectorLen
					endY := y + dirY*vectorLen

					// Draw vector arrow
					out.WriteString(fmt.Sprintf(
						`    <line x1="%.2f" y1="%.2f" x2="%.2f" y2="%.2f" stroke="%s" stroke-width="1.5" stroke-opacity="0.6"/>`+"\n",
						x, y, endX, endY, vectorColor,
					))

					// Arrow head
					arrowSize := 3.0
					angle := math.Atan2(dirY, dirX)
					leftWingX := endX - arrowSize*math.Cos(angle-0.5)
					leftWingY := endY - arrowSize*math.Sin(angle-0.5)
					rightWingX := endX - arrowSize*math.Cos(angle+0.5)
					rightWingY := endY - arrowSize*math.Sin(angle+0.5)

					out.WriteString(fmt.Sprintf(
						`    <polygon points="%.2f,%.2f %.2f,%.2f %.2f,%.2f" fill="%s" fill-opacity="0.6"/>`+"\n",
						endX, endY, leftWingX, leftWingY, rightWingX, rightWingY, vectorColor,
					))
				}
			}

			if nextIdx < n && toNext > 1e-6 { // only significant flows
				nextPoint := opts.Points[nextIdx]
				nextX := originX + (nextPoint.Lon-minLon)*scale
				nextY := originY + contentHeight - (nextPoint.Lat-minLat)*scale

				// Calculate vector length based on sediment volume
				vectorLen := math.Sqrt(toNext) * vectorScale * 0.05

				// Direction towards next point
				dirX := nextX - x
				dirY := nextY - y
				length := math.Hypot(dirX, dirY)

				if length > 1e-6 {
					// Normalize and scale
					dirX /= length
					dirY /= length

					endX := x + dirX*vectorLen
					endY := y + dirY*vectorLen

					// Draw vector arrow
					out.WriteString(fmt.Sprintf(
						`    <line x1="%.2f" y1="%.2f" x2="%.2f" y2="%.2f" stroke="%s" stroke-width="1.5" stroke-opacity="0.6"/>`+"\n",
						x, y, endX, endY, vectorColor,
					))

					// Arrow head
					arrowSize := 3.0
					angle := math.Atan2(dirY, dirX)
					leftWingX := endX - arrowSize*math.Cos(angle-0.5)
					leftWingY := endY - arrowSize*math.Sin(angle-0.5)
					rightWingX := endX - arrowSize*math.Cos(angle+0.5)
					rightWingY := endY - arrowSize*math.Sin(angle+0.5)

					out.WriteString(fmt.Sprintf(
						`    <polygon points="%.2f,%.2f %.2f,%.2f %.2f,%.2f" fill="%s" fill-opacity="0.6"/>`+"\n",
						endX, endY, leftWingX, leftWingY, rightWingX, rightWingY, vectorColor,
					))
				}
			}
		}
	}

	result := out.String()
	erosionCircles := strings.Count(result, `fill="`+erosionColor+`"`)
	accumCircles := strings.Count(result, `fill="`+accumulationColor+`"`)
	vectorLines := strings.Count(result, `stroke="`+vectorColor+`"`)

	fmt.Printf("📊 Generated sediment SVG: %d erosion circles, %d accum circles, %d vector elements\n",
		erosionCircles, accumCircles, vectorLines)

	return result
}
