package runtime

import (
	"github.com/yasakei/arq/internal/ast"
	"github.com/yasakei/arq/internal/parser"
	"bufio"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"runtime"
	"sort"
	"strings"
	"sync"
	"time"
)

type Env struct {
	vals     map[string]any
	parent   *Env
	schedule *taskSchedule
}

func newEnv(parent *Env) *Env { return &Env{vals: map[string]any{}, parent: parent} }

func (e *Env) get(name string) (any, bool) {
	if value, ok := e.vals[name]; ok {
		return value, true
	}
	if e.parent != nil {
		return e.parent.get(name)
	}
	return nil, false
}

func (e *Env) set(name string, value any) {
	if _, ok := e.vals[name]; ok {
		e.vals[name] = value
		return
	}
	if e.parent != nil {
		if _, ok := e.parent.get(name); ok {
			e.parent.set(name, value)
			return
		}
	}
	e.vals[name] = value
}

type fn struct {
	params []string
	body   ast.Block
	env    *Env
}

type ret struct{ v any }
type flow int

const (
	breakFlow flow = iota + 1
	continueFlow
)

func (r ret) Error() string  { return "return" }
func (f flow) Error() string { return "control flow" }

type Error struct {
	Code    string
	Pos     ast.Pos
	Path    string
	Message string
	Cause   error
}

func (e *Error) Error() string {
	code := e.Code
	if code == "" {
		code = "E200"
	}
	location := ""
	if e.Path != "" {
		location += e.Path + ":"
	}
	if e.Pos.Line > 0 {
		location += fmt.Sprintf("%d:%d", e.Pos.Line, e.Pos.Column)
	}
	if location != "" {
		location += ": "
	}
	return code + ":" + location + e.Message
}

func (e *Error) Unwrap() error { return e.Cause }

type RuntimeError = Error

func at(pos ast.Pos, err error) error {
	if err == nil {
		return nil
	}
	if _, ok := err.(*Error); ok {
		return err
	}
	return &Error{Code: "E200", Pos: pos, Message: err.Error(), Cause: err}
}

type Runner struct {
	Root     string
	Out, Err *os.File
	Tasks    map[string]ast.Task
	active   map[string]bool

	mu       sync.Mutex
	modules  map[string]*ast.File
	watchers []chan struct{}
}

func New(root string) *Runner {
	return &Runner{
		Root:    root,
		Out:     os.Stdout,
		Err:     os.Stderr,
		Tasks:   map[string]ast.Task{},
		active:  map[string]bool{},
		modules: map[string]*ast.File{},
	}
}

func (r *Runner) Load(path string) (*ast.File, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	return parser.Parse(string(b))
}

// LoadTasks parses a build file and every file it imports, registering all
// tasks without evaluating top-level statements. This is used by `arq list`,
// where discovering tasks must not run project code.
func (r *Runner) LoadTasks(path string) (*ast.File, error) {
	absolute, err := filepath.Abs(path)
	if err != nil {
		return nil, err
	}
	r.Tasks = map[string]ast.Task{}
	r.modules = map[string]*ast.File{}
	visiting := map[string]bool{}
	var collect func(string) (*ast.File, error)
	collect = func(path string) (*ast.File, error) {
		path, err = filepath.Abs(path)
		if err != nil {
			return nil, err
		}
		if file, ok := r.modules[path]; ok {
			return file, nil
		}
		if visiting[path] {
			return nil, &Error{Code: "E203", Path: path, Message: "import cycle detected"}
		}
		file, err := r.Load(path)
		if err != nil {
			return nil, err
		}
		visiting[path] = true
		for _, statement := range file.Statements {
			imp, ok := statement.(ast.Import)
			if !ok {
				continue
			}
			importPath := resolveImport(filepath.Dir(path), imp.Path)
			if _, err := collect(importPath); err != nil {
				return nil, &Error{Code: "E204", Pos: imp.At, Path: path, Message: fmt.Sprintf("import %q: %v", imp.Path, err), Cause: err}
			}
		}
		delete(visiting, path)
		r.modules[path] = file
		r.Prepare(file)
		return file, nil
	}
	return collect(absolute)
}

func (r *Runner) Prepare(f *ast.File) {
	for _, statement := range f.Statements {
		if task, ok := statement.(ast.Task); ok {
			r.Tasks[task.Name] = task
		}
	}
}

func (r *Runner) RunTask(name string) error {
	return r.runScheduled([]string{name}, newEnv(nil))
}

