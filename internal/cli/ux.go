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
	case cmdDimension:
		return cmdModel + " " + cmdDimension
	case cmdErosion:
		return cmdModel + " " + cmdErosion
	default:
		return command
	}
}

func getCommandUX(command string) commandUX {
	switch command {
	case cmdSource:
		return commandUX{
			Mode:        "проверка источника данных",
			Summary:     "показывает метаданные источника и сохраняет локальный сырой snapshot выбранного набора",
			RuntimeNote: "команда анализирует сырой payload источника и сохраняет snapshot без запуска метрик береговой линии и модельных этапов",
		}
	case cmdCoastline:
		return commandUX{
			Mode:        "анализ реальных данных",
			Summary:     "выводит геометрию и геодезические метрики для самой загруженной береговой линии",
			RuntimeNote: "показанная длина и `coastline.svg` соответствуют загруженной береговой линии без модельных преобразований",
		}
	case cmdDimension:
		return commandUX{
			Mode:        "фрактальный анализ",
			Summary:     "оценивает box-counting размерность на organic-итерациях, построенных от загруженной береговой линии",
			RuntimeNote: "диагностика размерности относится к organic-модели, используемой для фрактального анализа",
		}
	case cmdErosion:
		return commandUX{
			Mode:        "геоморфологическая модель",
			Summary:     "моделирует направленную волновую эрозию по fetch, экспозиции и shelter-эффекту бухт",
			RuntimeNote: "каждый шаг оценивает открытость сегмента к волнам и сглаживает берег сильнее на открытых мысах, чем в защищённых врезах",
		}
	case cmdAll:
		return commandUX{
			Mode:        "смешанный сценарий",
			Summary:     "запускает валидацию геометрии, фрактальный анализ и geomorphological моделирование эрозии",
			RuntimeNote: "первый этап относится к самой загруженной береговой линии; последующие этапы являются научными моделями, построенными на этой базовой геометрии",
		}
	default:
		return commandUX{}
	}
}
