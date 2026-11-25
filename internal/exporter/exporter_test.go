package exporter

import (
	"bytes"
	"image"
	"image/color"
	"image/png"
	"strings"
	"testing"

	"reacticons/internal/importer"
)

const plainSVG = `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 448 512"><path d="M0 0h1v1z"/></svg>`

const rectSVG = `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 448 512"><path d="M0 0h448v512H0z"/></svg>`

func TestApplyColor(t *testing.T) {
	t.Run("injects fill into root tag", func(t *testing.T) {
		want := `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 448 512" fill="#FF5722"><path d="M0 0h1v1z"/></svg>`
		if got := ApplyColor(plainSVG, "#FF5722"); got != want {
			t.Fatalf("ApplyColor inject =\n%s\nwant\n%s", got, want)
		}
	})

	t.Run("replaces existing fill, keeps child fills", func(t *testing.T) {
		in := `<svg xmlns="http://www.w3.org/2000/svg" fill="currentColor" viewBox="0 0 24 24"><path fill="#111" d="M0 0h1v1z"/></svg>`
		want := `<svg xmlns="http://www.w3.org/2000/svg" fill="#00AA33" viewBox="0 0 24 24"><path fill="#111" d="M0 0h1v1z"/></svg>`
		if got := ApplyColor(in, "#00AA33"); got != want {
			t.Fatalf("ApplyColor replace =\n%s\nwant\n%s", got, want)
		}
	})

	t.Run("replaces single-quoted fill", func(t *testing.T) {
		in := `<svg xmlns="http://www.w3.org/2000/svg" fill='#123456' viewBox="0 0 24 24"/>`
		want := `<svg xmlns="http://www.w3.org/2000/svg" fill="#ABCDEF" viewBox="0 0 24 24"/>`
		if got := ApplyColor(in, "#ABCDEF"); got != want {
			t.Fatalf("ApplyColor single-quote =\n%s\nwant\n%s", got, want)
		}
	})

	t.Run("empty color returns input unchanged", func(t *testing.T) {
		if got := ApplyColor(plainSVG, ""); got != plainSVG {
			t.Fatalf("ApplyColor with empty color =\n%s\nwant input unchanged", got)
		}
	})
}

func TestSVGBytes(t *testing.T) {
	if got := string(SVGBytes(plainSVG, "#FFFFFF")); got != ApplyColor(plainSVG, "#FFFFFF") {
		t.Fatalf("SVGBytes mismatch with ApplyColor:\n%s", got)
	}
	if got := string(SVGBytes(plainSVG, "")); got != plainSVG {
		t.Fatalf("SVGBytes empty color =\n%s\nwant original", got)
	}
}

func TestPNG(t *testing.T) {
	data, err := PNG(rectSVG, 48, "#336699")
	if err != nil {
		t.Fatalf("PNG: %v", err)
	}
	img, err := png.Decode(bytes.NewReader(data))
	if err != nil {
		t.Fatalf("decode PNG: %v", err)
	}
	if b := img.Bounds(); b.Dx() != 48 || b.Dy() != 48 {
		t.Fatalf("decoded bounds = %v, want 48×48", b)
	}
	// 448×512 fitted into 48: icon spans x∈[3,45]; center and margin checks.
	assertNRGBA(t, img, 24, 24, 0x33, 0x66, 0x99)
	assertNRGBA(t, img, 12, 36, 0x33, 0x66, 0x99)
	if _, _, _, a := img.At(1, 24).RGBA(); a != 0 {
		t.Fatalf("margin pixel (1,24) alpha = %d, want 0", a)
	}

	// Empty color: default black fill.
	data, err = PNG(rectSVG, 32, "")
	if err != nil {
		t.Fatalf("PNG default color: %v", err)
	}
	img, err = png.Decode(bytes.NewReader(data))
	if err != nil {
		t.Fatalf("decode PNG: %v", err)
	}
	assertNRGBA(t, img, 16, 16, 0x00, 0x00, 0x00)

	// Malformed SVG surfaces the rasterizer error.
	if _, err := PNG(`<svg><path d="M`, 32, ""); err == nil {
		t.Fatal("PNG malformed svg: want error, got nil")
	}
}

func assertNRGBA(t *testing.T, img image.Image, x, y int, r, g, b uint8) {
	t.Helper()
	c := color.NRGBAModel.Convert(img.At(x, y)).(color.NRGBA)
	if c.A == 0 {
		t.Fatalf("pixel (%d,%d) is transparent, want #%02X%02X%02X", x, y, r, g, b)
	}
	dr, dg, db := int(c.R)-int(r), int(c.G)-int(g), int(c.B)-int(b)
	if dr < -5 || dr > 5 || dg < -5 || dg > 5 || db < -5 || db > 5 {
		t.Fatalf("pixel (%d,%d) = #%02X%02X%02X, want #%02X%02X%02X ±5", x, y, c.R, c.G, c.B, r, g, b)
	}
}

func TestJSX(t *testing.T) {
	if got := JSX("FaBeer", importer.SourceReactIcons); got != "<FaBeer />" {
		t.Fatalf("JSX = %q, want %q", got, "<FaBeer />")
	}
	if got := JSX("home-minimal", importer.SourceIconify); got != "<Icon icon={homeMinimal} />" {
		t.Fatalf("JSX(iconify) = %q, want %q", got, "<Icon icon={homeMinimal} />")
	}
}

func TestImport(t *testing.T) {
	if got := Import("FaBeer", "fa6", importer.SourceReactIcons); got != `import { FaBeer } from "react-icons/fa6"` {
		t.Fatalf("Import = %q", got)
	}
	want := "import homeMinimal from \"@iconify-icons/solar/home-minimal\";\nimport { Icon } from \"@iconify/react\""
	if got := Import("home-minimal", "solar", importer.SourceIconify); got != want {
		t.Fatalf("Import(iconify) = %q, want %q", got, want)
	}
}

func TestComponent(t *testing.T) {
	t.Run("extracts viewBox and children", func(t *testing.T) {
		in := `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 448 512"><path d="M0 0h448v512H0z"/><path d="M64 64h64v64H64z"/></svg>`
		want := `export default function FaBeer(props) {
  const { size = "1em", color = "currentColor", ...rest } = props;
  return (
    <svg viewBox="0 0 448 512" fill={color} width={size} height={size} xmlns="http://www.w3.org/2000/svg" {...rest}>
      <path d="M0 0h448v512H0z" />
      <path d="M64 64h64v64H64z" />
    </svg>
  );
}
`
		if got := Component("FaBeer", in); got != want {
			t.Fatalf("Component =\n%q\nwant\n%q", got, want)
		}
	})

	t.Run("keeps grouped children on one line", func(t *testing.T) {
		in := `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24"><g fill="none"><path d="M0 0h1v1z"/></g></svg>`
		got := Component("MdGroup", in)
		wantLine := "      <g fill=\"none\"><path d=\"M0 0h1v1z\"/></g>"
		if !strings.Contains(got, wantLine) {
			t.Fatalf("Component missing line %q in:\n%s", wantLine, got)
		}
	})

	t.Run("defaults viewBox when missing", func(t *testing.T) {
		got := Component("IoPlain", `<svg xmlns="http://www.w3.org/2000/svg"><path d="M0 0h1v1z"/></svg>`)
		if !strings.Contains(got, `viewBox="0 0 24 24"`) {
			t.Fatalf("Component without viewBox =\n%s", got)
		}
	})
}
