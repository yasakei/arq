package project

import (
	"fmt"
	"strings"
)

type template struct {
	name  string
	build string
	files map[string]string
}

var templates = catalog()

func Templates() []string {
	return SupportedTemplates()
}

func SupportedTemplates() []string {
	names := make([]string, len(templates))
	for i, t := range templates {
		names[i] = t.name
	}
	return names
}

func Template(name string) string {
	t, err := lookupTemplate(name)
	if err != nil {
		return ""
	}
	return t.build
}

func lookupTemplate(name string) (template, error) {
	for _, t := range templates {
		if t.name == strings.ToLower(strings.TrimSpace(name)) {
			return t, nil
		}
	}
	return template{}, fmt.Errorf("unknown template %q; choose one of: %s", name, strings.Join(Templates(), ", "))
}

func task(name string, statements ...string) string {
	return "task " + name + " {\n    " + strings.Join(statements, "\n    ") + "\n}\n\n"
}

func commandTask(name, command string) string {
	return task(name, fmt.Sprintf("run %q", command))
}

func catalog() []template {
	return []template{
		template_empty(),
		template_go(),
		template_rust(),
		template_node(),
		template_typescript(),
		frontendReact(),
		frontendPreact(),
		vueTemplate(),
		svelteTemplate(),
		template_python(),
		template_django(),
		template_flask(),
		template_java(),
		template_kotlin(),
		template_c(),
		template_cpp(),
		template_cmake(),
		template_csharp(),
		template_zig(),
		template_lua(),
		template_swift(),
		mixedTemplate(),
	}
}