func (r *Runner) RunFile(path, task string) error {
	defer r.stopWatches()
	absolute, err := filepath.Abs(path)
	if err != nil {
		return err
	}
	f, err := r.Load(absolute)
	if err != nil {
		return err
	}
	r.Tasks = map[string]ast.Task{}
	r.active = map[string]bool{}
	r.modules = map[string]*ast.File{}
	env := newEnv(nil)
	if err := r.loadModule(absolute, f, env, map[string]bool{}); err != nil {
		return err
	}
	if task != "" {
		return r.runScheduledWithEnv([]string{task}, env)
	}
	return r.runTop(f, env)
}

func (r *Runner) loadModule(path string, file *ast.File, env *Env, visiting map[string]bool) error {
	path, err := filepath.Abs(path)
	if err != nil {
		return err
	}
	if _, ok := r.modules[path]; ok {
		return nil
	}
	if visiting[path] {
		return &Error{Code: "E203", Path: path, Message: "import cycle detected"}
	}
	visiting[path] = true
	for _, statement := range file.Statements {
		imp, ok := statement.(ast.Import)
		if !ok {
			continue
		}
		importPath := resolveImport(filepath.Dir(path), imp.Path)
		imported, err := r.Load(importPath)
		if err != nil {
			return &Error{Code: "E204", Pos: imp.At, Path: path, Message: fmt.Sprintf("import %q: %v", imp.Path, err), Cause: err}
		}
		if err := r.loadModule(importPath, imported, env, visiting); err != nil {
			return err
		}
	}
	delete(visiting, path)
	r.modules[path] = file
	r.Prepare(file)
	for _, statement := range file.Statements {
		switch statement.(type) {
		case ast.Import, ast.Task:
			continue
		}
		if _, err := r.evalStmt(statement, env); err != nil {
			return at(statement.Position(), err)
		}
	}
	return nil
}

func resolveImport(base, name string) string {
	path := name
	if !filepath.IsAbs(path) {
		path = filepath.Join(base, path)
	}
	if filepath.Ext(path) == "" {
		path += ".arq"
	}
	return filepath.Clean(path)
}

func (r *Runner) runTop(file *ast.File, env *Env) error {
	for _, statement := range file.Statements {
		switch statement.(type) {
		case ast.Task, ast.Import:
			continue
		}
		if _, err := r.evalStmt(statement, env); err != nil {
			return at(statement.Position(), err)
		}
	}
	return nil
}

func (r *Runner) runBlock(block ast.Block, env *Env) error {
	for _, statement := range block.Statements {
		value, err := r.evalStmt(statement, env)
		if err != nil {
			return err
		}
		switch value.(type) {
		case ret, flow:
			return nil
		}
	}
	return nil
}

func (r *Runner) evalStmt(statement ast.Stmt, env *Env) (any, error) {
	switch node := statement.(type) {
	case ast.Let:
		value, err := r.eval(node.Value, env)
		if err == nil {
			env.vals[node.Name] = value
		}
		return nil, err
	case ast.Assign:
		value, err := r.eval(node.Value, env)
		if err != nil {
			return nil, err
		}
		if node.Target == nil {
			env.set(node.Name, value)
			return nil, nil
		}
		if identifier, ok := node.Target.(ast.Ident); ok {
			env.set(identifier.Name, value)
			return nil, nil
		}
		return nil, r.setTarget(node.Target, value, env)
	case ast.ExprStmt:
		if identifier, ok := node.Expr.(ast.Ident); ok {
			if _, exists := r.Tasks[identifier.Name]; exists {
				return nil, r.runTaskInEnv(identifier.Name, env)
			}
		}
		return r.eval(node.Expr, env)
	case ast.If:
		value, err := r.eval(node.Cond, env)
		if err != nil {
			return nil, err
		}
		if truth(value) {
			return r.blockValue(node.Then, env)
		}
		if len(node.Else.Statements) > 0 {
			return r.blockValue(node.Else, env)
		}
	case ast.For:
		value, err := r.eval(node.Iterable, env)
		if err != nil {
			return nil, err
		}
		for _, item := range iterable(value) {
			env.set(node.Name, item)
			result, err := r.blockValue(node.Body, env)
			if err != nil {
				return nil, err
			}
			if result == breakFlow {
				break
			}
			if result == continueFlow {
				continue
			}
			if _, ok := result.(ret); ok {
				return result, nil
			}
		}
	case ast.Watch:
		pattern, err := r.eval(node.Pattern, env)
		if err != nil {
			return nil, err
		}
		r.startWatch(fmt.Sprint(pattern), node.Body, env)
	case ast.Function:
		env.vals[node.Name] = fn{params: node.Params, body: node.Body, env: env}
	case ast.Return:
		if node.Value == nil {
			return ret{}, nil
		}
		value, err := r.eval(node.Value, env)
		return ret{v: value}, err
	case ast.Break:
		return breakFlow, nil
	case ast.Continue:
		return continueFlow, nil
	case ast.Task, ast.Import:
		return nil, nil
	}
	return nil, nil
}

