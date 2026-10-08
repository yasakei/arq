package project

import (
	"github.com/yasakei/arq/internal/ast"
	"github.com/yasakei/arq/internal/parser"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// Command is a command found in a project's own automation files. Run is
// intentionally kept as a shell command because that is how package scripts,
// Make targets, and task runners expose their commands.
type Command struct {
	Name   string
	Run    string
	Dir    string
	Source string
}

// Codebase is the information arq can infer without a build.arq file.
type Codebase struct {
	Root      string
	Templates []string
	Commands  []Command
}

var codebaseFiles = map[string]bool{
	"go.mod":            true,
	"go.work":           true,
	"go.sum":            true,
	"cargo.toml":        true,
	"cargo.lock":        true,
	"package.json":      true,
	"package-lock.json": true,
	"pnpm-lock.yaml":    true,
	"yarn.lock":         true,
	"bun.lock":          true,
	"bun.lockb":         true,
	"tsconfig.json":     true,
	"pyproject.toml":    true,
	"poetry.lock":       true,
	"requirements.txt":  true,
	"setup.py":          true,
	"setup.cfg":         true,
	"pipfile":           true,
	"pom.xml":           true,
	"build.gradle":      true,
	"build.gradle.kts":  true,
	"cmakelists.txt":    true,
	"makefile":          true,
	"gnumakefile":       true,
	"justfile":          true,
	"taskfile.yml":      true,
	"taskfile.yaml":     true,
	"composer.json":     true,
	"gemfile":           true,
	"rakefile":          true,
	"mix.exs":           true,
	"package.swift":     true,
	"build.zig":         true,
	"build.arq":         true,
	"arqfile":           true,
}

var ignoredDirectories = map[string]bool{
	".git":         true,
	".hg":          true,
	".svn":         true,
	"node_modules": true,
	"vendor":       true,
	"target":       true,
	"dist":         true,
	"build":        true,
	".venv":        true,
	"venv":         true,
	"__pycache__":  true,
	".idea":        true,
	".vscode":      true,
	".opencode":    true,
	".direnv":      true,
	".cache":       true,
}

// Root finds the nearest directory that looks like a project. It works from
// a project subdirectory as well as from a file path, which is important for
// editors and shells that invoke arq from deep inside a repository.
func Root(start string) (string, error) {
	p, err := filepath.Abs(start)
	if err != nil {
		return "", err
	}
	info, err := os.Stat(p)
	if err != nil {
		return "", err
	}
	if !info.IsDir() {
		p = filepath.Dir(p)
	}
	nearest := ""
	sourceOnly := ""
	for {
		if buildFile(p) != "" {
			return p, nil
		}
		// A repository marker wins over a nested package marker so monorepos
		// are discovered as one codebase and their commands can be qualified
		// by directory.
		if hasRepositoryMarker(p) {
			return p, nil
		}
		if nearest == "" && hasProjectMarker(p) {
			nearest = p
		}
		if sourceOnly == "" && hasSourceMarker(p) {
			sourceOnly = p
		}
		next := filepath.Dir(p)
		if next == p {
			break
		}
		p = next
	}
	if nearest != "" {
		return nearest, nil
	}
	if sourceOnly != "" {
		return sourceOnly, nil
	}
	return "", fmt.Errorf("could not detect a codebase from %s", start)
}

// DiscoverCodebase returns all information arq can infer from start. A
// build.arq file is optional: existing projects can still be used through
// their package scripts, Make targets, and other native task files.
func DiscoverCodebase(start string) (Codebase, error) {
	root, err := Root(start)
	if err != nil {
		return Codebase{}, err
	}
	commands := DiscoverCommandsAtRoot(root)
	commands = uniqueCommands(append(commands, discoverArqCommands(root)...))
	return Codebase{
		Root:      root,
		Templates: Detect(root),
		Commands:  commands,
	}, nil
}

func buildFile(root string) string {
	for _, name := range []string{"build.arq", "Arqfile", "arqfile"} {
		path := filepath.Join(root, name)
		if info, err := os.Stat(path); err == nil && !info.IsDir() {
			return path
		}
	}
	return ""
}

func hasCodebaseMarker(root string) bool {
	entries, err := os.ReadDir(root)
	if err != nil {
		return false
	}
	for _, entry := range entries {
		name := entry.Name()
		lower := strings.ToLower(name)
		if name == ".git" || codebaseFiles[lower] || sourceFile(name) || strings.HasSuffix(lower, ".csproj") || strings.HasSuffix(lower, ".fsproj") || strings.HasSuffix(lower, ".sln") {
			return true
		}
	}
	return false
}

func hasProjectMarker(root string) bool {
	entries, err := os.ReadDir(root)
	if err != nil {
		return false
	}
	for _, entry := range entries {
		name := entry.Name()
		lower := strings.ToLower(name)
		if codebaseFiles[lower] || strings.HasSuffix(lower, ".csproj") || strings.HasSuffix(lower, ".fsproj") || strings.HasSuffix(lower, ".sln") {
			return true
		}
	}
	return false
}

func hasSourceMarker(root string) bool {
	entries, err := os.ReadDir(root)
	if err != nil {
		return false
	}
	for _, entry := range entries {
		if sourceFile(entry.Name()) {
			return true
		}
	}
	return false
}

func hasRepositoryMarker(root string) bool {
	entries, err := os.ReadDir(root)
	if err != nil {
		return false
	}
	for _, entry := range entries {
		if entry.Name() == ".git" {
			return true
		}
	}
	return false
}

func sourceFile(name string) bool {
	switch strings.ToLower(filepath.Ext(name)) {
	case ".go", ".rs", ".js", ".jsx", ".mjs", ".cjs", ".ts", ".tsx", ".py", ".java", ".kt", ".kts", ".c", ".h", ".cc", ".cpp", ".cxx", ".hpp", ".cs", ".fs", ".fsx", ".zig", ".lua", ".swift", ".rb", ".ex", ".exs", ".php", ".sh":
		return true
	default:
		return false
	}
}

func walkFiles(root string, visit func(string, os.FileInfo) error) error {
	return filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			if path != root && ignoredDirectories[strings.ToLower(info.Name())] {
				return filepath.SkipDir
			}
			return nil
		}
		return visit(path, info)
	})
}

