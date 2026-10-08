package project

func template_django() template {
	return template{
		name: "django",
		build: commandTask("build", "python manage.py check") + commandTask("test", "python manage.py test") +
			commandTask("dev", "python manage.py runserver 127.0.0.1:8000"),
		files: map[string]string{
			"requirements.txt": "Django>=5.2,<6\n",
			"manage.py":        "import os\nimport sys\n\nfrom django.core.management import execute_from_command_line\n\nos.environ.setdefault(\"DJANGO_SETTINGS_MODULE\", \"app.settings\")\n\nif __name__ == \"__main__\":\n    execute_from_command_line(sys.argv)\n",
			"app/__init__.py":  "",
			"app/settings.py":  "SECRET_KEY = \"development-only-replace-before-deployment\"\nDEBUG = True\nALLOWED_HOSTS = [\"localhost\", \"127.0.0.1\"]\nROOT_URLCONF = \"app.urls\"\nINSTALLED_APPS = []\nMIDDLEWARE = []\n",
			"app/urls.py":      "from django.http import HttpResponse\nfrom django.urls import path\n\n\ndef home(request):\n    return HttpResponse(\"hello from arq\")\n\n\nurlpatterns = [path(\"\", home)]\n",
			"app/tests.py":     "from django.test import SimpleTestCase\n\n\nclass HomeTest(SimpleTestCase):\n    def test_home(self):\n        response = self.client.get(\"/\")\n        self.assertEqual(response.status_code, 200)\n        self.assertContains(response, \"hello from arq\")\n",
		},
	}
}
