package geometry

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
	"time"
)

// WaveCondition описывает одно наблюдаемое или рассчитанное состояние волн в
// глубокой воде. Направление задаётся как направление, ОТКУДА приходят волны,
// по часовой стрелке от севера.
type WaveCondition struct {
	Time                   time.Time `json:"time,omitempty"`
	DurationHours          float64   `json:"duration_hours"`
	SignificantWaveHeightM float64   `json:"hs_m"`
	PeakPeriodSeconds      float64   `json:"tp_s"`
	DirectionFromDeg       float64   `json:"direction_from_deg"`
}

// WaveClimate объединяет временной ряд волн и его происхождение. Источник
// обязателен для исследовательского расчёта, чтобы результат был проверяемым.
type WaveClimate struct {
	Source     string          `json:"source"`
	Conditions []WaveCondition `json:"conditions"`
}

// LoadWaveClimate загружает волновой ряд из JSON либо CSV. JSON имеет поля
// source и conditions. CSV должен содержать столбцы duration_hours, hs_m,
// tp_s, direction_from_deg и необязательный time в RFC3339.
func LoadWaveClimate(path, sourceOverride string) (WaveClimate, error) {
	file, err := os.Open(path)
	if err != nil {
		return WaveClimate{}, fmt.Errorf("открытие волнового ряда %q: %w", path, err)
	}
	defer file.Close()

	var climate WaveClimate
	if strings.HasSuffix(strings.ToLower(path), ".csv") {
		climate, err = loadWaveClimateCSV(file)
	} else {
		var data json.RawMessage
		data, err = io.ReadAll(file)
		if err == nil {
			err = json.Unmarshal(data, &climate)
		}
		if err == nil && len(climate.Conditions) == 0 {
			climate, err = loadOpenMeteoMarineClimate(data)
		}
	}
	if err != nil {
		return WaveClimate{}, fmt.Errorf("чтение волнового ряда %q: %w", path, err)
	}
	if sourceOverride != "" {
		climate.Source = sourceOverride
	}
	if err := climate.Validate(); err != nil {
		return WaveClimate{}, err
	}
	return climate, nil
}

// loadOpenMeteoMarineClimate адаптирует открытый почасовой ответ Open-Meteo
// Marine к формату WaveClimate. Происхождение задаётся флагом --wave-source.
func loadOpenMeteoMarineClimate(data []byte) (WaveClimate, error) {
	var response struct {
		Hourly struct {
			Time          []string  `json:"time"`
			WaveHeight    []float64 `json:"wave_height"`
			WavePeriod    []float64 `json:"wave_period"`
			WaveDirection []float64 `json:"wave_direction"`
		} `json:"hourly"`
	}
	if err := json.Unmarshal(data, &response); err != nil {
		return WaveClimate{}, err
	}
	count := len(response.Hourly.Time)
	if count == 0 || len(response.Hourly.WaveHeight) != count || len(response.Hourly.WavePeriod) != count || len(response.Hourly.WaveDirection) != count {
		return WaveClimate{}, fmt.Errorf("Open-Meteo Marine не содержит согласованных почасовых wave_height, wave_period и wave_direction")
	}
	climate := WaveClimate{Conditions: make([]WaveCondition, 0, count)}
	for i, rawTime := range response.Hourly.Time {
		timestamp, err := time.Parse("2006-01-02T15:04", rawTime)
		if err != nil {
			return WaveClimate{}, fmt.Errorf("Open-Meteo Marine, time[%d]: %w", i, err)
		}
		climate.Conditions = append(climate.Conditions, WaveCondition{
			Time:                   timestamp.UTC(),
			DurationHours:          1,
			SignificantWaveHeightM: response.Hourly.WaveHeight[i],
			PeakPeriodSeconds:      response.Hourly.WavePeriod[i],
			DirectionFromDeg:       response.Hourly.WaveDirection[i],
		})
	}
	return climate, nil
}

