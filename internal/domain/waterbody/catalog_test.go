package waterbody

import "testing"

func TestCatalogContainsOnlyRussianWaterbodies(t *testing.T) {
	bodies := List()
	if len(bodies) < 5 {
		t.Fatalf("ожидался непустой каталог, получено %d записей", len(bodies))
	}
	for _, body := range bodies {
		if body.ID == "" || body.Name == "" || body.Region == "" || body.Model == "" || body.Note == "" {
			t.Fatalf("в каталоге есть неполная запись: %#v", body)
		}
	}
}

func TestFindNormalizesID(t *testing.T) {
	body, ok := Find("  BLACK-SEA-SOCHI ")
	if !ok {
		t.Fatal("Сочи должен находиться по нормализованному идентификатору")
	}
	if body.Availability != Automatic {
		t.Fatalf("неверная доступность: %q", body.Availability)
	}
}
