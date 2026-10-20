package handbook_test

import (
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"testing"
	"unicode"
)

func repoRoot(t *testing.T) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("cannot locate documentation examples")
	}
	return filepath.Clean(filepath.Join(filepath.Dir(file), "../.."))
}
func read(t *testing.T, path string) string {
	t.Helper()
	b, e := os.ReadFile(path)
	if e != nil {
		t.Fatal(e)
	}
	return string(b)
}
func dedent(s string) string {
	s = strings.Trim(s, "\r\n")
	lines := strings.Split(s, "\n")
	prefix := -1
	for _, line := range lines {
		if strings.TrimSpace(line) == "" {
			continue
		}
		n := len(line) - len(strings.TrimLeft(line, "\t "))
		if prefix < 0 || n < prefix {
			prefix = n
		}
	}
	if prefix > 0 {
		for i, line := range lines {
			if len(line) >= prefix {
				lines[i] = line[prefix:]
			}
		}
	}
	return strings.TrimSpace(strings.Join(lines, "\n"))
}
func markdownFiles(t *testing.T, root string) []string {
	t.Helper()
	var files []string
	err := filepath.WalkDir(root, func(path string, d os.DirEntry, e error) error {
		if e != nil {
			return e
		}
		if d.IsDir() {
			if d.Name() == ".git" {
				return filepath.SkipDir
			}
			return nil
		}
		if strings.HasSuffix(path, ".md") {
			files = append(files, path)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return files
}
func checkSnippets(t *testing.T) {
	t.Helper()
	root := repoRoot(t)
	source := read(t, filepath.Join(root, "examples/handbook/handbook_test.go"))
	srcRE := regexp.MustCompile(`(?ms)^\s*// doc:begin ([\w-]+)\n(.*?)^\s*// doc:end [\w-]+`)
	blocks := map[string]string{}
	for _, m := range srcRE.FindAllStringSubmatch(source, -1) {
		blocks[m[1]] = dedent(m[2])
	}
	docRE := regexp.MustCompile("(?s)<!-- example:([\\w-]+) -->\\n```go\\n(.*?)\\n```\\n<!-- /example -->")
	count := 0
	for _, path := range markdownFiles(t, root) {
		for _, m := range docRE.FindAllStringSubmatch(read(t, path), -1) {
			want, ok := blocks[m[1]]
			if !ok {
				t.Errorf("%s: unknown snippet %s", path, m[1])
				continue
			}
			if got := dedent(m[2]); got != want {
				t.Errorf("%s: snippet %s differs from the compile-checked source", path, m[1])
			}
			count++
		}
	}
	if count == 0 {
		t.Fatal("no documentation snippets checked")
	}
	for _, path := range []string{"README.md", "auth/stateful/README.md", "auth/jwt/README.md", "auth/stateless/README.md", "auth/single/README.md", "auth/transport/README.md", "cache/README.md", "cache/memory/README.md", "cache/redis/README.md", "cache/tiered/README.md", "authorization/README.md", "httpx/README.md", "logger/README.md", "middleware/README.md", "pagination/README.md", "ratelimiter/README.md", "health/README.md", "httpclient/README.md", "observability/README.md", "lifecycle/README.md"} {
		if len(docRE.FindAllStringSubmatch(read(t, filepath.Join(root, path)), -1)) == 0 {
			t.Errorf("%s has no compile-checked example", path)
		}
	}
	t.Logf("checked %d snippet occurrences against %d compiled source blocks", count, len(blocks))
}
func slug(s string) string {
	s = strings.ToLower(strings.ReplaceAll(s, "`", ""))
	var out strings.Builder
	for _, r := range s {
		switch {
		case r == ' ':
			out.WriteByte('-')
		case r == '-' || r == '_' || unicode.IsLetter(r) || unicode.IsDigit(r):
			out.WriteRune(r)
		}
	}
	return out.String()
}
func anchors(body string) map[string]bool {
	result := map[string]bool{}
	ids := regexp.MustCompile(`<a\s+id="([^"]+)"`)
	for _, m := range ids.FindAllStringSubmatch(body, -1) {
		result[m[1]] = true
	}
	headings := regexp.MustCompile(`(?m)^#{1,6}\s+(.+?)\s*$`)
	seen := map[string]int{}
	for _, m := range headings.FindAllStringSubmatch(body, -1) {
		key := slug(m[1])
		n := seen[key]
		seen[key]++
		if n > 0 {
			key = key + "-" + strconv.Itoa(n)
		}
		result[key] = true
	}
	return result
}
func TestDocumentationLinks(t *testing.T) {
	root := repoRoot(t)
	fences := regexp.MustCompile("(?s)```.*?```")
	inline := regexp.MustCompile("`[^`]*`")
	links := regexp.MustCompile(`\[[^\]\n]*\]\(([^)]+)\)`)
	count := 0
	for _, file := range markdownFiles(t, root) {
		body := read(t, file)
		prose := inline.ReplaceAllString(fences.ReplaceAllString(body, ""), "")
		for _, m := range links.FindAllStringSubmatch(prose, -1) {
			target := m[1]
			if strings.Contains(target, "://") || strings.HasPrefix(target, "mailto:") {
				continue
			}
			parts := strings.SplitN(target, "#", 2)
			path := file
			if parts[0] != "" {
				path = filepath.Clean(filepath.Join(filepath.Dir(file), parts[0]))
			}
			if _, e := os.Stat(path); e != nil {
				t.Errorf("%s: broken local link %s", file, target)
				continue
			}
			if len(parts) == 2 && parts[1] != "" && strings.HasSuffix(path, ".md") && !anchors(read(t, path))[parts[1]] {
				t.Errorf("%s: missing anchor in %s", file, target)
			}
			count++
		}
	}
	t.Logf("checked %d local documentation links", count)
}
