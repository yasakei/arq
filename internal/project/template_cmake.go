package project

func template_cmake() template {
	return template{
		name: "cmake",
		build: task("build", "configure", `run "cmake --build build"`) +
			commandTask("configure", "cmake -S . -B build") + task("test", "build", `run "ctest --test-dir build --output-on-failure"`),
		files: map[string]string{
			"CMakeLists.txt": "cmake_minimum_required(VERSION 3.16)\nproject(arq_app LANGUAGES CXX)\nadd_executable(app src/main.cpp)\ntarget_compile_features(app PRIVATE cxx_std_17)\nenable_testing()\nadd_test(NAME greeting COMMAND app)\nset_tests_properties(greeting PROPERTIES PASS_REGULAR_EXPRESSION \"hello from arq\")\n",
			"src/main.cpp":   "#include <iostream>\n\nint main() {\n    std::cout << \"hello from arq\\n\";\n}\n",
		},
	}
}
