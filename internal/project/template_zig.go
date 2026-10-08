package project

func template_zig() template {
	return template{
		name: "zig",
		build: task("build", `mkdir "build"`, `run "zig build-exe src/main.zig -femit-bin=build/app"`) +
			commandTask("test", "zig test src/main.zig") + commandTask("dev", "zig run src/main.zig"),
		files: map[string]string{"src/main.zig": "const std = @import(\"std\");\n\npub fn main() void {\n    std.debug.print(\"hello from arq\\n\", .{});\n}\n\ntest \"greeting\" {\n    try std.testing.expectEqualStrings(\"hello from arq\", greeting());\n}\n\nfn greeting() []const u8 {\n    return \"hello from arq\";\n}\n"},
	}
}
