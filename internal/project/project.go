package project

import (
	"fmt"
)

func Discover(start string) (string, error) {
	root, err := Root(start)
	if err != nil {
		return "", fmt.Errorf("could not find build.arq from %s", start)
	}
	if path := buildFile(root); path != "" {
		return path, nil
	}
	return "", fmt.Errorf("could not find build.arq from %s", start)
}
func Init(dir, template string) error {
	t, err := lookupTemplate(template)
	if err != nil {
		return err
	}
	return writeFiles(dir, map[string]string{"build.arq": t.build})
}

func New(dir, template string) error {
	t, err := lookupTemplate(template)
	if err != nil {
		return err
	}
	files := map[string]string{"build.arq": t.build}
	for name, content := range t.files {
		files[name] = content
	}
	return writeFiles(dir, files)
}

func Detect(dir string) []string {
	return detectTemplates(dir)
}
