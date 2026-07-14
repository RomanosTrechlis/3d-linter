// Package diff maps a git base ref to the set of added/modified lines in the
// working tree (F6 — findings ∩ changed lines).
package diff

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
)

// LineSet is file (slash-relative to repo root) → changed line numbers.
type LineSet map[string]map[int]bool

// Changed returns the lines added or modified between base and the working
// tree. It diffs from merge-base(base, HEAD) when one exists — PR semantics —
// and falls back to base itself (still correct on GitHub's premerged checkout).
func Changed(root, base string) (LineSet, error) {
	target := base
	if out, err := git(root, "merge-base", base, "HEAD"); err == nil {
		target = strings.TrimSpace(out)
	}
	out, err := git(root, "-c", "core.quotepath=false", "diff", "-U0", "--no-color", target)
	if err != nil {
		return nil, fmt.Errorf("git diff against %q failed: %v", base, err)
	}
	set := parseUnified(strings.NewReader(out))
	// Untracked files never show up in `git diff`, but locally they are the
	// most "introduced by this change" a line can get — count every line.
	if out, err := git(root, "ls-files", "--others", "--exclude-standard"); err == nil {
		for f := range strings.SplitSeq(strings.TrimSpace(out), "\n") {
			if f == "" {
				continue
			}
			data, err := os.ReadFile(filepath.Join(root, f))
			if err != nil {
				continue
			}
			lines := map[int]bool{}
			for i := 0; i <= bytes.Count(data, []byte("\n")); i++ {
				lines[i+1] = true
			}
			set[f] = lines
		}
	}
	return set, nil
}

func git(root string, args ...string) (string, error) {
	cmd := exec.Command("git", append([]string{"-C", root}, args...)...)
	var errb strings.Builder
	cmd.Stderr = &errb
	out, err := cmd.Output()
	if err != nil {
		msg := strings.TrimSpace(errb.String())
		if msg == "" {
			msg = err.Error()
		}
		return "", fmt.Errorf("%s", msg)
	}
	return string(out), nil
}

var hunkRe = regexp.MustCompile(`^@@ -\d+(?:,\d+)? \+(\d+)(?:,(\d+))? @@`)

func parseUnified(r io.Reader) LineSet {
	set := LineSet{}
	cur := ""
	sc := bufio.NewScanner(r)
	sc.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	for sc.Scan() {
		line := sc.Text()
		if strings.HasPrefix(line, "+++ ") {
			cur = newFilePath(line[4:])
			continue
		}
		m := hunkRe.FindStringSubmatch(line)
		if m == nil || cur == "" {
			continue
		}
		start, _ := strconv.Atoi(m[1])
		count := 1
		if m[2] != "" {
			count, _ = strconv.Atoi(m[2])
		}
		if count == 0 {
			continue // pure deletion
		}
		if set[cur] == nil {
			set[cur] = map[int]bool{}
		}
		for i := 0; i < count; i++ {
			set[cur][start+i] = true
		}
	}
	return set
}

func newFilePath(p string) string {
	if strings.HasPrefix(p, `"`) {
		if uq, err := strconv.Unquote(p); err == nil {
			p = uq
		}
	}
	if p == "/dev/null" {
		return ""
	}
	return strings.TrimPrefix(p, "b/")
}
