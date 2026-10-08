# arq

[![CI](https://github.com/yasakei/arq/actions/workflows/ci.yml/badge.svg)](https://github.com/yasakei/arq/actions/workflows/ci.yml)
[![Go](https://img.shields.io/badge/go-1.22-blue.svg)](https://go.dev/doc/install)
[![Release](https://img.shields.io/github/v/release/yasakei/arq?display_name=tag)](https://github.com/yasakei/arq/releases)
[![Platforms](https://img.shields.io/badge/platforms-linux%20%7C%20macos%20%7C%20windows-lightgrey.svg)](https://github.com/yasakei/arq/actions/workflows/ci.yml)

Arq is a small, cross-platform project automation runtime. It provides a single `build.arq` file with generic primitives for defining and running project tasks, without assuming a specific language or toolchain.

## Contents

- [Features](#features)
- [Requirements](#requirements)
- [Installation](#installation)
- [Quick start](#quick-start)
- [CLI reference](#cli-reference)
- [Language](#language)
- [Standard library](#standard-library)
- [Codebase discovery](#codebase-discovery)
- [Templates](#templates)
- [Development](#development)
- [Project layout](#project-layout)

## Features

- Single-file task definitions in `build.arq`
- Language-agnostic process execution (`run`, `exec`, `spawn`, `parallel`)
- Task dependencies with cycle detection
- Variables, functions, conditionals, and loops
- Filesystem and environment primitives
- Project scaffolding with 22 templates
- Zero-config codebase discovery: runs native commands from `package.json`, Makefiles, Justfiles, Taskfiles, Cargo aliases, Composer scripts, Rakefiles, and standard Go, Rust, Python, JVM, CMake, .NET, Swift, and Zig layouts before a `build.arq` file exists
- File watching, REPL, formatter, and terminal color support
- Works on Linux, macOS, and Windows

## Requirements

- Go 1.22 or later (to build from source)
- The toolchains for the tasks you run (Go, Rust, Node.js, Python, and so on)

Arq does not install language toolchains or package dependencies for you.

## Installation

Install the latest release with the installer script:

```sh
curl -fsSL https://yasakei.dev/arq.install | bash
```

Or with Go:

```sh
go install github.com/yasakei/arq/cmd/arq@latest
```

Or build from source:

```sh
git clone https://github.com/yasakei/arq.git
cd arq
go build -o build/arq ./cmd/arq
./build/arq version
```

The installer downloads the matching `linux`, `darwin`, or `windows` binary (`amd64` or `arm64`) from the latest GitHub release into `~/.local/bin`. Override the version with `ARQ_VERSION=v0.1.0` or the install directory with `--dir`:

```sh
curl -fsSL https://yasakei.dev/arq.install | bash -s -- --help
ARQ_VERSION=v0.1.0 curl -fsSL https://yasakei.dev/arq.install | bash
```

## Quick start

```sh
cd my-project
arq init
arq list
arq build
```

Scaffold a new project from a template without the interactive prompt:

```sh
arq init --template go my-project
arq new react my-ui
arq init --non-interactive
```

`arq init` detects an existing codebase and selects the most specific matching template when no template is supplied. It falls back to `empty` when no project type is recognized.

Scaffolding never installs packages, downloads files, or overwrites existing files. Generated `build.arq` files are executable automation code: review them before running them, as they can run local processes and modify files.

## CLI reference

| Command | Description |
| --- | --- |
| `arq [task]` | Run a task from `build.arq`, or a discovered project command if no matching task exists. With no arguments, executes the top-level statements in the build file. |
| `arq run <task>` | Explicit form of task invocation. |
| `arq file.arq [task]` | Run a specific `.arq` file, optionally selecting a task within it. |
| `arq list` | List tasks from `build.arq` (including imports) combined with discovered project commands. |
| `arq detect` | Print the detected project root, project types, and discovered commands with their sources. |
| `arq check` | Validate that `build.arq` parses and its tasks load. Without a build file, validates codebase detection. |
| `arq init [--template <name>] [--non-interactive] [dir]` | Create a `build.arq` file in `dir` (default: current directory). Opens an interactive template picker on a TTY unless `--template` or `--non-interactive` is given. |
| `arq new <template> [dir]` | Create a `build.arq` file plus starter project files in `dir`. |
| `arq templates` | List available scaffold templates. |
| `arq fmt [path] [--check]` | Normalize trailing whitespace and ensure the file ends with a newline. With `--check`, exits non-zero if the file is not formatted. Defaults to the discovered `build.arq`. |
| `arq repl` | Start an interactive session. Type `exit` or `quit` to leave. |
| `arq watch [task]` | Re-run the build file (or the given task) whenever the build file changes. |
| `arq version` | Print the arq version. |

Arq locates the project by walking up from the current directory to find `build.arq`, `Arqfile`, or `arqfile`, falling back to repository and language markers. Nested package commands are exposed with a directory prefix, for example `frontend:build`.

## Language

Example `build.arq`:

```arq
let version = "1.0.0"

task build {
    mkdir "build"
    run "go build -o build/app ./cmd/app"
}

task release {
    build
    print "releasing ${version}"
}
```

### Tasks and dependencies

```arq
task build {
    run "go build ./..."
}

task check after vet, test {
    print "ok"
}

# Equivalent explicit dependency syntax:
# task check depends on vet, test { ... }

task release {
    build      # calling another task by name runs it with cycle protection
    test
}
```

Tasks run once per invocation even when referenced multiple times. Task references inside a task body are treated as dependencies and scheduled accordingly.

### Variables and interpolation

```arq
let version = "1.0.0"
var count = 0
count = count + 1
print "releasing ${version}"
```

`let` and `var` both declare variables; assignment with `=` updates an existing binding. Double-quoted strings interpolate `${name}` placeholders. There are also numbers, booleans (`true`, `false`), `null`, arrays (`[1, 2, 3]`), and maps (`{name: "app", port: 8080}`).

### Functions, conditionals, and loops

```arq
fn greet(name) {
    return "hello " + name
}

if env("CI") == "true" {
    print "running in CI"
} else {
    print "running locally"
}

for file in glob("src/*.go") {
    print file
}
```

Supported statements: `if` / `else`, `for x in iterable`, `fn name(params)`, `return`, `break`, `continue`, and `watch`. Comments start with `#`. Expressions support `+ - * / %`, comparisons (`== != < > <= >=`), logical operators (`&& || !`), unary minus, indexing (`items[0]`, `config["key"]`), member access (`config.key`), and parentheses.

### Imports

```arq
import "tasks.arq"
import "shared/common.arq"
```

Imports load additional `.arq` files relative to the importing file. The `.arq` extension may be omitted. Import cycles are rejected. `arq list` includes tasks from all transitively imported files.

### File watching

```arq
watch "src/**/*.go" {
    print "source changed"
    exec("go", ["test", "./..."])
}
```

A `watch` block registers a glob pattern and re-evaluates its body when matches change. The CLI-level `arq watch [task]` instead watches the build file itself for edits.

## Standard library

### Output and text

| Function | Description |
| --- | --- |
| `print(...)` | Write arguments to standard output. |
| `color(style, text)` / `colour(style, text)` | Wrap text in ANSI codes. Styles: `black`, `red`, `green`, `yellow`, `blue`, `magenta`, `cyan`, `white`, `gray`/`grey`, `bold`, `dim`, `underline`, `reset`. Set `NO_COLOR=1` to disable. |
| `error(...)` | Abort execution with an error message. |

### Process execution

| Function | Description |
| --- | --- |
| `run "command"` | Run a shell command (`sh -c` on Unix, `cmd /C` on Windows) in the project root. |
| `exec(program, [args])` or `exec("full command")` | Run a program directly, or a shell command when given a single string. |
| `exec_in(dir, program, [args])` | Like `exec`, but in a subdirectory of the project root. |
| `output(program, [args])` | Run a command and return its trimmed standard output as a string. |
| `spawn(program, [args], [timeoutSecs])` | Start a background process and return a handle. |
| `spawn_in(dir, program, [args], [timeoutSecs])` | Like `spawn`, but in a subdirectory. |
| `wait(handle, [timeoutSecs])` | Wait for a spawned process; returns its exit code. |
| `kill(handle)` | Terminate a spawned process. |
| `timeout(command, seconds)` | Run a command with a deadline. |
| `sleep(seconds)` | Pause execution. |
| `parallel("taskA", "taskB")` / `wait_all(...)` | Run tasks concurrently. Accepts multiple names or an array of names. |
| `rust_target()` | Return the host target triple reported by `rustc -vV`. |

### Filesystem

| Function | Description |
| --- | --- |
| `read(path)` | Read a file relative to the project root. |
| `write(path, value)` | Write a file relative to the project root. |
| `copy(src, dst)` | Copy a file. |
| `move(src, dst)` | Rename or move a file. |
| `mkdir(path)` | Create a directory and parents. |
| `exists(path)` | Return whether a path exists. |
| `glob(pattern)` | Return matching paths relative to the project root. |
| `chmod(path, mode)` | Change file permissions (numeric mode). |
| `kill_matching(pattern)` | Terminate processes matching a pattern. |

### Environment and data

| Function | Description |
| --- | --- |
| `env(name)` | Return the value of an environment variable. |
| `cwd()` | Return the project root. |
| `keys(object)` | Return the sorted keys of a map. |
| `len(value)` | Return the length of an array, map, or string. |

## Codebase discovery

Arq works before a `build.arq` file exists. From any subdirectory, it finds the project root and infers commands from:

- `package.json` scripts (with npm, pnpm, Yarn, or Bun selected by lockfile)
- Makefiles, Justfiles, and Taskfiles
- `pyproject.toml` entry points, Cargo aliases, Composer scripts, and Rakefiles
- Executable project scripts and scripts in `bin/` or `scripts/`
- Standard commands for Go, Rust, Python, Django, Maven/Gradle, CMake, .NET, Swift, and Zig projects

Use `arq detect` to inspect the detected root, project types, and command sources. `arq list` merges these with Arq tasks, and `arq <name>` or `arq run <name>` runs either kind.

## Templates

`arq templates` lists all starters. `arq new <template> [directory]` generates starter files plus a `build.arq`; `arq init --template <name>` generates only the build file.

Available templates: `empty`, `go`, `rust`, `node`, `typescript`, `react`, `preact`, `vue`, `svelte`, `python`, `django`, `flask`, `java`, `kotlin`, `c`, `cpp`, `cmake`, `csharp`, `zig`, `lua`, `swift`, and `mixed` (Go backend plus Node frontend).

For example, the `go` template generates a `build` task (`go build`), a `test` task (`go test ./...`), a `dev` task (`go run .`), and a minimal module with a passing test. Frontend templates generate a Vite project with `build`, `dev`, and `preview` tasks.

## Development

This repository builds and tests itself with its own `build.arq`:

```sh
go test ./...
go vet ./...
go run ./cmd/arq check
go run ./cmd/arq build
go run ./cmd/arq test
```

The checked-in `build.arq` defines `build`, `test`, `vet`, `fmt`, `check`, `dev`, `install`, `release`, and `clean` tasks. Run `go run ./cmd/arq list` to see them.

## Project layout

```text
cmd/arq/          CLI entry point
internal/lexer/   Tokenizer
internal/parser/  Parser for tasks, expressions, and control flow
internal/ast/     Syntax tree definitions
internal/runtime/ Task scheduler, expression evaluator, processes, watch, REPL
internal/project/ Templates, scaffolding, and codebase discovery
internal/tui/     Interactive template picker
build.arq         Self-hosted build definition for this repository
```
