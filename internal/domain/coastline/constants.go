package coastline

import "time"

// Константы для путей к файлам по умолчанию
const (
	// DefaultCoastlineJSONPath - путь к файлу береговой линии по умолчанию
	DefaultCoastlineJSONPath = "data/black-sea.json"
	// DefaultCoastlineCacheDir - директория для кэширования данных береговой линии
	DefaultCoastlineCacheDir = "data/cache"
	// DefaultCoastlineSnapshotDir - директория для снимков береговой линии
	DefaultCoastlineSnapshotDir = "data/snapshots"
)

// Константы для HTTP-запросов
const (
	// defaultHTTPTimeout - таймаут HTTP-запросов по умолчанию
	defaultHTTPTimeout = 12 * time.Second
	// marineRegionsWFSURL - URL WFS-сервиса MarineRegions
	marineRegionsWFSURL = "https://geo.vliz.be/geoserver/MarineRegions/wfs"
	// blackSeaMarineRegionID - ID Чёрного моря в MarineRegions
	blackSeaMarineRegionID = 3319
)

// Константы для валидации координат
const (
	// minValidLatitude - минимальная допустимая широта
	minValidLatitude = -90.0
	// maxValidLatitude - максимальная допустимая широта
	maxValidLatitude = 90.0
	// minValidLongitude - минимальная допустимая долгота
	minValidLongitude = -180.0
	// maxValidLongitude - максимальная допустимая долгота
	maxValidLongitude = 180.0
	// pointPrecision - точность округления координат при создании ключа точки
	pointPrecision = 6
	// eps - эпсилон для сравнения чисел с плавающей точкой
	eps = 1e-9
)

// Константы для анализа геометрии
const (
	// longSegmentWarningKM - пороговая длина сегмента для предупреждения (км)
	longSegmentWarningKM = 450.0
	// locationMatchThreshold - пороговое расстояние для сопоставления точек с городами (градусы)
	locationMatchThreshold = 0.15
)

// Константы для форматирования вывода
const (
	// maxConsolePoints - максимальное количество точек для вывода в консоль
	maxConsolePoints = 30
)

// Пределы количества точек для оптимизации
const (
	// maxPointsForDuplicateCheck - максимальное количество точек для проверки дубликатов расположения
	maxPointsForDuplicateCheck = 200
)

// Права доступа к файлам
const (
	// dirPermissions - права доступа к директориям
	dirPermissions = 0o755
	// filePermissions - права доступа к файлам
	filePermissions = 0o644
)

// DefaultBlackSeaBounds - границы Чёрного моря по умолчанию
var DefaultBlackSeaBounds = GeoBounds{
	MinLat: 40.5,
	MaxLat: 46.8,
	MinLon: 27.0,
	MaxLon: 42.2,
}

// DefaultCoastlineGeoJSONURL - URL GeoJSON береговой линии по умолчанию
var DefaultCoastlineGeoJSONURL = buildDefaultCoastlineGeoJSONURL()
