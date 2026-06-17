package main

import (
	"fmt"
	"os"
	"strings"
	"unicode"
)

// ════════════════════════════════════════════════
// ТИПЫ И ТАБЛИЦЫ (из лексера)
// ════════════════════════════════════════════════

const (
	KEYWORD         = "KEYWORD"
	IDENTIFIER      = "IDENTIFIER"
	CONSTANT_INT    = "CONSTANT_INT"
	CONSTANT_FLOAT  = "CONSTANT_FLOAT"
	CONSTANT_STRING = "CONSTANT_STRING"
	CONSTANT_BOOL   = "CONSTANT_BOOL"
	OPERATOR        = "OPERATOR"
	DELIMITER       = "DELIMITER"
	EOF             = "EOF"
)

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

var operators2 = map[string]bool{
	"&&": true, "||": true, "++": true, "--": true,
	"==": true, "!=": true, "<=": true, ">=": true,
	"<<": true, ">>": true, "::": true,
}

var operators1 = map[rune]bool{
	'+': true, '-': true, '*': true, '/': true,
	'%': true, '=': true, '<': true, '>': true, '!': true,
}

var delimiters = map[rune]bool{
	'(': true, ')': true, '{': true, '}': true,
	';': true, ',': true, ':': true, '#': true,
}

var identifiers = map[string]bool{
	"add": true, "main": true,
	"x": true, "y": true, "result": true, "product": true,
	"diff": true, "isPositive": true, "count": true, "i": true,
	"a": true, "b": true, "s": true,
	"cout": true, "endl": true, "cin": true,
}

// ════════════════════════════════════════════════
// ЛЕКСЕР
// ════════════════════════════════════════════════

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
		if ch == '\n' { line++; i++; continue }
		if ch == ' ' || ch == '\t' || ch == '\r' { i++; continue }

		if ch == '"' {
			s := "\""
			i++
			for i < len(chars) && chars[i] != '"' {
				if chars[i] == '\n' { lexError("незакрытый строковый литерал", line) }
				s += string(chars[i]); i++
			}
			if i >= len(chars) { lexError("незакрытый строковый литерал", line) }
			s += "\""; i++
			tokens = append(tokens, Token{CONSTANT_STRING, s, line})
			continue
		}

		if unicode.IsDigit(ch) {
			num := ""; dots := 0
			for i < len(chars) && (unicode.IsDigit(chars[i]) || chars[i] == '.') {
				if chars[i] == '.' {
					if i+1 < len(chars) && chars[i+1] == '.' { break }
					dots++
					if dots > 1 { lexError(fmt.Sprintf("некорректное число: две точки в '%s'", num), line) }
				}
				num += string(chars[i]); i++
			}
			if i < len(chars) && unicode.IsLetter(chars[i]) {
				lexError(fmt.Sprintf("идентификатор начинается с цифры: '%s%s'", num, string(chars[i])), line)
			}
			ttype := CONSTANT_INT
			if dots == 1 { ttype = CONSTANT_FLOAT }
			tokens = append(tokens, Token{ttype, num, line})
			continue
		}

		if unicode.IsLetter(ch) || ch == '_' {
			word := ""
			for i < len(chars) && (unicode.IsLetter(chars[i]) || unicode.IsDigit(chars[i]) || chars[i] == '_') {
				word += string(chars[i]); i++
			}
			ttype := IDENTIFIER
			if boolConsts[word] {
				ttype = CONSTANT_BOOL
			} else if keywords[word] {
				ttype = KEYWORD
			} else if identifiers[word] {
				ttype = IDENTIFIER
			} else {
				lexError(fmt.Sprintf("неизвестный идентификатор: '%s'", word), line)
			}
			tokens = append(tokens, Token{ttype, word, line})
			continue
		}

		if i+1 < len(chars) {
			two := string(chars[i : i+2])
			if operators2[two] {
				tokens = append(tokens, Token{OPERATOR, two, line})
				i += 2; continue
			}
		}
		if operators1[ch] {
			tokens = append(tokens, Token{OPERATOR, string(ch), line})
			i++; continue
		}
		if delimiters[ch] {
			tokens = append(tokens, Token{DELIMITER, string(ch), line})
			i++; continue
		}
		lexError(fmt.Sprintf("недопустимый символ: '%s'", string(ch)), line)
	}

	tokens = append(tokens, Token{EOF, "", 0})
	return tokens
}