type packageManifest struct {
	Path             string
	Dir              string
	Scripts          map[string]string `json:"scripts"`
	Dependencies     map[string]string `json:"dependencies"`
	DevDependencies  map[string]string `json:"devDependencies"`
	PeerDependencies map[string]string `json:"peerDependencies"`
}

func readPackageManifest(path string) (packageManifest, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return packageManifest{}, err
	}
	var manifest packageManifest
	if err := json.Unmarshal(data, &manifest); err != nil {
		return packageManifest{}, err
	}
	manifest.Path = path
	manifest.Dir = filepath.Dir(path)
	return manifest, nil
}

func packageManifests(root string) []packageManifest {
	var manifests []packageManifest
	_ = walkFiles(root, func(path string, _ os.FileInfo) error {
		if strings.EqualFold(filepath.Base(path), "package.json") {
			if manifest, err := readPackageManifest(path); err == nil {
				manifests = append(manifests, manifest)
			}
		}
		return nil
	})
	sort.Slice(manifests, func(i, j int) bool { return manifests[i].Path < manifests[j].Path })
	return manifests
}

func detectTemplates(dir string) []string {
	root, err := Root(dir)
	if err != nil {
		root, _ = filepath.Abs(dir)
	}
	var paths []string
	_ = walkFiles(root, func(path string, _ os.FileInfo) error {
		paths = append(paths, path)
		return nil
	})
	packages := packageManifests(root)
	present := map[string]bool{}
	var families = map[string]bool{}
	mark := func(name, family string) {
		present[name] = true
		if family != "" {
			families[family] = true
		}
	}
	for _, path := range paths {
		base := filepath.Base(path)
		lower := strings.ToLower(base)
		ext := strings.ToLower(filepath.Ext(base))
		switch lower {
		case "go.mod", "go.work", "go.sum":
			mark("go", "go")
		case "cargo.toml", "cargo.lock":
			mark("rust", "rust")
		case "package.json", "package-lock.json", "pnpm-lock.yaml", "yarn.lock", "bun.lock", "bun.lockb":
			mark("node", "javascript")
		case "tsconfig.json":
			mark("typescript", "javascript")
		case "pyproject.toml", "requirements.txt", "setup.py", "setup.cfg", "pipfile", "poetry.lock":
			mark("python", "python")
		case "manage.py":
			mark("django", "python")
		case "pom.xml":
			mark("java", "jvm")
		case "build.gradle":
			mark("java", "jvm")
		case "build.gradle.kts":
			mark("kotlin", "jvm")
		case "cmakelists.txt":
			mark("cmake", "native")
		case "build.zig":
			mark("zig", "zig")
		case "package.swift":
			mark("swift", "swift")
		}
		if strings.HasSuffix(lower, ".csproj") || strings.HasSuffix(lower, ".fsproj") || strings.HasSuffix(lower, ".sln") {
			mark("csharp", "dotnet")
		}
		if lower == "app.py" {
			mark("flask", "python")
		}
		switch ext {
		case ".js", ".jsx", ".mjs", ".cjs":
			mark("node", "javascript")
		case ".ts", ".tsx":
			mark("typescript", "javascript")
		case ".py":
			mark("python", "python")
		case ".java":
			mark("java", "jvm")
		case ".kt", ".kts":
			mark("kotlin", "jvm")
		case ".c":
			mark("c", "native")
		case ".cc", ".cpp", ".cxx", ".hpp":
			mark("cpp", "native")
		case ".cs", ".fs", ".fsx":
			mark("csharp", "dotnet")
		case ".zig":
			mark("zig", "zig")
		case ".lua":
			mark("lua", "lua")
		case ".swift":
			mark("swift", "swift")
		}
	}
	for _, manifest := range packages {
		deps := map[string]bool{}
		for name := range manifest.Dependencies {
			deps[strings.ToLower(name)] = true
		}
		for name := range manifest.DevDependencies {
			deps[strings.ToLower(name)] = true
		}
		for name := range manifest.PeerDependencies {
			deps[strings.ToLower(name)] = true
		}
		switch {
		case deps["react"] || deps["react-dom"] || deps["@vitejs/plugin-react"]:
			mark("react", "javascript")
		case deps["preact"] || deps["@preact/preset-vite"]:
			mark("preact", "javascript")
		case deps["vue"] || deps["@vitejs/plugin-vue"]:
			mark("vue", "javascript")
		case deps["svelte"] || deps["@sveltejs/vite-plugin-svelte"]:
			mark("svelte", "javascript")
		}
		if deps["typescript"] {
			mark("typescript", "javascript")
		}
		if deps["django"] {
			mark("django", "python")
		}
		if deps["flask"] {
			mark("flask", "python")
		}
	}
	if len(families) > 1 {
		present["mixed"] = true
	}
	var suggestions []string
	for _, name := range Templates() {
		if present[name] {
			suggestions = append(suggestions, name)
		}
	}
	return suggestions
}

