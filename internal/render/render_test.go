package render

import (
	"fmt"
	"image"
	"sync"
	"testing"
)

// rectSVG covers its whole viewBox, so every canvas pixel inside the
// letterboxed icon area must be painted and the margins must be transparent.
const rectSVG = `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 448 512"><path d="M0 0h448v512H0z"/></svg>`

// plusSVG is a glyph-shaped icon (a plus sign) in the typical FontAwesome
// 448×512 viewBox; its center is solid while its corners are empty.
const plusSVG = `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 448 512"><path d="M192 0h64v192h192v64H256v192h-64V256H0v-64h192z"/></svg>`

func assertColor(t *testing.T, img *image.RGBA, x, y int, r, g, b uint8) {
	t.Helper()
	px := img.RGBAAt(x, y)
	if px.A == 0 {
		t.Fatalf("pixel (%d,%d) is transparent, want color #%02X%02X%02X", x, y, r, g, b)
	}
	dr, dg, db := int(px.R)-int(r), int(px.G)-int(g), int(px.B)-int(b)
	if dr < -5 || dr > 5 || dg < -5 || dg > 5 || db < -5 || db > 5 {
		t.Fatalf("pixel (%d,%d) = #%02X%02X%02X, want #%02X%02X%02X ±5", x, y, px.R, px.G, px.B, r, g, b)
	}
}

func assertTransparent(t *testing.T, img *image.RGBA, x, y int) {
	t.Helper()
	if px := img.RGBAAt(x, y); px.A != 0 {
		t.Fatalf("pixel (%d,%d) = #%02X%02X%02X%02X, want fully transparent", x, y, px.R, px.G, px.B, px.A)
	}
}

func TestRasterizeSizeAndCentering(t *testing.T) {
	img, err := Rasterize(rectSVG, 64, "#FF5722")
	if err != nil {
		t.Fatalf("Rasterize: %v", err)
	}
	if got := img.Bounds(); got.Dx() != 64 || got.Dy() != 64 {
		t.Fatalf("image bounds = %v, want 64×64", got)
	}
	// 448×512 fitted into 64: scale 0.125, icon spans x∈[4,60], y∈[0,64].
	assertColor(t, img, 32, 32, 0xFF, 0x57, 0x22)
	assertColor(t, img, 10, 10, 0xFF, 0x57, 0x22)
	assertTransparent(t, img, 2, 32)  // left margin
	assertTransparent(t, img, 61, 32) // right margin

	// Every painted pixel must carry the requested color.
	for y := 0; y < 64; y++ {
		for x := 0; x < 64; x++ {
			if px := img.RGBAAt(x, y); px.A != 0 {
				dr := int(px.R) - 0xFF
				dg := int(px.G) - 0x57
				db := int(px.B) - 0x22
				if dr < -5 || dr > 5 || dg < -5 || dg > 5 || db < -5 || db > 5 {
					t.Fatalf("painted pixel (%d,%d) = #%02X%02X%02X, want #FF5722 ±5", x, y, px.R, px.G, px.B)
				}
			}
		}
	}
}

func TestRasterizeGlyphCenter(t *testing.T) {
	img, err := Rasterize(plusSVG, 64, "#FF5722")
	if err != nil {
		t.Fatalf("Rasterize: %v", err)
	}
	if got := img.Bounds(); got.Dx() != 64 || got.Dy() != 64 {
		t.Fatalf("image bounds = %v, want 64×64", got)
	}
	// Canvas center maps to viewBox center (224,256), inside the vertical bar.
	assertColor(t, img, 32, 32, 0xFF, 0x57, 0x22)
	assertColor(t, img, 32, 16, 0xFF, 0x57, 0x22)
	assertColor(t, img, 32, 48, 0xFF, 0x57, 0x22)
	// Corners of the viewBox are outside the glyph and outside the fit area.
	assertTransparent(t, img, 2, 2)
	assertTransparent(t, img, 61, 61)
}

func TestRasterizeDefaultColorIsBlack(t *testing.T) {
	img, err := Rasterize(rectSVG, 32, "")
	if err != nil {
		t.Fatalf("Rasterize: %v", err)
	}
	assertColor(t, img, 16, 16, 0x00, 0x00, 0x00)
}

