package project

func template_python() template {
	return template{
		name: "python",
		build: commandTask("build", "python -m compileall -q app.py") + commandTask("test", "python -m unittest discover") +
			commandTask("dev", "python app.py"),
		files: map[string]string{
			"pyproject.toml": "[project]\nname = \"arq-app\"\nversion = \"0.1.0\"\nrequires-python = \">=3.10\"\n",
			"app.py":         "def greeting():\n    return \"hello from arq\"\n\n\nif __name__ == \"__main__\":\n    print(greeting())\n",
			"test_app.py":    "import unittest\n\nfrom app import greeting\n\n\nclass GreetingTest(unittest.TestCase):\n    def test_greeting(self):\n        self.assertEqual(greeting(), \"hello from arq\")\n",
		},
	}
}
