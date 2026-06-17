package main

import (
	"fmt"
	"os"
	"sort"
	"strings"
	"unicode"
)

// ============================================================
// Общие структуры
// ============================================================

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
	Type   string
	Value  string
	Line   int
	Column int
}

type CompilerError struct {
	Stage  string
	Msg    string
	Line   int
	Column int
}

func (e CompilerError) Error() string {
	if e.Line > 0 {
		return fmt.Sprintf("%s ошибка [строка %d, столбец %d]: %s", e.Stage, e.Line, e.Column, e.Msg)
	}
	return fmt.Sprintf("%s ошибка: %s", e.Stage, e.Msg)
}

// ============================================================
// ЛР1. Препроцессор C++
// Удаляет комментарии и лишние пробелы, но сохраняет строки.
// ============================================================

func checkInvalidChars(source string) error {
	for i, ch := range source {
		if ch < 32 && ch != '\t' && ch != '\n' && ch != '\r' {
			return CompilerError{Stage: "Препроцессор", Msg: fmt.Sprintf("недопустимый управляющий символ с кодом %d на позиции %d", ch, i)}
		}
	}
	return nil
}

func removeComments(source string) (string, error) {
	chars := []rune(source)
	var out strings.Builder
	line, col := 1, 1

	for i := 0; i < len(chars); {
		ch := chars[i]

		if ch == '\n' {
			out.WriteRune(ch)
			i++
			line++
			col = 1
			continue
		}

		// Строковый литерал. Комментарии внутри него не трогаем.
		if ch == '"' {
			out.WriteRune(ch)
			i++
			col++
			escaped := false
			closed := false

			for i < len(chars) {
				c := chars[i]
				out.WriteRune(c)
				i++

				if c == '\n' {
					return "", CompilerError{Stage: "Препроцессор", Msg: "незакрытый строковый литерал", Line: line, Column: col}
				}

				if c == '"' && !escaped {
					col++
					closed = true
					break
				}

				if c == '\\' && !escaped {
					escaped = true
				} else {
					escaped = false
				}
				col++
			}

			if !closed {
				return "", CompilerError{Stage: "Препроцессор", Msg: "незакрытый строковый литерал", Line: line, Column: col}
			}
			continue
		}

		// Однострочный комментарий // ...
		if ch == '/' && i+1 < len(chars) && chars[i+1] == '/' {
			i += 2
			col += 2
			for i < len(chars) && chars[i] != '\n' {
				i++
				col++
			}
			continue
		}

		// Многострочный комментарий /* ... */
		if ch == '/' && i+1 < len(chars) && chars[i+1] == '*' {
			startLine, startCol := line, col
			i += 2
			col += 2
			closed := false

			for i < len(chars) {
				if chars[i] == '*' && i+1 < len(chars) && chars[i+1] == '/' {
					i += 2
					col += 2
					closed = true
					break
				}
				if chars[i] == '\n' {
					out.WriteRune('\n')
					i++
					line++
					col = 1
				} else {
					i++
					col++
				}
			}

			if !closed {
				return "", CompilerError{Stage: "Препроцессор", Msg: "незакрытый многострочный комментарий", Line: startLine, Column: startCol}
			}
			continue
		}

		out.WriteRune(ch)
		i++
		col++
	}

	return out.String(), nil
}

func collapseSpacesOutsideStrings(line string) string {
	chars := []rune(line)
	var out strings.Builder
	inString := false
	escaped := false
	pendingSpace := false

	for _, ch := range chars {
		if inString {
			out.WriteRune(ch)
			if ch == '"' && !escaped {
				inString = false
			}
			if ch == '\\' && !escaped {
				escaped = true
			} else {
				escaped = false
			}
			continue
		}

		if ch == '"' {
			if pendingSpace && out.Len() > 0 {
				out.WriteRune(' ')
				pendingSpace = false
			}
			inString = true
			escaped = false
			out.WriteRune(ch)
			continue
		}

		if ch == ' ' || ch == '\t' || ch == '\r' {
			pendingSpace = true
			continue
		}

		if pendingSpace && out.Len() > 0 {
			out.WriteRune(' ')
		}
		pendingSpace = false
		out.WriteRune(ch)
	}

	return strings.TrimSpace(out.String())
}

func normalizeWhitespace(source string) string {
	rawLines := strings.Split(source, "\n")
	lines := make([]string, 0, len(rawLines))

	for _, line := range rawLines {
		normalized := collapseSpacesOutsideStrings(line)
		if strings.TrimSpace(normalized) != "" {
			lines = append(lines, normalized)
		}
	}
	return strings.TrimSpace(strings.Join(lines, "\n"))
}

func preprocess(source string) (string, error) {
	if err := checkInvalidChars(source); err != nil {
		return "", err
	}
	noComments, err := removeComments(source)
	if err != nil {
		return "", err
	}
	return normalizeWhitespace(noComments), nil
}

// ============================================================
// ЛР2. Лексический анализатор
// ============================================================

var keywords = map[string]bool{
	"int": true, "bool": true, "for": true, "while": true,
	"if": true, "else": true, "return": true, "void": true,
	"float": true, "double": true, "char": true, "string": true,
	"include": true, "iostream": true, "std": true,
}

var boolConsts = map[string]bool{"true": true, "false": true}

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