func (r *Runner) blockValue(block ast.Block, env *Env) (any, error) {
	for _, statement := range block.Statements {
		value, err := r.evalStmt(statement, env)
		if err != nil {
			return nil, err
		}
		if value != nil {
			if _, ok := value.(ret); ok {
				return value, nil
			}
			if _, ok := value.(flow); ok {
				return value, nil
			}
		}
	}
	return nil, nil
}

func truth(value any) bool {
	switch value := value.(type) {
	case nil:
		return false
	case bool:
		return value
	case string:
		return value != ""
	case float64:
		return value != 0
	default:
		return true
	}
}

func iterable(value any) []any {
	switch value := value.(type) {
	case []any:
		return value
	case string:
		result := make([]any, 0, len(value))
		for _, item := range value {
			result = append(result, string(item))
		}
		return result
	case map[string]any:
		keys := make([]string, 0, len(value))
		for key := range value {
			keys = append(keys, key)
		}
		sort.Strings(keys)
		result := make([]any, len(keys))
		for i, key := range keys {
			result[i] = key
		}
		return result
	default:
		return nil
	}
}

func (r *Runner) eval(expr ast.Expr, env *Env) (any, error) {
	var value any
	var err error
	var ok bool
	switch node := expr.(type) {
	case ast.Literal:
		if text, ok := node.Value.(string); ok {
			value = interpolate(text, env)
		} else {
			value = node.Value
		}
	case ast.Ident:
		value, ok = env.get(node.Name)
		if !ok {
			return nil, &Error{Code: "E201", Pos: node.At, Message: fmt.Sprintf("undefined variable %q", node.Name)}
		}
	case ast.Array:
		value = make([]any, 0, len(node.Items))
		for _, item := range node.Items {
			itemValue, itemErr := r.eval(item, env)
			if itemErr != nil {
				return nil, itemErr
			}
			value = append(value.([]any), itemValue)
		}
	case ast.Map:
		object := map[string]any{}
		for _, entry := range node.Entries {
			key, keyErr := r.eval(entry.Key, env)
			if keyErr != nil {
				return nil, keyErr
			}
			entryValue, valueErr := r.eval(entry.Value, env)
			if valueErr != nil {
				return nil, valueErr
			}
			object[fmt.Sprint(key)] = entryValue
		}
		value = object
	case ast.Index:
		return r.index(node, env)
	case ast.Member:
		return r.member(node, env)
	case ast.Unary:
		operand, operandErr := r.eval(node.Right, env)
		if operandErr != nil {
			return nil, operandErr
		}
		switch node.Op {
		case "!":
			value = !truth(operand)
		case "-":
			number, ok := numberValue(operand)
			if !ok {
				return nil, fmt.Errorf("cannot negate %T", operand)
			}
			value = -number
		}
	case ast.Binary:
		left, leftErr := r.eval(node.Left, env)
		if leftErr != nil {
			return nil, leftErr
		}
		if node.Op == "&&" && !truth(left) {
			return false, nil
		}
		if node.Op == "||" && truth(left) {
			return true, nil
		}
		right, rightErr := r.eval(node.Right, env)
		if rightErr != nil {
			return nil, rightErr
		}
		value, err = operate(left, node.Op, right)
	case ast.Call:
		value, err = r.call(node, env)
	default:
		err = fmt.Errorf("unsupported expression %T", expr)
	}
	if err != nil {
		return nil, at(expr.Position(), err)
	}
	return value, nil
}

func interpolate(text string, env *Env) string {
	for {
		start := strings.Index(text, "${")
		if start < 0 {
			return text
		}
		end := strings.Index(text[start+2:], "}")
		if end < 0 {
			return text
		}
		end += start + 2
		name := text[start+2 : end]
		if value, ok := env.get(name); ok {
			text = text[:start] + fmt.Sprint(value) + text[end+1:]
		} else {
			text = text[:start] + text[end+1:]
		}
	}
}

func numberValue(value any) (float64, bool) {
	number, ok := value.(float64)
	return number, ok
}

