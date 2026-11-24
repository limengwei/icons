// Package exporter converts icons to SVG/PNG bytes and React code snippets
// in the format used by the react-icons website.
package exporter

import (
	"bytes"
	"fmt"
	"image/png"
	"regexp"
	"strings"

	"reacticons/internal/render"
)

var (
	svgOpenTagRe  = regexp.MustCompile(`(?i)<svg\b[^>]*>`)
	fillAttrRe    = regexp.MustCompile(`(?i)\sfill\s*=\s*("[^"]*"|'[^']*')`)
	viewBoxAttrRe = regexp.MustCompile(`(?i)viewBox\s*=\s*"([^"]*)"`)
)

// ApplyColor returns svg with fill=color injected into the root <svg> tag,
// replacing an existing fill value there. Other attributes and child fills
// are left untouched. An empty color returns svg unchanged.
func ApplyColor(svg, color string) string {
	if color == "" {
		return svg
	}
	return svgOpenTagRe.ReplaceAllStringFunc(svg, func(tag string) string {
		if loc := fillAttrRe.FindStringIndex(tag); loc != nil {
			return tag[:loc[0]] + ` fill="` + color + `"` + tag[loc[1]:]
		}
		return tag[:len(tag)-1] + ` fill="` + color + `">`
	})
}

// SVGBytes returns the SVG source, with color applied, as raw bytes.
func SVGBytes(svg, color string) []byte {
	return []byte(ApplyColor(svg, color))
}

// PNG rasterizes the SVG at the given pixel size and color and returns the
// encoded PNG bytes.
func PNG(svg string, size int, color string) ([]byte, error) {
	img, err := render.Rasterize(svg, size, color)
	if err != nil {
		return nil, err
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		return nil, fmt.Errorf("exporter: encode png: %w", err)
	}
	return buf.Bytes(), nil
}

// JSX returns the JSX usage snippet for an icon, e.g. "<FaBeer />".
func JSX(name string) string {
	return "<" + name + " />"
}

// Import returns the react-icons import snippet, e.g.
// `import { FaBeer } from "react-icons/fa6"`.
func Import(name, set string) string {
	return `import { ` + name + ` } from "react-icons/` + set + `"`
}

// Component generates a standalone React component equivalent to the icon:
// the viewBox and all root-tag children are extracted from svg, everything
// else (size, color, extra props) stays configurable via props.
func Component(name, svg string) string {
	viewBox := "0 0 24 24"
	if m := viewBoxAttrRe.FindStringSubmatch(svg); m != nil {
		viewBox = m[1]
	}
	var b strings.Builder
	b.WriteString("export default function " + name + "(props) {\n")
	b.WriteString(`  const { size = "1em", color = "currentColor", ...rest } = props;` + "\n")
	b.WriteString("  return (\n")
	b.WriteString(`    <svg viewBox="` + viewBox + `" fill={color} width={size} height={size} xmlns="http://www.w3.org/2000/svg" {...rest}>` + "\n")
	if children := formatChildren(innerSVG(svg)); children != "" {
		b.WriteString(children)
		b.WriteString("\n")
	}
	b.WriteString("    </svg>\n")
	b.WriteString("  );\n")
	b.WriteString("}\n")
	return b.String()
}

// innerSVG returns the content between the root <svg ...> open tag and the
// final </svg> close tag.
func innerSVG(svg string) string {
	loc := svgOpenTagRe.FindStringIndex(svg)
	if loc == nil {
		return ""
	}
	inner := svg[loc[1]:]
	if i := strings.LastIndex(inner, "</svg>"); i >= 0 {
		inner = inner[:i]
	}
	return inner
}

// formatChildren renders the root tag's children as JSX lines: each
// top-level element on its own line, indented six spaces, with self-closing
// tags normalized to the prettier style " />".
func formatChildren(inner string) string {
	var lines []string
	i, n := 0, len(inner)
	for i < n {
		for i < n && isSpace(inner[i]) {
			i++
		}
		if i >= n {
			break
		}
		start := i
		end, ok := elementEnd(inner, i)
		if !ok {
			end = n
		}
		i = end
		el := strings.TrimSpace(inner[start:i])
		if el == "" {
			continue
		}
		if strings.HasSuffix(el, "/>") && !strings.HasSuffix(el, " />") {
			el = el[:len(el)-2] + " />"
		}
		lines = append(lines, "      "+el)
	}
	return strings.Join(lines, "\n")
}

// elementEnd returns the index just past the element starting at s[i]
// (which must be '<'), matching nested same-name tags. ok is false when the
// element is malformed or unterminated.
func elementEnd(s string, i int) (end int, ok bool) {
	if i >= len(s) || s[i] != '<' {
		return 0, false
	}
	nameEnd := i + 1
	for nameEnd < len(s) && !isSpace(s[nameEnd]) && s[nameEnd] != '>' && s[nameEnd] != '/' {
		nameEnd++
	}
	if nameEnd >= len(s) {
		return 0, false
	}
	name := s[i+1 : nameEnd]
	tagEnd := nameEnd
	for tagEnd < len(s) && s[tagEnd] != '>' {
		tagEnd++
	}
	if tagEnd >= len(s) {
		return 0, false
	}
	if s[tagEnd-1] == '/' { // self-closing
		return tagEnd + 1, true
	}
	pos, depth := tagEnd+1, 1
	for pos < len(s) && depth > 0 {
		lt := strings.IndexByte(s[pos:], '<')
		if lt < 0 {
			return 0, false
		}
		p := pos + lt
		if strings.HasPrefix(s[p:], "</") {
			gt := strings.IndexByte(s[p:], '>')
			if gt < 0 {
				return 0, false
			}
			if strings.TrimSpace(s[p+2:p+gt]) == name {
				depth--
			}
			pos = p + gt + 1
			continue
		}
		innerNameEnd := p + 1
		for innerNameEnd < len(s) && !isSpace(s[innerNameEnd]) && s[innerNameEnd] != '>' && s[innerNameEnd] != '/' {
			innerNameEnd++
		}
		innerTagEnd := innerNameEnd
		for innerTagEnd < len(s) && s[innerTagEnd] != '>' {
			innerTagEnd++
		}
		if innerTagEnd >= len(s) {
			return 0, false
		}
		if s[p+1:innerNameEnd] == name && s[innerTagEnd-1] != '/' {
			depth++
		}
		pos = innerTagEnd + 1
	}
	if depth != 0 {
		return 0, false
	}
	return pos, true
}

func isSpace(b byte) bool {
	return b == ' ' || b == '\t' || b == '\n' || b == '\r' || b == '\f'
}
