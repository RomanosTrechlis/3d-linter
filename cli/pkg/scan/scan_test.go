package scan

import (
	"strings"
	"testing"
)

func keysOf(toks []Token) map[string]bool {
	m := map[string]bool{}
	for _, t := range toks {
		m[t.Key] = true
	}
	return m
}

func TestTokenize(t *testing.T) {
	keys := keysOf(tokenize("getUserName black-list HTTPServer x kebab-case-id", 1))
	for _, want := range []string{
		"get", "user", "name", "getusername", // camelCase + compound
		"black", "list", "blacklist", // kebab + compound
		"http", "server", "httpserver", // acronym boundary
		"x", "kebab", "case", "id", "kebabcaseid",
	} {
		if !keys[want] {
			t.Errorf("missing token key %q", want)
		}
	}
}

func TestTokenColumns(t *testing.T) {
	toks := tokenize("a sessionUser", 7)
	for _, tok := range toks {
		if tok.Key == "user" {
			if tok.Col != 10 || tok.Line != 7 || tok.Text != "User" {
				t.Errorf("got %+v, want Text=User Line=7 Col=10", tok)
			}
			return
		}
	}
	t.Fatal("no user token found")
}

func TestMaskStrings(t *testing.T) {
	for _, c := range []struct {
		in       string
		keepUser bool
	}{
		{`x := "the user id"`, false},
		{`y = 'user' + z`, false},
		{"tmpl := `user template`", false},
		{`esc := "a \" user"`, false},
		{`// don't flag the user's contraction handling`, true}, // apostrophes are not openers
	} {
		masked := maskStrings(c.in)
		if got := strings.Contains(masked, "user"); got != c.keepUser {
			t.Errorf("maskStrings(%q) = %q, keepUser=%v want %v", c.in, masked, got, c.keepUser)
		}
	}
}

func TestSuppressions(t *testing.T) {
	src := "a := b() // 3dl:allow user -- passport field\nc := d() /* 3dl:allow customer */\n"
	f := scanFile("x.go", []byte(src), Options{})
	if len(f.Allows) != 2 {
		t.Fatalf("got %d allows, want 2: %+v", len(f.Allows), f.Allows)
	}
	if a := f.Allows[0]; a.Word != "user" || a.Reason != "passport field" || a.Line != 1 {
		t.Errorf("allow[0] = %+v", a)
	}
	if a := f.Allows[1]; a.Word != "customer" || a.Reason != "" || a.Line != 2 {
		t.Errorf("allow[1] = %+v (reasonless directive must have empty Reason)", a)
	}
	// the directive text itself must not be tokenized
	if keysOf(f.Tokens)["user"] {
		t.Error("directive word leaked into tokens")
	}
}

func TestStringsOptIn(t *testing.T) {
	f := scanFile("x.go", []byte(`v := "user"`), Options{Strings: true})
	if !keysOf(f.Tokens)["user"] {
		t.Error("with Strings: true, literal contents should be scanned")
	}
}