func operate(left any, operator string, right any) (any, error) {
	leftNumber, leftOK := numberValue(left)
	rightNumber, rightOK := numberValue(right)
	if leftOK && rightOK {
		switch operator {
		case "+":
			return leftNumber + rightNumber, nil
		case "-":
			return leftNumber - rightNumber, nil
		case "*":
			return leftNumber * rightNumber, nil
		case "/":
			if rightNumber == 0 {
				return nil, fmt.Errorf("division by zero")
			}
			return leftNumber / rightNumber, nil
		case "%":
			if rightNumber == 0 {
				return nil, fmt.Errorf("division by zero")
			}
			return float64(int64(leftNumber) % int64(rightNumber)), nil
		case "==":
			return leftNumber == rightNumber, nil
		case "!=":
			return leftNumber != rightNumber, nil
		case "<":
			return leftNumber < rightNumber, nil
		case ">":
			return leftNumber > rightNumber, nil
		case "<=":
			return leftNumber <= rightNumber, nil
		case ">=":
			return leftNumber >= rightNumber, nil
		}
	}
	switch operator {
	case "+":
		return fmt.Sprint(left) + fmt.Sprint(right), nil
	case "==":
		return reflect.DeepEqual(left, right), nil
	case "!=":
		return !reflect.DeepEqual(left, right), nil
	case "&&":
		return truth(left) && truth(right), nil
	case "||":
		return truth(left) || truth(right), nil
	}
	return nil, fmt.Errorf("cannot apply %s to %T and %T", operator, left, right)
}

func (r *Runner) index(node ast.Index, env *Env) (any, error) {
	target, err := r.eval(node.Target, env)
	if err != nil {
		return nil, err
	}
	key, err := r.eval(node.Key, env)
	if err != nil {
		return nil, err
	}
	switch target := target.(type) {
	case map[string]any:
		value, ok := target[fmt.Sprint(key)]
		if !ok {
			return nil, &Error{Code: "E205", Pos: node.At, Message: fmt.Sprintf("key %q not found", fmt.Sprint(key))}
		}
		return value, nil
	case []any:
		index, ok := integerIndex(key)
		if !ok || index < 0 || index >= len(target) {
			return nil, fmt.Errorf("array index out of range")
		}
		return target[index], nil
	case string:
		index, ok := integerIndex(key)
		chars := []rune(target)
		if !ok || index < 0 || index >= len(chars) {
			return nil, fmt.Errorf("string index out of range")
		}
		return string(chars[index]), nil
	default:
		return nil, fmt.Errorf("cannot index %T", target)
	}
}

func (r *Runner) member(node ast.Member, env *Env) (any, error) {
	target, err := r.eval(node.Target, env)
	if err != nil {
		return nil, err
	}
	if object, ok := target.(map[string]any); ok {
		value, exists := object[node.Name]
		if !exists {
			return nil, &Error{Code: "E205", Pos: node.At, Message: fmt.Sprintf("member %q not found", node.Name)}
		}
		return value, nil
	}
	return nil, fmt.Errorf("cannot access member %q on %T", node.Name, target)
}

func integerIndex(value any) (int, bool) {
	number, ok := value.(float64)
	return int(number), ok && number == float64(int(number))
}

func (r *Runner) setTarget(target ast.Expr, value any, env *Env) error {
	switch target := target.(type) {
	case ast.Index:
		container, err := r.eval(target.Target, env)
		if err != nil {
			return err
		}
		key, err := r.eval(target.Key, env)
		if err != nil {
			return err
		}
		switch container := container.(type) {
		case map[string]any:
			container[fmt.Sprint(key)] = value
			return nil
		case []any:
			index, ok := integerIndex(key)
			if !ok || index < 0 || index >= len(container) {
				return fmt.Errorf("array index out of range")
			}
			container[index] = value
			return nil
		default:
			return fmt.Errorf("cannot assign through %T", container)
		}
	case ast.Member:
		container, err := r.eval(target.Target, env)
		if err != nil {
			return err
		}
		if object, ok := container.(map[string]any); ok {
			object[target.Name] = value
			return nil
		}
		return fmt.Errorf("cannot assign member on %T", container)
	default:
		return fmt.Errorf("invalid assignment target")
	}
}

func shellCommand(command string) *exec.Cmd {
	if runtime.GOOS == "windows" {
		return exec.Command("cmd", "/C", command)
	}
	return exec.Command("sh", "-c", command)
}

