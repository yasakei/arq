package project

import (
	"encoding/json"
	"fmt"
)

func packageJSON(scripts, dependencies, devDependencies map[string]string) string {
	pkg := map[string]any{"name": "arq-app", "version": "0.1.0", "private": true, "type": "module", "scripts": scripts}
	if len(dependencies) > 0 {
		pkg["dependencies"] = dependencies
	}
	if len(devDependencies) > 0 {
		pkg["devDependencies"] = devDependencies
	}
	b, _ := json.MarshalIndent(pkg, "", "  ")
	return string(b) + "\n"
}

func frontend(name, entry, source string, dependencies map[string]string, plugin, version, binding string) template {
	return template{
		name:  name,
		build: commandTask("build", "npm run build") + commandTask("dev", "npm run dev") + commandTask("preview", "npm run preview"),
		files: map[string]string{
			"package.json":   packageJSON(map[string]string{"build": "vite build", "dev": "vite --host 127.0.0.1", "preview": "vite preview --host 127.0.0.1"}, dependencies, map[string]string{"vite": "^6.1.0", plugin: version}),
			"index.html":     "<!doctype html>\n<html lang=\"en\"><head><meta charset=\"UTF-8\"><meta name=\"viewport\" content=\"width=device-width, initial-scale=1.0\"><title>arq app</title></head><body><div id=\"app\"></div><script type=\"module\" src=\"/" + entry + "\"></script></body></html>\n",
			"vite.config.js": fmt.Sprintf("import { defineConfig } from 'vite';\nimport %s from '%s';\n\nexport default defineConfig({ plugins: [%s()] });\n", binding, plugin, binding),
			entry:            source,
		},
	}
}

func frontendReact() template {
	return frontend("react", "src/main.jsx", "import React from 'react';\nimport { createRoot } from 'react-dom/client';\n\ncreateRoot(document.getElementById('app')).render(<h1>hello from arq</h1>);\n", map[string]string{"react": "^19.0.0", "react-dom": "^19.0.0"}, "@vitejs/plugin-react", "^4.3.4", "react")
}
func frontendPreact() template {
	return frontend("preact", "src/main.jsx", "import { render } from 'preact';\n\nrender(<h1>hello from arq</h1>, document.getElementById('app'));\n", map[string]string{"preact": "^10.26.0"}, "@preact/preset-vite", "^2.10.1", "preact")
}

func vueTemplate() template {
	t := frontend("vue", "src/main.js", "import { createApp } from 'vue';\nimport App from './App.vue';\n\ncreateApp(App).mount('#app');\n", map[string]string{"vue": "^3.5.13"}, "@vitejs/plugin-vue", "^5.2.1", "vue")
	t.files["src/App.vue"] = "<template>\n  <h1>hello from arq</h1>\n</template>\n"
	return t
}

func svelteTemplate() template {
	t := frontend("svelte", "src/main.js", "import { mount } from 'svelte';\nimport App from './App.svelte';\n\nmount(App, { target: document.getElementById('app') });\n", map[string]string{"svelte": "^5.20.0"}, "@sveltejs/vite-plugin-svelte", "^5.0.3", "svelte")
	t.files["vite.config.js"] = "import { defineConfig } from 'vite';\nimport { svelte } from '@sveltejs/vite-plugin-svelte';\n\nexport default defineConfig({ plugins: [svelte()] });\n"
	t.files["src/App.svelte"] = "<h1>hello from arq</h1>\n"
	return t
}

func mixedTemplate() template {
	return template{name: "mixed", build: task("build", "backend", "frontend") + task("backend", `mkdir "build"`, `run "go build -C backend -o ../build/backend ."`) + commandTask("frontend", "npm --prefix frontend run build") + task("test", `run "go test -C backend ./..."`, `run "npm --prefix frontend test"`), files: map[string]string{
		"backend/go.mod":            "module example.com/arq-app/backend\n\ngo 1.22\n",
		"backend/main.go":           "package main\n\nimport \"fmt\"\n\nfunc main() { fmt.Println(\"hello from the backend\") }\n",
		"frontend/package.json":     packageJSON(map[string]string{"build": "node --check src/index.js", "test": "node --test"}, nil, nil),
		"frontend/src/index.js":     "export const greeting = 'hello from the frontend';\nconsole.log(greeting);\n",
		"frontend/test/app.test.js": "import assert from 'node:assert/strict';\nimport test from 'node:test';\nimport { greeting } from '../src/index.js';\n\ntest('greeting', () => assert.equal(greeting, 'hello from the frontend'));\n",
	}}
}
