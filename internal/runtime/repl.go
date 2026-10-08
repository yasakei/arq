package runtime

import (
	"github.com/yasakei/arq/internal/ast"
	"github.com/yasakei/arq/internal/parser"
)

type REPL struct {
	Runner *Runner
	env    *Env
}

type Session = REPL

func NewREPL(root string) *REPL {
	runner := New(root)
	return &REPL{Runner: runner, env: newEnv(nil)}
}

func NewSession(root string) *REPL { return NewREPL(root) }

func (r *REPL) Reset() {
	if r == nil {
		return
	}
	r.env = newEnv(nil)
	r.Runner.Tasks = map[string]ast.Task{}
}

func (r *REPL) Eval(source string) (any, error) {
	file, err := parser.Parse(source)
	if err != nil {
		return nil, err
	}
	r.Runner.Prepare(file)
	var result any
	for _, statement := range file.Statements {
		value, err := r.Runner.evalStmt(statement, r.env)
		if err != nil {
			return nil, at(statement.Position(), err)
		}
		if value != nil {
			if returned, ok := value.(ret); ok {
				result = returned.v
			} else {
				result = value
			}
		}
	}
	return result, nil
}

func (r *REPL) Run(source string) error {
	_, err := r.Eval(source)
	return err
}
