package project

func template_cpp() template {
	return template{
		name:  "cpp",
		build: task("build", `mkdir "build"`, `run "c++ -std=c++17 -Wall -Wextra -o build/app src/main.cpp"`),
		files: map[string]string{"src/main.cpp": "#include <iostream>\n\nint main() {\n    std::cout << \"hello from arq\\n\";\n}\n"},
	}
}