// SuggestTemplate selects the most specific starter from Detect's results.
func SuggestTemplate(names []string) string {
	priority := []string{"mixed", "react", "preact", "vue", "svelte", "django", "flask", "typescript", "node", "rust", "go", "kotlin", "java", "cmake", "cpp", "c", "csharp", "zig", "lua", "swift", "python"}
	present := map[string]bool{}
	for _, name := range names {
		present[name] = true
	}
	for _, name := range priority {
		if present[name] {
			return name
		}
	}
	return ""
}

// DiscoverCommands finds commands in the nearest codebase. It is the public
// entry point used by the CLI; the root-specific helper is useful when the
// caller has already performed discovery.
func DiscoverCommands(start string) ([]Command, error) {
	root, err := Root(start)
	if err != nil {
		return nil, err
	}
	return DiscoverCommandsAtRoot(root), nil
}

func DiscoverCommandsAtRoot(root string) []Command {
	var commands []Command
	for _, manifest := range packageManifests(root) {
		namePrefix := relativePrefix(root, filepath.Dir(manifest.Path))
		manager := packageManager(root, filepath.Dir(manifest.Path))
		for name := range manifest.Scripts {
			commands = append(commands, Command{
				Name:   qualify(namePrefix, name),
				Run:    manager + " " + packageScriptArg(manager) + " " + shellArg(name),
				Dir:    filepath.Dir(manifest.Path),
				Source: relativePath(root, manifest.Path),
			})
		}
	}
	_ = walkFiles(root, func(path string, info os.FileInfo) error {
		base := strings.ToLower(filepath.Base(path))
		switch base {
		case "makefile", "gnumakefile":
			commands = append(commands, makeCommands(root, path)...)
		case "justfile":
			commands = append(commands, justCommands(root, path)...)
		case "taskfile.yml", "taskfile.yaml":
			commands = append(commands, taskfileCommands(root, path)...)
		case "pyproject.toml":
			commands = append(commands, pyprojectCommands(root, path)...)
		case "config.toml":
			if strings.EqualFold(filepath.Base(filepath.Dir(path)), ".cargo") {
				commands = append(commands, cargoAliasCommands(root, path)...)
			}
		case "composer.json":
			commands = append(commands, composerCommands(root, path)...)
		case "rakefile":
			commands = append(commands, rakeCommands(root, path)...)
		}
		isExecutable := info.Mode().Perm()&0111 != 0 && runnableScript(root, path)
		isScriptFile := scriptExtension(path) && scriptDirectory(root, path)
		if isExecutable || isScriptFile {
			name := filepath.Base(path)
			commands = append(commands, Command{
				Name:   qualify(relativePrefix(root, filepath.Dir(path)), name),
				Run:    scriptCommand(root, path, info),
				Dir:    root,
				Source: relativePath(root, path),
			})
		}
		return nil
	})
	commands = append(commands, defaultCommands(root, commands)...)
	return uniqueCommands(commands)
}

