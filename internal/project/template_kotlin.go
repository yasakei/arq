package project

func template_kotlin() template {
	return template{
		name: "kotlin",
		build: commandTask("build", "gradle build") + commandTask("test", "gradle test") +
			commandTask("dev", "gradle run"),
		files: map[string]string{
			"settings.gradle.kts": "rootProject.name = \"arq-app\"\n",
			"build.gradle.kts": `plugins {
    kotlin("jvm") version "2.1.10"
    application
}

repositories {
    mavenCentral()
}

kotlin {
    jvmToolchain(17)
}

application {
    mainClass.set("MainKt")
}
`,
			"src/main/kotlin/Main.kt": "fun main() {\n    println(\"hello from arq\")\n}\n",
		},
	}
}
