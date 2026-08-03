package coastline

import "strings"

// isNotEmpty проверяет, что строка не пустая после удаления пробелов
func isNotEmpty(s string) bool {
	return strings.TrimSpace(s) != ""
}

// isEmpty проверяет, что строка пустая после удаления пробелов
func isEmpty(s string) bool {
	return strings.TrimSpace(s) == ""
}

// orDefault возвращает строку или значение по умолчанию, если строка пуста
func orDefault(s, defaultValue string) string {
	if isNotEmpty(s) {
		return s
	}
	return defaultValue
}

// trimSpace удаляет пробельные символы из начала и конца строки
func trimSpace(s string) string {
	return strings.TrimSpace(s)
}
