package parser

import (
	"github.com/yasakei/arq/internal/ast"
	"github.com/yasakei/arq/internal/lexer"
	"fmt"
)

type Parser struct {
	t []lexer.Token
	i int
}

func Parse(src string) (*ast.File, error) {
	t, err := lexer.Lex(src)
	if err != nil {
		return nil, err
	}
	p := &Parser{t: t}
	return p.file()
}

func (p *Parser) cur() lexer.Token { return p.t[p.i] }

func (p *Parser) next() {
	if p.i < len(p.t)-1 {
		p.i++
	}
}

func (p *Parser) skip() {
	for p.cur().Kind == lexer.Newline || p.cur().Kind == lexer.Semicolon {
		p.next()
	}
}

func (p *Parser) want(k lexer.Kind) (lexer.Token, error) {
	x := p.cur()
	if x.Kind != k {
		return x, fmt.Errorf("E102:%d:%d: expected %q, got %q", x.Line, x.Column, k, x.Text)
	}
	p.next()
	return x, nil
}

func (p *Parser) file() (*ast.File, error) {
	f := &ast.File{}
	for {
		p.skip()
		if p.cur().Kind == lexer.EOF {
			return f, nil
		}
		s, err := p.stmt()
		if err != nil {
			return nil, err
		}
		f.Statements = append(f.Statements, s)
	}
}

func (p *Parser) stmt() (ast.Stmt, error) {
	x := p.cur()
	if x.Kind == lexer.Keyword {
		switch x.Text {
		case "let", "var":
			p.next()
			n, err := p.want(lexer.Ident)
			if err != nil {
				return nil, err
			}
			if _, err = p.want(lexer.Assign); err != nil {
				return nil, err
			}
			v, err := p.expr(0)
			return ast.Let{At: pos(x), Name: n.Text, Value: v, Mutable: x.Text == "var"}, err
		case "import":
			return p.importStmt()
		case "if":
			return p.ifStmt()
		case "for":
			return p.forStmt()
		case "watch":
			return p.watchStmt()
		case "fn":
			return p.fnStmt()
		case "task":
			return p.taskStmt()
		case "return":
			p.next()
			if p.cur().Kind == lexer.Newline || p.cur().Kind == lexer.RBrace || p.cur().Kind == lexer.EOF {
				return ast.Return{At: pos(x)}, nil
			}
			v, err := p.expr(0)
			return ast.Return{At: pos(x), Value: v}, err
		case "break":
			p.next()
			return ast.Break{At: pos(x)}, nil
		case "continue":
			p.next()
			return ast.Continue{At: pos(x)}, nil
		}
	}

	lhs, err := p.expr(0)
	if err != nil {
		return nil, err
	}
	if p.cur().Kind == lexer.Assign {
		at := lhs.Position()
		p.next()
		value, err := p.expr(0)
		if err != nil {
			return nil, err
		}
		switch target := lhs.(type) {
		case ast.Ident:
			return ast.Assign{At: at, Name: target.Name, Value: value, Target: target}, nil
		case ast.Index, ast.Member:
			return ast.Assign{At: at, Value: value, Target: lhs}, nil
		default:
			return nil, fmt.Errorf("E102:%d:%d: invalid assignment target", at.Line, at.Column)
		}
	}
	return ast.ExprStmt{At: pos(x), Expr: lhs}, nil
}

func (p *Parser) watchStmt() (ast.Stmt, error) {
	x := p.cur()
	p.next()
	pattern, err := p.expr(0)
	if err != nil {
		return nil, err
	}
	body, err := p.block()
	return ast.Watch{At: pos(x), Pattern: pattern, Body: body}, err
}

func pos(t lexer.Token) ast.Pos { return ast.Pos{Line: t.Line, Column: t.Column} }