func TestRasterizeCurrentColor(t *testing.T) {
	// currentColor on a child element must resolve to the requested color;
	// oksvg itself cannot parse currentColor and would error out.
	svg := `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24"><g fill="currentColor"><path d="M0 0h24v24H0z"/></g></svg>`
	img, err := Rasterize(svg, 32, "#00AA33")
	if err != nil {
		t.Fatalf("Rasterize with currentColor: %v", err)
	}
	assertColor(t, img, 16, 16, 0x00, 0xAA, 0x33)

	// Without a requested color currentColor falls back to black.
	img, err = Rasterize(`<svg xmlns="http://www.w3.org/2000/svg" fill="currentColor" viewBox="0 0 24 24"><path d="M0 0h24v24H0z"/></svg>`, 32, "")
	if err != nil {
		t.Fatalf("Rasterize with currentColor fallback: %v", err)
	}
	assertColor(t, img, 16, 16, 0x00, 0x00, 0x00)
}

func TestRasterizeMissingViewBox(t *testing.T) {
	svg := `<svg xmlns="http://www.w3.org/2000/svg"><path d="M0 0h24v24H0z"/></svg>`
	img, err := Rasterize(svg, 32, "#123456")
	if err != nil {
		t.Fatalf("Rasterize: %v", err)
	}
	if got := img.Bounds(); got.Dx() != 32 || got.Dy() != 32 {
		t.Fatalf("image bounds = %v, want 32×32", got)
	}
	// Default 24×24 viewBox: full-bleed path covers the whole canvas.
	assertColor(t, img, 16, 16, 0x12, 0x34, 0x56)
}

func TestRasterizeErrors(t *testing.T) {
	if _, err := Rasterize(rectSVG, 0, "#000000"); err == nil {
		t.Fatal("size 0: want error, got nil")
	}
	if _, err := Rasterize(rectSVG, -8, ""); err == nil {
		t.Fatal("negative size: want error, got nil")
	}
	if _, err := Rasterize(`<svg><path d="M`, 32, ""); err == nil {
		t.Fatal("malformed svg: want error, got nil")
	}
}

func TestCacheLRU(t *testing.T) {
	img1 := image.NewRGBA(image.Rect(0, 0, 1, 1))
	img2 := image.NewRGBA(image.Rect(0, 0, 2, 2))
	img3 := image.NewRGBA(image.Rect(0, 0, 3, 3))

	c := NewCache(2)
	c.Put("a", img1)
	c.Put("b", img2)
	if got, ok := c.Get("a"); !ok || got != img1 {
		t.Fatalf("Get(a) after two puts = %v,%v; want %v,true", got, ok, img1)
	}
	// "a" was just used, so "b" is the least recently used and gets evicted.
	c.Put("c", img3)
	if _, ok := c.Get("b"); ok {
		t.Fatal("Get(b) after eviction: want miss, got hit")
	}
	if got, ok := c.Get("a"); !ok || got != img1 {
		t.Fatalf("Get(a) after eviction = %v,%v; want %v,true", got, ok, img1)
	}
	if got, ok := c.Get("c"); !ok || got != img3 {
		t.Fatalf("Get(c) = %v,%v; want %v,true", got, ok, img3)
	}

	// Putting an existing key updates its value without evicting others.
	c.Put("a", img3)
	if got, ok := c.Get("a"); !ok || got != img3 {
		t.Fatalf("Get(a) after update = %v,%v; want %v,true", got, ok, img3)
	}
	if _, ok := c.Get("c"); !ok {
		t.Fatal("Get(c) after updating a: want hit, got miss")
	}

	// Keys never stored are misses.
	if _, ok := c.Get("missing"); ok {
		t.Fatal("Get(missing): want miss, got hit")
	}
}

func TestCacheConcurrent(t *testing.T) {
	c := NewCache(8)
	base := image.NewRGBA(image.Rect(0, 0, 4, 4))
	var wg sync.WaitGroup
	for g := 0; g < 8; g++ {
		wg.Add(1)
		go func(g int) {
			defer wg.Done()
			for i := 0; i < 300; i++ {
				key := fmt.Sprintf("k%d", (g+i)%8)
				c.Get(key)
				c.Put(key, base)
			}
		}(g)
	}
	wg.Wait()
	// 8 distinct keys, capacity 8: all must still be cached and non-nil.
	for i := 0; i < 8; i++ {
		if got, ok := c.Get(fmt.Sprintf("k%d", i)); !ok || got == nil {
			t.Fatalf("Get(k%d) after concurrent writes = %v,%v; want non-nil,true", i, got, ok)
		}
	}
}