// ════════════════════════════════════════════════
// AST
// ════════════════════════════════════════════════

type ASTNode struct {
	Kind     string
	Attrs    map[string]string
	Children []*ASTNode
}

func newNode(kind string) *ASTNode {
	return &ASTNode{Kind: kind, Attrs: map[string]string{}}
}

func (n *ASTNode) set(k, v string) *ASTNode {
	n.Attrs[k] = v
	return n
}

func (n *ASTNode) add(child *ASTNode) *ASTNode {
	if child != nil {
		n.Children = append(n.Children, child)
	}
	return n
}

func printAST(node *ASTNode, prefix string, last bool) {
	connector := "└── "
	if !last { connector = "├── " }
	line := prefix + connector + node.Kind
	if len(node.Attrs) > 0 {
		parts := []string{}
		for k, v := range node.Attrs {
			parts = append(parts, k+": "+v)
		}
		line += " [" + strings.Join(parts, ", ") + "]"
	}
	fmt.Println(line)
	childPrefix := prefix + "    "
	if !last { childPrefix = prefix + "│   " }
	for i, child := range node.Children {
		printAST(child, childPrefix, i == len(node.Children)-1)
	}
}

// ════════════════════════════════════════════════
// ПАРСЕР
// ════════════════════════════════════════════════

type Parser struct {
	tokens    []Token
	pos       int
	declared  map[string]bool  // объявленные переменные
	functions map[string]int   // имя функции → число параметров
	inFunc    bool             // внутри функции?
	inLoop    int              // глубина вложенности циклов
}

func newParser(tokens []Token) *Parser {
	return &Parser{
		tokens:    tokens,
		declared:  map[string]bool{},
		functions: map[string]int{},
	}
}

func (p *Parser) current() Token {
	if p.pos < len(p.tokens) { return p.tokens[p.pos] }
	return Token{EOF, "", 0}
}

func (p *Parser) peekType() string { return p.current().Type }
func (p *Parser) peekVal() string  { return p.current().Value }

func (p *Parser) advance() Token {
	t := p.current(); p.pos++; return t
}

func (p *Parser) match(ttype, tval string) bool {
	t := p.current()
	return t.Type == ttype && (tval == "" || t.Value == tval)
}

func (p *Parser) expect(ttype, tval string) Token {
	t := p.current()
	if t.Type != ttype || (tval != "" && t.Value != tval) {
		want := ttype
		if tval != "" { want = "'" + tval + "'" }
		p.parseError(fmt.Sprintf("ожидался %s, получен '%s' (%s)", want, t.Value, t.Type))
	}
	return p.advance()
}

func (p *Parser) parseError(msg string) {
	t := p.current()
	fmt.Fprintf(os.Stderr, "Синтаксическая ошибка [позиция %d, строка %d]: %s. Текущий токен: '%s' (%s)\n",
		p.pos, t.Line, msg, t.Value, t.Type)
	os.Exit(1)
}

// ── Предварительный проход: собираем сигнатуры функций ──────────────────────
func (p *Parser) collectFunctions() {
	for i := 0; i < len(p.tokens)-1; i++ {
		// Ищем: тип ( идентификатор (
		// Например: int add ( int a , int b )
		// Упрощённо: KEYWORD IDENTIFIER (
		if p.tokens[i].Type == KEYWORD &&
			i+1 < len(p.tokens) && p.tokens[i+1].Type == IDENTIFIER &&
			i+2 < len(p.tokens) && p.tokens[i+2].Value == "(" {
			fname := p.tokens[i+1].Value
			// считаем параметры — количество запятых на глубине 1 + 1 (если не пусто)
			count := 0
			j := i + 3
			depth := 1
			hasParams := false
			for j < len(p.tokens) && depth > 0 {
				if p.tokens[j].Value == "(" { depth++ }
				if p.tokens[j].Value == ")" {
					depth--
					if depth == 0 { break }
				}
				if depth == 1 && p.tokens[j].Type == KEYWORD {
					// это тип параметра
					count++
					hasParams = true
				}
				j++
			}
			if !hasParams { count = 0 }
			p.functions[fname] = count
		}
	}
}

