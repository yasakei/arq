package runtime

import (
	"github.com/yasakei/arq/internal/ast"
	"fmt"
	"sort"
	"sync"
)

type taskSchedule struct {
	mu      sync.Mutex
	done    map[string]bool
	running map[string]bool
}

func (r *Runner) RunTasks(names ...string) error {
	if len(names) == 0 {
		for name := range r.Tasks {
			names = append(names, name)
		}
		sort.Strings(names)
	}
	return r.runScheduled(names, newEnv(nil))
}

func (r *Runner) runScheduled(names []string, env *Env) error {
	return r.runScheduledWithEnv(names, env)
}

func (r *Runner) runScheduledWithEnv(names []string, env *Env) error {
	graph := map[string][]string{}
	visiting := map[string]bool{}
	var collect func(string) error
	collect = func(name string) error {
		if _, ok := graph[name]; ok {
			return nil
		}
		task, ok := r.Tasks[name]
		if !ok {
			return fmt.Errorf("task %q not found", name)
		}
		if visiting[name] {
			return &Error{Code: "E206", Pos: task.At, Message: fmt.Sprintf("task cycle detected at %q", name)}
		}
		visiting[name] = true
		dependencies := append([]string(nil), task.Deps...)
		dependencies = append(dependencies, r.taskReferences(task.Body)...)
		seen := map[string]bool{}
		for _, dependency := range dependencies {
			if dependency == name || seen[dependency] {
				if dependency == name {
					return &Error{Code: "E206", Pos: task.At, Message: fmt.Sprintf("task cycle detected at %q", name)}
				}
				continue
			}
			if _, ok := r.Tasks[dependency]; !ok {
				continue
			}
			seen[dependency] = true
			if err := collect(dependency); err != nil {
				return err
			}
		}
		graph[name] = dependencies[:0]
		for dependency := range seen {
			graph[name] = append(graph[name], dependency)
		}
		sort.Strings(graph[name])
		delete(visiting, name)
		return nil
	}
	for _, name := range names {
		if err := collect(name); err != nil {
			return err
		}
	}

	schedule := &taskSchedule{done: map[string]bool{}, running: map[string]bool{}}
	pending := map[string]bool{}
	for name := range graph {
		pending[name] = true
	}
	for len(pending) > 0 {
		var ready []string
		for name := range pending {
			readyNow := true
			for _, dependency := range graph[name] {
				if !schedule.done[dependency] {
					readyNow = false
					break
				}
			}
			if readyNow {
				ready = append(ready, name)
			}
		}
		if len(ready) == 0 {
			return &Error{Code: "E206", Message: "task dependency cycle detected"}
		}
		sort.Strings(ready)
		errCh := make(chan error, len(ready))
		var group sync.WaitGroup
		for _, name := range ready {
			name := name
			group.Add(1)
			go func() {
				defer group.Done()
				child := newEnv(env)
				child.schedule = schedule
				errCh <- r.runTaskInEnv(name, child)
			}()
		}
		group.Wait()
		close(errCh)
		for err := range errCh {
			if err != nil {
				return err
			}
		}
		for _, name := range ready {
			delete(pending, name)
		}
	}
	return nil
}

func (r *Runner) runTaskInEnv(name string, env *Env) error {
	task, ok := r.Tasks[name]
	if !ok {
		return fmt.Errorf("task %q not found", name)
	}
	if env.schedule != nil {
		env.schedule.mu.Lock()
		if env.schedule.done[name] {
			env.schedule.mu.Unlock()
			return nil
		}
		if env.schedule.running[name] {
			env.schedule.mu.Unlock()
			return &Error{Code: "E206", Pos: task.At, Message: fmt.Sprintf("task cycle detected at %q", name)}
		}
		env.schedule.running[name] = true
		env.schedule.mu.Unlock()
		err := r.runBlock(task.Body, env)
		env.schedule.mu.Lock()
		delete(env.schedule.running, name)
		if err == nil {
			env.schedule.done[name] = true
		}
		env.schedule.mu.Unlock()
		return err
	}
	r.mu.Lock()
	if r.active[name] {
		r.mu.Unlock()
		return &Error{Code: "E206", Pos: task.At, Message: fmt.Sprintf("task cycle detected at %q", name)}
	}
	r.active[name] = true
	r.mu.Unlock()
	err := r.runBlock(task.Body, env)
	r.mu.Lock()
	delete(r.active, name)
	r.mu.Unlock()
	return err
}

func (r *Runner) taskReferences(block ast.Block) []string {
	var result []string
	seen := map[string]bool{}
	var add func(string)
	add = func(name string) {
		if _, ok := r.Tasks[name]; ok && !seen[name] {
			seen[name] = true
			result = append(result, name)
		}
	}
	var expr func(ast.Expr)
	expr = func(node ast.Expr) {
		switch node := node.(type) {
		case ast.Ident:
			add(node.Name)
		case ast.Array:
			for _, item := range node.Items {
				expr(item)
			}
		case ast.Map:
			for _, item := range node.Entries {
				expr(item.Key)
				expr(item.Value)
			}
		case ast.Index:
			expr(node.Target)
			expr(node.Key)
		case ast.Member:
			expr(node.Target)
		case ast.Unary:
			expr(node.Right)
		case ast.Binary:
			expr(node.Left)
			expr(node.Right)
		case ast.Call:
			expr(node.Callee)
			for _, arg := range node.Args {
				expr(arg)
			}
		}
	}
	var stmt func(ast.Stmt)
	stmt = func(node ast.Stmt) {
		switch node := node.(type) {
		case ast.ExprStmt:
			if identifier, ok := node.Expr.(ast.Ident); ok {
				add(identifier.Name)
			}
			expr(node.Expr)
		case ast.Let:
			expr(node.Value)
		case ast.Assign:
			expr(node.Value)
			expr(node.Target)
		case ast.If:
			expr(node.Cond)
			for _, child := range node.Then.Statements {
				stmt(child)
			}
			for _, child := range node.Else.Statements {
				stmt(child)
			}
		case ast.For:
			expr(node.Iterable)
			for _, child := range node.Body.Statements {
				stmt(child)
			}
		case ast.Function:
			for _, child := range node.Body.Statements {
				stmt(child)
			}
		case ast.Return:
			if node.Value != nil {
				expr(node.Value)
			}
		}
	}
	for _, statement := range block.Statements {
		stmt(statement)
	}
	return result
}

func (r *Runner) parallel(args []any, env *Env) (any, error) {
	var names []string
	for _, arg := range args {
		if list, ok := arg.([]any); ok {
			for _, item := range list {
				names = append(names, fmt.Sprint(item))
			}
		} else {
			names = append(names, fmt.Sprint(arg))
		}
	}
	if len(names) == 0 {
		return nil, fmt.Errorf("parallel needs task names")
	}
	return nil, r.runScheduledWithEnv(names, env)
}