func discoverArqCommands(root string) []Command {
	path := buildFile(root)
	if path == "" {
		return nil
	}
	var commands []Command
	loaded := map[string]bool{}
	var visit func(string) error
	visit = func(path string) error {
		absolute, err := filepath.Abs(path)
		if err != nil {
			return err
		}
		if loaded[absolute] {
			return nil
		}
		data, err := os.ReadFile(absolute)
		if err != nil {
			return err
		}
		file, err := parser.Parse(string(data))
		if err != nil {
			return err
		}
		loaded[absolute] = true
		for _, statement := range file.Statements {
			if imp, ok := statement.(ast.Import); ok {
				importPath := imp.Path
				if filepath.Ext(importPath) == "" {
					importPath += ".arq"
				}
				if !filepath.IsAbs(importPath) {
					importPath = filepath.Join(filepath.Dir(absolute), importPath)
				}
				if err := visit(filepath.Clean(importPath)); err != nil {
					return err
				}
			}
		}
		for _, statement := range file.Statements {
			if task, ok := statement.(ast.Task); ok {
				commands = append(commands, Command{
					Name:   task.Name,
					Run:    "arq " + task.Name,
					Dir:    root,
					Source: relativePath(root, absolute),
				})
			}
		}
		return nil
	}
	if err := visit(path); err != nil {
		return nil
	}
	return commands
}

