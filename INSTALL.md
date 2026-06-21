# Установка Litora-CLI v1.2

## Требования

Litora-CLI не требует дополнительных зависимостей для работы. Бинарные файлы включают всё необходимое для выполнения.

## Скачивание

Выберите соответствующий архив для вашей операционной системы:

- **Linux (amd64)**: `lito-v1.2-linux-amd64.zip`
- **Windows (amd64)**: `lito-v1.2-windows-amd64.zip`
- **macOS (Intel)**: `lito-v1.2-darwin-amd64.zip`
- **macOS (Apple Silicon)**: `lito-v1.2-darwin-arm64.zip`

## Установка

### Linux

1. Распакуйте архив:
```bash
unzip lito-v1.2-linux-amd64.zip
```

2. Сделайте файл исполняемым:
```bash
chmod +x lito-linux-amd64
```

3. Переместите в директорию из PATH (опционально):
```bash
sudo mv lito-linux-amd64 /usr/local/bin/lito
```

### macOS

1. Распакуйте архив:
```bash
unzip lito-v1.2-darwin-*.zip
```

2. Сделайте файл исполняемым:
```bash
chmod +x lito-darwin-*
```

3. Переместите в директорию из PATH (опционально):
```bash
sudo mv lito-darwin-* /usr/local/bin/lito
```

*Примечание: На macOS может потребоваться разрешение запуска в настройках безопасности.*

### Windows

1. Распакуйте архив (дважды кликните или используйте PowerShell):
```powershell
Expand-Archive lito-v1.2-windows-amd64.zip
```

2. Переместите `lito-windows-amd64.exe` в директорию из PATH или добавьте текущую директорию в PATH.

3. Используйте `lito-windows-amd64.exe` или переименуйте в `lito.exe`.

## Проверка установки

```bash
# Linux/macOS
./lito-linux-amd64 --help
# или если переименовали в lito
lito --help

# Windows
lito-windows-amd64.exe --help
```

## Первый запуск

```bash
# Проверка источника данных
lito source

# Полный научный сценарий
lito all --iterations 3 --steps 5
```

## Дополнительные инструменты

Для использования Python скриптов анализа данных требуется Python 3.8+:

```bash
pip install pandas matplotlib seaborn numpy scipy
```

## Поддержка

- Репозиторий: https://github.com/Nickitas/litora-cli
- Лицензия: MIT