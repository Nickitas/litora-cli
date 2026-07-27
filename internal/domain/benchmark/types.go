package benchmark

import "coastal-geometry/internal/domain/geometry"

// BenchmarkSite represents a calibrated coastline segment with known erosion data
type BenchmarkSite struct {
	// Identification
	ID          string `json:"id"`
	Name        string `json:"name"`
	Region      string `json:"region"`
	Country     string `json:"country"`
	Description string `json:"description"`

	// Geometry
	Coastline []geometry.LatLon `json:"coastline"`
	Bounds    Bounds            `json:"bounds"`

	// Erosion observations
	ObservedErosion []ErosionObservation `json:"observed_erosion"`

	// Coastal characteristics
	CoastType         CoastType `json:"coast_type"`
	DominantLithology string    `json:"dominant_lithology"`
	MeanWaveHeight    float64   `json:"mean_wave_height_m"`
	MeanWavePeriod    float64   `json:"mean_wave_period_s"`
	MeanWaveDirection float64   `json:"mean_wave_direction_deg"`

	// Metadata
	DataSource       string   `json:"data_source"`
	References       []string `json:"references"`
	DataQuality      Quality  `json:"data_quality"`
	ObservationYears Range    `json:"observation_years"`
}

// Bounds represents the geographic bounds of a benchmark site
type Bounds struct {
	MinLat float64 `json:"min_lat"`
	MaxLat float64 `json:"max_lat"`
	MinLon float64 `json:"min_lon"`
	MaxLon float64 `json:"max_lon"`
}

// ErosionObservation represents measured erosion at a specific location and time
type ErosionObservation struct {
	// Location
	LatLon geometry.LatLon `json:"lat_lon"`

	// Measurement
	ShorelineChangeRate float64 `json:"shoreline_change_rate_m_per_year"` // positive = erosion, negative = accretion
	Uncertainty         float64 `json:"uncertainty_m_per_year"`

	// Timing
	StartDate string `json:"start_date"` // YYYY-MM-DD
	EndDate   string `json:"end_date"`   // YYYY-MM-DD

	// Method
	MeasurementMethod string `json:"measurement_method"` // "aerial_photography", "satellite", "field_survey"
	DataResolution    string `json:"data_resolution"`    // e.g. "10m"
}

// CoastType describes the dominant coastal characteristics
type CoastType string

const (
	CoastTypeSandy      CoastType = "sandy"      // sandy beaches
	CoastTypeCliff      CoastType = "cliff"      // erodible cliffs
	CoastTypeRocky      CoastType = "rocky"      // hard rock
	CoastTypeMuddy      CoastType = "muddy"      // mud flats
	CoastTypeMixed      CoastType = "mixed"      // mixed characteristics
	CoastTypeArtificial CoastType = "artificial" // engineered/seawalled
)

// Quality represents the data quality rating
type Quality string

const (
	QualityHigh   Quality = "high"   // direct measurements, <10% uncertainty
	QualityMedium Quality = "medium" // indirect measurements, 10-30% uncertainty
	QualityLow    Quality = "low"    // estimated, >30% uncertainty
)

// Range represents a numeric range
type Range struct {
	Min float64 `json:"min"`
	Max float64 `json:"max"`
}

// CalibrationResult stores the calibrated parameters for a site
type CalibrationResult struct {
	SiteID string `json:"site_id"`

	// Calibrated parameters
	ErosionStrength       float64 `json:"erosion_strength"`
	ErosionStrengthUncert float64 `json:"erosion_strength_uncertainty"`
	WaveHeightMultiplier  float64 `json:"wave_height_multiplier"`
	LithologyFactor       float64 `json:"lithology_factor"`

	// Validation metrics
	ValidationMetrics ValidationMetrics `json:"validation_metrics"`

	// Calibration metadata
	CalibrationDate string `json:"calibration_date"`
	Calibrator      string `json:"calibrator"`
	Notes           string `json:"notes"`
}

// ValidationMetrics compares modeled vs observed erosion
type ValidationMetrics struct {
	RMSE        float64 `json:"rmse_m_per_year"` // root mean square error
	MAE         float64 `json:"mae_m_per_year"`  // mean absolute error
	MBE         float64 `json:"mbe_m_per_year"`  // mean bias error
	RSquared    float64 `json:"r_squared"`       // coefficient of determination
	N           int     `json:"n_observations"`  // number of observations
	PValue      float64 `json:"p_value"`         // statistical significance
	Significant bool    `json:"significant"`     // is correlation significant?
}

// CalibrationParameters represents the parameter space for calibration
type CalibrationParameters struct {
	// Parameter ranges to search
	ErosionStrengthRange      Range `json:"erosion_strength_range"`
	WaveHeightMultiplierRange Range `json:"wave_height_multiplier_range"`
	LithologyFactorRange      Range `json:"lithology_factor_range"`

	// Search resolution
	Steps int `json:"steps"` // number of steps in each dimension

	// Optimization
	OptimizationTarget string `json:"optimization_target"` // "rmse", "mae", "rsquared"
	MaxIterations      int    `json:"max_iterations"`
}
