package runtime

import (
	"os"
	"path/filepath"
	"testing"
)

func TestTaskRunsAndInterpolates(t *testing.T) {
	d := t.TempDir()
	p := filepath.Join(d, "build.arq")
	src := "let who = \"arq\"\ntask hello {\n print \"hi ${who}\"\n}\n"
	if err := os.WriteFile(p, []byte(src), 0644); err != nil {
		t.Fatal(err)
	}
	r := New(d)
	if err := r.RunFile(p, "hello"); err != nil {
		t.Fatal(err)
	}
}

func TestCycleIsRejected(t *testing.T) {
	d := t.TempDir()
	p := filepath.Join(d, "build.arq")
	if err := os.WriteFile(p, []byte("task a { b }\ntask b { a }\n"), 0644); err != nil {
		t.Fatal(err)
	}
	r := New(d)
	if err := r.RunFile(p, "a"); err == nil {
		t.Fatal("expected cycle error")
	}
}

func TestLoadTasksIncludesImportedTasks(t *testing.T) {
	d := t.TempDir()
	mainPath := filepath.Join(d, "build.arq")
	libPath := filepath.Join(d, "tasks.arq")
	if err := os.WriteFile(mainPath, []byte("import \"tasks.arq\"\ntask root {}\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(libPath, []byte("task imported {}\n"), 0644); err != nil {
		t.Fatal(err)
	}
	r := New(d)
	if _, err := r.LoadTasks(mainPath); err != nil {
		t.Fatal(err)
	}
	if _, ok := r.Tasks["root"]; !ok {
		t.Fatal("root task was not discovered")
	}
	if _, ok := r.Tasks["imported"]; !ok {
		t.Fatal("imported task was not discovered")
	}
}

func TestColoredText(t *testing.T) {
	value, err := colored([]any{"green", "ok"})
	if err != nil || value != "\033[32mok\033[0m" {
		t.Fatalf("colored() = %q, %v", value, err)
	}
}
