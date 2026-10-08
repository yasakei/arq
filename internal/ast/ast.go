package ast

type Pos struct{ Line, Column int }
type Node interface {
	node()
	Position() Pos
}
type Expr interface {
	Node
	expr()
}
type Stmt interface {
	Node
	stmt()
}

type File struct{ Statements []Stmt }
type Block struct {
	At         Pos
	Statements []Stmt
}

func (b Block) node()         {}
func (b Block) Position() Pos { return b.At }

type Ident struct {
	At   Pos
	Name string
}

func (i Ident) node()         {}
func (i Ident) expr()         {}
func (i Ident) Position() Pos { return i.At }

type Literal struct {
	At    Pos
	Value any
}

func (l Literal) node()         {}
func (l Literal) expr()         {}
func (l Literal) Position() Pos { return l.At }

type Array struct {
	At    Pos
	Items []Expr
}

func (a Array) node()         {}
func (a Array) expr()         {}
func (a Array) Position() Pos { return a.At }

type MapEntry struct {
	Key   Expr
	Value Expr
}

type Map struct {
	At      Pos
	Entries []MapEntry
}

func (m Map) node()         {}
func (m Map) expr()         {}
func (m Map) Position() Pos { return m.At }

type Index struct {
	At     Pos
	Target Expr
	Key    Expr
}

func (i Index) node()         {}
func (i Index) expr()         {}
func (i Index) Position() Pos { return i.At }

type Member struct {
	At     Pos
	Target Expr
	Name   string
}

func (m Member) node()         {}
func (m Member) expr()         {}
func (m Member) Position() Pos { return m.At }

type Unary struct {
	At    Pos
	Op    string
	Right Expr
}

func (u Unary) node()         {}
func (u Unary) expr()         {}
func (u Unary) Position() Pos { return u.At }

type Binary struct {
	At    Pos
	Left  Expr
	Op    string
	Right Expr
}

func (b Binary) node()         {}
func (b Binary) expr()         {}
func (b Binary) Position() Pos { return b.At }

type Call struct {
	At     Pos
	Callee Expr
	Args   []Expr
}

func (c Call) node()         {}
func (c Call) expr()         {}
func (c Call) Position() Pos { return c.At }

type ExprStmt struct {
	At   Pos
	Expr Expr
}

func (s ExprStmt) node()         {}
func (s ExprStmt) stmt()         {}
func (s ExprStmt) Position() Pos { return s.At }

type Let struct {
	At      Pos
	Name    string
	Value   Expr
	Mutable bool
}

func (s Let) node()         {}
func (s Let) stmt()         {}
func (s Let) Position() Pos { return s.At }

type Assign struct {
	At     Pos
	Name   string
	Value  Expr
	Target Expr
}

func (s Assign) node()         {}
func (s Assign) stmt()         {}
func (s Assign) Position() Pos { return s.At }

type If struct {
	At         Pos
	Cond       Expr
	Then, Else Block
}

func (s If) node()         {}
func (s If) stmt()         {}
func (s If) Position() Pos { return s.At }

type For struct {
	At       Pos
	Name     string
	Iterable Expr
	Body     Block
}

type Watch struct {
	At      Pos
	Pattern Expr
	Body    Block
}

func (s Watch) node()         {}
func (s Watch) stmt()         {}
func (s Watch) Position() Pos { return s.At }

func (s For) node()         {}
func (s For) stmt()         {}
func (s For) Position() Pos { return s.At }

type Return struct {
	At    Pos
	Value Expr
}

func (s Return) node()         {}
func (s Return) stmt()         {}
func (s Return) Position() Pos { return s.At }

type Break struct{ At Pos }

func (s Break) node()         {}
func (s Break) stmt()         {}
func (s Break) Position() Pos { return s.At }

type Continue struct{ At Pos }

func (s Continue) node()         {}
func (s Continue) stmt()         {}
func (s Continue) Position() Pos { return s.At }

type Function struct {
	At     Pos
	Name   string
	Params []string
	Body   Block
}

func (s Function) node()         {}
func (s Function) stmt()         {}
func (s Function) Position() Pos { return s.At }

type Task struct {
	At   Pos
	Name string
	Deps []string
	Body Block
}

func (s Task) node()         {}
func (s Task) stmt()         {}
func (s Task) Position() Pos { return s.At }

type Import struct {
	At    Pos
	Path  string
	Alias string
}

func (s Import) node()         {}
func (s Import) stmt()         {}
func (s Import) Position() Pos { return s.At }
