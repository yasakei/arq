package project

func template_swift() template {
	return template{
		name:  "swift",
		build: commandTask("build", "swift build") + commandTask("dev", "swift run"),
		files: map[string]string{
			"Package.swift":             "// swift-tools-version: 5.9\nimport PackageDescription\n\nlet package = Package(\n    name: \"ArqApp\",\n    targets: [.executableTarget(name: \"ArqApp\")]\n)\n",
			"Sources/ArqApp/main.swift": "print(\"hello from arq\")\n",
		},
	}
}