func tokenize(source string) ([]Token, error) {
	var tokens []Token
	chars := []rune(source)
	i := 0
	line, col := 1, 1

	for i < len(chars) {
		ch := chars[i]

		if ch == '\n' {
			line++
			col = 1
			i++
			continue
		}

		if ch == ' ' || ch == '\t' || ch == '\r' {
			col++
			i++
			continue
		}

		startLine, startCol := line, col

		// Строковый литерал
		if ch == '"' {
			var s strings.Builder
			s.WriteRune('"')
			i++
			col++
			escaped := false
			closed := false

			for i < len(chars) {
				c := chars[i]
				if c == '\n' {
					return nil, CompilerError{Stage: "Лексическая", Msg: "незакрытый строковый литерал", Line: startLine, Column: startCol}
				}
				s.WriteRune(c)
				i++
				col++

				if c == '"' && !escaped {
					closed = true
					break
				}
				if c == '\\' && !escaped {
					escaped = true
				} else {
					escaped = false
				}
			}

			if !closed {
				return nil, CompilerError{Stage: "Лексическая", Msg: "незакрытый строковый литерал", Line: startLine, Column: startCol}
			}

			tokens = append(tokens, Token{Type: CONSTANT_STRING, Value: s.String(), Line: startLine, Column: startCol})
			continue
		}

		// Числовая константа
		if unicode.IsDigit(ch) {
			var num strings.Builder
			dots := 0
			for i < len(chars) && (unicode.IsDigit(chars[i]) || chars[i] == '.') {
				if chars[i] == '.' {
					dots++
					if dots > 1 {
						return nil, CompilerError{Stage: "Лексическая", Msg: "некорректное число: больше одной точки", Line: startLine, Column: startCol}
					}
				}
				num.WriteRune(chars[i])
				i++
				col++
			}

			if i < len(chars) && (unicode.IsLetter(chars[i]) || chars[i] == '_') {
				return nil, CompilerError{Stage: "Лексическая", Msg: "идентификатор не может начинаться с цифры", Line: startLine, Column: startCol}
			}

			ttype := CONSTANT_INT
			if dots == 1 {
				ttype = CONSTANT_FLOAT
			}
			tokens = append(tokens, Token{Type: ttype, Value: num.String(), Line: startLine, Column: startCol})
			continue
		}

		// Идентификатор, ключевое слово или bool-константа
		if unicode.IsLetter(ch) || ch == '_' {
			var word strings.Builder
			for i < len(chars) && (unicode.IsLetter(chars[i]) || unicode.IsDigit(chars[i]) || chars[i] == '_') {
				word.WriteRune(chars[i])
				i++
				col++
			}
			value := word.String()
			ttype := IDENTIFIER
			if boolConsts[value] {
				ttype = CONSTANT_BOOL
			} else if keywords[value] {
				ttype = KEYWORD
			}
			tokens = append(tokens, Token{Type: ttype, Value: value, Line: startLine, Column: startCol})
			continue
		}

		// Двухсимвольный оператор
		if i+1 < len(chars) {
			two := string([]rune{chars[i], chars[i+1]})
			if operators2[two] {
				tokens = append(tokens, Token{Type: OPERATOR, Value: two, Line: startLine, Column: startCol})
				i += 2
				col += 2
				continue
			}
		}

		// Односимвольный оператор
		if operators1[ch] {
			tokens = append(tokens, Token{Type: OPERATOR, Value: string(ch), Line: startLine, Column: startCol})
			i++
			col++
			continue
		}

		// Разделитель
		if delimiters[ch] {
			tokens = append(tokens, Token{Type: DELIMITER, Value: string(ch), Line: startLine, Column: startCol})
			i++
			col++
			continue
		}

		return nil, CompilerError{Stage: "Лексическая", Msg: fmt.Sprintf("недопустимый символ '%s'", string(ch)), Line: startLine, Column: startCol}
	}

	tokens = append(tokens, Token{Type: EOF, Value: "", Line: line, Column: col})
	return tokens, nil
}

func formatTokens(tokens []Token) string {
	var b strings.Builder
	fmt.Fprintf(&b, "%-5s | %-18s | %-18s | %s\n", "№", "Лексема", "Тип", "Строка")
	b.WriteString(strings.Repeat("-", 60) + "\n")
	for i, t := range tokens {
		if t.Type == EOF {
			continue
		}
		fmt.Fprintf(&b, "%-5d | %-18s | %-18s | %d\n", i+1, t.Value, t.Type, t.Line)
	}
	return b.String()
}

// ============================================================
// ЛР3. AST и синтаксический анализатор рекурсивным спуском
// ============================================================

type ASTNode struct {
	Kind     string
	Attrs    map[string]string
	Children []*ASTNode
}

