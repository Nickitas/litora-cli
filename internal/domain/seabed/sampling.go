package seabed

import (
	"fmt"

	"coastal-geometry/internal/domain/geometry"
)

// RegularGridSampler адаптирует существующую регулярную BathymetryGrid к
// узловому контракту модели морского дна.
type RegularGridSampler struct {
	Grid *geometry.BathymetryGrid
}

// SampleElevation возвращает точную, билинейную или ближайшую выборку GEBCO с
// расстоянием до фактически использованной исходной точки.
func (sampler RegularGridSampler) SampleElevation(latitudeDeg, longitudeDeg, maxSourceDistanceM float64) (Sample, error) {
	if sampler.Grid == nil {
		return Sample{}, fmt.Errorf("регулярная батиметрическая сетка не задана")
	}
	details, err := sampler.Grid.SampleDepthDetailed(latitudeDeg, longitudeDeg, maxSourceDistanceM)
	if err != nil {
		return Sample{}, err
	}
	method := SamplingNearest
	if details.Exact {
		method = SamplingExact
	} else if details.Interpolated {
		method = SamplingBilinear
	}
	return Sample{
		ElevationM:        details.ElevationM,
		Method:            method,
		SourceDistanceM:   details.SourceDistanceMeters,
		SourceDistanceSet: true,
	}, nil
}
