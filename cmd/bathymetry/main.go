package main

import (
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
)

const (
	gebcoURL = "https://www.gebco.net/data-products/gridded-bathymetry-data/gebco2026-grid"
)

func main() {
	if len(os.Args) < 2 {
		printUsage()
		os.Exit(1)
	}

	command := os.Args[1]

	switch command {
	case "download":
		downloadBathymetry()
	case "convert":
		convertBathymetry()
	case "help":
		printUsage()
	default:
		fmt.Printf("Неизвестная команда: %s\n", command)
		printUsage()
		os.Exit(1)
	}
}

func printUsage() {
	fmt.Println("Инструменты подготовки батиметрии Lito")
	fmt.Println("\nИспользование:")
	fmt.Println("  bathymetry download    Загрузка батиметрических данных")
	fmt.Println("  bathymetry convert     Конвертация NetCDF в JSON")
	fmt.Println("  bathymetry help        Показать эту справку")
	fmt.Println("\nПримеры:")
	fmt.Println("  bathymetry download")
	fmt.Println("  bathymetry convert --input gebco.nc --output bathymetry.json")
}

func downloadBathymetry() {
	fmt.Println("🌊 Загрузка батиметрических данных для Чёрного моря...")
	fmt.Println("\nДля загрузки данных GEBCO выполните следующие шаги:")
	fmt.Println("\n1. Посетите GEBCO Data Viewer:")
	fmt.Printf("   %s\n", gebcoURL)
	fmt.Println("\n2. Выберите регион Чёрного моря:")
	fmt.Println("   - Север: 47.5°N")
	fmt.Println("   - Юг: 40.5°N")
	fmt.Println("   - Запад: 27.0°E")
	fmt.Println("   - Восток: 42.5°E")
	fmt.Println("\n3. Скачайте NetCDF файл (.nc)")
	fmt.Println("\n4. Сохраните точный URL, название и DOI выбранной версии продукта")
	fmt.Println("\n5. Не удаляйте исходный NetCDF: его SHA-256 записывается в паспорт")
	fmt.Println("\nПосле загрузки используйте 'bathymetry convert' с полным набором полей происхождения")

	// Попытка открыть браузер (если поддерживается системе)
	if err := openBrowser(gebcoURL); err == nil {
		fmt.Println("\n🌐 Открыт браузер с GEBCO Data Viewer")
	}

	fmt.Println("\nАвтоматизированная загрузка и конвертация:")
	fmt.Println("   cmd/bathymetry/convert/download_bathymetry.sh URL_GEBCO_2026 ВЫХОДНОЙ_JSON")
}

func convertBathymetry() {
	// Проверка наличия Python
	pythonCmd := findPythonWithVenv()
	if pythonCmd == "" {
		log.Fatal("❌ Python не найден. Установите Python 3 для конвертации данных")
	}

	// Проверка наличия скрипта конвертации
	scriptPath := "cmd/bathymetry/convert/convert_bathymetry.py"
	if _, err := os.Stat(scriptPath); os.IsNotExist(err) {
		log.Fatalf("❌ Скрипт конвертации не найден: %s", scriptPath)
	}

	// Проверка зависимостей
	fmt.Println("🔄 Проверка зависимостей...")
	checkCmd := exec.Command(pythonCmd, "-c", "import xarray, netCDF4")
	if err := checkCmd.Run(); err != nil {
		fmt.Println("❌ Отсутствуют зависимости Python")
		fmt.Println("💡 Установите зависимости:")
		fmt.Println("   pip install -r cmd/bathymetry/convert/requirements.txt")
		fmt.Println("   Или активируйте виртуальное окружение:")
		fmt.Println("   source scripts/venv/bin/activate")
		os.Exit(1)
	}

	fmt.Println("🔄 Конвертация батиметрических данных...")

	// Запуск Python скрипта с передачей аргументов
	args := []string{scriptPath}
	args = append(args, os.Args[2:]...) // Передать все аргументы после 'convert'

	cmd := exec.Command(pythonCmd, args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		log.Fatalf("❌ Ошибка конвертации: %v", err)
	}

	fmt.Println("\n✅ Конвертация завершена!")
}

func findPythonWithVenv() string {
	// Сначала проверяем виртуальное окружение
	venvPython := "scripts/venv/bin/python3"
	if _, err := os.Stat(venvPython); err == nil {
		return venvPython
	}

	// Затем проверяем системный Python
	commands := []string{"python3", "python"}

	for _, cmd := range commands {
		if _, err := exec.LookPath(cmd); err == nil {
			return cmd
		}
	}

	return ""
}

func openBrowser(url string) error {
	var cmd *exec.Cmd

	switch {
	case filepath.Base(os.Getenv("SHELL")) == "zsh" || filepath.Base(os.Getenv("SHELL")) == "bash":
		cmd = exec.Command("open", url) // macOS
	default:
		return fmt.Errorf("открытие браузера не поддерживается на этой платформе")
	}

	return cmd.Start()
}
