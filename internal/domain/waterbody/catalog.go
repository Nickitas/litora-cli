package waterbody

import (
	"sort"
	"strings"
)

// Type определяет географический тип водоёма.
type Type string

const (
	// Sea обозначает море или морской участок побережья.
	Sea Type = "море"
	// Lake обозначает озеро или внутренний водоём с ветровым волнением.
	Lake Type = "озеро"
	// River обозначает реку с преобладающим русловым процессом.
	River Type = "река"
)

// Availability отражает возможность выполнить расчёт в текущей версии Lito.
type Availability string

const (
	// Automatic означает, что Lito самостоятельно получает проверенный стартовый набор.
	Automatic Availability = "автоматический стартовый набор"
	// UserData означает, что пользователь должен передать реальные данные участка.
	UserData Availability = "требуются данные пользователя"
	// DifferentModel означает, что для водоёма требуется ещё не реализованная модель.
	DifferentModel Availability = "требуется другая модель"
)

// Waterbody описывает доступный в каталоге водоём или участок водоёма РФ.
// Для CERC обязательно передавать фактические волнения и батиметрию даже при
// выборе водоёма из этого каталога.
type Waterbody struct {
	ID           string       `json:"id"`
	Name         string       `json:"name"`
	Region       string       `json:"region"`
	Type         Type         `json:"type"`
	Model        string       `json:"model"`
	Availability Availability `json:"availability"`
	Note         string       `json:"note"`
}

var catalog = []Waterbody{
	{
		ID:           "black-sea-sochi",
		Name:         "Чёрное море — Сочи",
		Region:       "Краснодарский край",
		Type:         Sea,
		Model:        "вдольбереговая CERC",
		Availability: Automatic,
		Note:         "Доступна автоматическая загрузка открытого стартового набора; для исследования замените его данными участка.",
	},
	{
		ID:           "sea-of-azov-russia",
		Name:         "Азовское море — российское побережье",
		Region:       "Краснодарский край, Ростовская область, Республика Крым",
		Type:         Sea,
		Model:        "вдольбереговая CERC",
		Availability: UserData,
		Note:         "Передайте локальный береговой сегмент, батиметрию и измеренный либо моделируемый волновой ряд.",
	},
	{
		ID:           "baltic-sea-kaliningrad",
		Name:         "Балтийское море — Калининградская область",
		Region:       "Калининградская область",
		Type:         Sea,
		Model:        "вдольбереговая CERC",
		Availability: UserData,
		Note:         "Расчёт доступен по реальным локальным данным; необходима калибровка коэффициента CERC.",
	},
	{
		ID:           "caspian-sea-dagestan",
		Name:         "Каспийское море — Дагестан",
		Region:       "Республика Дагестан",
		Type:         Sea,
		Model:        "вдольбереговая CERC",
		Availability: UserData,
		Note:         "Расчёт доступен по реальным локальным данным; учитывайте колебания уровня Каспия отдельно.",
	},
	{
		ID:           "lake-baikal",
		Name:         "Озеро Байкал",
		Region:       "Республика Бурятия, Иркутская область",
		Type:         Lake,
		Model:        "вдольбереговая CERC",
		Availability: UserData,
		Note:         "Применимо только к волновому береговому участку после локальной калибровки и с данными волн и глубин.",
	},
	{
		ID:           "lake-ladoga",
		Name:         "Ладожское озеро",
		Region:       "Республика Карелия, Ленинградская область",
		Type:         Lake,
		Model:        "вдольбереговая CERC",
		Availability: UserData,
		Note:         "Применимо только к волновому береговому участку после локальной калибровки и с данными волн и глубин.",
	},
	{
		ID:           "lake-onega",
		Name:         "Онежское озеро",
		Region:       "Республика Карелия, Ленинградская и Вологодская области",
		Type:         Lake,
		Model:        "вдольбереговая CERC",
		Availability: UserData,
		Note:         "Применимо только к волновому береговому участку после локальной калибровки и с данными волн и глубин.",
	},
	{
		ID:           "volga",
		Name:         "Река Волга",
		Region:       "Российская Федерация",
		Type:         River,
		Model:        "русловая модель берегообрушения",
		Availability: DifferentModel,
		Note:         "Требуется отдельная модель течения, уровней воды, русловых наносов и устойчивости откоса; CERC здесь не применяется.",
	},
	{
		ID:           "neva",
		Name:         "Река Нева",
		Region:       "Санкт-Петербург, Ленинградская область",
		Type:         River,
		Model:        "русловая модель берегообрушения",
		Availability: DifferentModel,
		Note:         "Требуется отдельная модель течения, уровней воды, русловых наносов и устойчивости откоса; CERC здесь не применяется.",
	},
	{
		ID:           "don",
		Name:         "Река Дон",
		Region:       "Российская Федерация",
		Type:         River,
		Model:        "русловая модель берегообрушения",
		Availability: DifferentModel,
		Note:         "Требуется отдельная модель течения, уровней воды, русловых наносов и устойчивости откоса; CERC здесь не применяется.",
	},
}

// List возвращает отсортированную независимую копию каталога.
func List() []Waterbody {
	result := append([]Waterbody(nil), catalog...)
	sort.Slice(result, func(i, j int) bool { return result[i].Name < result[j].Name })
	return result
}

// Find ищет водоём по стабильному идентификатору без учёта регистра и пробелов.
func Find(id string) (Waterbody, bool) {
	normalized := strings.ToLower(strings.TrimSpace(id))
	for _, body := range catalog {
		if body.ID == normalized {
			return body, true
		}
	}
	return Waterbody{}, false
}
