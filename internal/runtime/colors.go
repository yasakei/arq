package runtime

import (
	"fmt"
	"os"
	"strings"
)

var ansiStyles = map[string]string{
	"black": "30", "red": "31", "green": "32", "yellow": "33",
	"blue": "34", "magenta": "35", "cyan": "36", "white": "37",
	"gray": "90", "grey": "90", "bold": "1", "dim": "2", "underline": "4",
	"reset": "0",
}

func colored(args []any) (any, error) {
	if len(args) < 2 {
		return nil, fmt.Errorf("color needs a style and text")
	}
	text := fmt.Sprint(args[len(args)-1])
	if os.Getenv("NO_COLOR") != "" {
		return text, nil
	}
	var codes []string
	for _, raw := range args[:len(args)-1] {
		style := strings.ToLower(fmt.Sprint(raw))
		code, ok := ansiStyles[style]
		if !ok {
			return nil, fmt.Errorf("unknown color style %q", style)
		}
		codes = append(codes, code)
	}
	return "\033[" + strings.Join(codes, ";") + "m" + text + "\033[0m", nil
}
