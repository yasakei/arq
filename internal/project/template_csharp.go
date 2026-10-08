package project

func template_csharp() template {
	return template{
		name:  "csharp",
		build: commandTask("build", "dotnet build") + commandTask("dev", "dotnet run"),
		files: map[string]string{
			"App.csproj": "<Project Sdk=\"Microsoft.NET.Sdk\">\n  <PropertyGroup>\n    <OutputType>Exe</OutputType>\n    <TargetFramework>net8.0</TargetFramework>\n    <ImplicitUsings>enable</ImplicitUsings>\n    <Nullable>enable</Nullable>\n  </PropertyGroup>\n</Project>\n",
			"Program.cs": "Console.WriteLine(\"hello from arq\");\n",
		},
	}
}
