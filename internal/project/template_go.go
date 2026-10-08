package project

func template_go() template {
	return template{
		name: "go",
		build: task("build", `mkdir "build"`, `run "go build -o build/app ."`) +
			commandTask("test", "go test ./...") + commandTask("dev", "go run ."),
		files: map[string]string{
			"go.mod": "module example.com/arq-app\n\ngo 1.22\n",
			"main.go": `package main

import "fmt"

func greeting() string { return "hello from arq" }

func main() { fmt.Println(greeting()) }
`,
			"main_test.go": `package main

import "testing"

func TestGreeting(t *testing.T) {
	if got := greeting(); got != "hello from arq" {
		t.Fatalf("unexpected greeting: %q", got)
	}
}
`,
		},
	}
}