func newNode(kind string) *ASTNode {
	return &ASTNode{Kind: kind, Attrs: map[string]string{}, Children: []*ASTNode{}}
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

func (n *ASTNode) get(k string) string { return n.Attrs[k] }

func printAST(node *ASTNode, prefix string, last bool) {
	connector := "└── "
	if !last {
		connector = "├── "
	}
	line := prefix + connector + node.Kind
	if len(node.Attrs) > 0 {
		keys := make([]string, 0, len(node.Attrs))
		for k := range node.Attrs {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		parts := make([]string, 0, len(keys))
		for _, k := range keys {
			parts = append(parts, k+": "+node.Attrs[k])
		}
		line += " [" + strings.Join(parts, ", ") + "]"
	}
	fmt.Println(line)

	childPrefix := prefix + "    "
	if !last {
		childPrefix = prefix + "│   "
	}
	for i, child := range node.Children {
		printAST(child, childPrefix, i == len(node.Children)-1)
	}
}

type Parser struct {
	tokens []Token
	pos    int
}

func newParser(tokens []Token) *Parser { return &Parser{tokens: tokens} }

func (p *Parser) current() Token {
	if p.pos < len(p.tokens) {
		return p.tokens[p.pos]
	}
	return Token{Type: EOF}
}

func (p *Parser) peekType() string { return p.current().Type }
func (p *Parser) peekVal() string  { return p.current().Value }

func (p *Parser) advance() Token {
	t := p.current()
	p.pos++
	return t
}

func (p *Parser) match(ttype, tval string) bool {
	t := p.current()
	return t.Type == ttype && (tval == "" || t.Value == tval)
}

func (p *Parser) expect(ttype, tval string) (Token, error) {
	t := p.current()
	if t.Type != ttype || (tval != "" && t.Value != tval) {
		want := ttype
		if tval != "" {
			want = "'" + tval + "'"
		}
		return t, CompilerError{Stage: "Синтаксическая", Msg: fmt.Sprintf("ожидался %s, получен '%s' (%s)", want, t.Value, t.Type), Line: t.Line, Column: t.Column}
	}
	return p.advance(), nil
}

func parseTypeName(p *Parser) (string, error) {
	// std::string
	if p.match(KEYWORD, "std") {
		_, _ = p.expect(KEYWORD, "std")
		if _, err := p.expect(OPERATOR, "::"); err != nil {
			return "", err
		}
		t, err := p.expect(KEYWORD, "string")
		if err != nil {
			return "", err
		}
		return "std::" + t.Value, nil
	}

	t, err := p.expect(KEYWORD, "")
	if err != nil {
		return "", err
	}
	switch t.Value {
	case "int", "bool", "float", "double", "char", "string", "void":
		if t.Value == "string" {
			return "std::string", nil
		}
		return t.Value, nil
	default:
		return "", CompilerError{Stage: "Синтаксическая", Msg: fmt.Sprintf("ожидался тип данных, получено '%s'", t.Value), Line: t.Line, Column: t.Column}
	}
}

func (p *Parser) parseProgram() (*ASTNode, error) {
	node := newNode("Program")
	for p.match(DELIMITER, "#") {
		inc, err := p.parseInclude()
		if err != nil {
			return nil, err
		}
		node.add(inc)
	}

	for p.peekType() != EOF {
		fn, err := p.parseFuncDecl()
		if err != nil {
			return nil, err
		}
		node.add(fn)
	}
	return node, nil
}

func (p *Parser) parseInclude() (*ASTNode, error) {
	if _, err := p.expect(DELIMITER, "#"); err != nil {
		return nil, err
	}
	if _, err := p.expect(KEYWORD, "include"); err != nil {
		return nil, err
	}
	if _, err := p.expect(OPERATOR, "<"); err != nil {
		return nil, err
	}
	lib := p.current()
	if lib.Type != KEYWORD && lib.Type != IDENTIFIER {
		return nil, CompilerError{Stage: "Синтаксическая", Msg: "ожидалось имя подключаемой библиотеки", Line: lib.Line, Column: lib.Column}
	}
	p.advance()
	if _, err := p.expect(OPERATOR, ">"); err != nil {
		return nil, err
	}
	return newNode("Include").set("lib", lib.Value).set("line", fmt.Sprint(lib.Line)), nil
}

func (p *Parser) parseFuncDecl() (*ASTNode, error) {
	start := p.current()
	retType, err := parseTypeName(p)
	if err != nil {
		return nil, err
	}
	nameTok, err := p.expect(IDENTIFIER, "")
	if err != nil {
		return nil, err
	}
	node := newNode("FuncDecl").set("name", nameTok.Value).set("return_type", retType).set("line", fmt.Sprint(start.Line))

	if _, err := p.expect(DELIMITER, "("); err != nil {
		return nil, err
	}
	params, err := p.parseParamList()
	if err != nil {
		return nil, err
	}
	node.add(params)
	if _, err := p.expect(DELIMITER, ")"); err != nil {
		return nil, err
	}

	if !p.match(DELIMITER, "{") {
		t := p.current()
		return nil, CompilerError{Stage: "Синтаксическая", Msg: fmt.Sprintf("функция '%s' не имеет тела — ожидался '{'", nameTok.Value), Line: t.Line, Column: t.Column}
	}
	block, err := p.parseBlock()
	if err != nil {
		return nil, err
	}
	node.add(block)
	return node, nil
}

func (p *Parser) parseParamList() (*ASTNode, error) {
	node := newNode("ParamList")
	if p.match(DELIMITER, ")") {
		return node, nil
	}
	param, err := p.parseParam()
	if err != nil {
		return nil, err
	}
	node.add(param)
	for p.match(DELIMITER, ",") {
		p.advance()
		param, err := p.parseParam()
		if err != nil {
			return nil, err
		}
		node.add(param)
	}
	return node, nil
}

func (p *Parser) parseParam() (*ASTNode, error) {
	start := p.current()
	typ, err := parseTypeName(p)
	if err != nil {
		return nil, err
	}
	name, err := p.expect(IDENTIFIER, "")
	if err != nil {
		return nil, err
	}
	return newNode("Param").set("name", name.Value).set("type", typ).set("line", fmt.Sprint(start.Line)), nil
}

func (p *Parser) parseBlock() (*ASTNode, error) {
	start, err := p.expect(DELIMITER, "{")
	if err != nil {
		return nil, err
	}
	node := newNode("Block").set("line", fmt.Sprint(start.Line))
	for !p.match(DELIMITER, "}") {
		if p.peekType() == EOF {
			return nil, CompilerError{Stage: "Синтаксическая", Msg: "незакрытый блок: ожидался '}'", Line: start.Line, Column: start.Column}
		}
		stmt, err := p.parseStmt()
		if err != nil {
			return nil, err
		}
		node.add(stmt)
	}
	if _, err := p.expect(DELIMITER, "}"); err != nil {
		return nil, err
	}
	return node, nil
}

func (p *Parser) parseStmt() (*ASTNode, error) {
	t := p.current()

	if p.match(DELIMITER, ";") {
		p.advance()
		return nil, nil
	}

	if p.match(DELIMITER, "{") {
		return p.parseBlock()
	}

	if t.Type == KEYWORD {
		switch t.Value {
		case "std":
			// std::string s = ... или std::cout << ...;
			if p.pos+2 < len(p.tokens) && p.tokens[p.pos+1].Value == "::" && p.tokens[p.pos+2].Value == "string" {
				return p.parseVarDecl()
			}
			return p.parseExprStmt()
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

	if t.Type == IDENTIFIER {
		next := ""
		if p.pos+1 < len(p.tokens) {
			next = p.tokens[p.pos+1].Value
		}
		if next == "=" {
			return p.parseAssignStmt()
		}
		if next == "++" || next == "--" {
			return p.parseIncExprOrStmt()
		}
		return p.parseExprStmt()
	}

	if t.Type == KEYWORD && t.Value == "std" {
		return p.parseExprStmt()
	}

	return nil, CompilerError{Stage: "Синтаксическая", Msg: fmt.Sprintf("неожиданный токен '%s' (%s) — не является началом оператора", t.Value, t.Type), Line: t.Line, Column: t.Column}
}

func (p *Parser) parseVarDecl() (*ASTNode, error) {
	start := p.current()
	typ, err := parseTypeName(p)
	if err != nil {
		return nil, err
	}
	name, err := p.expect(IDENTIFIER, "")
	if err != nil {
		return nil, err
	}
	node := newNode("VarDecl").set("type", typ).set("name", name.Value).set("line", fmt.Sprint(start.Line))

	if p.match(OPERATOR, "=") {
		p.advance()
		expr, err := p.parseExpr()
		if err != nil {
			return nil, err
		}
		node.add(expr)
	}
	if p.match(DELIMITER, ";") {
		p.advance()
	}
	return node, nil
}

func (p *Parser) parseAssignStmt() (*ASTNode, error) {
	name, err := p.expect(IDENTIFIER, "")
	if err != nil {
		return nil, err
	}
	if _, err := p.expect(OPERATOR, "="); err != nil {
		return nil, err
	}
	expr, err := p.parseExpr()
	if err != nil {
		return nil, err
	}
	if p.match(DELIMITER, ";") {
		p.advance()
	}
	return newNode("AssignStmt").set("target", name.Value).set("line", fmt.Sprint(name.Line)).add(expr), nil
}

func (p *Parser) parseIfStmt() (*ASTNode, error) {
	start, _ := p.expect(KEYWORD, "if")
	if _, err := p.expect(DELIMITER, "("); err != nil {
		return nil, err
	}
	cond, err := p.parseExpr()
	if err != nil {
		return nil, err
	}
	cond.Kind = "Condition"
	if _, err := p.expect(DELIMITER, ")"); err != nil {
		return nil, err
	}
	thenBlock, err := p.parseBlock()
	if err != nil {
		return nil, err
	}
	thenBlock.Kind = "ThenBlock"

	node := newNode("IfStmt").set("line", fmt.Sprint(start.Line)).add(cond).add(thenBlock)
	if p.match(KEYWORD, "else") {
		p.advance()
		elseBlock, err := p.parseBlock()
		if err != nil {
			return nil, err
		}
		elseBlock.Kind = "ElseBlock"
		node.add(elseBlock)
	}
	return node, nil
}

func (p *Parser) parseForStmt() (*ASTNode, error) {
	start, _ := p.expect(KEYWORD, "for")
	if _, err := p.expect(DELIMITER, "("); err != nil {
		return nil, err
	}

	init, err := p.parseVarDecl()
	if err != nil {
		return nil, err
	}
	cond, err := p.parseExpr()
	if err != nil {
		return nil, err
	}
	cond.Kind = "Condition"
	if _, err := p.expect(DELIMITER, ";"); err != nil {
		return nil, err
	}
	step, err := p.parseIncExprOrStmt()
	if err != nil {
		return nil, err
	}
	if _, err := p.expect(DELIMITER, ")"); err != nil {
		return nil, err
	}
	body, err := p.parseBlock()
	if err != nil {
		return nil, err
	}
	return newNode("ForStmt").set("line", fmt.Sprint(start.Line)).add(init).add(cond).add(step).add(body), nil
}

func (p *Parser) parseWhileStmt() (*ASTNode, error) {
	start, _ := p.expect(KEYWORD, "while")
	if _, err := p.expect(DELIMITER, "("); err != nil {
		return nil, err
	}
	cond, err := p.parseExpr()
	if err != nil {
		return nil, err
	}
	cond.Kind = "Condition"
	if _, err := p.expect(DELIMITER, ")"); err != nil {
		return nil, err
	}
	body, err := p.parseBlock()
	if err != nil {
		return nil, err
	}
	return newNode("WhileStmt").set("line", fmt.Sprint(start.Line)).add(cond).add(body), nil
}

func (p *Parser) parseReturnStmt() (*ASTNode, error) {
	start, _ := p.expect(KEYWORD, "return")
	node := newNode("ReturnStmt").set("line", fmt.Sprint(start.Line))
	if !p.match(DELIMITER, ";") {
		expr, err := p.parseExpr()
		if err != nil {
			return nil, err
		}
		node.add(expr)
	}
	if p.match(DELIMITER, ";") {
		p.advance()
	}
	return node, nil
}

func (p *Parser) parseExprStmt() (*ASTNode, error) {
	start := p.current()
	expr, err := p.parseExpr()
	if err != nil {
		return nil, err
	}
	if p.match(DELIMITER, ";") {
		p.advance()
	}
	return newNode("ExprStmt").set("line", fmt.Sprint(start.Line)).add(expr), nil
}

func (p *Parser) parseIncExprOrStmt() (*ASTNode, error) {
	name, err := p.expect(IDENTIFIER, "")
	if err != nil {
		return nil, err
	}
	op := p.current()
	if op.Type != OPERATOR || (op.Value != "++" && op.Value != "--") {
		return nil, CompilerError{Stage: "Синтаксическая", Msg: "ожидался инкремент или декремент", Line: op.Line, Column: op.Column}
	}
	p.advance()
	if p.match(DELIMITER, ";") {
		p.advance()
	}
	return newNode("IncStmt").set("target", name.Value).set("op", op.Value).set("line", fmt.Sprint(name.Line)), nil
}

func precedence(op string) int {
	switch op {
	case "||":
		return 1
	case "&&":
		return 2
	case "==", "!=":
		return 3
	case "<", "<=", ">", ">=":
		return 4
	case "<<", ">>":
		return 5
	case "+", "-":
		return 6
	case "*", "/", "%":
		return 7
	}
	return 0
}

func (p *Parser) parseExpr() (*ASTNode, error) {
	return p.parseBinaryExpr(1)
}

func (p *Parser) parseBinaryExpr(minPrec int) (*ASTNode, error) {
	left, err := p.parsePrimary()
	if err != nil {
		return nil, err
	}

	for p.peekType() == OPERATOR {
		op := p.peekVal()
		prec := precedence(op)
		if prec < minPrec || op == "++" || op == "--" || op == "=" {
			break
		}
		opTok := p.advance()
		right, err := p.parseBinaryExpr(prec + 1)
		if err != nil {
			return nil, err
		}
		left = newNode("BinaryExpr").set("op", opTok.Value).set("line", fmt.Sprint(opTok.Line)).add(left).add(right)
	}
	return left, nil
}

func (p *Parser) parsePrimary() (*ASTNode, error) {
	t := p.current()

	if t.Type == IDENTIFIER || (t.Type == KEYWORD && t.Value == "std") {
		p.advance()

		if p.match(OPERATOR, "::") {
			p.advance()
			member := p.current()
			if member.Type != IDENTIFIER && member.Type != KEYWORD {
				return nil, CompilerError{Stage: "Синтаксическая", Msg: "после :: ожидался идентификатор", Line: member.Line, Column: member.Column}
			}
			p.advance()
			return newNode("MemberExpr").set("object", t.Value).set("member", member.Value).set("line", fmt.Sprint(t.Line)), nil
		}

		if p.match(DELIMITER, "(") {
			p.advance()
			call := newNode("CallExpr").set("func", t.Value).set("line", fmt.Sprint(t.Line))
			args, err := p.parseArgList()
			if err != nil {
				return nil, err
			}
			call.add(args)
			if _, err := p.expect(DELIMITER, ")"); err != nil {
				return nil, err
			}
			return call, nil
		}

		return newNode("Identifier").set("name", t.Value).set("line", fmt.Sprint(t.Line)), nil
	}

	if t.Type == CONSTANT_INT || t.Type == CONSTANT_FLOAT || t.Type == CONSTANT_STRING || t.Type == CONSTANT_BOOL {
		p.advance()
		return newNode("Literal").set("type", t.Type).set("value", t.Value).set("line", fmt.Sprint(t.Line)), nil
	}

	if p.match(DELIMITER, "(") {
		p.advance()
		expr, err := p.parseExpr()
		if err != nil {
			return nil, err
		}
		if _, err := p.expect(DELIMITER, ")"); err != nil {
			return nil, err
		}
		return expr, nil
	}

	return nil, CompilerError{Stage: "Синтаксическая", Msg: fmt.Sprintf("ожидалось выражение, получен '%s' (%s)", t.Value, t.Type), Line: t.Line, Column: t.Column}
}

func (p *Parser) parseArgList() (*ASTNode, error) {
	node := newNode("ArgList")
	if p.match(DELIMITER, ")") {
		return node, nil
	}
	first, err := p.parseExpr()
	if err != nil {
		return nil, err
	}
	node.add(first)
	for p.match(DELIMITER, ",") {
		p.advance()
		arg, err := p.parseExpr()
		if err != nil {
			return nil, err
		}
		node.add(arg)
	}
	return node, nil
}

// ============================================================
// ЛР4. Семантический анализатор и генерация триад
// ============================================================

const (
	TypeInt     = "int"
	TypeFloat   = "float"
	TypeDouble  = "double"
	TypeBool    = "bool"
	TypeChar    = "char"
	TypeString  = "std::string"
	TypeVoid    = "void"
	TypeStream  = "std::ostream"
	TypeUnknown = "unknown"
)

var numericTypes = map[string]bool{TypeInt: true, TypeFloat: true, TypeDouble: true}

type Symbol struct {
	Name        string
	Type        string
	Category    string
	Scope       string
	Declared    bool
	Initialized bool
	Line        int
	Extra       string
}

type Scope struct {
	Name    string
	Parent  *Scope
	Symbols map[string]*Symbol
}

func newScope(name string, parent *Scope) *Scope {
	return &Scope{Name: name, Parent: parent, Symbols: map[string]*Symbol{}}
}

func (s *Scope) resolve(name string) *Symbol {
	for cur := s; cur != nil; cur = cur.Parent {
		if sym, ok := cur.Symbols[name]; ok {
			return sym
		}
	}
	return nil
}

type Triad struct {
	Index int
	Op    string
	Arg1  string
	Arg2  string
}

func (t Triad) String() string {
	if t.Arg2 == "" {
		return fmt.Sprintf("%d) (%s, %s)", t.Index, t.Op, t.Arg1)
	}
	return fmt.Sprintf("%d) (%s, %s, %s)", t.Index, t.Op, t.Arg1, t.Arg2)
}

type SemanticAnalyzer struct {
	global          *Scope
	scope           *Scope
	scopeCounter    int
	symbols         []*Symbol
	errors          []CompilerError
	triads          []Triad
	functions       map[string]*Symbol
	currentFunction *Symbol
}

func newSemanticAnalyzer() *SemanticAnalyzer {
	g := newScope("global", nil)
	a := &SemanticAnalyzer{
		global:    g,
		scope:     g,
		functions: map[string]*Symbol{},
	}
	a.installBuiltins()
	return a
}

func (a *SemanticAnalyzer) installBuiltins() {
	cout := &Symbol{Name: "std::cout", Type: TypeStream, Category: "builtin", Scope: "global", Declared: true, Initialized: true, Extra: "стандартный поток вывода"}
	endl := &Symbol{Name: "std::endl", Type: TypeStream, Category: "builtin", Scope: "global", Declared: true, Initialized: true, Extra: "манипулятор конца строки"}
	a.global.Symbols[cout.Name] = cout
	a.global.Symbols[endl.Name] = endl
	a.symbols = append(a.symbols, cout, endl)
}

func (a *SemanticAnalyzer) addError(msg string, node *ASTNode) {
	line := atoiSafe(node.get("line"))
	a.errors = append(a.errors, CompilerError{Stage: "Семантическая", Msg: msg, Line: line})
}

func atoiSafe(s string) int {
	n := 0
	for _, ch := range s {
		if ch < '0' || ch > '9' {
			return 0
		}
		n = n*10 + int(ch-'0')
	}
	return n
}

func (a *SemanticAnalyzer) pushScope(prefix string) {
	a.scopeCounter++
	a.scope = newScope(fmt.Sprintf("%s_%d", prefix, a.scopeCounter), a.scope)
}

func (a *SemanticAnalyzer) popScope() {
	if a.scope.Parent != nil {
		a.scope = a.scope.Parent
	}
}

func (a *SemanticAnalyzer) declare(name, typ, category string, node *ASTNode, initialized bool, extra string) *Symbol {
	if existing, ok := a.scope.Symbols[name]; ok {
		a.addError(fmt.Sprintf("повторное объявление идентификатора '%s' в области видимости '%s'", name, a.scope.Name), node)
		return existing
	}
	sym := &Symbol{
		Name:        name,
		Type:        normalizeType(typ),
		Category:    category,
		Scope:       a.scope.Name,
		Declared:    true,
		Initialized: initialized,
		Line:        atoiSafe(node.get("line")),
		Extra:       extra,
	}
	a.scope.Symbols[name] = sym
	a.symbols = append(a.symbols, sym)
	if category == "function" {
		a.functions[name] = sym
	}
	return sym
}

func (a *SemanticAnalyzer) newTriad(op, arg1, arg2 string) string {
	t := Triad{Index: len(a.triads) + 1, Op: op, Arg1: arg1, Arg2: arg2}
	a.triads = append(a.triads, t)
	return fmt.Sprintf("^%d", t.Index)
}

func (a *SemanticAnalyzer) patchTriad(index int, arg1 *string, arg2 *string) {
	if index <= 0 || index > len(a.triads) {
		return
	}
	if arg1 != nil {
		a.triads[index-1].Arg1 = *arg1
	}
	if arg2 != nil {
		a.triads[index-1].Arg2 = *arg2
	}
}

func (a *SemanticAnalyzer) nextTriadIndex() int { return len(a.triads) + 1 }
func triadRef(i int) string                     { return fmt.Sprintf("^%d", i) }

func analyzeSemantic(ast *ASTNode) ([]*Symbol, []CompilerError, []Triad) {
	a := newSemanticAnalyzer()
	a.analyze(ast)
	return a.symbols, a.errors, a.triads
}

func (a *SemanticAnalyzer) analyze(ast *ASTNode) {
	if ast == nil || ast.Kind != "Program" {
		a.addError("корневой узел AST должен быть Program", ast)
		return
	}

	// Первый проход: объявления функций. Так add можно вызвать из main.
	for _, child := range ast.Children {
		if child.Kind == "FuncDecl" {
			a.declareFunctionHeader(child)
		}
	}

	// Второй проход: тела функций.
	for _, child := range ast.Children {
		if child.Kind == "FuncDecl" {
			a.analyzeFunction(child)
		}
	}
}

func (a *SemanticAnalyzer) declareFunctionHeader(fn *ASTNode) {
	name := fn.get("name")
	returnType := fn.get("return_type")
	params := getChild(fn, "ParamList")
	paramTypes := []string{}
	if params != nil {
		for _, p := range params.Children {
			paramTypes = append(paramTypes, normalizeType(p.get("type")))
		}
	}
	a.declare(name, returnType, "function", fn, true, "("+strings.Join(paramTypes, ", ")+")")
}

func (a *SemanticAnalyzer) analyzeFunction(fn *ASTNode) {
	fnSym := a.functions[fn.get("name")]
	a.currentFunction = fnSym
	a.pushScope("function_" + fn.get("name"))

	params := getChild(fn, "ParamList")
	if params != nil {
		for _, p := range params.Children {
			a.declare(p.get("name"), p.get("type"), "parameter", p, true, "")
		}
	}

	body := getChild(fn, "Block")
	if body != nil {
		a.analyzeBlock(body, false)
	}

	a.popScope()
	a.currentFunction = nil
}

func (a *SemanticAnalyzer) analyzeBlock(block *ASTNode, createScope bool) {
	if createScope {
		a.pushScope("block")
	}
	for _, stmt := range block.Children {
		a.analyzeStmt(stmt)
	}
	if createScope {
		a.popScope()
	}
}

func (a *SemanticAnalyzer) analyzeStmt(stmt *ASTNode) {
	if stmt == nil {
		return
	}
	switch stmt.Kind {
	case "Block", "ThenBlock", "ElseBlock":
		a.analyzeBlock(stmt, true)
	case "VarDecl":
		a.analyzeVarDecl(stmt)
	case "AssignStmt":
		a.analyzeAssign(stmt)
	case "ExprStmt":
		if len(stmt.Children) > 0 {
			a.analyzeExpr(stmt.Children[0])
		}
	case "IncStmt":
		a.analyzeInc(stmt)
	case "ReturnStmt":
		a.analyzeReturn(stmt)
	case "IfStmt":
		a.analyzeIf(stmt)
	case "ForStmt":
		a.analyzeFor(stmt)
	case "WhileStmt":
		a.analyzeWhile(stmt)
	default:
		a.addError("неподдерживаемый оператор AST: "+stmt.Kind, stmt)
	}
}

func (a *SemanticAnalyzer) analyzeVarDecl(decl *ASTNode) {
	name := decl.get("name")
	typ := decl.get("type")
	initialized := len(decl.Children) > 0
	sym := a.declare(name, typ, "variable", decl, initialized, "")

	if len(decl.Children) > 0 {
		initType, initPlace := a.analyzeExpr(decl.Children[0])
		if !isAssignable(sym.Type, initType) {
			a.addError(fmt.Sprintf("тип инициализатора '%s' несовместим с типом переменной '%s'", initType, sym.Type), decl)
		}
		sym.Initialized = true
		a.newTriad(":=", name, initPlace)
	}
}

func (a *SemanticAnalyzer) analyzeAssign(stmt *ASTNode) {
	name := stmt.get("target")
	sym := a.scope.resolve(name)
	if sym == nil {
		a.addError("присваивание необъявленной переменной '"+name+"'", stmt)
		if len(stmt.Children) > 0 {
			_, place := a.analyzeExpr(stmt.Children[0])
			a.newTriad(":=", name, place)
		}
		return
	}
	if len(stmt.Children) == 0 {
		a.addError("оператор присваивания без правой части", stmt)
		return
	}
	rightType, rightPlace := a.analyzeExpr(stmt.Children[0])
	if !isAssignable(sym.Type, rightType) {
		a.addError(fmt.Sprintf("тип правой части '%s' несовместим с типом левой части '%s'", rightType, sym.Type), stmt)
	}
	sym.Initialized = true
	a.newTriad(":=", name, rightPlace)
}

func (a *SemanticAnalyzer) analyzeInc(stmt *ASTNode) {
	name := stmt.get("target")
	sym := a.scope.resolve(name)
	if sym == nil {
		a.addError("инкремент/декремент необъявленной переменной '"+name+"'", stmt)
		a.newTriad(stmt.get("op"), name, "")
		return
	}
	if !numericTypes[sym.Type] {
		a.addError(fmt.Sprintf("оператор %s применим только к числовому типу, получен '%s'", stmt.get("op"), sym.Type), stmt)
	}
	sym.Initialized = true
	a.newTriad(stmt.get("op"), name, "")
}

func (a *SemanticAnalyzer) analyzeReturn(stmt *ASTNode) {
	exprType, exprPlace := TypeVoid, ""
	if len(stmt.Children) > 0 {
		exprType, exprPlace = a.analyzeExpr(stmt.Children[0])
	}
	expected := TypeUnknown
	if a.currentFunction != nil {
		expected = a.currentFunction.Type
	}
	if !isAssignable(expected, exprType) {
		a.addError(fmt.Sprintf("тип возвращаемого выражения '%s' несовместим с типом функции '%s'", exprType, expected), stmt)
	}
	a.newTriad("return", exprPlace, "")
}

func (a *SemanticAnalyzer) analyzeIf(stmt *ASTNode) {
	if len(stmt.Children) < 2 {
		a.addError("неполный оператор if", stmt)
		return
	}
	condType, condPlace := a.analyzeExpr(stmt.Children[0])
	if !isConditionType(condType) {
		a.addError("условие if должно иметь тип bool или числовой тип, получен '"+condType+"'", stmt)
	}

	ifFalseIndex := a.nextTriadIndex()
	a.newTriad("if_false", condPlace, "?")

	a.analyzeStmt(stmt.Children[1])
	if len(stmt.Children) > 2 {
		gotoEndIndex := a.nextTriadIndex()
		a.newTriad("goto", "?", "")
		elseStart := triadRef(a.nextTriadIndex())
		a.patchTriad(ifFalseIndex, nil, &elseStart)
		a.analyzeStmt(stmt.Children[2])
		end := triadRef(a.nextTriadIndex())
		a.patchTriad(gotoEndIndex, &end, nil)
	} else {
		end := triadRef(a.nextTriadIndex())
		a.patchTriad(ifFalseIndex, nil, &end)
	}
}

func (a *SemanticAnalyzer) analyzeFor(stmt *ASTNode) {
	if len(stmt.Children) < 4 {
		a.addError("неполный оператор for", stmt)
		return
	}
	a.pushScope("for")
	a.analyzeStmt(stmt.Children[0]) // init

	loopStart := a.nextTriadIndex()
	condType, condPlace := a.analyzeExpr(stmt.Children[1])
	if !isConditionType(condType) {
		a.addError("условие for должно иметь тип bool или числовой тип, получен '"+condType+"'", stmt)
	}

	ifFalseIndex := a.nextTriadIndex()
	a.newTriad("if_false", condPlace, "?")
	a.analyzeStmt(stmt.Children[3]) // body
	a.analyzeStmt(stmt.Children[2]) // step
	a.newTriad("goto", triadRef(loopStart), "")
	end := triadRef(a.nextTriadIndex())
	a.patchTriad(ifFalseIndex, nil, &end)
	a.popScope()
}

func (a *SemanticAnalyzer) analyzeWhile(stmt *ASTNode) {
	if len(stmt.Children) < 2 {
		a.addError("неполный оператор while", stmt)
		return
	}
	loopStart := a.nextTriadIndex()
	condType, condPlace := a.analyzeExpr(stmt.Children[0])
	if !isConditionType(condType) {
		a.addError("условие while должно иметь тип bool или числовой тип, получен '"+condType+"'", stmt)
	}

	ifFalseIndex := a.nextTriadIndex()
	a.newTriad("if_false", condPlace, "?")
	a.analyzeStmt(stmt.Children[1])
	a.newTriad("goto", triadRef(loopStart), "")
	end := triadRef(a.nextTriadIndex())
	a.patchTriad(ifFalseIndex, nil, &end)
}

func (a *SemanticAnalyzer) analyzeExpr(expr *ASTNode) (string, string) {
	if expr == nil {
		return TypeUnknown, "?"
	}
	switch expr.Kind {
	case "Condition":
		if len(expr.Children) == 0 {
			// Condition может быть бывшим Identifier/Literal/BinaryExpr с атрибутами.
			if expr.get("name") != "" {
				return a.analyzeExpr(newNode("Identifier").set("name", expr.get("name")).set("line", expr.get("line")))
			}
		}
		originalKind := expr.Kind
		expr.Kind = inferConditionOriginalKind(expr)
		typ, place := a.analyzeExpr(expr)
		expr.Kind = originalKind
		return typ, place
	case "Literal":
		switch expr.get("type") {
		case CONSTANT_INT:
			return TypeInt, expr.get("value")
		case CONSTANT_FLOAT:
			return TypeDouble, expr.get("value")
		case CONSTANT_BOOL:
			return TypeBool, expr.get("value")
		case CONSTANT_STRING:
			return TypeString, expr.get("value")
		}
		return TypeUnknown, expr.get("value")
	case "Identifier":
		name := expr.get("name")
		sym := a.scope.resolve(name)
		if sym == nil {
			// Функция как идентификатор возможна перед вызовом, но обычное использование — ошибка.
			if fn, ok := a.functions[name]; ok {
				return fn.Type, name
			}
			a.addError("использование необъявленного идентификатора '"+name+"'", expr)
			return TypeUnknown, name
		}
		if (sym.Category == "variable" || sym.Category == "parameter") && !sym.Initialized {
			a.addError("использование неинициализированной переменной '"+name+"'", expr)
		}
		return sym.Type, name
	case "MemberExpr":
		object := expr.get("object")
		member := expr.get("member")
		full := object + "::" + member
		if sym := a.scope.resolve(full); sym != nil {
			return sym.Type, full
		}
		// std::cout и std::endl установлены как builtin.
		if sym := a.global.resolve(full); sym != nil {
			return sym.Type, full
		}
		return TypeUnknown, full
	case "CallExpr":
		return a.analyzeCall(expr)
	case "BinaryExpr":
		if len(expr.Children) < 2 {
			a.addError("бинарное выражение должно иметь два операнда", expr)
			return TypeUnknown, "?"
		}
		leftType, leftPlace := a.analyzeExpr(expr.Children[0])
		rightType, rightPlace := a.analyzeExpr(expr.Children[1])
		op := expr.get("op")
		resultType := a.binaryResultType(op, leftType, rightType, expr)
		return resultType, a.newTriad(op, leftPlace, rightPlace)
	default:
		a.addError("неподдерживаемое выражение AST: "+expr.Kind, expr)
		return TypeUnknown, "?"
	}
}

func inferConditionOriginalKind(expr *ASTNode) string {
	if expr.get("op") != "" {
		return "BinaryExpr"
	}
	if expr.get("name") != "" {
		return "Identifier"
	}
	if expr.get("type") != "" {
		return "Literal"
	}
	return expr.Kind
}

func (a *SemanticAnalyzer) analyzeCall(call *ASTNode) (string, string) {
	name := call.get("func")
	argsNode := getChild(call, "ArgList")
	argTypes := []string{}
	argPlaces := []string{}
	if argsNode != nil {
		for _, arg := range argsNode.Children {
			t, p := a.analyzeExpr(arg)
			argTypes = append(argTypes, t)
			argPlaces = append(argPlaces, p)
		}
	}

	fn, ok := a.functions[name]
	if !ok {
		a.addError("вызов необъявленной функции '"+name+"'", call)
		return TypeUnknown, a.newTriad("call "+name, strings.Join(argPlaces, ", "), "")
	}

	expected := parseSignature(fn.Extra)
	if len(expected) != len(argTypes) {
		a.addError(fmt.Sprintf("функция '%s' ожидает %d аргумент(ов), передано %d", name, len(expected), len(argTypes)), call)
	} else {
		for i := range expected {
			if !isAssignable(expected[i], argTypes[i]) {
				a.addError(fmt.Sprintf("аргумент %d функции '%s' имеет тип '%s', ожидался '%s'", i+1, name, argTypes[i], expected[i]), call)
			}
		}
	}
	return fn.Type, a.newTriad("call "+name, strings.Join(argPlaces, ", "), "")
}

func (a *SemanticAnalyzer) binaryResultType(op, left, right string, node *ASTNode) string {
	l, r := normalizeType(left), normalizeType(right)

	if op == "<<" || op == ">>" {
		if l == TypeStream {
			return TypeStream
		}
		a.addError(fmt.Sprintf("оператор %s ожидает поток слева, получен '%s'", op, left), node)
		return TypeStream
	}

	if op == "+" || op == "-" || op == "*" || op == "/" || op == "%" {
		if numericTypes[l] && numericTypes[r] {
			if l == TypeDouble || r == TypeDouble || l == TypeFloat || r == TypeFloat {
				return TypeDouble
			}
			return TypeInt
		}
		a.addError(fmt.Sprintf("оператор %s ожидает числовые операнды, получены '%s' и '%s'", op, left, right), node)
		return TypeUnknown
	}

	if op == "<" || op == "<=" || op == ">" || op == ">=" || op == "==" || op == "!=" {
		if areComparable(l, r) {
			return TypeBool
		}
		a.addError(fmt.Sprintf("оператор %s неприменим к типам '%s' и '%s'", op, left, right), node)
		return TypeBool
	}

	if op == "&&" || op == "||" {
		if isConditionType(l) && isConditionType(r) {
			return TypeBool
		}
		a.addError(fmt.Sprintf("логический оператор %s ожидает bool/числовые операнды, получены '%s' и '%s'", op, left, right), node)
		return TypeBool
	}

	return TypeUnknown
}

func normalizeType(t string) string {
	t = strings.TrimSpace(t)
	if t == "string" {
		return TypeString
	}
	return t
}

func isAssignable(target, source string) bool {
	t, s := normalizeType(target), normalizeType(source)
	if t == TypeUnknown || s == TypeUnknown {
		return true
	}
	if t == s {
		return true
	}
	if t == TypeDouble && (s == TypeInt || s == TypeFloat) {
		return true
	}
	if t == TypeFloat && s == TypeInt {
		return true
	}
	return false
}

func isConditionType(t string) bool {
	t = normalizeType(t)
	return t == TypeBool || numericTypes[t] || t == TypeUnknown
}

func areComparable(left, right string) bool {
	if left == TypeUnknown || right == TypeUnknown {
		return true
	}
	if left == right {
		return true
	}
	return numericTypes[left] && numericTypes[right]
}

func parseSignature(sig string) []string {
	sig = strings.TrimSpace(sig)
	if !strings.HasPrefix(sig, "(") || !strings.HasSuffix(sig, ")") {
		return nil
	}
	inside := strings.TrimSpace(sig[1 : len(sig)-1])
	if inside == "" {
		return []string{}
	}
	parts := strings.Split(inside, ",")
	for i := range parts {
		parts[i] = strings.TrimSpace(parts[i])
	}
	return parts
}

func getChild(node *ASTNode, kind string) *ASTNode {
	if node == nil {
		return nil
	}
	for _, child := range node.Children {
		if child.Kind == kind {
			return child
		}
	}
	return nil
}

func formatSymbols(symbols []*Symbol) string {
	var b strings.Builder
	fmt.Fprintf(&b, "%-14s | %-12s | %-10s | %-18s | %-8s | %-8s | %s\n", "Имя", "Тип", "Категория", "Область", "Объявл.", "Иниц.", "Строка")
	b.WriteString(strings.Repeat("-", 95) + "\n")
	for _, s := range symbols {
		fmt.Fprintf(&b, "%-14s | %-12s | %-10s | %-18s | %-8t | %-8t | %d\n", s.Name, s.Type, s.Category, s.Scope, s.Declared, s.Initialized, s.Line)
	}
	return b.String()
}

func formatTriads(triads []Triad) string {
	var b strings.Builder
	for _, t := range triads {
		b.WriteString(t.String())
		b.WriteByte('\n')
	}
	return b.String()
}

func printSection(title, text string) {
	fmt.Println()
	fmt.Println(strings.Repeat("=", 80))
	fmt.Println(title)
	fmt.Println(strings.Repeat("=", 80))
	if text != "" {
		fmt.Println(text)
	}
}

// ============================================================
// main: полный цикл ЛР1 → ЛР2 → ЛР3 → ЛР4.
// Если этап завершился ошибкой, следующий этап не запускается.
// ============================================================

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintln(os.Stderr, "Использование: go run compiler_lr4.go <test.cpp>")
		os.Exit(2)
	}

	data, err := os.ReadFile(os.Args[1])
	if err != nil {
		fmt.Fprintf(os.Stderr, "Ошибка чтения файла: %v\n", err)
		os.Exit(2)
	}

	// ЛР1
	cleaned, err := preprocess(string(data))
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	printSection("ЛР1. Очищенный код", cleaned)

	// ЛР2
	tokens, err := tokenize(cleaned)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	printSection("ЛР2. Таблица токенов", formatTokens(tokens))

	// ЛР3
	parser := newParser(tokens)
	ast, err := parser.parseProgram()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	printSection("ЛР3. Абстрактное синтаксическое дерево (AST)", "")
	printAST(ast, "", true)
	fmt.Fprintln(os.Stderr, "Синтаксический анализ завершён успешно. Ошибок не найдено.")

	// ЛР4
	symbols, semErrors, triads := analyzeSemantic(ast)
	printSection("ЛР4. Таблица символов", formatSymbols(symbols))
	printSection("ЛР4. Триады", formatTriads(triads))

	if len(semErrors) > 0 {
		fmt.Println("Семантический анализ завершён с ошибками.")
		for i, e := range semErrors {
			fmt.Printf("%d. %s\n", i+1, e.Error())
		}
		os.Exit(1)
	}

	fmt.Println("Семантический анализ завершён успешно. Ошибок не найдено.")
}
