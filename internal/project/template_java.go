package project

func template_java() template {
	return template{
		name: "java",
		build: commandTask("build", "mvn package") + commandTask("test", "mvn test") +
			task("dev", "build", `run "java -cp target/classes Main"`),
		files: map[string]string{
			"pom.xml": `<project xmlns="http://maven.apache.org/POM/4.0.0" xmlns:xsi="http://www.w3.org/2001/XMLSchema-instance" xsi:schemaLocation="http://maven.apache.org/POM/4.0.0 https://maven.apache.org/xsd/maven-4.0.0.xsd">
  <modelVersion>4.0.0</modelVersion>
  <groupId>example</groupId>
  <artifactId>arq-app</artifactId>
  <version>0.1.0</version>
  <properties>
    <maven.compiler.release>17</maven.compiler.release>
    <project.build.sourceEncoding>UTF-8</project.build.sourceEncoding>
  </properties>
  <build>
    <plugins>
      <plugin>
        <groupId>org.apache.maven.plugins</groupId>
        <artifactId>maven-compiler-plugin</artifactId>
        <version>3.13.0</version>
      </plugin>
    </plugins>
  </build>
</project>
`,
			"src/main/java/Main.java": "public class Main {\n    public static void main(String[] args) {\n        System.out.println(\"hello from arq\");\n    }\n}\n",
		},
	}
}
