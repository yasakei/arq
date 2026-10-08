package main

import (
	"github.com/yasakei/arq/internal/project"
	"github.com/yasakei/arq/internal/runtime"
	"github.com/yasakei/arq/internal/tui"
	"bufio"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

var version = "0.1.0"

func main() {
	if len(os.Args) < 2 {
		runTask("")
		return
	}
	switch os.Args[1] {
	case "version", "--version", "-V":
		fmt.Println("arq", version)
	case "help", "--help", "-h":
		printUsage()
	case "init":
		initCmd(os.Args[2:])
	case "new":
		newCmd(os.Args[2:])
	case "templates":
		for _, name := range project.Templates() {
			fmt.Println(name)
		}
	case "detect":
		detectCmd()
	case "list":
		listCmd()
	case "check":
		checkCmd()
	case "fmt":
		fmtCmd(os.Args[2:])
	case "repl":
		replCmd()
	case "watch":
		watchCmd(os.Args[2:])
	case "run":
		if len(os.Args) < 3 {
			fatal("run needs a task")
		}
		runTask(os.Args[2])
	default:
		if strings.HasSuffix(os.Args[1], ".arq") {
			task := ""
			if len(os.Args) > 2 {
				task = os.Args[2]
			}
			runFile(os.Args[1], task)
		} else {
			runTask(os.Args[1])
		}
	}
}

func replCmd() {
	r := runtime.NewREPL(".")
	s := bufio.NewScanner(os.Stdin)
	for {
		fmt.Print("arq> ")
		if !s.Scan() {
			return
		}
		line := strings.TrimSpace(s.Text())
		if line == "" {
			continue
		}
		if line == "exit" || line == "quit" {
			return
		}
		value, err := r.Eval(line)
		if err != nil {
			fmt.Fprintln(os.Stderr, "arq:", err)
			continue
		}
		if value != nil {
			fmt.Println(value)
		}
	}
}

func fmtCmd(args []string) {
	check := false
	for _, arg := range args {
		if arg == "--check" {
			check = true
		}
	}
	path := "build.arq"
	if len(args) == 0 {
		path = find()
	} else if !strings.HasPrefix(args[0], "-") {
		path = args[0]
	}
	b, err := os.ReadFile(path)
	if err != nil {
		fatal(err.Error())
	}
	formatted := formatSource(string(b))
	if check {
		if string(b) != formatted {
			fatal(path + " is not formatted")
		}
		fmt.Println("ok", path)
		return
	}
	if err := os.WriteFile(path, []byte(formatted), 0644); err != nil {
		fatal(err.Error())
	}
	fmt.Println("formatted", path)
}

func formatSource(source string) string {
	lines := strings.Split(strings.ReplaceAll(source, "\r\n", "\n"), "\n")
	for i := range lines {
		lines[i] = strings.TrimRight(lines[i], " \t")
	}
	return strings.TrimRight(strings.Join(lines, "\n"), "\n") + "\n"
}

func watchCmd(args []string) {
	task := ""
	if len(args) > 0 {
		task = args[0]
	}
	path := find()
	var last time.Time
	for {
		info, err := os.Stat(path)
		if err != nil {
			fatal(err.Error())
		}
		if info.ModTime().After(last) {
			last = info.ModTime()
			runFile(path, task)
		}
		time.Sleep(500 * time.Millisecond)
	}
}
func initCmd(args []string) {
	f := flag.NewFlagSet("init", flag.ExitOnError)
	non := f.Bool("non-interactive", false, "disable the interactive setup")
	tpl := f.String("template", "", "scaffold template")
	f.Parse(args)
	if f.NArg() > 1 {
		fatal("usage: arq init [dir]")
	}
	dir := "."
	if f.NArg() > 0 {
		dir = f.Arg(0)
	}
	explicitTemplate := false
	f.Visit(func(x *flag.Flag) {
		explicitTemplate = explicitTemplate || x.Name == "template"
	})
	if !explicitTemplate && !*non && isTTY() {
		x, e := tui.Run(dir)
		if e == nil && x != "" {
			*tpl = x
		}
	}
	if *tpl == "" {
		*tpl = project.SuggestTemplate(project.Detect(dir))
		if *tpl == "" {
			*tpl = "empty"
		}
	}
	if e := project.Init(dir, *tpl); e != nil {
		fatal(e.Error())
	}
	fmt.Printf("created %s\n", filepath.Join(dir, "build.arq"))
}
func newCmd(args []string) {
	if len(args) < 1 || len(args) > 2 {
		fatal("usage: arq new <template> [dir]")
	}
	dir := "."
	if len(args) == 2 {
		dir = args[1]
	}
	if e := project.New(dir, args[0]); e != nil {
		fatal(e.Error())
	}
	fmt.Printf("created %s\n", filepath.Join(dir, "build.arq"))
}
func isTTY() bool { fi, _ := os.Stdin.Stat(); return fi.Mode()&os.ModeCharDevice != 0 }
func find() string {
	p, e := project.Discover(".")
	if e != nil {
		fatal("could not find build.arq in the detected codebase")
	}
	return p
}

func locateProject() (string, string, error) {
	root, err := project.Root(".")
	if err != nil {
		return "", "", err
	}
	build, _ := project.Discover(root)
	return root, build, nil
}

func runTask(task string) {
	root, build, err := locateProject()
	if err != nil {
		fatal(err.Error())
	}
	if build != "" {
		if task == "" {
			runFile(build, task)
			return
		}
		r := runtime.New(root)
		if _, err := r.LoadTasks(build); err != nil {
			fatal(err.Error())
		}
		if _, ok := r.Tasks[task]; ok {
			runFile(build, task)
			return
		}
	}
	runDetectedCommand(root, task)
}

func runDetectedCommand(root, task string) {
	if task == "" {
		task = "build"
	}
	for _, command := range project.DiscoverCommandsAtRoot(root) {
		if command.Name != task {
			continue
		}
		if err := runtime.RunCommand(command.Dir, command.Run, os.Stdout, os.Stderr); err != nil {
			fatal(err.Error())
		}
		return
	}
	fatal(fmt.Sprintf("task or command %q not found", task))
}

func runFile(p, t string) {
	r := runtime.New(filepath.Dir(p))
	if e := r.RunFile(p, t); e != nil {
		fatal(e.Error())
	}
}
func listCmd() {
	root, build, err := locateProject()
	if err != nil {
		fatal(err.Error())
	}
	names := map[string]bool{}
	if build != "" {
		r := runtime.New(root)
		if _, err := r.LoadTasks(build); err != nil {
			fatal(err.Error())
		}
		for name := range r.Tasks {
			names[name] = true
		}
	}
	for _, command := range project.DiscoverCommandsAtRoot(root) {
		names[command.Name] = true
	}
	if len(names) == 0 {
		fatal("no tasks or commands detected")
	}
	var sorted []string
	for name := range names {
		sorted = append(sorted, name)
	}
	sort.Strings(sorted)
	for _, name := range sorted {
		fmt.Println(name)
	}
}
func checkCmd() {
	root, build, err := locateProject()
	if err != nil {
		fatal(err.Error())
	}
	if build == "" {
		codebase, err := project.DiscoverCodebase(root)
		if err != nil {
			fatal(err.Error())
		}
		fmt.Printf("ok %s", codebase.Root)
		if len(codebase.Templates) > 0 {
			fmt.Printf(" (%s)", strings.Join(codebase.Templates, ", "))
		}
		fmt.Println()
		return
	}
	r := runtime.New(root)
	if _, err := r.LoadTasks(build); err != nil {
		fatal(err.Error())
	}
	fmt.Println("ok", build)
}

func detectCmd() {
	codebase, err := project.DiscoverCodebase(".")
	if err != nil {
		fatal(err.Error())
	}
	fmt.Println("root", codebase.Root)
	if len(codebase.Templates) == 0 {
		fmt.Println("templates", "unknown")
	} else {
		fmt.Println("templates", strings.Join(codebase.Templates, ", "))
	}
	if len(codebase.Commands) == 0 {
		fmt.Println("commands", "none")
		return
	}
	fmt.Println("commands")
	for _, command := range codebase.Commands {
		fmt.Printf("  %-24s %s (%s)\n", command.Name, command.Run, command.Source)
	}
}
func fatal(s string) { fmt.Fprintln(os.Stderr, "arq: "+s); os.Exit(1) }

func printUsage() {
	fmt.Println(`arq - project automation runtime

Usage:
  arq [task]              run a task from build.arq or a discovered command
  arq run <task>          run a task explicitly
  arq file.arq [task]     run a specific .arq file
  arq list                list tasks and discovered commands
  arq detect              show detected project root, types, and commands
  arq check               validate the build file or codebase detection
  arq init [dir]          create a build.arq (add --template, --non-interactive)
  arq new <template> [dir]  scaffold a new project with starter files
  arq templates           list available scaffolding templates
  arq fmt [path]          format a build file (add --check to verify only)
  arq repl                start an interactive session
  arq watch [task]        re-run when the build file changes
  arq version             print the arq version
  arq help                print this help`)
}
