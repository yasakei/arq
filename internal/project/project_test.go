package project

import (
	"github.com/yasakei/arq/internal/parser"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestSupportedTemplates(t *testing.T) {
	want := []string{"empty", "go", "rust", "node", "typescript", "react", "preact", "vue", "svelte", "python", "django", "flask", "java", "kotlin", "c", "cpp", "cmake", "csharp", "zig", "lua", "swift", "mixed"}
	if got := SupportedTemplates(); !reflect.DeepEqual(got, want) {
		t.Fatalf("templates = %#v, want %#v", got, want)
	}
}

func TestNewWritesParseableLocalScaffolds(t *testing.T) {
	for _, name := range SupportedTemplates() {
		t.Run(name, func(t *testing.T) {
			dir := t.TempDir()
			if err := New(dir, name); err != nil {
				t.Fatal(err)
			}
			if _, err := parser.Parse(Template(name)); err != nil {
				t.Fatalf("build.arq is not parseable: %v", err)
			}
			if _, err := os.Stat(filepath.Join(dir, "build.arq")); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestInitOnlyWritesAutomation(t *testing.T) {
	dir := t.TempDir()
	if err := Init(dir, "GO"); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(dir, "build.arq")); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(dir, "go.mod")); !os.IsNotExist(err) {
		t.Fatalf("init created go.mod: %v", err)
	}
}

func TestNewRefusesExistingFilesBeforeWriting(t *testing.T) {
	dir := t.TempDir()
	mainPath := filepath.Join(dir, "main.go")
	if err := os.WriteFile(mainPath, []byte("keep"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := New(dir, "go"); err == nil {
		t.Fatal("expected existing file error")
	}
	if _, err := os.Stat(filepath.Join(dir, "build.arq")); !os.IsNotExist(err) {
		t.Fatalf("partial scaffold was written: %v", err)
	}
	b, err := os.ReadFile(mainPath)
	if err != nil {
		t.Fatal(err)
	}
	if string(b) != "keep" {
		t.Fatalf("existing file changed to %q", b)
	}
}

func TestDetectOnlyReturnsSupportedSuggestions(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "package.json"), []byte("{}"), 0644); err != nil {
		t.Fatal(err)
	}
	if got, want := Detect(dir), []string{"node"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("suggestions = %#v, want %#v", got, want)
	}
}

func TestDetectFindsNestedStacksAndMixedCodebases(t *testing.T) {
	dir := t.TempDir()
	if err := os.Mkdir(filepath.Join(dir, ".git"), 0755); err != nil {
		t.Fatal(err)
	}
	frontend := filepath.Join(dir, "frontend")
	backend := filepath.Join(dir, "backend")
	if err := os.MkdirAll(frontend, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(filepath.Join(frontend, "src"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(backend, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(frontend, "package.json"), []byte(`{"dependencies":{"react":"^19"},"scripts":{"build":"vite build"}}`), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(backend, "go.mod"), []byte("module example.com/backend\n"), 0644); err != nil {
		t.Fatal(err)
	}
	got := Detect(filepath.Join(frontend, "src"))
	want := []string{"go", "node", "react", "mixed"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("suggestions = %#v, want %#v", got, want)
	}
	root, err := Root(filepath.Join(frontend, "src"))
	if err != nil {
		t.Fatal(err)
	}
	if root != dir {
		t.Fatalf("root = %q, want %q", root, dir)
	}
}

func TestDiscoverCommandsFromProjectFiles(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "package.json"), []byte(`{"scripts":{"build":"go build ./...","test":"go test ./..."}}`), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "Makefile"), []byte("lint:\n\tgo vet ./...\n"), 0644); err != nil {
		t.Fatal(err)
	}
	commands, err := DiscoverCommands(dir)
	if err != nil {
		t.Fatal(err)
	}
	got := map[string]string{}
	for _, command := range commands {
		got[command.Name] = command.Run
	}
	if got["build"] != "npm run build" || got["test"] != "npm run test" || got["lint"] != "make lint" {
		t.Fatalf("commands = %#v", got)
	}
}

func TestRootPrefersNearestArqFileOverNestedPackage(t *testing.T) {
	dir := t.TempDir()
	nested := filepath.Join(dir, "frontend")
	if err := os.MkdirAll(nested, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "build.arq"), []byte("task build {}\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(nested, "package.json"), []byte(`{"scripts":{"build":"vite build"}}`), 0644); err != nil {
		t.Fatal(err)
	}
	root, err := Root(nested)
	if err != nil {
		t.Fatal(err)
	}
	if root != dir {
		t.Fatalf("root = %q, want %q", root, dir)
	}
	build, err := Discover(nested)
	if err != nil || build != filepath.Join(dir, "build.arq") {
		t.Fatalf("build = %q, err = %v", build, err)
	}
}

func TestDiscoverCommandsAddsNativeDefaults(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module example.com/app\n"), 0644); err != nil {
		t.Fatal(err)
	}
	commands, err := DiscoverCommands(dir)
	if err != nil {
		t.Fatal(err)
	}
	got := map[string]string{}
	for _, command := range commands {
		got[command.Name] = command.Run
	}
	for name, want := range map[string]string{
		"build": "go build ./...",
		"test":  "go test ./...",
		"fmt":   "go fmt ./...",
		"vet":   "go vet ./...",
	} {
		if got[name] != want {
			t.Fatalf("command %q = %q, want %q", name, got[name], want)
		}
	}
}

func TestRootPrefersModuleOverSourceDirectory(t *testing.T) {
	dir := t.TempDir()
	nested := filepath.Join(dir, "internal", "project")
	if err := os.MkdirAll(nested, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module example.com/app\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(nested, "project.go"), []byte("package project\n"), 0644); err != nil {
		t.Fatal(err)
	}
	root, err := Root(nested)
	if err != nil {
		t.Fatal(err)
	}
	if root != dir {
		t.Fatalf("root = %q, want %q", root, dir)
	}
}
