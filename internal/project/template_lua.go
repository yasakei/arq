package project

func template_lua() template {
	return template{
		name:  "lua",
		build: commandTask("build", "luac -p main.lua") + commandTask("dev", "lua main.lua"),
		files: map[string]string{"main.lua": "print(\"hello from arq\")\n"},
	}
}
