package project

func template_empty() template {
	return template{
		name:  "empty",
		build: task("build", `print "Hello from Arq"`),
	}
}
