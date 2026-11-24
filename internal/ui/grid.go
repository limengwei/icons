package ui

import (
	"github.com/nsgo-dev/nsgo/base"
	"github.com/nsgo-dev/nsgo/core"
	"github.com/nsgo-dev/nsgo/ui"

	"reacticons/internal/dataset"
)

// cellSize is one grid cell's icon box.
const cellSize = 76

// gridSize is the icon's box inside a cell; core.Icon draws vectors, so the
// number is only layout, not rasterization resolution.
const gridSize = 52

// maxGridRows caps what the grid claims as its Count. NSGo's List is
// validated in the hundreds-to-thousands range (its DataTable exists for
// millions), so the grid never grows past this; deeper results need a
// keyword. The whole dataset holds ~51k icons, so the cap keeps the default
// all-sets view inside the list's comfort zone.
const maxGridRows = 10_000

// gridIconColor tints every cell icon (fill="currentColor" on the root svg
// inherits this text color).
var gridIconColor = core.RGB(0xe5e5e5)

// gridState feeds the virtualized grid from the current result set.
type gridState struct {
	results *core.Signal[[]dataset.Icon]
	onOpen  func(dataset.Icon)
	ds      *dataset.Dataset
	svgs    *svgCache
}

// iconGrid renders the virtualized grid. Cells draw their icon as a vector
// through core.Icon: sharp at any size, tinted by the cell's text color.
func iconGrid(cx *core.Scope, s gridState) core.Node {
	// A new result set (set picked, search submitted) resets the scroll to
	// the top, so the grid never stays stranded at an offset from the
	// previous, longer result. The first run happens before mount, where
	// ScrollToIndex is a safe no-op.
	listRef := &core.Ref{}
	cx.Effect(func() {
		s.results.Get()
		listRef.ScrollToIndex(0, false)
	})
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
			// and the detail panel never overlap cells; the icon inside is
			// fitted and centered in its box.
			Style: core.Style{Width: core.Pct(100), Height: core.Pt(cellSize), CornerRadius: 8,
				Background: core.RGB(0x1c1c1e), Color: gridIconColor,
				AlignItems: core.AlignCenter, JustifyContent: core.JustifyCenter},
			StyleFunc: func() core.Style {
				return core.Style{Hover: &core.Style{Background: core.RGB(0x2a2a2e)}}
			},
			Children: []core.Node{
				core.Icon{
					SVGFunc: func() *core.SVG {
						ic, ok := rowAt(i)
						if !ok {
							return blankSVG
						}
						return iconSVG(s.ds, s.svgs, ic.Set, ic.Name)
					},
					Style: core.Style{Width: core.Pt(gridSize), Height: core.Pt(gridSize)},
				},
			},
		}
	}
	return core.View{Class: "flex-1 min-h-0", Children: []core.Node{base.List{
		Ref:           listRef,
		Columns:       10,
		EstimatedSize: cellSize + 12,
		Gap:           8,
		ListStyle:     core.Style{Padding: core.All(12)},
		// The outer view must be flex-bounded or base.List grows to its
		// content height and the list virtualizes every row.
		Style: core.Style{FlexGrow: 1, FlexBasis: core.Pct(0), FlexShrink: 1, Width: core.Pct(100)},
		Count: func() int { return min(len(s.results.Get()), maxGridRows) },
		Row: func(cx *core.Scope, row core.ListRow) core.Node {
			s.results.Get() // rebuild rows when the result set changes
			return cell(row.Index())
		},
		Empty: core.Text{Content: "无匹配图标", Style: core.Style{FontSize: 13, Color: core.RGB(0x737373)}},
	}}}
}
