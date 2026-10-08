package project

func template_c() template {
	return template{
		name:  "c",
		build: task("build", `mkdir "build"`, `run "cc -std=c11 -Wall -Wextra -o build/app src/main.c"`),
		files: map[string]string{"src/main.c": "#include <stdio.h>\n\nint main(void) {\n    puts(\"hello from arq\");\n    return 0;\n}\n"},
	}
}
