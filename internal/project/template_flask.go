package project

func template_flask() template {
	return template{
		name: "flask",
		build: commandTask("build", "python -m compileall -q app.py") + commandTask("test", "python -m unittest discover") +
			commandTask("dev", "python -m flask --app app run --host 127.0.0.1"),
		files: map[string]string{
			"requirements.txt": "Flask>=3.1,<4\n",
			"app.py":           "from flask import Flask\n\napp = Flask(__name__)\n\n\n@app.get(\"/\")\ndef home():\n    return \"hello from arq\"\n",
			"test_app.py":      "import unittest\n\nfrom app import app\n\n\nclass HomeTest(unittest.TestCase):\n    def test_home(self):\n        response = app.test_client().get(\"/\")\n        self.assertEqual(response.status_code, 200)\n        self.assertEqual(response.text, \"hello from arq\")\n",
		},
	}
}