func defaultCommands(root string, existing []Command) []Command {
	present := map[string]bool{}
	for _, command := range existing {
		present[command.Name] = true
	}
	var result []Command
	add := func(name, run, source string) {
		if present[name] {
			return
		}
		present[name] = true
		result = append(result, Command{Name: name, Run: run, Dir: root, Source: source})
	}
	detected := map[string]bool{}
	for _, name := range Detect(root) {
		detected[name] = true
	}
	has := func(name string) bool {
		return detected[name]
	}
	if has("go") {
		add("build", "go build ./...", "detected:go")
		add("test", "go test ./...", "detected:go")
		add("fmt", "go fmt ./...", "detected:go")
		add("vet", "go vet ./...", "detected:go")
	}
	if has("rust") {
		add("build", "cargo build", "detected:rust")
		add("check", "cargo check", "detected:rust")
		add("test", "cargo test", "detected:rust")
		add("fmt", "cargo fmt", "detected:rust")
		add("clippy", "cargo clippy", "detected:rust")
		if fileExists(filepath.Join(root, "src", "main.rs")) {
			add("run", "cargo run", "detected:rust")
		}
	}
	if has("django") {
		add("runserver", "python manage.py runserver", "detected:django")
		add("test", "python manage.py test", "detected:django")
		add("migrate", "python manage.py migrate", "detected:django")
	}
	if has("python") {
		add("compile", "python -m compileall .", "detected:python")
		add("test", "python -m unittest discover", "detected:python")
	}
	if has("java") {
		if fileExists(filepath.Join(root, "pom.xml")) {
			add("build", "mvn package", "detected:maven")
			add("test", "mvn test", "detected:maven")
		} else {
			gradle := "gradle"
			if fileExists(filepath.Join(root, "gradlew")) {
				gradle = "./gradlew"
			}
			add("build", gradle+" build", "detected:gradle")
			add("test", gradle+" test", "detected:gradle")
		}
	}
	if has("kotlin") && !has("java") {
		gradle := "gradle"
		if fileExists(filepath.Join(root, "gradlew")) {
			gradle = "./gradlew"
		}
		add("build", gradle+" build", "detected:kotlin")
		add("test", gradle+" test", "detected:kotlin")
	}
	if has("cmake") {
		add("build", "cmake --build build", "detected:cmake")
		add("test", "ctest --test-dir build", "detected:cmake")
	}
	if has("csharp") {
		add("build", "dotnet build", "detected:dotnet")
		add("test", "dotnet test", "detected:dotnet")
	}
	if has("swift") {
		add("build", "swift build", "detected:swift")
		add("test", "swift test", "detected:swift")
	}
	if has("zig") {
		add("build", "zig build", "detected:zig")
		add("test", "zig build test", "detected:zig")
	}
	return result
}

func fileExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}

func relativePath(root, path string) string {
	path, err := filepath.Rel(root, path)
	if err != nil {
		return filepath.ToSlash(path)
	}
	return filepath.ToSlash(path)
}

func relativePrefix(root, dir string) string {
	rel := relativePath(root, dir)
	if rel == "." {
		return ""
	}
	return rel
}

func qualify(prefix, name string) string {
	if prefix == "" {
		return name
	}
	return prefix + ":" + name
}

func packageManager(root, dir string) string {
	for _, candidate := range []struct {
		name string
		file string
	}{
		{"pnpm", "pnpm-lock.yaml"},
		{"yarn", "yarn.lock"},
		{"bun", "bun.lock"},
		{"bun", "bun.lockb"},
	} {
		for _, base := range []string{dir, root} {
			if _, err := os.Stat(filepath.Join(base, candidate.file)); err == nil {
				return candidate.name
			}
		}
	}
	return "npm"
}

func packageScriptArg(manager string) string {
	if manager == "yarn" {
		return "run"
	}
	return "run"
}

