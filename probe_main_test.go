package main

// Temporary investigation probe: renders icons through the same path the UI
// grid uses, in a real platform window under `go tool nsgo test -target
// windows`, and counts icons whose pixels never differ from the background.
// Delete once the missing-icons issue is resolved.

import (
	"strings"
	"testing"

	"github.com/nsgo-dev/nsgo/core"
	"github.com/nsgo-dev/nsgo/nsgotest"

	"reacticons/internal/dataset"
)

func TestMain(m *testing.M) { nsgotest.Main(m) }

const probeGridSize = 52

var (
	probeIconColor = core.RGB(0xe5e5e5)
	probeBlank     = core.MustParseSVG([]byte(`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24"></svg>`))
)

// withCurrentColor mirrors internal/ui.withCurrentColor.
func withCurrentColor(svg string) string {
	i := strings.Index(svg, "<svg")
	if i < 0 {
		return svg
	}
	rest := svg[i+4:]
	if len(rest) > 0 && rest[0] != ' ' && rest[0] != '\t' && rest[0] != '\n' && rest[0] != '\r' && rest[0] != '>' {
		return svg
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

func TestProbeIconPixels(t *testing.T) {
	ds, err := dataset.Open()
	if err != nil {
		t.Fatal(err)
	}

	var sig *core.Signal[*core.SVG]
	app := nsgotest.Start(t, func(cx *core.Scope) core.Node {
		sig = core.NewSignal(cx, probeBlank)
		return core.View{
			TestID: "host",
			Style:  core.Style{Width: core.Pt(64), Height: core.Pt(64), Background: core.RGB(0x1c1c1e), AlignItems: core.AlignCenter, JustifyContent: core.JustifyCenter},
			Children: []core.Node{core.Icon{
				TestID:  "probe",
				SVGFunc: func() *core.SVG { return sig.Get() },
				Style:   core.Style{Width: core.Pt(probeGridSize), Height: core.Pt(probeGridSize), Color: probeIconColor},
			}},
		}
	}, nsgotest.Size(64, 64))

	paintedPixels := func() int {
		app.Settle()
		img := app.Capture()
		n := 0
		for y := img.Bounds().Min.Y; y < img.Bounds().Max.Y; y++ {
			for x := img.Bounds().Min.X; x < img.Bounds().Max.X; x++ {
				r, g, b, _ := img.At(x, y).RGBA()
				if r>>8 > 0x60 || g>>8 > 0x60 || b>>8 > 0x60 {
					n++
				}
			}
		}
		return n
	}

	for _, set := range []string{"di", "wi", "sl", "gr", "fi", "fa6"} {
		icons := ds.Search("", set)
		var invisible []string
		for _, ic := range icons {
			svg, err := ds.GetSVG(set, ic.Name)
			if err != nil {
				continue
			}
			doc, err := core.ParseSVG([]byte(withCurrentColor(svg)))
			if err != nil {
				continue
			}
			app.Post(func() { sig.Set(doc) })
			if paintedPixels() == 0 {
				invisible = append(invisible, ic.Name)
			}
		}
		t.Logf("[%s] total=%d invisible=%d", set, len(icons), len(invisible))
		if len(invisible) > 0 {
			sample := invisible
			if len(sample) > 8 {
				sample = sample[:8]
			}
			t.Logf("  invisible sample: %v", sample)
		}
	}
}
