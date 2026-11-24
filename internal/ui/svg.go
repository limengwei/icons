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
