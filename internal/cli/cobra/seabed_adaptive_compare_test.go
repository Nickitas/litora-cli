package cobra

import "testing"

func TestParseAdaptiveComparisonLevels(t *testing.T) {
	levels, err := parseAdaptiveComparisonLevels("detailed:200:1000,coarse:500:1000")
	if err != nil {
		t.Fatal(err)
	}
	if len(levels) != 2 || levels[0].Name != "Подробная сетка" || levels[1].MinimumSizeM != 500 {
		t.Fatalf("контрольные уровни разобраны неверно: %+v", levels)
	}
	for _, invalid := range []string{"", "bad", "same:500:500", "x:200:1000,x:300:1000"} {
		if _, err := parseAdaptiveComparisonLevels(invalid); err == nil {
			t.Fatalf("ожидалась ошибка для %q", invalid)
		}
	}
}