func shellArg(value string) string {
	if value != "" && strings.IndexFunc(value, func(r rune) bool {
		return !(r == '_' || r == '-' || r == '.' || r == ':' || r == '/' || r == '@' || r == '+' ||
			r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9')
	}) < 0 {
		return value
	}
	return "'" + strings.ReplaceAll(value, "'", "'\\''") + "'"
}

func makeCommands(root, path string) []Command {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	prefix := relativePrefix(root, filepath.Dir(path))
	var result []Command
	for _, line := range strings.Split(string(data), "\n") {
		if line == "" || line[0] == '\t' || strings.HasPrefix(strings.TrimSpace(line), "#") {
			continue
		}
		colon := strings.IndexByte(line, ':')
		if colon < 1 || strings.Contains(line[:colon], "=") {
			continue
		}
		for _, target := range strings.Fields(line[:colon]) {
			if target == ".PHONY" || strings.HasPrefix(target, ".") || strings.ContainsAny(target, "%$()") {
				continue
			}
			result = append(result, Command{Name: qualify(prefix, target), Run: "make " + shellArg(target), Dir: filepath.Dir(path), Source: relativePath(root, path)})
		}
	}
	return result
}

func justCommands(root, path string) []Command {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	prefix := relativePrefix(root, filepath.Dir(path))
	var result []Command
	for _, line := range strings.Split(string(data), "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") || strings.HasPrefix(trimmed, "[") {
			continue
		}
		colon := strings.IndexByte(trimmed, ':')
		if colon < 1 {
			continue
		}
		if strings.Contains(trimmed[:colon], "=") {
			continue
		}
		name := strings.Fields(trimmed[:colon])[0]
		if name == "set" || strings.ContainsAny(name, "=()") {
			continue
		}
		result = append(result, Command{Name: qualify(prefix, name), Run: "just " + shellArg(name), Dir: filepath.Dir(path), Source: relativePath(root, path)})
	}
	return result
}

func taskfileCommands(root, path string) []Command {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	prefix := relativePrefix(root, filepath.Dir(path))
	var result []Command
	inTasks := false
	for _, line := range strings.Split(string(data), "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed == "tasks:" {
			inTasks = true
			continue
		}
		if !inTasks || trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}
		indent := len(line) - len(strings.TrimLeft(line, " \t"))
		if indent != 2 || strings.HasPrefix(trimmed, "-") {
			continue
		}
		colon := strings.IndexByte(trimmed, ':')
		if colon < 1 {
			continue
		}
		name := strings.TrimSpace(trimmed[:colon])
		if strings.ContainsAny(name, " #{}[]") {
			continue
		}
		result = append(result, Command{Name: qualify(prefix, name), Run: "task " + shellArg(name), Dir: filepath.Dir(path), Source: relativePath(root, path)})
	}
	return result
}

func pyprojectCommands(root, path string) []Command {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	prefix := relativePrefix(root, filepath.Dir(path))
	var result []Command
	section := ""
	for _, line := range strings.Split(string(data), "\n") {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "[") && strings.HasSuffix(trimmed, "]") {
			section = strings.Trim(trimmed, "[]")
			continue
		}
		if section != "project.scripts" && section != "tool.poetry.scripts" {
			continue
		}
		name, _, ok := strings.Cut(trimmed, "=")
		if !ok {
			continue
		}
		name = strings.TrimSpace(name)
		if !validCommandName(name) {
			continue
		}
		result = append(result, Command{Name: qualify(prefix, name), Run: name, Dir: filepath.Dir(path), Source: relativePath(root, path)})
	}
	return result
}

func cargoAliasCommands(root, path string) []Command {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	var result []Command
	inAliases := false
	for _, line := range strings.Split(string(data), "\n") {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "[") {
			inAliases = trimmed == "[alias]"
			continue
		}
		if !inAliases {
			continue
		}
		name, _, ok := strings.Cut(trimmed, "=")
		name = strings.TrimSpace(name)
		if !ok || !validCommandName(name) {
			continue
		}
		result = append(result, Command{Name: qualify(relativePrefix(root, filepath.Dir(path)), name), Run: "cargo " + name, Dir: root, Source: relativePath(root, path)})
	}
	return result
}

