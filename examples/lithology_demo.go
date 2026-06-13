package main

import (
	"fmt"
	"log"
	"coastal-geometry/internal/domain/geometry"
)

func main() {
	fmt.Println("=== Литологический модуль FRAES ===\n")

	// 1. Загрузка профиля Чёрного моря
	profile, err := geometry.LoadLithologyProfileFromFile("data/black-sea-lithology.json")
	if err != nil {
		log.Printf("Предупреждение: не удалось загрузить профиль: %v", err)
		fmt.Println("Используется дефолтный профиль...")
		profile = geometry.CreateDefaultBlackSeaProfile()
	}

	// 2. Статистика профиля
	stats := profile.GetStatistics()
	fmt.Printf("Профиль: %s\n", stats["name"])
	fmt.Printf("Точек замера: %v\n", stats["num_points"])
	fmt.Printf("Классов пород: %v\n", stats["num_classes"])
	fmt.Printf("Диапазон сопротивления: %.1f - %.1f\n",
		stats["resistance_min"], stats["resistance_max"])
	fmt.Printf("Среднее сопротивление: %.2f\n\n", stats["resistance_mean"])

	// 3. Проверка литологии в ключевых точках
	testPoints := []struct {
		lat, lon float64
		name     string
	}{
		{46.2, 33.0, "Крым (южный берег)"},
		{44.4, 38.0, "Анапа (бархан)"},
		{41.6, 38.0, "Турция (серпентинит)"},
		{45.0, 29.5, "Дельта Дуная"},
		{43.5, 28.0, "Болгария (Варна)"},
	}

	fmt.Println("Литология в ключевых точках:")
	fmt.Println("─────────────────────────────────────────────────────────────")
	for _, tp := range testPoints {
		lith := profile.GetLithologyAt(tp.lat, tp.lon)
		erosionRate := 10.0 / lith.Resistance // Базовая эрозия / сопротивление

		fmt.Printf("%s:\n", tp.name)
		fmt.Printf("  Координаты: %.2f°, %.2f°\n", tp.lat, tp.lon)
		fmt.Printf("  Порода: %s\n", lith.Class)
		fmt.Printf("  Сопротивление: %.1f\n", lith.Resistance)
		fmt.Printf("  Описание: %s\n", lith.Description)
		fmt.Printf("  Относительная эрозия: %.1f м/год (при базе 10 м)\n", erosionRate)
		fmt.Println()
	}

	// 4. Демонстрация IDW интерполяции
	fmt.Println("IDW интерполяция между точками:")
	fmt.Println("─────────────────────────────────────────────────────────────")

	crimeaPoint := struct{ lat, lon float64 }{46.0, 34.5}
	turkeyPoint := struct{ lat, lon float64 }{41.0, 40.0}

	midLat := (crimeaPoint.lat + turkeyPoint.lat) / 2
	midLon := (crimeaPoint.lon + turkeyPoint.lon) / 2

	lithCrimea := profile.GetLithologyAt(crimeaPoint.lat, crimeaPoint.lon)
	lithTurkey := profile.GetLithologyAt(turkeyPoint.lat, turkeyPoint.lon)
	lithMid := profile.GetLithologyAt(midLat, midLon)

	fmt.Printf("Крым (%.1f°, %.1f°): %s (R=%.1f)\n", crimeaPoint.lat, crimeaPoint.lon, lithCrimea.Class, lithCrimea.Resistance)
	fmt.Printf("Турция (%.1f°, %.1f°): %s (R=%.1f)\n", turkeyPoint.lat, turkeyPoint.lon, lithTurkey.Class, lithTurkey.Resistance)
	fmt.Printf("Середина (%.1f°, %.1f°): %s (R=%.1f) ← интерполяция\n", midLat, midLon, lithMid.Class, lithMid.Resistance)
	fmt.Println()

	// 5. Эффект на эрозию
	fmt.Println("Влияние литологии на скорость эрозии:")
	fmt.Println("─────────────────────────────────────────────────────────────")

	resistanceClasses := []struct {
		name string
		r    float64
	}{
		{"Дельта Дуная (ил)", 0.9},
		{"Глина", 1.2},
		{"Песчаник", 2.8},
		{"Известняк", 4.5},
		{"Вулканит", 7.0},
		{"Серпентинит", 9.0},
	}

	baseErosion := 10.0

	fmt.Printf("Базовая эрозия: %.1f м/год\n\n", baseErosion)
	for _, rc := range resistanceClasses {
		actualErosion := baseErosion / rc.r
		fmt.Printf("  %s (R=%.1f): %.1f м/год (%.0f%% от базы)\n",
			rc.name, rc.r, actualErosion, (actualErosion/baseErosion)*100)
	}

	fmt.Println("\nВывод: Разница в эрозии между серпентинитом и дельтой — ~10x")
}