// ── Program ──────────────────────────────────────────────────────────────────
func (p *Parser) parseProgram() *ASTNode {
	p.collectFunctions()
	node := newNode("Program")
	// Сначала директивы #include
	for p.match(DELIMITER, "#") {
		node.add(p.parseInclude())
	}
	// Потом объявления функций
	for p.peekType() != EOF {
		node.add(p.parseFuncDecl())
	}
	return node
}

// ── #include <iostream> ──────────────────────────────────────────────────────
func (p *Parser) parseInclude() *ASTNode {
	p.expect(DELIMITER, "#")
	p.expect(KEYWORD, "include")
	p.expect(OPERATOR, "<")
	lib := p.expect(KEYWORD, "").Value
	p.expect(OPERATOR, ">")
	return newNode("Include").set("lib", lib)
}

// ── Объявление функции: int add(int a, int b) { ... } ───────────────────────
func (p *Parser) parseFuncDecl() *ASTNode {
	retType := p.expect(KEYWORD, "").Value
	name := p.expect(IDENTIFIER, "").Value
	node := newNode("FuncDecl").set("name", name).set("return_type", retType)

	p.expect(DELIMITER, "(")
	p.inFunc = true
	node.add(p.parseParamList())
	p.expect(DELIMITER, ")")

	if !p.match(DELIMITER, "{") {
		p.parseError(fmt.Sprintf("функция '%s' не имеет тела — ожидался '{'", name))
	}
	node.add(p.parseBlock())
	p.inFunc = false
	return node
}

// ── Список параметров ────────────────────────────────────────────────────────
func (p *Parser) parseParamList() *ASTNode {
	node := newNode("ParamList")
	if p.match(DELIMITER, ")") { return node }
	node.add(p.parseParam())
	for p.match(DELIMITER, ",") {
		p.advance()
		node.add(p.parseParam())
	}
	return node
}

func (p *Parser) parseParam() *ASTNode {
	ptype := p.expect(KEYWORD, "").Value
	name := p.expect(IDENTIFIER, "").Value
	p.declared[name] = true
	return newNode("Param").set("name", name).set("type", ptype)
}

// ── Блок { ... } ─────────────────────────────────────────────────────────────
func (p *Parser) parseBlock() *ASTNode {
	p.expect(DELIMITER, "{")
	node := newNode("Block")
	for !p.match(DELIMITER, "}") {
		if p.peekType() == EOF {
			p.parseError("незакрытый блок: ожидался '}'")
		}
		node.add(p.parseStmt())
	}
	p.expect(DELIMITER, "}")
	return node
}

// ── Оператор ─────────────────────────────────────────────────────────────────
func (p *Parser) parseStmt() *ASTNode {
	t := p.current()

	// Пропускаем точки с запятой
	if p.match(DELIMITER, ";") {
		p.advance()
		return nil
	}

	if t.Type == KEYWORD {
		switch t.Value {
		case "int", "bool", "float", "double", "char", "string":
			return p.parseVarDecl()
		case "if":
			return p.parseIfStmt()
		case "for":
			return p.parseForStmt()
		case "while":
			return p.parseWhileStmt()
		case "return":
			return p.parseReturnStmt()
		}
	}

	if t.Type == KEYWORD && t.Value == "std" {
		// смотрим вперёд: std :: string name = ... → это объявление переменной
		if p.pos+2 < len(p.tokens) && p.tokens[p.pos+1].Value == "::" && p.tokens[p.pos+2].Value == "string" {
			return p.parseStdStringDecl()
		}
		return p.parseExprStmt()
	}

	if t.Type == IDENTIFIER {
		next := ""
		if p.pos+1 < len(p.tokens) { next = p.tokens[p.pos+1].Value }
		switch next {
		case "=":
			return p.parseAssignStmt()
		case "++", "--":
			return p.parseIncStmt()
		case "(":
			return p.parseExprStmt()
		case "::": // std::cout
			return p.parseExprStmt()
		}
	}

	p.parseError(fmt.Sprintf("неожиданный токен '%s' (%s) — не является началом оператора", t.Value, t.Type))
	return nil
}

