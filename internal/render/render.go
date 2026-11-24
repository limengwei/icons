// Package render rasterizes SVG icons to pixel images and caches recently
// rendered icons in a thread-safe LRU cache.
package render

import (
	"container/list"
	"fmt"
	"image"
	"regexp"
	"strings"
	"sync"

	"github.com/srwiley/oksvg"
	"github.com/srwiley/rasterx"
)

// defaultViewBoxSize is used when the SVG carries no usable viewBox.
const defaultViewBoxSize = 24

var (
	svgOpenTagRe = regexp.MustCompile(`(?i)<svg\b[^>]*>`)
	fillAttrRe   = regexp.MustCompile(`(?i)\sfill\s*=\s*("[^"]*"|'[^']*')`)
)

// Rasterize renders svg onto a size×size transparent canvas.
//
// The icon is scaled uniformly (min of size/viewBoxW and size/viewBoxH) and
// centered on the canvas, never stretched. When color is non-empty it is
// injected into (or replaces the fill of) the root <svg> tag first. Any
// remaining "currentColor" references are resolved to color, or to black
// when color is empty, because oksvg cannot parse currentColor.
func Rasterize(svg string, size int, color string) (*image.RGBA, error) {
	if size <= 0 {
		return nil, fmt.Errorf("render: size must be positive, got %d", size)
	}
	src := svg
	if color != "" {
		src = applyColor(src, color)
	}
	resolved := color
	if resolved == "" {
		resolved = "#000000"
	}
	src = strings.ReplaceAll(src, "currentColor", resolved)

	icon, err := oksvg.ReadIconStream(strings.NewReader(src))
	if err != nil {
		return nil, fmt.Errorf("render: parse svg: %w", err)
	}

	vbW, vbH := icon.ViewBox.W, icon.ViewBox.H
	if vbW <= 0 || vbH <= 0 {
		vbW, vbH = defaultViewBoxSize, defaultViewBoxSize
		// SetTarget divides by ViewBox internally, so the fallback must be
		// written back or the scale factor becomes infinite.
		icon.ViewBox.W, icon.ViewBox.H = vbW, vbH
	}

	// Uniform scale, centered target rectangle: SetTarget(0,0,size,size)
	// would stretch non-square icons, so compute the fit ourselves.
	scale := float64(size) / vbW
	if s := float64(size) / vbH; s < scale {
		scale = s
	}
	w, h := vbW*scale, vbH*scale
	x := (float64(size) - w) / 2
	y := (float64(size) - h) / 2
	icon.SetTarget(x, y, w, h)

	img := image.NewRGBA(image.Rect(0, 0, size, size))
	scanner := rasterx.NewScannerGV(size, size, img, img.Bounds())
	icon.Draw(rasterx.NewDasher(size, size, scanner), 1.0)
	return img, nil
}

// applyColor injects fill into the root <svg> tag, replacing an existing
// fill value there; other attributes and child fills are left untouched.
// This is a private twin of exporter.ApplyColor so that render and exporter
// never import each other.
func applyColor(svg, color string) string {
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

// Cache is a thread-safe LRU cache of rasterized icon images.
type Cache struct {
	mu       sync.Mutex
	capacity int
	ll       *list.List // front = most recently used
	items    map[string]*list.Element
}

type cacheEntry struct {
	key string
	img *image.RGBA
}

// NewCache returns an LRU cache holding at most capacity images.
func NewCache(capacity int) *Cache {
	return &Cache{
		capacity: capacity,
		ll:       list.New(),
		items:    make(map[string]*list.Element),
	}
}

// Get returns the cached image for key and reports whether it was present.
// A present key is promoted to most recently used.
func (c *Cache) Get(key string) (*image.RGBA, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	el, ok := c.items[key]
	if !ok {
		return nil, false
	}
	c.ll.MoveToFront(el)
	return el.Value.(*cacheEntry).img, true
}

// Put stores img under key (updating it if already cached) and evicts the
// least recently used entries once the capacity is exceeded.
func (c *Cache) Put(key string, img *image.RGBA) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if el, ok := c.items[key]; ok {
		c.ll.MoveToFront(el)
		el.Value.(*cacheEntry).img = img
		return
	}
	c.items[key] = c.ll.PushFront(&cacheEntry{key: key, img: img})
	for c.ll.Len() > c.capacity {
		oldest := c.ll.Back()
		if oldest == nil {
			return
		}
		c.ll.Remove(oldest)
		delete(c.items, oldest.Value.(*cacheEntry).key)
	}
}
