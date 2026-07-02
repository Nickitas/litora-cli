package svg

import (
	"fmt"
	"math"
	"os"
	"strings"
)

// EnhancedDocument extends Document with additional map elements
type EnhancedDocument struct {
	Document
	GridOptions      *GridOptions
	CompassOptions   *CompassOptions
	MarkerOptions    *MarkerOptions
	IsolineOptions   *IsolineOptions
}

// GridOptions configures coordinate grid display
type GridOptions struct {
	Show           bool
	ShowLatLabels  bool
	ShowLonLabels  bool
	LatStep        float64  // degrees between latitude lines
	LonStep        float64  // degrees between longitude lines
	LineColor      string
	LabelColor     string
	FontSize       float64
	Opacity        float64
	DashArray      string
}

// CompassOptions configures compass/wind rose display
type CompassOptions struct {
	Show           bool
	X              float64  // SVG x position (0 = auto position)
	Y              float64  // SVG y position (0 = auto position)
	Size           float64  // pixels
	WindDirection  float64  // degrees from north (for wave erosion)
	ShowWindArrow  bool
	Label          string   // optional label text
	Style          string   // "modern", "classic", "minimal"
}

// MarkerOptions configures key point markers
type MarkerOptions struct {
	Show           bool
	Markers        []Marker
	DefaultSize    float64
	DefaultColor   string
	ShowLabels     bool
}

// Marker represents a labeled point on the map
type Marker struct {
	Lat          float64
	Lon          float64
	Label        string
	Color        string
	Size         float64
	Shape        string  // "circle", "square", "diamond", "triangle"
	Tooltip      string
}

// IsolineOptions configures depth/height contour lines
type IsolineOptions struct {
	Show           bool
	BathymetryGrid interface{ GetIsolinePoints(depthStep float64) []Isoline }
	DepthStep      float64  // meters between contour lines
	MinDepth       float64
	MaxDepth       float64
	LineColor      string
	LabelColor     string
	LineWidth      float64
	Opacity        float64
	LabelInterval  int      // label every N lines
}