func (p *Parser) parseStdStringDecl() *ASTNode {
    p.expect(KEYWORD, "std")
    p.expect(OPERATOR, "::")
    p.expect(KEYWORD, "string")
    name := p.expect(IDENTIFIER, "").Value

    if p.declared[name] {
        p.parseError(fmt.Sprintf("переменная '%s' уже объявлена", name))
    }
    p.declared[name] = true

    node := newNode("VarDecl").set("type", "std::string").set("name", name)
    if p.match(OPERATOR, "=") {
        p.advance()
        node.add(p.parseExpr())
    }
    if p.match(DELIMITER, ";") { p.advance() }
    return node
}

// ── Объявление переменной: int x = 10; ───────────────────────────────────────
func (p *Parser) parseVarDecl() *ASTNode {
	vtype := p.advance().Value
	name := p.expect(IDENTIFIER, "").Value

	if p.declared[name] {
		p.parseError(fmt.Sprintf("переменная '%s' уже объявлена", name))
	}
	p.declared[name] = true

	node := newNode("VarDecl").set("type", vtype).set("name", name)

	if p.match(OPERATOR, "=") {
		p.advance()
		node.add(p.parseExpr())
	}
	// необязательная ;
	if p.match(DELIMITER, ";") { p.advance() }
	return node
}

// ── Присваивание: result = x + y; ────────────────────────────────────────────
func (p *Parser) parseAssignStmt() *ASTNode {
	name := p.advance().Value
	if !p.declared[name] {
		p.parseError(fmt.Sprintf("переменная '%s' используется без объявления", name))
	}
	p.expect(OPERATOR, "=")
	node := newNode("AssignStmt").set("target", name)
	node.add(p.parseExpr())
	if p.match(DELIMITER, ";") { p.advance() }
	return node
}

// ── Инкремент/декремент: count++; ────────────────────────────────────────────
func (p *Parser) parseIncStmt() *ASTNode {
	name := p.advance().Value
	if !p.declared[name] {
		p.parseError(fmt.Sprintf("переменная '%s' используется без объявления", name))
	}
	op := p.advance().Value
	if p.match(DELIMITER, ";") { p.advance() }
	return newNode("IncStmt").set("target", name).set("op", op)
}

// ── return ────────────────────────────────────────────────────────────────────
func (p *Parser) parseReturnStmt() *ASTNode {
	if !p.inFunc {
		p.parseError("оператор 'return' используется вне функции")
	}
	p.expect(KEYWORD, "return")
	node := newNode("ReturnStmt")
	node.add(p.parseExpr())
	if p.match(DELIMITER, ";") { p.advance() }
	return node
}

// ── if-else ───────────────────────────────────────────────────────────────────
func (p *Parser) parseIfStmt() *ASTNode {
	p.expect(KEYWORD, "if")
	p.expect(DELIMITER, "(")
	node := newNode("IfStmt")
	cond := p.parseExpr()
	cond.Kind = "Condition"
	node.add(cond)
	p.expect(DELIMITER, ")")
	then := p.parseBlock()
	then.Kind = "ThenBlock"
	node.add(then)
	if p.match(KEYWORD, "else") {
		p.advance()
		els := p.parseBlock()
		els.Kind = "ElseBlock"
		node.add(els)
	}
	return node
}

// ── for ───────────────────────────────────────────────────────────────────────
func (p *Parser) parseForStmt() *ASTNode {
	p.expect(KEYWORD, "for")
	p.expect(DELIMITER, "(")
	// int i = 1; i <= 5; i++
	node := newNode("ForStmt")
	// init
	init := p.parseVarDecl() // int i = 1 (точку с запятой parseVarDecl съедает)
	node.add(init)
	// condition
	cond := p.parseExpr()
	cond.Kind = "Condition"
	node.add(cond)
	p.expect(DELIMITER, ";")
	// step
	step := p.parseIncStmt() // i++ (точку с запятой parseIncStmt не ждёт здесь)
	node.add(step)
	p.expect(DELIMITER, ")")
	p.inLoop++
	node.add(p.parseBlock())
	p.inLoop--
	return node
}

