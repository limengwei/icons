// Command extract-svg extracts every SVG icon from an unpacked react-icons
// npm package directory into data/package/{set}/{IconName}.svg.
//
// Usage: go run ./cmd/extract-svg [srcDir] [dstDir]
//
// Defaults: srcDir = data/react-icons/react-icons/package, dstDir = data/package.
// It parses each {set}/index.js (or index.mjs), pulls the pure-JSON argument
// of every GenIcon(...) call, converts it to a standard <svg> document and
// writes one file per icon.
package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

type node struct {
	Tag   string            `json:"tag"`
	Attr  map[string]string `json:"attr"`
	Child []*node           `json:"child"`
}

var nameRe = regexp.MustCompile(`(?:module\.exports\.|export\s+(?:const|function)\s+)([A-Za-z_$][\w$]*)`)

func main() {
	src := "data/react-icons/react-icons/package"
	dst := "data/package"
	if len(os.Args) > 1 {
		src = os.Args[1]
	}
	if len(os.Args) > 2 {
		dst = os.Args[2]
	}

	entries, err := os.ReadDir(src)
	if err != nil {
		fatal("read src dir: %v", err)
	}

	setIDRe := regexp.MustCompile(`^[a-z0-9]+$`)
	totalSets, totalIcons := 0, 0
	for _, e := range entries {
		if !e.IsDir() || !setIDRe.MatchString(e.Name()) || e.Name() == "lib" {
			continue
		}
		set := e.Name()
		body, ok := readSetFile(src, set)
		if !ok {
			continue
		}
		n, err := extractSet(body, filepath.Join(dst, set))
		if err != nil {
			fatal("set %s: %v", set, err)
		}
		totalSets++
		totalIcons += n
		fmt.Printf("%-5s %5d icons\n", set, n)
	}
	fmt.Printf("\ndone: %d sets, %d icons -> %s\n", totalSets, totalIcons, dst)
}

// readSetFile returns the body of the set's index.js, or index.mjs.
func readSetFile(src, set string) (string, bool) {
	for _, name := range []string{"index.js", "index.mjs"} {
		b, err := os.ReadFile(filepath.Join(src, set, name))
		if err == nil {
			return string(b), true
		}
	}
	return "", false
}

// extractSet finds every GenIcon({...}) in body and writes one SVG per icon.
func extractSet(body, outDir string) (int, error) {
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		return 0, err
	}
	count := 0
	for i := 0; ; {
		loc := nameRe.FindStringSubmatchIndex(body[i:])
		if loc == nil {
			break
		}
		nameStart := i + loc[2] // group 1: the icon name itself
		nameEnd := i + loc[3]
		name := body[nameStart:nameEnd]
		i = nameEnd

		jsonStr, ok := nextGenIconJSON(body[i:])
		if !ok {
			return count, fmt.Errorf("no GenIcon payload after %s", name)
		}
		i += len(jsonStr) + strings.Index(body[i:], jsonStr)

		svg, err := toSVG(jsonStr)
		if err != nil {
			return count, fmt.Errorf("icon %s: %v", name, err)
		}
		out := filepath.Join(outDir, name+".svg")
		if err := os.WriteFile(out, []byte(svg), 0o644); err != nil {
			return count, err
		}
		count++
	}
	return count, nil
}

// nextGenIconJSON returns the balanced-brace JSON string of the first
// GenIcon( call in s, skipping quoted strings while matching braces.
func nextGenIconJSON(s string) (string, bool) {
	k := strings.Index(s, "GenIcon(")
	if k < 0 {
		return "", false
	}
	j := k + len("GenIcon(")
	for j < len(s) && (s[j] == ' ' || s[j] == '\t' || s[j] == '\n' || s[j] == '\r') {
		j++
	}
	if j >= len(s) || s[j] != '{' {
		return "", false
	}
	depth, inStr, esc := 0, false, false
	for m := j; m < len(s); m++ {
		c := s[m]
		switch {
		case inStr && esc:
			esc = false
		case inStr && c == '\\':
			esc = true
		case inStr && c == '"':
			inStr = false
		case c == '"':
			inStr = true
		case !inStr && c == '{':
			depth++
		case !inStr && c == '}':
			depth--
			if depth == 0 {
				return s[j : m+1], true
			}
		}
	}
	return "", false
}

// toSVG converts a GenIcon JSON node into a standalone <svg> document.
func toSVG(jsonStr string) (string, error) {
	var root node
	if err := json.Unmarshal([]byte(jsonStr), &root); err != nil {
		return "", fmt.Errorf("bad JSON: %w", err)
	}
	var b strings.Builder
	writeNode(&b, &root, true)
	return b.String(), nil
}

func writeNode(b *strings.Builder, n *node, isRoot bool) {
	b.WriteByte('<')
	b.WriteString(n.Tag)
	if isRoot {
		b.WriteString(` xmlns="http://www.w3.org/2000/svg"`)
	}
	keys := make([]string, 0, len(n.Attr))
	for k := range n.Attr {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		fmt.Fprintf(b, ` %s="%s"`, k, xmlEscape(n.Attr[k]))
	}
	if len(n.Child) == 0 {
		b.WriteString("/>")
		return
	}
	b.WriteByte('>')
	for _, c := range n.Child {
		writeNode(b, c, false)
	}
	b.WriteString("</")
	b.WriteString(n.Tag)
	b.WriteByte('>')
}

func xmlEscape(s string) string {
	r := strings.NewReplacer("&", "&amp;", "<", "&lt;", ">", "&gt;", `"`, "&quot;")
	return r.Replace(s)
}

func fatal(format string, args ...any) {
	fmt.Fprintf(os.Stderr, "extract-svg: "+format+"\n", args...)
	os.Exit(1)
}
