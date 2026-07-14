package diff

import (
	"strings"
	"testing"
)

const sample = `diff --git a/foo.go b/foo.go
index 0000000..1111111 100644
--- a/foo.go
+++ b/foo.go
@@ -10,2 +12,3 @@ func x()
+a
+b
+c
@@ -20 +25 @@
+d
diff --git a/gone.go b/gone.go
--- a/gone.go
+++ /dev/null
@@ -1,3 +0,0 @@
-x
diff --git "a/sp ace.go" "b/sp ace.go"
--- "a/sp ace.go"
+++ "b/sp ace.go"
@@ -0,0 +1 @@
+y
`

func TestParseUnified(t *testing.T) {
	set := parseUnified(strings.NewReader(sample))
	for _, ln := range []int{12, 13, 14, 25} {
		if !set["foo.go"][ln] {
			t.Errorf("foo.go line %d missing: %v", ln, set["foo.go"])
		}
	}
	if set["foo.go"][11] || set["foo.go"][15] {
		t.Error("line range leaked outside hunks")
	}
	if len(set["gone.go"]) != 0 {
		t.Error("deleted files must contribute no lines")
	}
	if !set["sp ace.go"][1] {
		t.Errorf("quoted path not unescaped: %v", set)
	}
}