// ── while ─────────────────────────────────────────────────────────────────────
func (p *Parser) parseWhileStmt() *ASTNode {
	p.expect(KEYWORD, "while")
	p.expect(DELIMITER, "(")
	node := newNode("WhileStmt")
	cond := p.parseExpr()
	cond.Kind = "Condition"
	node.add(cond)
	p.expect(DELIMITER, ")")
	p.inLoop++
	node.add(p.parseBlock())
	p.inLoop--
	return node
}

// ── Вызов функции как оператор: cout << ... ; ────────────────────────────────
func (p *Parser) parseExprStmt() *ASTNode {
	node := newNode("ExprStmt")
	node.add(p.parseExpr())
	if p.match(DELIMITER, ";") { p.advance() }
	return node
}

// ── Выражение ─────────────────────────────────────────────────────────────────
func (p *Parser) parseExpr() *ASTNode {
	left := p.parsePrimary()
	for p.peekType() == OPERATOR && p.peekVal() != "++" && p.peekVal() != "--" {
		op := p.advance().Value
		right := p.parsePrimary()
		node := newNode("BinaryExpr").set("op", op)
		node.add(left)
		node.add(right)
		left = node
	}
	return left
}

// ── Первичное выражение ───────────────────────────────────────────────────────
func (p *Parser) parsePrimary() *ASTNode {
	t := p.current()


	// std::cout
	if t.Type == KEYWORD && t.Value == "std" {
		p.advance()
		p.expect(OPERATOR, "::")
		member := p.advance().Value
		return newNode("MemberExpr").set("object", "std").set("member", member)
	}
	// Идентификатор
	if t.Type == IDENTIFIER {
		p.advance()
		// std::cout — составной идентификатор
		if p.match(OPERATOR, "::") {
			p.advance()
			member := p.advance().Value
			node := newNode("MemberExpr").set("object", t.Value).set("member", member)
			// может быть << expr
			return node
		}
		// Вызов функции
		if p.match(DELIMITER, "(") {
			p.advance()
			node := newNode("CallExpr").set("func", t.Value)
			args := p.parseArgList()
			node.add(args)
			p.expect(DELIMITER, ")")
			// Проверка числа аргументов
			if exp, ok := p.functions[t.Value]; ok {
				got := len(args.Children)
				if exp != got {
					p.parseError(fmt.Sprintf("функция '%s' ожидает %d аргумент(ов), передано %d", t.Value, exp, got))
				}
			}
			return node
		}
		// Просто переменная
		if !p.declared[t.Value] {
			p.parseError(fmt.Sprintf("переменная '%s' используется без объявления", t.Value))
		}
		return newNode("Identifier").set("name", t.Value)
	}

	// Константы
	if t.Type == CONSTANT_INT || t.Type == CONSTANT_FLOAT ||
		t.Type == CONSTANT_STRING || t.Type == CONSTANT_BOOL {
		p.advance()
		return newNode("Literal").set("type", t.Type).set("value", t.Value)
	}

	// Выражение в скобках
	if p.match(DELIMITER, "(") {
		p.advance()
		node := p.parseExpr()
		p.expect(DELIMITER, ")")
		return node
	}

	p.parseError(fmt.Sprintf("ожидалось выражение, получен '%s' (%s)", t.Value, t.Type))
	return nil
}

// ── Список аргументов ─────────────────────────────────────────────────────────
func (p *Parser) parseArgList() *ASTNode {
	node := newNode("ArgList")
	if p.match(DELIMITER, ")") { return node }
	node.add(p.parseExpr())
	for p.match(DELIMITER, ",") {
		p.advance()
		node.add(p.parseExpr())
	}
	return node
}

// ════════════════════════════════════════════════
// MAIN
// ════════════════════════════════════════════════

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintln(os.Stderr, "Использование: go run parser/main.go <cleaned.cpp>")
		os.Exit(1)
	}
	data, err := os.ReadFile(os.Args[1])
	if err != nil {
		fmt.Fprintf(os.Stderr, "Ошибка чтения файла: %v\n", err)
		os.Exit(1)
	}

	tokens := tokenize(string(data))
	parser := newParser(tokens)
	ast := parser.parseProgram()

	fmt.Println("Синтаксический анализатор")
	printAST(ast, "", true)
	fmt.Fprintln(os.Stderr, "\nСинтаксический анализ завершён успешно. Ошибок не найдено.")
}