// RunCommand executes a command discovered from a project's native task
// configuration using the same cross-platform shell behavior as arq's run
// primitive.
func RunCommand(dir, command string, out, errOut io.Writer) error {
	cmd := shellCommand(command)
	cmd.Dir = dir
	cmd.Stdout = out
	cmd.Stderr = errOut
	return cmd.Run()
}

func commandParts(args []any) ([]string, bool, error) {
	if len(args) == 0 {
		return nil, false, fmt.Errorf("command needs a program")
	}
	if len(args) == 1 {
		return []string{fmt.Sprint(args[0])}, true, nil
	}
	parts := []string{fmt.Sprint(args[0])}
	list, ok := args[1].([]any)
	if !ok {
		return nil, false, fmt.Errorf("command arguments must be an array")
	}
	for _, arg := range list {
		parts = append(parts, fmt.Sprint(arg))
	}
	return parts, false, nil
}

func (r *Runner) call(call ast.Call, env *Env) (any, error) {
	name := ""
	if identifier, ok := call.Callee.(ast.Ident); ok {
		name = identifier.Name
	}
	var args []any
	for _, arg := range call.Args {
		value, err := r.eval(arg, env)
		if err != nil {
			return nil, err
		}
		args = append(args, value)
	}
	switch name {
	case "print":
		fmt.Fprintln(r.Out, args...)
		return nil, nil
	case "color", "colour":
		return colored(args)
	case "run":
		if len(args) < 1 {
			return nil, fmt.Errorf("run needs a command")
		}
		cmd := shellCommand(fmt.Sprint(args[0]))
		cmd.Dir, cmd.Stdout, cmd.Stderr = r.Root, r.Out, r.Err
		return nil, cmd.Run()
	case "exec":
		parts, shell, err := commandParts(args)
		if err != nil {
			return nil, err
		}
		var cmd *exec.Cmd
		if shell {
			cmd = shellCommand(parts[0])
		} else {
			cmd = exec.Command(parts[0], parts[1:]...)
		}
		cmd.Dir, cmd.Stdout, cmd.Stderr = r.Root, r.Out, r.Err
		return nil, cmd.Run()
	case "exec_in":
		if len(args) < 3 {
			return nil, fmt.Errorf("exec_in needs a directory, command, and arguments")
		}
		parts, shell, err := commandParts(args[1:])
		if err != nil {
			return nil, err
		}
		var cmd *exec.Cmd
		if shell {
			cmd = shellCommand(parts[0])
		} else {
			cmd = exec.Command(parts[0], parts[1:]...)
		}
		cmd.Dir = filepath.Join(r.Root, fmt.Sprint(args[0]))
		cmd.Stdout, cmd.Stderr = r.Out, r.Err
		return nil, cmd.Run()
	case "output":
		parts, shell, err := commandParts(args)
		if err != nil {
			return nil, err
		}
		var cmd *exec.Cmd
		if shell {
			cmd = shellCommand(parts[0])
		} else {
			cmd = exec.Command(parts[0], parts[1:]...)
		}
		cmd.Dir = r.Root
		data, err := cmd.Output()
		return strings.TrimSpace(string(data)), err
	case "rust_target":
		cmd := exec.Command("rustc", "-vV")
		cmd.Dir = r.Root
		data, err := cmd.Output()
		if err != nil {
			return nil, err
		}
		for _, line := range strings.Split(string(data), "\n") {
			if strings.HasPrefix(line, "host: ") {
				return strings.TrimSpace(strings.TrimPrefix(line, "host: ")), nil
			}
		}
		return nil, fmt.Errorf("rustc did not report a host target")
	case "spawn":
		return r.spawn(args)
	case "spawn_in":
		if len(args) < 3 {
			return nil, fmt.Errorf("spawn_in needs a directory, command, and arguments")
		}
		return r.spawnAt(args[1:], filepath.Join(r.Root, fmt.Sprint(args[0])))
	case "wait":
		return waitProcess(args)
	case "kill":
		return killProcess(args)
	case "timeout":
		return r.timeout(args)
	case "sleep":
		if len(args) != 1 {
			return nil, fmt.Errorf("sleep needs seconds")
		}
		time.Sleep(time.Duration(numberOr(args[0], 0) * float64(time.Second)))
		return nil, nil
	case "parallel", "wait_all":
		return r.parallel(args, env)
	case "read":
		if len(args) < 1 {
			return nil, fmt.Errorf("read needs a path")
		}
		data, err := os.ReadFile(filepath.Join(r.Root, fmt.Sprint(args[0])))
		return string(data), err
	case "write":
		if len(args) < 2 {
			return nil, fmt.Errorf("write needs a path and value")
		}
		return nil, os.WriteFile(filepath.Join(r.Root, fmt.Sprint(args[0])), []byte(fmt.Sprint(args[1])), 0644)
	case "copy":
		if len(args) < 2 {
			return nil, fmt.Errorf("copy needs a source and destination")
		}
		in, err := os.Open(filepath.Join(r.Root, fmt.Sprint(args[0])))
		if err != nil {
			return nil, err
		}
		defer in.Close()
		out, err := os.Create(filepath.Join(r.Root, fmt.Sprint(args[1])))
		if err != nil {
			return nil, err
		}
		defer out.Close()
		_, err = io.Copy(out, in)
		return nil, err
	case "move":
		if len(args) < 2 {
			return nil, fmt.Errorf("move needs a source and destination")
		}
		return nil, os.Rename(filepath.Join(r.Root, fmt.Sprint(args[0])), filepath.Join(r.Root, fmt.Sprint(args[1])))
	case "chmod":
		if len(args) < 2 {
			return nil, fmt.Errorf("chmod needs a path and mode")
		}
		var mode uint32
		if n, ok := args[1].(float64); ok {
			mode = uint32(n)
		} else {
			return nil, fmt.Errorf("chmod mode must be a number")
		}
		return nil, os.Chmod(filepath.Join(r.Root, fmt.Sprint(args[0])), os.FileMode(mode))
	case "kill_matching":
		return nil, killMatching(filepath.Join(r.Root, fmt.Sprint(args[0])))
	case "mkdir":
		if len(args) < 1 {
			return nil, fmt.Errorf("mkdir needs a path")
		}
		return nil, os.MkdirAll(filepath.Join(r.Root, fmt.Sprint(args[0])), 0755)
	case "exists":
		if len(args) < 1 {
			return nil, fmt.Errorf("exists needs a path")
		}
		_, err := os.Stat(filepath.Join(r.Root, fmt.Sprint(args[0])))
		return err == nil, nil
	case "env":
		if len(args) < 1 {
			return nil, fmt.Errorf("env needs a variable name")
		}
		return os.Getenv(fmt.Sprint(args[0])), nil
	case "cwd":
		return r.Root, nil
	case "glob":
		if len(args) < 1 {
			return nil, fmt.Errorf("glob needs a pattern")
		}
		matches, err := filepath.Glob(filepath.Join(r.Root, fmt.Sprint(args[0])))
		result := make([]any, len(matches))
		for i, match := range matches {
			result[i] = strings.TrimPrefix(match, r.Root+string(os.PathSeparator))
		}
		return result, err
	case "keys":
		if len(args) != 1 {
			return nil, fmt.Errorf("keys needs an object")
		}
		object, ok := args[0].(map[string]any)
		if !ok {
			return nil, fmt.Errorf("keys needs an object")
		}
		keys := make([]string, 0, len(object))
		for key := range object {
			keys = append(keys, key)
		}
		sort.Strings(keys)
		result := make([]any, len(keys))
		for i, key := range keys {
			result[i] = key
		}
		return result, nil
	case "len":
		if len(args) != 1 {
			return nil, fmt.Errorf("len needs a value")
		}
		switch value := args[0].(type) {
		case []any:
			return float64(len(value)), nil
		case map[string]any:
			return float64(len(value)), nil
		case string:
			return float64(len([]rune(value))), nil
		default:
			return nil, fmt.Errorf("cannot get length of %T", args[0])
		}
	case "error":
		return nil, fmt.Errorf("%s", fmt.Sprint(args...))
	}
	if value, ok := env.get(name); ok {
		if function, ok := value.(fn); ok {
			local := newEnv(function.env)
			for i, parameter := range function.params {
				if i < len(args) {
					local.vals[parameter] = args[i]
				}
			}
			result, err := r.blockValue(function.body, local)
			if returned, ok := result.(ret); ok {
				return returned.v, err
			}
			return nil, err
		}
	}
	if task, ok := r.Tasks[name]; ok {
		return nil, r.runTaskInEnv(task.Name, env)
	}
	return nil, fmt.Errorf("unknown function or task %q", name)
}

func numberOr(value any, fallback float64) float64 {
	if number, ok := value.(float64); ok {
		return number
	}
	return fallback
}

func ReadLine() string {
	scanner := bufio.NewScanner(os.Stdin)
	if scanner.Scan() {
		return scanner.Text()
	}
	return ""
}
