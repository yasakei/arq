package parser

import (
	"github.com/yasakei/arq/internal/ast"
	"github.com/yasakei/arq/internal/lexer"
	"fmt"
	"strconv"
)

var prec = map[string]int{"||": 1, "&&": 2, "==": 3, "!=": 3, "<": 4, ">": 4, "<=": 4, ">=": 4, "+": 5, "-": 5, "*": 6, "/": 6, "%": 6}

func (p *Parser) expr(min int) (ast.Expr, error) {
	left, err := p.primary()
	if err != nil {
		return nil, err
	}
	for p.cur().Kind == lexer.Op {
		op := p.cur()
		pr, ok := prec[op.Text]
		if !ok || pr < min {
			break
		}
		p.next()
		right, err := p.expr(pr + 1)
		if err != nil {
			return nil, err
		}
		left = ast.Binary{At: pos(op), Left: left, Op: op.Text, Right: right}
	}
	return left, nil
}

func (p *Parser) primary() (ast.Expr, error) {
	x := p.cur()
	p.next()
	var value ast.Expr
	switch x.Kind {
	case lexer.Number:
		n, err := strconv.ParseFloat(x.Text, 64)
		if err != nil {
			return nil, fmt.Errorf("E102:%d:%d: invalid number %q", x.Line, x.Column, x.Text)
		}
		value = ast.Literal{At: pos(x), Value: n}
	case lexer.String:
		value = ast.Literal{At: pos(x), Value: x.Text}
	case lexer.Ident:
		value = ast.Ident{At: pos(x), Name: x.Text}
	case lexer.Keyword:
		switch x.Text {
		case "true":
			value = ast.Literal{At: pos(x), Value: true}
		case "false":
			value = ast.Literal{At: pos(x), Value: false}
		case "null":
			value = ast.Literal{At: pos(x), Value: nil}
		default:
			return nil, fmt.Errorf("E102:%d:%d: expected expression", x.Line, x.Column)
		}
	case lexer.Op:
		if x.Text != "!" && x.Text != "-" {
			return nil, fmt.Errorf("E102:%d:%d: expected expression", x.Line, x.Column)
		}
		right, err := p.primary()
		return ast.Unary{At: pos(x), Op: x.Text, Right: right}, err
	case lexer.LParen:
		var err error
		value, err = p.expr(0)
		if err == nil {
			_, err = p.want(lexer.RParen)
		}
		if err != nil {
			return nil, err
		}
	case lexer.LBracket:
		items, err := p.list(lexer.RBracket)
		if err != nil {
			return nil, err
		}
		value = ast.Array{At: pos(x), Items: items}
	case lexer.LBrace:
		entries, err := p.object()
		if err != nil {
			return nil, err
		}
		value = ast.Map{At: pos(x), Entries: entries}
	default:
		return nil, fmt.Errorf("E102:%d:%d: expected expression", x.Line, x.Column)
	}
	return p.post(value)
}

func (p *Parser) newlines() {
	for p.cur().Kind == lexer.Newline {
		p.next()
	}
}

func (p *Parser) list(end lexer.Kind) ([]ast.Expr, error) {
	var items []ast.Expr
	p.newlines()
	for p.cur().Kind != end {
		item, err := p.expr(0)
		if err != nil {
			return nil, err
		}
		items = append(items, item)
		p.newlines()
		if p.cur().Kind != lexer.Comma {
			break
		}
		p.next()
		p.newlines()
	}
	_, err := p.want(end)
	return items, err
}

func (p *Parser) object() ([]ast.MapEntry, error) {
	var entries []ast.MapEntry
	p.newlines()
	for p.cur().Kind != lexer.RBrace {
		x := p.cur()
		var key ast.Expr
		var err error
		if x.Kind == lexer.Ident || x.Kind == lexer.Keyword {
			p.next()
			key = ast.Literal{At: pos(x), Value: x.Text}
		} else {
			key, err = p.expr(0)
		}
		if err != nil {
			return nil, err
		}
		if _, err = p.want(lexer.Colon); err != nil {
			return nil, err
		}
		p.newlines()
		value, err := p.expr(0)
		if err != nil {
			return nil, err
		}
		entries = append(entries, ast.MapEntry{Key: key, Value: value})
		p.newlines()
		if p.cur().Kind != lexer.Comma {
			break
		}
		p.next()
		p.newlines()
	}
	_, err := p.want(lexer.RBrace)
	return entries, err
}

func (p *Parser) post(value ast.Expr) (ast.Expr, error) {
	for {
		switch p.cur().Kind {
		case lexer.LParen:
			p.next()
			args, err := p.list(lexer.RParen)
			if err != nil {
				return nil, err
			}
			value = ast.Call{At: value.Position(), Callee: value, Args: args}
		case lexer.LBracket:
			p.next()
			key, err := p.expr(0)
			if err != nil {
				return nil, err
			}
			if _, err = p.want(lexer.RBracket); err != nil {
				return nil, err
			}
			value = ast.Index{At: value.Position(), Target: value, Key: key}
		case lexer.Dot:
			p.next()
			name := p.cur()
			if name.Kind != lexer.Ident && name.Kind != lexer.Keyword {
				return nil, fmt.Errorf("E102:%d:%d: expected member name", name.Line, name.Column)
			}
			p.next()
			value = ast.Member{At: value.Position(), Target: value, Name: name.Text}
		default:
			if id, ok := value.(ast.Ident); ok && (p.cur().Kind == lexer.String || p.cur().Kind == lexer.Number) {
				arg, err := p.primary()
				if err != nil {
					return nil, err
				}
				value = ast.Call{At: id.Position(), Callee: id, Args: []ast.Expr{arg}}
			}
			return value, nil
		}
	}
}
