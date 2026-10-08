package project

func template_rust() template {
	return template{
		name: "rust",
		build: commandTask("build", "cargo build --release") + commandTask("test", "cargo test") +
			commandTask("dev", "cargo run"),
		files: map[string]string{
			"Cargo.toml": "[package]\nname = \"arq-app\"\nversion = \"0.1.0\"\nedition = \"2021\"\n",
			"src/main.rs": `fn main() {
    println!("hello from arq");
}
`,
		},
	}
}
