.PHONY: all build test clean bathymetry help erosion erosion-with-bathymetry build-all build-release

# Переменные
BINARY_NAME=lito
GO=go
BATHYMETRY_FILE=data/black-sea-bathymetry.json
VERSION=v1.2
BUILD_DIR=build
DIST_DIR=dist

all: build

# Сборка
build:
	@echo "🔨 Сборка $(BINARY_NAME)..."
	$(GO) build -o $(BINARY_NAME) ./cmd/lito
	@echo "✓ Готово: ./$(BINARY_NAME)"

# Тесты
test:
	@echo "🧪 Запуск тестов..."
	$(GO) test -v ./...

# Быстрая проверка
test-quick:
	@echo "⚡ Быстрый тест..."
	$(GO) test -v ./internal/domain/geometry/... -run "TestBathymetry|TestWaveErosion"

# Очистка
clean:
	@echo "🧹 Очистка..."
	rm -f $(BINARY_NAME)
	rm -f data/*.nc
	@echo "✓ Очищено"

# Загрузка батиметрии
bathymetry:
	@echo "📊 Загрузка батиметрии GEBCO для Чёрного моря..."
	@if [ -f "$(BATHYMETRY_FILE)" ]; then \
		echo "⚠️  Файл уже существует: $(BATHYMETRY_FILE)"; \
		read -p "Обновить? (y/N): " answer; \
		[ "$$answer" = "y" ] || (echo "Пропуск."; exit 0); \
	fi
	@$(GO) run cmd/bathymetry/main.go download

# Загрузка батиметрии (через Python)
bathymetry-python:
	@echo "📊 Загрузка батиметрии через Python..."
	@bash cmd/bathymetry/convert/download_bathymetry.sh

# Конвертация батиметрии
bathymetry-convert:
	@echo "🔄 Конвертация батиметрических данных..."
	@if [ -z "$(INPUT)" ] || [ -z "$(OUTPUT)" ]; then \
		echo "❌ Укажите INPUT и OUTPUT"; \
		echo "Пример: make bathymetry-convert INPUT=file.nc OUTPUT=data.json RESOLUTION=0.01 BOUNDS='40.5 46.5 27.5 42.5'"; \
		exit 1; \
	fi
	@$(GO) run cmd/bathymetry/main.go convert --input "$(INPUT)" --output "$(OUTPUT)" --resolution "$(RESOLUTION)" --bounds $(BOUNDS)

# Проверка батиметрии
check-bathymetry:
	@if [ -f "$(BATHYMETRY_FILE)" ]; then \
		echo "✓ Батиметрия существует: $(BATHYMETRY_FILE)"; \
		stats=$$(file $(BATHYMETRY_FILE)); \
		size=$$(du -h $(BATHYMETRY_FILE) | cut -f1); \
		echo "  Размер: $$size"; \
	else \
		echo "❌ Батиметрия не найдена: $(BATHYMETRY_FILE)"; \
		echo ""; \
		echo "Для загрузки выполните:"; \
		echo "  make bathymetry"; \
		false; \
	fi

# Эрозия без батиметрии
erosion:
	@echo "🌊 Запуск волновой эрозии (без батиметрии)..."
	./$(BINARY_NAME) model erosion \
		--steps 5 \
		--erosion-strength 30 \
		--wave-direction 0 \
		--wind-speed 12 \
		--output ./output/erosion-no-bathymetry

# Эрозия с батиметрией (автоматическая загрузка если нужно)
erosion-with-bathymetry:
	@echo "🌊 Запуск волновой эрозии (с батиметрией)..."
	@if [ ! -f "$(BATHYMETRY_FILE)" ]; then \
		echo "⚠️  Батиметрия не найдена, загрузка..."; \
		$(MAKE) bathymetry; \
	fi
	@./$(BINARY_NAME) model erosion \
		--steps 5 \
		--erosion-strength 30 \
		--wave-direction 0 \
		--wind-speed 12 \
		--bathymetry $(BATHYMETRY_FILE) \
		--output ./output/erosion-with-bathymetry

# Полный цикл проверки
demo: clean build bathymetry
	@echo ""
	@echo "🚀 Запуск полного сценария (all) с волновой эрозией и батиметрией..."
	@./$(BINARY_NAME) all --iterations 3 --steps 5 --erosion-strength 30 --bathymetry $(BATHYMETRY_FILE) --output ./output/demo
	@echo ""
	@echo "🎉 Демо завершено!"
	@echo "Результаты: ./output/demo/"
	@echo ""
	@echo "Созданные файлы:"
	@echo "  - coastline.svg (валидация геометрии береговой линии)"
	@echo "  - dimension_iter_*.svg (фрактальный анализ по итерациям)"
	@echo "  - erosion_step_*.svg (волновая эрозия с батиметрией)"

# Кросс-платформенная сборка
build-all:
	@echo "🔨 Сборка для всех платформ..."
	@mkdir -p $(BUILD_DIR)

	@echo "  📦 Linux (amd64)..."
	@GOOS=linux GOARCH=amd64 $(GO) build -o $(BUILD_DIR)/$(BINARY_NAME)-linux-amd64 ./cmd/lito
	@echo "    ✓ $(BUILD_DIR)/$(BINARY_NAME)-linux-amd64"

	@echo "  📦 Windows (amd64)..."
	@GOOS=windows GOARCH=amd64 $(GO) build -o $(BUILD_DIR)/$(BINARY_NAME)-windows-amd64.exe ./cmd/lito
	@echo "    ✓ $(BUILD_DIR)/$(BINARY_NAME)-windows-amd64.exe"

	@echo "  📦 macOS (Intel)..."
	@GOOS=darwin GOARCH=amd64 $(GO) build -o $(BUILD_DIR)/$(BINARY_NAME)-darwin-amd64 ./cmd/lito
	@echo "    ✓ $(BUILD_DIR)/$(BINARY_NAME)-darwin-amd64"

	@echo "  📦 macOS (Apple Silicon)..."
	@GOOS=darwin GOARCH=arm64 $(GO) build -o $(BUILD_DIR)/$(BINARY_NAME)-darwin-arm64 ./cmd/lito
	@echo "    ✓ $(BUILD_DIR)/$(BINARY_NAME)-darwin-arm64"

	@echo ""
	@echo "✓ Все бинарники собраны в $(BUILD_DIR)/"

# Создание release архивов
build-release: build-all
	@echo "📦 Создание release архивов..."
	@mkdir -p $(DIST_DIR)

	@echo "  🗜️  Linux..."
	@zip -q -j $(DIST_DIR)/$(BINARY_NAME)-$(VERSION)-linux-amd64.zip $(BUILD_DIR)/$(BINARY_NAME)-linux-amd64
	@echo "    ✓ $(DIST_DIR)/$(BINARY_NAME)-$(VERSION)-linux-amd64.zip"

	@echo "  🗜️  Windows..."
	@zip -q -j $(DIST_DIR)/$(BINARY_NAME)-$(VERSION)-windows-amd64.zip $(BUILD_DIR)/$(BINARY_NAME)-windows-amd64.exe
	@echo "    ✓ $(DIST_DIR)/$(BINARY_NAME)-$(VERSION)-windows-amd64.zip"

	@echo "  🗜️  macOS (Intel)..."
	@zip -q -j $(DIST_DIR)/$(BINARY_NAME)-$(VERSION)-darwin-amd64.zip $(BUILD_DIR)/$(BINARY_NAME)-darwin-amd64
	@echo "    ✓ $(DIST_DIR)/$(BINARY_NAME)-$(VERSION)-darwin-amd64.zip"

	@echo "  🗜️  macOS (Apple Silicon)..."
	@zip -q -j $(DIST_DIR)/$(BINARY_NAME)-$(VERSION)-darwin-arm64.zip $(BUILD_DIR)/$(BINARY_NAME)-darwin-arm64
	@echo "    ✓ $(DIST_DIR)/$(BINARY_NAME)-$(VERSION)-darwin-arm64.zip"

	@echo ""
	@echo "✓ Release архивы созданы в $(DIST_DIR)/"

# Справка
help:
	@echo "Доступные цели:"
	@echo "  make build                - Сборка $(BINARY_NAME) для текущей платформы"
	@echo "  make build-all            - Сборка для всех платформ"
	@echo "  make build-release        - Создание release архивов"
	@echo "  make test                 - Все тесты"
	@echo "  make test-quick           - Быстрые тесты"
	@echo "  make clean                - Очистка"
	@echo "  make bathymetry           - Загрузка батиметрии (Go)"
	@echo "  make bathymetry-python    - Загрузка батиметрии (Python)"
	@echo "  make check-bathymetry     - Проверка наличия батиметрии"
	@echo "  make erosion              - Эрозия без батиметрии"
	@echo "  make erosion-with-bathymetry - Эрозия с батиметрией"
	@echo "  make demo                 - Полный научный сценарий"
	@echo ""
	@echo "Научные сценарии:"
	@echo "  make build && make demo"
	@echo "  make bathymetry && make erosion-with-bathymetry"
