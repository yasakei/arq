package lexer

import "testing"

func TestMultilineString(t *testing.T) {
	tokens, err := Lex("print(\"\"\"hello\nworld\"\"\")")
	if err != nil {
		t.Fatal(err)
	}
	if got := tokens[2].Text; got != "hello\nworld" {
		t.Fatalf("string = %q", got)
	}
}
