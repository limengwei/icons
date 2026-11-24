package ui

import (
	"fmt"
	"sync"

	"github.com/nsgo-dev/nsgo/base"
	"github.com/nsgo-dev/nsgo/core"
	"github.com/nsgo-dev/nsgo/ui"

	"reacticons/internal/dataset"
	"reacticons/internal/render"
)

// cellSize is one grid cell's icon box.
const cellSize = 76

// gridSize / gridColor are the rasterization parameters for grid thumbnails.
const (
	gridSize  = 48
	gridColor = "#E5E5E5"
)

// gridState feeds the virtualized grid from the current result set.
type gridState struct {
	results *core.Signal[[]dataset.Icon]
	refresh *core.Signal[int] // bumped when a rasterization lands
	onOpen  func(dataset.Icon)
	ds      *dataset.Dataset
	cache   *render.Cache
}

// in-flight rasterization dedup: keys currently being rendered by a
// background goroutine, so one SourceFunc call per miss is enough no matter
// how often the grid re-renders while the icon is loading.
var (
	inFlightMu sync.Mutex
	inFlight   = map[string]bool{}
)

func cacheKey(ic dataset.Icon) string {
	return fmt.Sprintf("%s|%s|%d|%s", ic.Set, ic.Name, gridSize, gridColor)
}

// startRasterize renders ic in the background and bumps refresh when the
// image lands in the cache. Returning without a bump on error is deliberate:
// only a re-render of the cell (scroll / search) retries it.
func startRasterize(cx *core.Scope, s gridState, key string, ic dataset.Icon) {
	inFlightMu.Lock()
	if inFlight[key] {
		inFlightMu.Unlock()
		return
	}
	inFlight[key] = true
	inFlightMu.Unlock()

	go func() {
		defer func() {
			inFlightMu.Lock()
			delete(inFlight, key)
			inFlightMu.Unlock()
		}()
		svg, err := s.ds.GetSVG(ic.Set, ic.Name)
		if err != nil {
			return
		}
		img, err := render.Rasterize(svg, gridSize, gridColor)
		if err != nil {
			return
		}
		s.cache.Put(key, img)
		cx.Post(func() { s.refresh.Set(s.refresh.Get() + 1) })
	}()
}

// iconGrid renders the virtualized grid. Cells resolve their icon through
// the render cache; a miss schedules a background rasterization and shows a
// placeholder until refresh lands.
func iconGrid(cx *core.Scope, s gridState) core.Node {
	rowAt := func(i int) (dataset.Icon, bool) {
		res := s.results.Get()
		if i < 0 || i >= len(res) {
			return dataset.Icon{}, false
		}
		return res[i], true
	}
	cell := func(i int) core.Node {
		return ui.Press{
			OnPress: func() {
				if ic, ok := rowAt(i); ok {
					s.onOpen(ic)
				}
			},
			// The cell fills its (flexible) column width so window resizes
			// and the detail panel never overlap cells; the image inside
			// scales with Contain.
			Style: core.Style{Width: core.Pct(100), Height: core.Pt(cellSize), CornerRadius: 8,
				Background: core.RGB(0x1c1c1e), AlignItems: core.AlignCenter, JustifyContent: core.JustifyCenter},
			StyleFunc: func() core.Style {
				return core.Style{Hover: &core.Style{Background: core.RGB(0x2a2a2e)}}
			},
			Children: []core.Node{
				core.Image{
					SourceFunc: func() core.ImageSource {
						s.refresh.Get() // depend on rasterization arrivals
						ic, ok := rowAt(i)
						if !ok {
							return core.ImageSource{}
						}
						key := cacheKey(ic)
						if img, ok := s.cache.Get(key); ok {
							return core.ImageSource{Pixels: img, Size: core.Size{Width: core.Points(img.Bounds().Dx()), Height: core.Points(img.Bounds().Dy())}}
						}
						startRasterize(cx, s, key, ic)
						return core.ImageSource{}
					},
					ResizeMode: core.Contain,
					Style:      core.Style{Width: core.Pct(100), Height: core.Pct(100)},
				},
			},
		}
	}
	return core.View{Class: "flex-1 min-h-0", Children: []core.Node{base.List{
		Columns:       10,
		EstimatedSize: cellSize + 12,
		Gap:           8,
		ListStyle:     core.Style{Padding: core.All(12)},
		// The outer view must be flex-bounded or base.List grows to its
		// content height and the list virtualizes every row.
		Style: core.Style{FlexGrow: 1, FlexBasis: core.Pct(0), FlexShrink: 1, Width: core.Pct(100)},
		Count: func() int { return len(s.results.Get()) },
		Row: func(cx *core.Scope, row core.ListRow) core.Node {
			s.refresh.Get() // depend on rasterization arrivals
			s.results.Get() // rebuild rows when the result set changes
			return cell(row.Index())
		},
		Empty: core.Text{Content: "无匹配图标", Style: core.Style{FontSize: 13, Color: core.RGB(0x737373)}},
	}}}
}