// Isoline represents a contour line
type Isoline struct {
	Depth  float64
	Points []struct {
		Lat float64
		Lon float64
	}
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

	plotWidth := float64(canvasWidth) - sidebarWidth - 2*padding
	header, headerBottom := buildHeader(doc.Title, doc.Subtitle, padding, plotWidth)
	plotTopY := headerBottom + 24
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
	var gridElements, compassElements, markerElements, isolineElements string

	if doc.GridOptions != nil && doc.GridOptions.Show {
		gridElements = buildCoordinateGrid(*doc.GridOptions, minLat, maxLat, minLon, maxLon, originX, originY, contentWidth, contentHeight, scale)
	}

	if doc.CompassOptions != nil && doc.CompassOptions.Show {
		compassElements = buildCompass(*doc.CompassOptions, originX, originY, contentWidth, contentHeight, plotWidth, plotHeight)
	}

	if doc.MarkerOptions != nil && doc.MarkerOptions.Show {
		markerElements = buildMarkers(*doc.MarkerOptions, doc.MarkerOptions.Markers, minLat, minLon, originX, originY, contentHeight, scale)
	}

	if doc.IsolineOptions != nil && doc.IsolineOptions.Show {
		isolineElements = buildIsolines(*doc.IsolineOptions, minLat, minLon, originX, originY, contentHeight, scale)
	}

	sidebarX := padding + plotWidth + 28
	legend, legendBottom := buildLegend(doc.Layers, sidebarX, plotTopY+10, sidebarWidth-56)
	statCards, statCardsBottom := buildStatCards(doc.StatCards, sidebarX, legendBottom+20, sidebarWidth-56)
	charts, chartsBottom := buildCharts(doc.Charts, sidebarX, statCardsBottom+18, sidebarWidth-56)
	alerts, alertsBottom := buildAlerts(doc.Alerts, sidebarX, chartsBottom+18, sidebarWidth-56)
	metaStartY := math.Max(608.0, alertsBottom+26)
	meta, metaBottom := buildMetaCard(doc.Meta, sidebarX, metaStartY, sidebarWidth-56)

	documentHeight := canvasHeight
	sidebarBottom := max(max(legendBottom, statCardsBottom), max(chartsBottom, alertsBottom))
	sidebarBottom = max(sidebarBottom, metaBottom)
	requiredHeight := int(math.Ceil(sidebarBottom + padding))
	if requiredHeight > documentHeight {
		documentHeight = requiredHeight
	}

	scaleBar := buildScaleBar(minLat, maxLat, minLon, maxLon, plotWidth, scale, padding, float64(documentHeight)-padding-scaleBarYGap)

	svg := fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8"?>
<svg xmlns="http://www.w3.org/2000/svg" width="%d" height="%d" viewBox="0 0 %d %d">
  <rect width="100%%" height="100%%" fill="#f7f4ea"/>
  <rect x="20" y="20" width="%d" height="%d" rx="28" fill="#fcfbf7" stroke="#d6d0c4"/>
  <rect x="%.0f" y="20" width="%.0f" height="%d" rx="24" fill="#f0ece2" stroke="#d6d0c4"/>
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
		padding+plotWidth+8, sidebarWidth-16, documentHeight-40,
		header,
		gridElements,
		isolineElements,
		layers.String(),
		highlights.String(),
		markerElements,
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
		size = 32  // Reduced to 32 for more compact display
	}

	// Auto position: bottom-right corner, below the plot area to avoid overlap
	x := opts.X
	if x <= 0 {
		x = originX + contentWidth - size - 10  // Right side with minimal padding
	}
	y := opts.Y
	if y <= 0 {
		// Position below the plot area, well below to avoid overlap
		y = originY + contentHeight + size + 30
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
			`      <text x="0" y="%.1f" font-family="Helvetica, Arial, sans-serif" font-size="10" fill="#4f6d7a" text-anchor="middle" transform="rotate(-45, %.0f, %.0f)">%s</text>`+"\n",
			radius+14, label,
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
			radius+14, label,
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

	// N indicator
	out.WriteString(fmt.Sprintf(
		`      <text x="0" y="%.1f" font-family="Helvetica, Arial, sans-serif" font-size="11" font-weight="700" fill="#16324f" text-anchor="middle">N</text>`+"\n",
		-radius+12,
	))

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
			radius+12, label,
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

// buildIsolines creates depth contour lines
func buildIsolines(opts IsolineOptions, minLat, minLon, originX, originY, contentHeight, scale float64) string {
	var out strings.Builder

	lineColor := opts.LineColor
	if lineColor == "" {
		lineColor = "#4a90b8"
	}

	labelColor := opts.LabelColor
	if labelColor == "" {
		labelColor = "#2c5f7a"
	}

	lineWidth := opts.LineWidth
	if lineWidth <= 0 {
		lineWidth = 1.2
	}

	opacity := opts.Opacity
	if opacity <= 0 {
		opacity = 0.5
	}

	depthStep := opts.DepthStep
	if depthStep <= 0 {
		depthStep = 50 // 50 meters
	}

	// Get isolines from bathymetry grid
	if opts.BathymetryGrid == nil {
		return ""
	}

	isolines := opts.BathymetryGrid.GetIsolinePoints(depthStep)

	labelInterval := opts.LabelInterval
	if labelInterval <= 0 {
		labelInterval = 2 // Label every 2nd line
	}

	for i, iso := range isolines {
		if len(iso.Points) < 2 {
			continue
		}

		// Build polyline points
		var polylinePoints strings.Builder
		for j, p := range iso.Points {
			if j > 0 {
				polylinePoints.WriteByte(' ')
			}
			px := originX + (p.Lon-minLon)*scale
			py := originY + contentHeight - (p.Lat-minLat)*scale
			polylinePoints.WriteString(fmt.Sprintf("%.2f,%.2f", px, py))
		}

		// Draw contour line
		out.WriteString(fmt.Sprintf(
			`    <polyline fill="none" stroke="%s" stroke-width="%.2f" stroke-opacity="%.2f" stroke-linejoin="round" points="%s"/>`+"\n",
			lineColor, lineWidth, opacity, polylinePoints.String(),
		))

		// Add depth label at interval
		if i%labelInterval == 0 && len(iso.Points) > 10 {
			midIdx := len(iso.Points) / 2
			midPoint := iso.Points[midIdx]
			labelX := originX + (midPoint.Lon-minLon)*scale
			labelY := originY + contentHeight - (midPoint.Lat-minLat)*scale

			depthLabel := fmt.Sprintf("%.0fm", iso.Depth)
			out.WriteString(fmt.Sprintf(
				`    <text x="%.2f" y="%.2f" font-family="Helvetica, Arial, sans-serif" font-size="9" fill="%s" text-anchor="middle" transform="rotate(-15, %.2f, %.2f)">%s</text>`+"\n",
				labelX, labelY, labelColor, labelX, labelY, depthLabel,
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
