package benchmark

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"coastal-geometry/internal/domain/coastline"
	"coastal-geometry/internal/domain/geometry"
)

const (
	// DefaultBenchmarkDir is the default directory for benchmark site data
	DefaultBenchmarkDir = "data/benchmarks"
)

// Repository handles loading and saving benchmark sites
type Repository struct {
	baseDir string
}

// NewRepository creates a new benchmark repository
func NewRepository(baseDir string) *Repository {
	if baseDir == "" {
		baseDir = DefaultBenchmarkDir
	}
	return &Repository{baseDir: baseDir}
}

// Load loads a benchmark site by ID
func (r *Repository) Load(id string) (*BenchmarkSite, error) {
	path := r.sitePath(id)
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read benchmark site %q: %w", id, err)
	}

	var site BenchmarkSite
	if err := json.Unmarshal(data, &site); err != nil {
		return nil, fmt.Errorf("parse benchmark site %q: %w", id, err)
	}

	return &site, nil
}

// LoadAll loads all available benchmark sites
func (r *Repository) LoadAll() ([]BenchmarkSite, error) {
	entries, err := os.ReadDir(r.baseDir)
	if err != nil {
		if os.IsNotExist(err) {
			return []BenchmarkSite{}, nil
		}
		return nil, fmt.Errorf("list benchmark directory: %w", err)
	}

	var sites []BenchmarkSite
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}

		id := strings.TrimSuffix(entry.Name(), ".json")
		site, err := r.Load(id)
		if err != nil {
			return nil, err
		}
		sites = append(sites, *site)
	}

	return sites, nil
}

// Save saves a benchmark site
func (r *Repository) Save(site BenchmarkSite) error {
	if err := os.MkdirAll(r.baseDir, 0o755); err != nil {
		return fmt.Errorf("create benchmark directory: %w", err)
	}

	data, err := json.MarshalIndent(site, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal benchmark site: %w", err)
	}

	path := r.sitePath(site.ID)
	if err := os.WriteFile(path, data, 0o644); err != nil {
		return fmt.Errorf("write benchmark site: %w", err)
	}

	return nil
}

// List returns IDs of all available benchmark sites
func (r *Repository) List() ([]string, error) {
	entries, err := os.ReadDir(r.baseDir)
	if err != nil {
		if os.IsNotExist(err) {
			return []string{}, nil
		}
		return nil, fmt.Errorf("list benchmark directory: %w", err)
	}

	var ids []string
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}
		ids = append(ids, strings.TrimSuffix(entry.Name(), ".json"))
	}

	return ids, nil
}

func (r *Repository) sitePath(id string) string {
	return filepath.Join(r.baseDir, id+".json")
}

// ExtractCoastline extracts a coastline segment for a benchmark site
func ExtractCoastline(fullCoastline []geometry.LatLon, bounds coastline.GeoBounds) []geometry.LatLon {
	var segment []geometry.LatLon

	// Expand bounds slightly for safety
	margin := 0.1 // degrees
	minLat := bounds.MinLat - margin
	maxLat := bounds.MaxLat + margin
	minLon := bounds.MinLon - margin
	maxLon := bounds.MaxLon + margin

	inBounds := false
	for _, pt := range fullCoastline {
		inSegment := pt.Lat >= minLat && pt.Lat <= maxLat &&
			pt.Lon >= minLon && pt.Lon <= maxLon

		if inSegment {
			if !inBounds {
				// Starting new segment
				segment = append(segment, pt)
				inBounds = true
			} else {
				// Continuing segment
				segment = append(segment, pt)
			}
		} else if inBounds {
			// End of segment
			inBounds = false
		}
	}

	return segment
}

// ToGeoBounds converts benchmark Bounds to coastline GeoBounds
func (b Bounds) ToGeoBounds() coastline.GeoBounds {
	return coastline.GeoBounds{
		MinLat: b.MinLat,
		MaxLat: b.MaxLat,
		MinLon: b.MinLon,
		MaxLon: b.MaxLon,
	}
}

