package main

import (
	"fmt"
	"os"
	"regexp"
	"strings"
)

// checkInvalidChars проверяет наличие недопустимых управляющих символов
func checkInvalidChars(source string) error {
	for i, ch := range source {
		if ch < 32 && ch != '\t' && ch != '\n' && ch != '\r' {
			return fmt.Errorf("недопустимый символ с кодом %d на позиции %d", ch, i)
		}
	}
	return nil
}

// checkUnclosedComments проверяет незакрытые многострочные комментарии
func checkUnclosedComments(source string) error {
	openCount := strings.Count(source, "/*")
	closeCount := strings.Count(source, "*/")
	if openCount > closeCount {
		return fmt.Errorf("незакрытый многострочный комментарий (найдено %d '/*' и %d '*/')", openCount, closeCount)
	}
	if closeCount > openCount {
		return fmt.Errorf("лишний символ закрытия '*/' (найдено %d '/*' и %d '*/')", openCount, closeCount)
	}
	return nil
}

// protectStrings заменяет строковые литералы на плейсхолдеры,
// чтобы комментарии внутри строк не удалялись
func protectStrings(source string) (string, []string) {
	var strings_ []string
	var result strings.Builder
	chars := []rune(source)
	i := 0
	for i < len(chars) {
		if chars[i] == '"' {
			s := string('"')
			i++
			for i < len(chars) && chars[i] != '"' && chars[i] != '\n' {
				s += string(chars[i])
				i++
			}
			if i < len(chars) && chars[i] == '"' {
				s += string('"')
				i++
			}
			placeholder := fmt.Sprintf("\x00STR%d\x00", len(strings_))
			strings_ = append(strings_, s)
			result.WriteString(placeholder)
		} else {
			result.WriteRune(chars[i])
			i++
		}
	}
	return result.String(), strings_
}

// restoreStrings возвращает строковые литералы на место
func restoreStrings(source string, strings_ []string) string {
	for idx, s := range strings_ {
		placeholder := fmt.Sprintf("\x00STR%d\x00", idx)
		source = strings.ReplaceAll(source, placeholder, s)
	}
	return source
}

func preprocess(source string) (string, error) {
	// 1. Проверка недопустимых символов
	if err := checkInvalidChars(source); err != nil {
		return "", err
	}

	// 2. Защищаем строковые литералы
	source, savedStrings := protectStrings(source)

	// 3. Проверка незакрытых комментариев
	if err := checkUnclosedComments(source); err != nil {
		return "", err
	}

	// 4. Удаление многострочных комментариев /* ... */
	reMulti := regexp.MustCompile(`(?s)/\*.*?\*/`)
	source = reMulti.ReplaceAllString(source, "")

	// 5. Удаление однострочных комментариев // ...
	reSingle := regexp.MustCompile(`(?m)//.*$`)
	source = reSingle.ReplaceAllString(source, "")

	// 6. Удаление пробелов и табуляций в начале и конце каждой строки
	reTrim := regexp.MustCompile(`(?m)^[ \t]+|[ \t]+$`)
	source = reTrim.ReplaceAllString(source, "")

	// 7. Замена нескольких пробелов/табуляций подряд на один пробел
	reSpaces := regexp.MustCompile(`[ \t]+`)
	source = reSpaces.ReplaceAllString(source, " ")

	// 8. Удаление пустых строк
	reEmptyLines := regexp.MustCompile(`\n\s*\n`)
	for reEmptyLines.MatchString(source) {
		source = reEmptyLines.ReplaceAllString(source, "\n")
	}

	// 9. Возвращаем строковые литералы
	source = restoreStrings(source, savedStrings)

	return strings.TrimSpace(source), nil
}

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintln(os.Stderr, "Использование: go run preprocessor.go <файл>")
		os.Exit(1)
	}

	data, err := os.ReadFile(os.Args[1])
	if err != nil {
		fmt.Fprintf(os.Stderr, "Ошибка чтения файла: %v\n", err)
		os.Exit(1)
	}

	result, err := preprocess(string(data))
	if err != nil {
		fmt.Fprintln(os.Stderr, "Ошибка:", err)
		os.Exit(1)
	}

	fmt.Println(result)
	fmt.Fprintln(os.Stderr, "Ошибок не выявлено.")
}