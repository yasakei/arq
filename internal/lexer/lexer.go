package lexer

import "fmt"

type Kind int

const (
	EOF Kind = iota
	Ident
	Number
	String
	Newline
	LBrace
	RBrace
	LParen
	RParen
	LBracket
	RBracket
	Comma
	Semicolon
	Colon
	Dot
	Op
	Assign
	Keyword
)

type Token struct {
	Kind         Kind
	Text         string
	Line, Column int
}

var keywords = map[string]bool{"let": true, "var": true, "if": true, "else": true, "for": true, "in": true, "fn": true, "return": true, "break": true, "continue": true, "task": true, "true": true, "false": true, "null": true, "import": true, "after": true, "depends": true, "on": true, "as": true, "from": true, "watch": true}

func Lex(src string) ([]Token, error) {
	var out []Token
	line, col := 1, 1
	b := []rune(src)
	add := func(k Kind, s string, l, c int) { out = append(out, Token{k, s, l, c}) }
	for i := 0; i < len(b); {
		c := b[i]
		l, cc := line, col
		if c == ' ' || c == '\t' || c == '\r' {
			i++
			col++
			continue
		}
		if c == '\n' {
			add(Newline, "\n", l, cc)
			i++
			line++
			col = 1
			continue
		}
		if c == '#' {
			for i < len(b) && b[i] != '\n' {
				i++
				col++
			}
			continue
		}
		if c == '"' {
			if i+2 < len(b) && b[i+1] == '"' && b[i+2] == '"' {
				i += 3
				col += 3
				start := i
				for i+2 < len(b) && !(b[i] == '"' && b[i+1] == '"' && b[i+2] == '"') {
					if b[i] == '\n' {
						line++
						col = 1
					} else {
						col++
					}
					i++
				}
				if i+2 >= len(b) {
					return nil, fmt.Errorf("E101:%d:%d: unterminated multiline string", l, cc)
				}
				add(String, string(b[start:i]), l, cc)
				i += 3
				col += 3
				continue
			}
			i++
			col++
			var s []rune
			for i < len(b) && b[i] != '"' {
				if b[i] == '\\' && i+1 < len(b) {
					i++
					col++
					switch b[i] {
					case 'n':
						s = append(s, '\n')
					case 't':
						s = append(s, '\t')
					default:
						s = append(s, b[i])
					}
				} else {
					s = append(s, b[i])
				}
				i++
				col++
			}
			if i >= len(b) {
				return nil, fmt.Errorf("E101:%d:%d: unterminated string", l, cc)
			}
			i++
			col++
			add(String, string(s), l, cc)
			continue
		}
		if c >= '0' && c <= '9' {
			j := i
			dots := 0
			for i < len(b) && ((b[i] >= '0' && b[i] <= '9') || b[i] == '.') {
				if b[i] == '.' {
					dots++
				}
				i++
				col++
			}
			if dots > 1 {
				return nil, fmt.Errorf("E100:%d:%d: invalid number %q", l, cc, string(b[j:i]))
			}
			add(Number, string(b[j:i]), l, cc)
			continue
		}
		if (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || c == '_' {
			start := i
			for i < len(b) && ((b[i] >= 'a' && b[i] <= 'z') || (b[i] >= 'A' && b[i] <= 'Z') || (b[i] >= '0' && b[i] <= '9') || b[i] == '_') {
				i++
				col++
			}
			s := string(b[start:i])
			if keywords[s] {
				add(Keyword, s, l, cc)
			} else {
				add(Ident, s, l, cc)
			}
			continue
		}
		one := map[rune]Kind{'{': LBrace, '}': RBrace, '(': LParen, ')': RParen, '[': LBracket, ']': RBracket, ',': Comma, ';': Semicolon, ':': Colon, '.': Dot}
		if k, ok := one[c]; ok {
			add(k, string(c), l, cc)
			i++
			col++
			continue
		}
		if c == '=' || c == '!' || c == '<' || c == '>' || c == '&' || c == '|' || c == '+' || c == '-' || c == '*' || c == '/' || c == '%' {
			j := i
			i++
			col++
			if i < len(b) && ((c == '=' && b[i] == '=') || (c == '!' && b[i] == '=') || (c == '<' && b[i] == '=') || (c == '>' && b[i] == '=') || (c == '&' && b[i] == '&') || (c == '|' && b[i] == '|')) {
				i++
				col++
			}
			s := string(b[j:i])
			if s == "=" {
				add(Assign, s, l, cc)
			} else {
				add(Op, s, l, cc)
			}
			continue
		}
		return nil, fmt.Errorf("E100:%d:%d: unexpected character %q", l, cc, c)
	}
	out = append(out, Token{EOF, "", line, col})
	return out, nil
}
