package ui

import (
	"container/list"
	"fmt"
	"strings"
	"sync"

	"github.com/nsgo-dev/nsgo/core"

	"reacticons/internal/dataset"
)

// blankSVG is returned when an icon cannot be served, so a cell always has a
// valid document to draw.
var blankSVG = core.MustParseSVG([]byte(`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24"></svg>`))

// robotSVG is the toolbar MCP button's icon (explicit fill, not tinted).
var robotSVG = core.MustParseSVG([]byte(`<svg xmlns="http://www.w3.org/2000/svg" fill="#E5E5E5" viewBox="0 0 24 24"><path d="M13.5 2C13.5 2.44425 13.3069 2.84339 13 3.11805V5H18C19.6569 5 21 6.34315 21 8V18C21 19.6569 19.6569 21 18 21H6C4.34315 21 3 19.6569 3 18V8C3 6.34315 4.34315 5 6 5H11V3.11805C10.6931 2.84339 10.5 2.44425 10.5 2C10.5 1.17157 11.1716 0.5 12 0.5C12.8284 0.5 13.5 1.17157 13.5 2ZM0 10H2V16H0V10ZM24 10H22V16H24V10ZM9 14.5C9.82843 14.5 10.5 13.8284 10.5 13C10.5 12.1716 9.82843 11.5 9 11.5C8.17157 11.5 7.5 12.1716 7.5 13C7.5 13.8284 8.17157 14.5 9 14.5ZM16.5 13C16.5 12.1716 15.8284 11.5 15 11.5C14.1716 11.5 13.5 12.1716 13.5 13C13.5 13.8284 14.1716 14.5 15 14.5C15.8284 14.5 16.5 13.8284 16.5 13Z"/></svg>`))

// previewChecker is the transparency checkerboard behind the detail-panel
// preview: alternating dark/light gray squares, 16pt each, 224pt overall.
var previewChecker = func() *core.SVG {
	const n, cell = 14, 16
	var b strings.Builder
	b.WriteString(`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 224 224">`)
	b.WriteString(`<rect width="224" height="224" fill="#242429"/>`)
	for y := 0; y < n; y++ {
		for x := 0; x < n; x++ {
			if (x+y)%2 == 0 {
				fmt.Fprintf(&b, `<rect x="%d" y="%d" width="%d" height="%d" fill="#313136"/>`, x*cell, y*cell, cell, cell)
			}
		}
	}
	b.WriteString(`</svg>`)
	return core.MustParseSVG([]byte(b.String()))
}()

// withCurrentColor injects fill="currentColor" on the root <svg> tag when it
// carries no fill of its own, so core.Icon tints the drawing with the text
// color it inherits — the same react-icons' GenIcon does at runtime.
func withCurrentColor(svg string) string {
	i := strings.Index(svg, "<svg")
	if i < 0 {
		return svg
	}
	rest := svg[i+4:]
	if len(rest) > 0 && rest[0] != ' ' && rest[0] != '\t' && rest[0] != '\n' && rest[0] != '\r' && rest[0] != '>' {
		return svg // <svgSomethingElse>, not our root tag
	}
	end := strings.Index(rest, ">")
	if end < 0 {
		return svg
	}
	if strings.Contains(rest[:end], "fill=") {
		return svg
	}
	return svg[:i+4] + ` fill="currentColor"` + rest
}

// entry pairs a document with its key so LRU eviction can unmap it.
type entry struct {
	key string
	svg *core.SVG
}

// svgCache memoizes parsed *core.SVG documents keyed "set|name". NSGo draws
// them as vectors at whatever size the layout gives, so icons stay sharp at
// any zoom or DPI without rasterizing bitmaps per size.
type svgCache struct {
	mu    sync.Mutex
	cap   int
	ll    *list.List // front = most recently used
	elems map[string]*list.Element
}

func newSVGCache(capacity int) *svgCache {
	return &svgCache{cap: capacity, ll: list.New(), elems: make(map[string]*list.Element, capacity)}
}

func (c *svgCache) get(key string) (*core.SVG, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if el, ok := c.elems[key]; ok {
		c.ll.MoveToFront(el)
		return el.Value.(*entry).svg, true
	}
	return nil, false
}

func (c *svgCache) set(key string, svg *core.SVG) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if el, ok := c.elems[key]; ok {
		c.ll.MoveToFront(el)
		el.Value.(*entry).svg = svg
		return
	}
	c.elems[key] = c.ll.PushFront(&entry{key: key, svg: svg})
	for c.ll.Len() > c.cap {
		oldest := c.ll.Back()
		if oldest == nil {
			return
		}
		c.ll.Remove(oldest)
		delete(c.elems, oldest.Value.(*entry).key)
	}
}

// iconSVG resolves an icon to a cached, parsed vector document. It parses on
// a miss (fast, sub-millisecond) and falls back to a blank document on error.
func iconSVG(ds *dataset.Dataset, c *svgCache, set, name string) *core.SVG {
	key := set + "|" + name
	if svg, ok := c.get(key); ok {
		return svg
	}
	svg, err := ds.GetSVG(set, name)
	if err != nil {
		return blankSVG
	}
	doc, err := core.ParseSVG([]byte(withCurrentColor(svg)))
	if err != nil {
		return blankSVG
	}
	c.set(key, doc)
	return doc
}