func loadWaveClimateCSV(reader io.Reader) (WaveClimate, error) {
	csvReader := csv.NewReader(reader)
	header, err := csvReader.Read()
	if err != nil {
		return WaveClimate{}, err
	}
	indexes := make(map[string]int, len(header))
	for i, name := range header {
		indexes[strings.TrimSpace(name)] = i
	}
	for _, name := range []string{"duration_hours", "hs_m", "tp_s", "direction_from_deg"} {
		if _, ok := indexes[name]; !ok {
			return WaveClimate{}, fmt.Errorf("в CSV отсутствует обязательный столбец %q", name)
		}
	}

	climate := WaveClimate{}
	for rowNumber := 2; ; rowNumber++ {
		record, readErr := csvReader.Read()
		if readErr == io.EOF {
			break
		}
		if readErr != nil {
			return WaveClimate{}, readErr
		}
		value := func(column string) (float64, error) {
			index := indexes[column]
			if index >= len(record) {
				return 0, fmt.Errorf("строка %d короче заголовка", rowNumber)
			}
			return strconv.ParseFloat(strings.TrimSpace(record[index]), 64)
		}
		condition := WaveCondition{}
		if condition.DurationHours, err = value("duration_hours"); err != nil {
			return WaveClimate{}, fmt.Errorf("строка %d, duration_hours: %w", rowNumber, err)
		}
		if condition.SignificantWaveHeightM, err = value("hs_m"); err != nil {
			return WaveClimate{}, fmt.Errorf("строка %d, hs_m: %w", rowNumber, err)
		}
		if condition.PeakPeriodSeconds, err = value("tp_s"); err != nil {
			return WaveClimate{}, fmt.Errorf("строка %d, tp_s: %w", rowNumber, err)
		}
		if condition.DirectionFromDeg, err = value("direction_from_deg"); err != nil {
			return WaveClimate{}, fmt.Errorf("строка %d, direction_from_deg: %w", rowNumber, err)
		}
		if index, ok := indexes["time"]; ok && index < len(record) && strings.TrimSpace(record[index]) != "" {
			condition.Time, err = time.Parse(time.RFC3339, strings.TrimSpace(record[index]))
			if err != nil {
				return WaveClimate{}, fmt.Errorf("строка %d, time: %w", rowNumber, err)
			}
		}
		climate.Conditions = append(climate.Conditions, condition)
	}
	return climate, nil
}

// Validate проверяет физическую допустимость волнового ряда до расчёта.
func (c WaveClimate) Validate() error {
	if strings.TrimSpace(c.Source) == "" {
		return fmt.Errorf("для волнового ряда укажите источник данных")
	}
	if len(c.Conditions) == 0 {
		return fmt.Errorf("волновой ряд не содержит состояний")
	}
	hasTimestamp := false
	hasMissingTimestamp := false
	for i, condition := range c.Conditions {
		if condition.DurationHours <= 0 || condition.DurationHours > 24*31 {
			return fmt.Errorf("состояние волн %d: duration_hours должно быть в диапазоне (0; 744]", i)
		}
		if condition.SignificantWaveHeightM <= 0 || condition.SignificantWaveHeightM > 30 {
			return fmt.Errorf("состояние волн %d: hs_m должно быть в диапазоне (0; 30]", i)
		}
		if condition.PeakPeriodSeconds < 1 || condition.PeakPeriodSeconds > 30 {
			return fmt.Errorf("состояние волн %d: tp_s должно быть в диапазоне [1; 30]", i)
		}
		if condition.DirectionFromDeg < 0 || condition.DirectionFromDeg >= 360 {
			return fmt.Errorf("состояние волн %d: direction_from_deg должно быть в диапазоне [0; 360)", i)
		}
		if condition.Time.IsZero() {
			hasMissingTimestamp = true
		} else {
			hasTimestamp = true
		}
		if i > 0 && !condition.Time.IsZero() && !c.Conditions[i-1].Time.IsZero() {
			previous := c.Conditions[i-1]
			if !condition.Time.After(previous.Time) {
				return fmt.Errorf("состояние волн %d: временная метка должна быть строго позже предыдущей", i)
			}
			if condition.Time.Before(previous.Time.Add(time.Duration(previous.DurationHours * float64(time.Hour)))) {
				return fmt.Errorf("состояние волн %d: интервал перекрывает предыдущее состояние", i)
			}
		}
	}
	if hasTimestamp && hasMissingTimestamp {
		return fmt.Errorf("во временном ряду либо задайте time для всех состояний, либо не задавайте ни для одного")
	}
	return nil
}