func (p *Parser) importStmt() (ast.Stmt, error) {
	x := p.cur()
	p.next()
	var alias, path string
	if p.cur().Kind == lexer.String {
		path = p.cur().Text
		p.next()
	} else if p.cur().Kind == lexer.Ident {
		alias = p.cur().Text
		p.next()
		if p.cur().Kind == lexer.Keyword && (p.cur().Text == "from" || p.cur().Text == "as") {
			p.next()
		}
		if p.cur().Kind == lexer.String || p.cur().Kind == lexer.Ident {
			path = p.cur().Text
			p.next()
		} else {
			return nil, fmt.Errorf("E102:%d:%d: expected import path", p.cur().Line, p.cur().Column)
		}
	} else {
		return nil, fmt.Errorf("E102:%d:%d: expected import path", p.cur().Line, p.cur().Column)
	}
	return ast.Import{At: pos(x), Path: path, Alias: alias}, nil
}

func (p *Parser) block() (ast.Block, error) {
	x, err := p.want(lexer.LBrace)
	if err != nil {
		return ast.Block{}, err
	}
	b := ast.Block{At: pos(x)}
	for {
		p.skip()
		if p.cur().Kind == lexer.RBrace {
			p.next()
			return b, nil
		}
		if p.cur().Kind == lexer.EOF {
			return b, fmt.Errorf("E102:%d:%d: expected }", p.cur().Line, p.cur().Column)
		}
		s, err := p.stmt()
		if err != nil {
			return b, err
		}
		b.Statements = append(b.Statements, s)
	}
}

func (p *Parser) ifStmt() (ast.Stmt, error) {
	x := p.cur()
	p.next()
	c, err := p.expr(0)
	if err != nil {
		return nil, err
	}
	b, err := p.block()
	if err != nil {
		return nil, err
	}
	var el ast.Block
	p.skip()
	if p.cur().Kind == lexer.Keyword && p.cur().Text == "else" {
		p.next()
		el, err = p.block()
	}
	return ast.If{At: pos(x), Cond: c, Then: b, Else: el}, err
}

func (p *Parser) forStmt() (ast.Stmt, error) {
	x := p.cur()
	p.next()
	n, err := p.want(lexer.Ident)
	if err != nil {
		return nil, err
	}
	if p.cur().Text != "in" {
		return nil, fmt.Errorf("E102:%d:%d: expected in", p.cur().Line, p.cur().Column)
	}
	p.next()
	v, err := p.expr(0)
	if err != nil {
		return nil, err
	}
	b, err := p.block()
	return ast.For{At: pos(x), Name: n.Text, Iterable: v, Body: b}, err
}

func (p *Parser) fnStmt() (ast.Stmt, error) {
	x := p.cur()
	p.next()
	n, err := p.want(lexer.Ident)
	if err != nil {
		return nil, err
	}
	if _, err = p.want(lexer.LParen); err != nil {
		return nil, err
	}
	var params []string
	for p.cur().Kind != lexer.RParen {
		a, err := p.want(lexer.Ident)
		if err != nil {
			return nil, err
		}
		params = append(params, a.Text)
		if p.cur().Kind == lexer.Comma {
			p.next()
		} else {
			break
		}
	}
	if _, err = p.want(lexer.RParen); err != nil {
		return nil, err
	}
	b, err := p.block()
	return ast.Function{At: pos(x), Name: n.Text, Params: params, Body: b}, err
}

func (p *Parser) taskStmt() (ast.Stmt, error) {
	x := p.cur()
	p.next()
	n, err := p.want(lexer.Ident)
	if err != nil {
		return nil, err
	}
	var deps []string
	if p.cur().Kind == lexer.Keyword && (p.cur().Text == "after" || p.cur().Text == "depends") {
		p.next()
		if p.cur().Kind == lexer.Keyword && p.cur().Text == "on" {
			p.next()
		}
		for {
			d, err := p.want(lexer.Ident)
			if err != nil {
				return nil, err
			}
			deps = append(deps, d.Text)
			if p.cur().Kind != lexer.Comma {
				break
			}
			p.next()
		}
	}
	b, err := p.block()
	return ast.Task{At: pos(x), Name: n.Text, Deps: deps, Body: b}, err
}
