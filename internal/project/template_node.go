package project

func template_node() template {
	return template{
		name: "node",
		build: commandTask("build", "npm run build") + commandTask("test", "npm test") +
			commandTask("dev", "npm run dev"),
		files: map[string]string{
			"package.json":     packageJSON(map[string]string{"build": "node --check src/index.js", "test": "node --test", "dev": "node src/index.js"}, nil, nil),
			"src/index.js":     "export const greeting = 'hello from arq';\nconsole.log(greeting);\n",
			"test/app.test.js": "import assert from 'node:assert/strict';\nimport test from 'node:test';\nimport { greeting } from '../src/index.js';\n\ntest('greeting', () => assert.equal(greeting, 'hello from arq'));\n",
		},
	}
}