func composerCommands(root, path string) []Command {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	var manifest struct {
		Scripts map[string]json.RawMessage `json:"scripts"`
	}
	if json.Unmarshal(data, &manifest) != nil {
		return nil
	}
	prefix := relativePrefix(root, filepath.Dir(path))
	var result []Command
	for name := range manifest.Scripts {
		result = append(result, Command{Name: qualify(prefix, name), Run: "composer run-script " + shellArg(name), Dir: filepath.Dir(path), Source: relativePath(root, path)})
	}
	return result
}

func rakeCommands(root, path string) []Command {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	prefix := relativePrefix(root, filepath.Dir(path))
	var result []Command
	for _, line := range strings.Split(string(data), "\n") {
		trimmed := strings.TrimSpace(line)
		for _, marker := range []string{"task :", "task \"", "task '"} {
			if !strings.HasPrefix(trimmed, marker) {
				continue
			}
			name := strings.TrimLeft(trimmed[len(marker):], " :\"'")
			fields := strings.FieldsFunc(name, func(r rune) bool { return r == '"' || r == '\'' || r == ' ' || r == '}' })
			if len(fields) == 0 {
				break
			}
			name = fields[0]
			if validCommandName(name) {
				result = append(result, Command{Name: qualify(prefix, name), Run: "bundle exec rake " + shellArg(name), Dir: filepath.Dir(path), Source: relativePath(root, path)})
			}
			break
		}
	}
	return result
}

func validCommandName(name string) bool {
	if name == "" {
		return false
	}
	for _, r := range name {
		if !(r == '_' || r == '-' || r == ':' || r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9') {
			return false
		}
	}
	return true
}

func runnableScript(root, path string) bool {
	rel := filepath.ToSlash(relativePath(root, path))
	return strings.HasPrefix(rel, "bin/") || strings.HasPrefix(rel, "scripts/") || filepath.Dir(path) == root
}

func scriptDirectory(root, path string) bool {
	rel := filepath.ToSlash(relativePath(root, path))
	return strings.HasPrefix(rel, "bin/") || strings.HasPrefix(rel, "scripts/")
}

func scriptExtension(path string) bool {
	switch strings.ToLower(filepath.Ext(path)) {
	case ".sh", ".bash", ".py", ".js", ".mjs", ".cjs", ".rb":
		return true
	default:
		return false
	}
}

func scriptCommand(root, path string, info os.FileInfo) string {
	rel := relativePath(root, path)
	if info.Mode().Perm()&0111 != 0 {
		return "./" + rel
	}
	switch strings.ToLower(filepath.Ext(path)) {
	case ".sh", ".bash":
		return "sh " + shellArg(rel)
	case ".py":
		return "python " + shellArg(rel)
	case ".js", ".mjs", ".cjs":
		return "node " + shellArg(rel)
	case ".rb":
		return "ruby " + shellArg(rel)
	default:
		return "./" + rel
	}
}

func uniqueCommands(commands []Command) []Command {
	sort.Slice(commands, func(i, j int) bool {
		if commands[i].Name != commands[j].Name {
			return commands[i].Name < commands[j].Name
		}
		return commands[i].Source < commands[j].Source
	})
	used := map[string]bool{}
	for i := range commands {
		base := commands[i].Name
		if !used[base] {
			used[base] = true
			continue
		}
		prefix := strings.TrimSuffix(filepath.Base(commands[i].Source), filepath.Ext(commands[i].Source))
		candidate := prefix + ":" + base
		for n := 2; used[candidate]; n++ {
			candidate = fmt.Sprintf("%s:%s:%d", prefix, base, n)
		}
		commands[i].Name = candidate
		used[candidate] = true
	}
	sort.Slice(commands, func(i, j int) bool { return commands[i].Name < commands[j].Name })
	return commands
}
