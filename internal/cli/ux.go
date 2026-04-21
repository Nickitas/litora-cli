package cli

type commandUX struct {
	Mode        string
	Summary     string
	RuntimeNote string
}

func canonicalCommandPath(command string) string {
	switch command {
	case cmdSource:
		return cmdSource
	case cmdCoastline:
		return cmdReal + " " + cmdCoastline
	case cmdParadox:
		return cmdModel + " " + cmdParadox
	case cmdKoch:
		return cmdModel + " " + cmdKoch
	case cmdKochOrganic:
		return cmdModel + " " + cmdKochOrganic
	case cmdDimension:
		return cmdModel + " " + cmdDimension
	case cmdErosion:
		return cmdModel + " " + cmdErosion
	default:
		return command
	}
}

func legacyAlias(command string) string {
	switch command {
	case cmdCoastline, cmdParadox, cmdKoch, cmdKochOrganic, cmdDimension, cmdErosion:
		return command
	default:
		return ""
	}
}

func getCommandUX(command string) commandUX {
	switch command {
	case cmdSource:
		return commandUX{
			Mode:        "проверка источника данных",
			Summary:     "показывает метаданные источника и сохраняет локальный сырой snapshot выбранного набора",
			RuntimeNote: "команда анализирует сырой payload источника и сохраняет snapshot без запуска метрик береговой линии и синтетических модельных этапов",
		}
	case cmdCoastline:
		return commandUX{
			Mode:        "анализ реальных данных",
			Summary:     "выводит геометрию и геодезические метрики для самой загруженной береговой линии",
			RuntimeNote: "показанная длина и `coastline.svg` соответствуют загруженной береговой линии без синтетических преобразований",
		}
	case cmdParadox:
		return commandUX{
			Mode:        "синтетическая демонстрация",
			Summary:     "использует загруженную береговую линию только как базовую полилинию, а затем добавляет синтетические детали для демонстрации парадокса",
			RuntimeNote: "итерация 0 соответствует загруженной береговой линии; более высокие уровни являются синтетическими уточнениями, а не прямыми измерениями реального мира",
		}
	case cmdKoch:
		return commandUX{
			Mode:        "синтетическая демонстрация",
			Summary:     "использует загруженную береговую линию как базовую полилинию для классической модели Коха",
			RuntimeNote: "итерация 0 соответствует загруженной береговой линии; последующие итерации Коха являются синтетическими модельными кривыми, построенными на её основе",
		}
	case cmdKochOrganic:
		return commandUX{
			Mode:        "синтетическая демонстрация",
			Summary:     "использует загруженную береговую линию как базовую полилинию для organic-фрактальной модели",
			RuntimeNote: "итерация 0 соответствует загруженной береговой линии; последующие итерации синтетические и настраиваются jitter-параметрами",
		}
	case cmdDimension:
		return commandUX{
			Mode:        "синтетическая демонстрация",
			Summary:     "оценивает box-counting размерность на синтетических organic-итерациях, построенных от загруженной береговой линии",
			RuntimeNote: "диагностика размерности относится к сгенерированной organic-модели, а не напрямую к сырой геометрии береговой линии",
		}
	case cmdErosion:
		return commandUX{
			Mode:        "синтетическая демонстрация",
			Summary:     "моделирует направленную волновую эрозию по fetch, экспозиции и shelter-эффекту бухт",
			RuntimeNote: "каждый шаг оценивает открытость сегмента к волнам и сглаживает берег сильнее на открытых мысах, чем в защищённых врезах",
		}
	case cmdAll:
		return commandUX{
			Mode:        "смешанный сценарий",
			Summary:     "начинает с реальных метрик береговой линии, затем запускает синтетические этапы парадокса и фрактальной модели",
			RuntimeNote: "первый этап относится к самой загруженной береговой линии; последующие этапы являются синтетическими демонстрациями, построенными на этой базовой геометрии",
		}
	default:
		return commandUX{}
	}
}