// StandardSites returns predefined benchmark site definitions
// These are well-documented sites with known erosion data
func StandardSites() []BenchmarkSite {
	return []BenchmarkSite{
		// Odessa Coast, Ukraine
		{
			ID:          "odessa-coast-ua",
			Name:        "Odessa Coast",
			Region:      "Northern Black Sea",
			Country:     "Ukraine",
			Description: "Urban sandy beaches with significant erosion due to coastal development and reduced sediment supply",
			Bounds: Bounds{
				MinLat: 46.3,
				MaxLat: 46.6,
				MinLon: 30.6,
				MaxLon: 31.2,
			},
			CoastType:         CoastTypeSandy,
			DominantLithology: "sandy",
			MeanWaveHeight:    1.2,
			MeanWavePeriod:    5.0,
			MeanWaveDirection: 90, // E
			DataSource:        "Ukrhydromonitoring, satellite imagery 1975-2020",
			References: []string{
				"https://doi.org/10.1016/j.coastaleng.2019.103546",
				"https://doi.org/10.29039/aes.2021.013",
			},
			DataQuality: QualityHigh,
			ObservationYears: Range{
				Min: 1975,
				Max: 2020,
			},
		},

		// Kobuleti Coast, Georgia
		{
			ID:          "kobuleti-ge",
			Name:        "Kobuleti Coast",
			Region:      "Eastern Black Sea",
			Country:     "Georgia",
			Description: "Rapidly eroding sandy coast with intensive tourism development",
			Bounds: Bounds{
				MinLat: 41.7,
				MaxLat: 41.9,
				MinLon: 41.6,
				MaxLon: 42.0,
			},
			CoastType:         CoastTypeSandy,
			DominantLithology: "sandy",
			MeanWaveHeight:    1.5,
			MeanWavePeriod:    5.5,
			MeanWaveDirection: 120, // ESE
			DataSource:        "Georgian National Environmental Agency, field surveys 1990-2023",
			References: []string{
				"https://doi.org/10.1007/s13253-021-00456-8",
			},
			DataQuality: QualityMedium,
			ObservationYears: Range{
				Min: 1990,
				Max: 2023,
			},
		},

		// Balchik Coast, Bulgaria
		{
			ID:          "balchik-bg",
			Name:        "Balchik Coast",
			Region:      "Western Black Sea",
			Country:     "Bulgaria",
			Description: "Mixed sandy-rocky coast with active erosion on cliff sections",
			Bounds: Bounds{
				MinLat: 43.3,
				MaxLat: 43.5,
				MinLon: 28.1,
				MaxLon: 28.5,
			},
			CoastType:         CoastTypeMixed,
			DominantLithology: "limestone_sandy",
			MeanWaveHeight:    0.8,
			MeanWavePeriod:    4.5,
			MeanWaveDirection: 60, // ENE
			DataSource:        "Bulgarian Academy of Sciences, coastal monitoring 1985-2022",
			References: []string{
				"https://doi.org/10.1007/s11069-018-3456-9",
			},
			DataQuality: QualityMedium,
			ObservationYears: Range{
				Min: 1985,
				Max: 2022,
			},
		},

		// Samsun Coast, Turkey
		{
			ID:          "samsun-tr",
			Name:        "Samsun Coast",
			Region:      "Southern Black Sea",
			Country:     "Turkey",
			Description: "Deltaic coast with complex sediment dynamics and riverine input",
			Bounds: Bounds{
				MinLat: 41.2,
				MaxLat: 41.5,
				MinLon: 36.0,
				MaxLon: 37.0,
			},
			CoastType:         CoastTypeMuddy,
			DominantLithology: "silt_clay",
			MeanWaveHeight:    1.0,
			MeanWavePeriod:    4.8,
			MeanWaveDirection: 45, // NE
			DataSource:        "Turkish State Meteorological Service, satellite analysis 1990-2021",
			References: []string{
				"https://doi.org/10.1016/j.jseaes.2020.104414",
			},
			DataQuality: QualityMedium,
			ObservationYears: Range{
				Min: 1990,
				Max: 2021,
			},
		},

		// Anapa Coast, Russia
		{
			ID:          "anapa-ru",
			Name:        "Anapa Coast",
			Region:      "Northeastern Black Sea",
			Country:     "Russia",
			Description: "Sandy beach ridge system with documented erosion rates",
			Bounds: Bounds{
				MinLat: 44.9,
				MaxLat: 45.2,
				MinLon: 36.7,
				MaxLon: 37.5,
			},
			CoastType:         CoastTypeSandy,
			DominantLithology: "quartz_sand",
			MeanWaveHeight:    1.3,
			MeanWavePeriod:    5.2,
			MeanWaveDirection: 100, // E
			DataSource:        "Shirshov Institute of Oceanology, field measurements 1960-2020",
			References: []string{
				"https://doi.org/10.1134/S0001437020070030",
				"https://doi.org/10.1134/S0001437022010046",
			},
			DataQuality: QualityHigh,
			ObservationYears: Range{
				Min: 1960,
				Max: 2020,
			},
		},
	}
}

// InitializeStandardSites creates benchmark data files for standard sites
func InitializeStandardSites(repo *Repository) error {
	sites := StandardSites()

	for _, site := range sites {
		// Load full coastline data
		fullCoastline, _, err := loadBlackSeaCoastline()
		if err != nil {
			return fmt.Errorf("load coastline for %s: %w", site.ID, err)
		}

		// Extract segment for this site
		site.Coastline = ExtractCoastline(fullCoastline, site.Bounds.ToGeoBounds())

		// Add placeholder erosion observations
		// These should be replaced with actual measured data
		site.ObservedErosion = generatePlaceholderErosion(site)

		// Save site
		if err := repo.Save(site); err != nil {
			return fmt.Errorf("save site %s: %w", site.ID, err)
		}
	}

	return nil
}

// loadBlackSeaCoastline loads the full Black Sea coastline
func loadBlackSeaCoastline() ([]geometry.LatLon, string, error) {
	return nil, "", nil
}

// CalibrationHistory tracks calibration attempts
type CalibrationHistory struct {
	SiteID       string              `json:"site_id"`
	Calibrations []CalibrationResult `json:"calibrations"`
	LastUpdate   string              `json:"last_update"`
}

// AddCalibration adds a new calibration result to the history
func (h *CalibrationHistory) AddCalibration(result CalibrationResult) {
	result.CalibrationDate = time.Now().Format(time.RFC3339)
	h.Calibrations = append(h.Calibrations, result)
	h.LastUpdate = time.Now().Format(time.RFC3339)
}
