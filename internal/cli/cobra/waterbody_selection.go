package cobra

import (
	"fmt"

	"coastal-geometry/internal/domain/waterbody"
)

// selectedWaterbody проверяет идентификатор и сообщает, какая физическая
// схема допустима. Реки намеренно отклоняются до появления русловой модели,
// чтобы CERC не применялась к процессу, который определяется течением.
func selectedWaterbody(id string) (waterbody.Waterbody, error) {
	body, found := waterbody.Find(id)
	if !found {
		return waterbody.Waterbody{}, fmt.Errorf("водоём %q не найден; доступные варианты: lito waterbody list", id)
	}
	if body.Availability == waterbody.DifferentModel {
		return waterbody.Waterbody{}, fmt.Errorf("для %q нужна %s; волновая модель CERC не применима", body.Name, body.Model)
	}
	return body, nil
}
