package main

import (
	"fmt"
	"os"
	"strings"
	"unicode"
)

// Типы токенов
const (
	KEYWORD         = "KEYWORD"
	IDENTIFIER      = "IDENTIFIER"
	CONSTANT_INT    = "CONSTANT_INT"
	CONSTANT_FLOAT  = "CONSTANT_FLOAT"
	CONSTANT_STRING = "CONSTANT_STRING"
	CONSTANT_BOOL   = "CONSTANT_BOOL"
	OPERATOR        = "OPERATOR"
	DELIMITER       = "DELIMITER"
)

// Token — одна лексема
type Token struct {
	Type  string
	Value string
	Line  int
}

var keywords = map[string]bool{
	"int": true, "bool": true, "for": true, "while": true,
	"if": true, "else": true, "return": true, "void": true,
	"float": true, "double": true, "char": true, "string": true,
	"include": true, "iostream": true, "std": true,
}

var boolConsts = map[string]bool{
	"true": true, "false": true,
}

// Двухсимвольные операторы — проверяем первыми
var operators2 = map[string]bool{
	"&&": true, "||": true, "++": true, "--": true,
	"==": true, "!=": true, "<=": true, ">=": true,
	"<<": true, ">>": true, "::": true,
}

var operators1 = map[rune]bool{
	'+': true, '-': true, '*': true, '/': true,
	'%': true, '=': true, '<': true, '>': true,
	'!': true,
}

var delimiters = map[rune]bool{
	'(': true, ')': true, '{': true, '}': true,
	';': true, ',': true, ':': true, '#': true,
}

func lexError(msg string, line int) {
	fmt.Fprintf(os.Stderr, "Лексическая ошибка на строке %d: %s\n", line, msg)
	os.Exit(1)
}

func tokenize(source string) []Token {
	var tokens []Token
	chars := []rune(source)
	i := 0
	line := 1

	for i < len(chars) {
		ch := chars[i]

		// Перевод строки
		if ch == '\n' {
			line++
			i++
			continue
		}

		// Пробельные символы — пропускаем
		if ch == ' ' || ch == '\t' || ch == '\r' {
			i++
			continue
		}

		// Строковый литерал
		if ch == '"' {
			s := "\""
			i++
			for i < len(chars) && chars[i] != '"' {
				if chars[i] == '\n' {
					lexError("незакрытый строковый литерал", line)
				}
				s += string(chars[i])
				i++
			}
			if i >= len(chars) {
				lexError("незакрытый строковый литерал", line)
			}
			s += "\""
			i++
			tokens = append(tokens, Token{CONSTANT_STRING, s, line})
			continue
		}

		// Числовая константа
		if unicode.IsDigit(ch) {
			num := ""
			dots := 0
			for i < len(chars) && (unicode.IsDigit(chars[i]) || chars[i] == '.') {
				if chars[i] == '.' {
					// Проверяем — не оператор .. ?
					if i+1 < len(chars) && chars[i+1] == '.' {
						break
					}
					dots++
					if dots > 1 {
						lexError(fmt.Sprintf("некорректное число: две точки в '%s'", num), line)
					}
				}
				num += string(chars[i])
				i++
			}
			// Число не может заканчиваться буквой
			if i < len(chars) && unicode.IsLetter(chars[i]) {
				lexError(fmt.Sprintf("идентификатор начинается с цифры: '%s%s'", num, string(chars[i])), line)
			}
			// Запятая не разделитель дробной части
			if i < len(chars) && chars[i] == ',' && i+1 < len(chars) && unicode.IsDigit(chars[i+1]) {
				lexError(fmt.Sprintf("запятая не является разделителем дробной части в '%s,'", num), line)
			}
			ttype := CONSTANT_INT
			if dots == 1 {
				ttype = CONSTANT_FLOAT
			}
			tokens = append(tokens, Token{ttype, num, line})
			continue
		}

		// Идентификатор или ключевое слово
		if unicode.IsLetter(ch) || ch == '_' {
			word := ""
			for i < len(chars) && (unicode.IsLetter(chars[i]) || unicode.IsDigit(chars[i]) || chars[i] == '_') {
				word += string(chars[i])
				i++
			}
			ttype := IDENTIFIER
			if boolConsts[word] {
				ttype = CONSTANT_BOOL
			} else if keywords[word] {
				ttype = KEYWORD
			}
			tokens = append(tokens, Token{ttype, word, line})
			continue
		}

		// Двухсимвольный оператор
		if i+1 < len(chars) {
			two := string(chars[i : i+2])
			if operators2[two] {
				tokens = append(tokens, Token{OPERATOR, two, line})
				i += 2
				continue
			}
		}

		// Односимвольный оператор
		if operators1[ch] {
			tokens = append(tokens, Token{OPERATOR, string(ch), line})
			i++
			continue
		}

		// Разделитель
		if delimiters[ch] {
			tokens = append(tokens, Token{DELIMITER, string(ch), line})
			i++
			continue
		}

		lexError(fmt.Sprintf("недопустимый символ: '%s'", string(ch)), line)
	}

	return tokens
}

func printTable(tokens []Token) {
	fmt.Printf("%-30s | %s\n", "Лексема", "Тип")
	fmt.Println(strings.Repeat("-", 50))
	for _, t := range tokens {
		fmt.Printf("%-30s | %s\n", t.Value, t.Type)
	}
}

func printSequence(tokens []Token) {
	parts := make([]string, len(tokens))
	for i, t := range tokens {
		parts[i] = fmt.Sprintf("(%s, %s)", t.Type, t.Value)
	}
	fmt.Printf("\n[%s]\n", strings.Join(parts, ", "))
}

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintln(os.Stderr, "Использование: go run lexer.go <файл>")
		os.Exit(1)
	}

	data, err := os.ReadFile(os.Args[1])
	if err != nil {
		fmt.Fprintf(os.Stderr, "Ошибка чтения файла: %v\n", err)
		os.Exit(1)
	}

	tokens := tokenize(string(data))
	printTable(tokens)
	printSequence(tokens)
	fmt.Fprintf(os.Stderr, "\nЛексический анализ завершён успешно. Обнаружено %d токенов. Ошибок не найдено.\n", len(tokens))
}