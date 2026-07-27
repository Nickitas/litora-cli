package benchmark

import (
	"fmt"
	"strings"

	"coastal-geometry/internal/domain/geometry"
)

// SiteSpec defines parameters for creating a new benchmark site
type SiteSpec struct {
	Name              string
	ID                string
	Region            string
	Country           string
	Description       string
	Bounds            Bounds
	CoastType         CoastType
	DominantLithology string
	MeanWaveHeight    float64
	MeanWavePeriod    float64
	MeanWaveDirection float64
	DataQuality       Quality
	ObservationYears  Range
	DataSource        string
	References        []string
}

// Validate returns an error if spec is incomplete or inconsistent
func (s *SiteSpec) Validate() error {
	if strings.TrimSpace(s.Name) == "" {
		return fmt.Errorf("name is required")
	}
	if strings.TrimSpace(s.ID) == "" {
		// Auto-generate ID from name
		s.ID = slugifyID(s.Name)
	}
	if s.Bounds.MinLat >= s.Bounds.MaxLat {
		return fmt.Errorf("bounds: min_lat must be < max_lat")
	}
	if s.Bounds.MinLon >= s.Bounds.MaxLon {
		return fmt.Errorf("bounds: min_lon must be < max_lon")
	}
	if s.Bounds.MinLat < -90 || s.Bounds.MaxLat > 90 {
		return fmt.Errorf("bounds: lat out of range [-90, 90]")
	}
	if s.Bounds.MinLon < -180 || s.Bounds.MaxLon > 180 {
		return fmt.Errorf("bounds: lon out of range [-180, 180]")
	}
	if s.ObservationYears.Min == 0 && s.ObservationYears.Max == 0 {
		// Default to recent decade
		s.ObservationYears = Range{Min: 2000, Max: 2024}
	}
	if s.ObservationYears.Min > s.ObservationYears.Max {
		return fmt.Errorf("observation_years: min must be <= max")
	}
	if s.CoastType == "" {
		s.CoastType = CoastTypeMixed
	}
	if s.DataQuality == "" {
		s.DataQuality = QualityMedium
	}
	return nil
}

// Build constructs a BenchmarkSite from the spec by extracting coastline from
// the provided full coastline
func (s *SiteSpec) Build(fullCoastline []geometry.LatLon) BenchmarkSite {
	site := BenchmarkSite{
		ID:                s.ID,
		Name:              s.Name,
		Region:            s.Region,
		Country:           s.Country,
		Description:       s.Description,
		Bounds:            s.Bounds,
		CoastType:         s.CoastType,
		DominantLithology: s.DominantLithology,
		MeanWaveHeight:    s.MeanWaveHeight,
		MeanWavePeriod:    s.MeanWavePeriod,
		MeanWaveDirection: s.MeanWaveDirection,
		DataSource:        s.DataSource,
		References:        s.References,
		DataQuality:       s.DataQuality,
		ObservationYears:  s.ObservationYears,
		ObservedErosion:   []ErosionObservation{},
	}

	if len(fullCoastline) > 0 {
		site.Coastline = ExtractCoastline(fullCoastline, s.Bounds.ToGeoBounds())
	}

	return site
}

// slugifyID converts a name to a valid ID slug
// Example: "San Francisco Bay" -> "san-francisco-bay"
func slugifyID(name string) string {
	var b strings.Builder
	prevDash := true // allow trimming leading dashes
	for _, r := range strings.TrimSpace(strings.ToLower(name)) {
		switch {
		case r >= 'a' && r <= 'z':
			b.WriteRune(r)
			prevDash = false
		case r >= '0' && r <= '9':
			b.WriteRune(r)
			prevDash = false
		case !prevDash:
			b.WriteByte('-')
			prevDash = true
		}
	}
	return strings.Trim(b.String(), "-")
}

// ParseBounds parses "min_lat,max_lat,min_lon,max_lon" string into Bounds
func ParseBounds(s string) (Bounds, error) {
	parts := strings.Split(s, ",")
	if len(parts) != 4 {
		return Bounds{}, fmt.Errorf("bounds must have 4 comma-separated values, got %d", len(parts))
	}
	var values [4]float64
	for i, p := range parts {
		p = strings.TrimSpace(p)
		_, err := fmt.Sscanf(p, "%f", &values[i])
		if err != nil {
			return Bounds{}, fmt.Errorf("invalid bound %q: %w", p, err)
		}
	}
	return Bounds{
		MinLat: values[0],
		MaxLat: values[1],
		MinLon: values[2],
		MaxLon: values[3],
	}, nil
}

// PresetCoastType returns a CoastType from a string
func PresetCoastType(s string) (CoastType, error) {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "sandy":
		return CoastTypeSandy, nil
	case "cliff":
		return CoastTypeCliff, nil
	case "rocky":
		return CoastTypeRocky, nil
	case "muddy":
		return CoastTypeMuddy, nil
	case "mixed":
		return CoastTypeMixed, nil
	case "artificial":
		return CoastTypeArtificial, nil
	case "":
		return CoastTypeMixed, nil
	default:
		return "", fmt.Errorf("unknown coast type %q (valid: sandy, cliff, rocky, muddy, mixed, artificial)", s)
	}
}

// PresetQuality returns a Quality from a string
func PresetQuality(s string) (Quality, error) {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "high":
		return QualityHigh, nil
	case "medium":
		return QualityMedium, nil
	case "low":
		return QualityLow, nil
	case "":
		return QualityMedium, nil
	default:
		return "", fmt.Errorf("unknown quality %q (valid: high, medium, low)", s)
	}
}
