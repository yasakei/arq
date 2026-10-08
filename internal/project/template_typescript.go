package project

func template_typescript() template {
	return template{
		name: "typescript",
		build: commandTask("build", "npm run build") + task("test", "build", `run "npm test"`) +
			commandTask("dev", "npm run dev"),
		files: map[string]string{
			"package.json":     packageJSON(map[string]string{"build": "tsc", "test": "node --test", "dev": "tsc --watch"}, nil, map[string]string{"typescript": "^5.7.3"}),
			"tsconfig.json":    "{\n  \"compilerOptions\": {\n    \"target\": \"ES2022\",\n    \"module\": \"NodeNext\",\n    \"moduleResolution\": \"NodeNext\",\n    \"strict\": true,\n    \"rootDir\": \"src\",\n    \"outDir\": \"dist\"\n  },\n  \"include\": [\"src\"]\n}\n",
			"src/index.ts":     "export const greeting: string = 'hello from arq';\nconsole.log(greeting);\n",
			"test/app.test.js": "import assert from 'node:assert/strict';\nimport test from 'node:test';\nimport { greeting } from '../dist/index.js';\n\ntest('greeting', () => assert.equal(greeting, 'hello from arq'));\n",
		},
	}
}
