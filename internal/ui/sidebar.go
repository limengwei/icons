package ui

import (
	"fmt"
	"strings"

	"github.com/nsgo-dev/nsgo/base"
	"github.com/nsgo-dev/nsgo/core"
	"github.com/nsgo-dev/nsgo/ui"

	"reacticons/internal/dataset"
)

// sidebarState is the left column's inputs.
type sidebarState struct {
	ds     *dataset.Dataset
	set    *core.Signal[string] // "" = all sets
	onPick func(string)
}

// sidebar renders the set list with counts and a filter box.
func sidebar(cx *core.Scope, s sidebarState) core.Node {
	filter := core.NewSignal(cx, "")
	return core.View{Class: "w-60 flex-col border-r border-neutral-800 bg-neutral-900 min-h-0", Children: []core.Node{
		core.View{Class: "p-3 gap-2 flex-col", Children: []core.Node{
			ui.Press{
				OnPress: func() { s.onPick("") },
				Style: core.Style{Width: core.Pct(100), Padding: core.Symmetric(5, 8), CornerRadius: 6,
					Background: core.RGB(0x262626), FontSize: 13, TextAlign: core.TextAlignLeft},
				StyleFunc: func() core.Style {
					if s.set.Get() == "" {
						return core.Style{Background: core.RGB(0x3f3f46)}
					}
					return core.Style{}
				},
				Children: []core.Node{
					core.Text{Content: fmt.Sprintf("全部图标（%d）", s.ds.Total())},
				},
			},
			ui.Input{
				Value:       filter.Get,
				OnChange:    filter.Set,
				Placeholder: "过滤集合…",
			},
		}},
		core.View{Class: "flex-1 min-h-0", Children: []core.Node{setList(s, filter)}},
	}}
}

// visibleSets filters sets by a case-insensitive substring over the display
// name or the set id (e.g. "fa6").
func visibleSets(all []dataset.SetInfo, filter string) []dataset.SetInfo {
	filter = strings.TrimSpace(strings.ToLower(filter))
	if filter == "" {
		return all
	}
	out := make([]dataset.SetInfo, 0, 8)
	for _, s := range all {
		if strings.Contains(strings.ToLower(s.Name), filter) ||
			strings.Contains(strings.ToLower(s.ID), filter) {
			out = append(out, s)
		}
	}
	return out
}

func setList(s sidebarState, filter *core.Signal[string]) base.List {
	visible := func() []dataset.SetInfo { return visibleSets(s.ds.Sets(), filter.Get()) }
	return base.List{
		Count:         func() int { return len(visible()) },
		EstimatedSize: 28,
		Gap:           2,
		ListStyle:     core.Style{Padding: core.All(4)},
		// Flex-bound so the list virtualizes instead of growing to its
		// content height (see iconGrid).
		Style: core.Style{FlexGrow: 1, FlexBasis: core.Pct(0), FlexShrink: 1, Width: core.Pct(100)},
		Empty: core.Text{Content: "无匹配集合", Style: core.Style{FontSize: 12, Color: core.RGB(0x737373)}},
		Row: func(cx *core.Scope, row core.ListRow) core.Node {
			return ui.Press{
				OnPress: func() { s.onPick(visible()[row.Index()].ID) },
				Style: core.Style{Width: core.Pct(100), Padding: core.Symmetric(4, 8),
					CornerRadius: 6, Background: core.RGB(0x262626)},
				StyleFunc: func() core.Style {
					if s.set.Get() == visible()[row.Index()].ID {
						return core.Style{Background: core.RGB(0x3f3f46)}
					}
					return core.Style{}
				},
				Children: []core.Node{
					core.Text{ContentFunc: func() string { return visible()[row.Index()].Name },
						Style: core.Style{FontSize: 12, TextOverflow: core.EllipsizeTail, Color: core.RGB(0xd4d4d4)}},
					core.Text{ContentFunc: func() string { return fmt.Sprintf("(%d)", visible()[row.Index()].Count) },
						Style: core.Style{FontSize: 10, Color: core.RGB(0x737373)}},
				},
			}
		},
		OnActivate: func(i int) { s.onPick(visible()[i].ID) },
	}
}
