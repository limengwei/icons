package ui

import (
	"github.com/nsgo-dev/nsgo/core"
	"github.com/nsgo-dev/nsgo/ui"
)

// toolbarState is the top bar's inputs.
type toolbarState struct {
	query      *core.Signal[string]
	onChange   func(string)
	onSubmit   func()
	onSettings func()
}

// toolbar renders the search box and the MCP settings button (robot icon
// plus the label).
func toolbar(cx *core.Scope, s toolbarState) core.Node {
	return core.View{Class: "p-3 gap-2 flex-row items-center", Children: []core.Node{
		core.View{Class: "flex-1 min-w-0", Children: []core.Node{
			ui.Input{
				Value:       s.query.Get,
				OnChange:    s.onChange,
				OnSubmit:    func(string) { s.onSubmit() },
				Placeholder: "搜索图标（如 beer / youtube）…",
				AutoFocus:   true,
			},
		}},
		ui.Press{
			OnPress: s.onSettings,
			Label:   "MCP 设置",
			Style: core.Style{Padding: core.Symmetric(6, 12), CornerRadius: 6,
				Background: core.RGB(0x262626), Gap: 6,
				AlignItems: core.AlignCenter, JustifyContent: core.JustifyCenter},
			StyleFunc: func() core.Style {
				return core.Style{Hover: &core.Style{Background: core.RGB(0x2a2a2e)}}
			},
			Children: []core.Node{
				core.Icon{SVG: robotSVG, Style: core.Style{Width: core.Pt(18), Height: core.Pt(18)}},
				core.Text{Content: "MCP", Style: core.Style{FontSize: 12, Color: core.RGB(0xe5e5e5)}},
			},
		},
	}}
}
