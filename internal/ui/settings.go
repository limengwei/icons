package ui

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/nsgo-dev/nsgo/core"
	"github.com/nsgo-dev/nsgo/ui"

	"reacticons/internal/dataset"
	"reacticons/internal/settings"
)

// settingsState is the settings dialog's inputs.
type settingsState struct {
	st      *settings.Settings
	ds      *dataset.Dataset
	toaster *ui.Toaster
	onSaved func(port int)
}

// settingsPanel is a centered modal overlay editing the app's settings:
// the MCP HTTP port. Closing reports the (possibly updated) port through
// onSaved, which is also how the app dismisses the overlay.
func settingsPanel(cx *core.Scope, s settingsState) core.Node {
	// The port the running servers answer on; a saved change only takes
	// effect after a restart, so it stays what the app started with.
	effective := s.st.MCPPort
	portSig := core.NewSignal(cx, strconv.Itoa(effective))

	validPort := func() (int, bool) {
		p, err := strconv.Atoi(strings.TrimSpace(portSig.Get()))
		return p, err == nil && p >= 1 && p <= 65535
	}
	portValid := func() bool {
		_, ok := validPort()
		return ok
	}

	save := func() {
		p, ok := validPort()
		if !ok {
			return
		}
		s.st.MCPPort = p
		// Reload first so fields saved elsewhere since startup (the detail
		// panel's last save dir) survive this write.
		if fresh, err := settings.Load(); err == nil && fresh.LastSaveDir != "" {
			s.st.LastSaveDir = fresh.LastSaveDir
		}
		if err := s.st.Save(); err != nil {
			s.toaster.Show(ui.ToastContent{Type: ui.ToastError, Title: "保存失败", Description: err.Error()})
			return
		}
		s.toaster.Show(ui.ToastContent{Type: ui.ToastSuccess, Title: "设置已保存",
			Description: "MCP 端口重启后生效"})
		s.onSaved(p)
	}

	labelStyle := core.Style{FontSize: 11, Color: core.RGB(0xa3a3a3)}
	return core.View{Class: "absolute inset-0 bg-black/60 z-50 flex-row items-center justify-center", Children: []core.Node{
		core.View{Class: "w-[460px] rounded-xl bg-neutral-900 border border-neutral-700 p-5 gap-4 flex-col", Children: []core.Node{
			core.View{Class: "flex-row items-center justify-between", Children: []core.Node{
				core.Text{Content: "设置", Style: core.Style{FontSize: 16}},
				ui.Button{Text: "关闭", OnPress: func() { s.onSaved(s.st.MCPPort) }},
			}},

			// MCP port.
			core.View{Class: "gap-1 flex-col", Children: []core.Node{
				core.Text{Content: "MCP HTTP 端口", Style: labelStyle},
				ui.Input{Value: portSig.Get, OnChange: portSig.Set, Style: core.Style{FontSize: 12}},
				core.Show{When: func() bool { return !portValid() }, Then: func(cx *core.Scope) core.Node {
					return core.Text{Content: "端口须为 1–65535 的整数", Style: core.Style{FontSize: 10, Color: core.RGB(0xf87171)}}
				}},
				core.Text{Content: fmt.Sprintf("当前生效端口：%d（重启应用后切换）", effective),
					Style: core.Style{FontSize: 10, Color: core.RGB(0x737373)}},
			}},

			// Read-only app information.
			core.View{Class: "gap-2 flex-col pt-3 border-t border-neutral-800", Children: []core.Node{
				core.Text{Content: fmt.Sprintf("数据版本：v%s", s.ds.Version()),
					Style: core.Style{FontSize: 11, Color: core.RGB(0xa3a3a3)}},
				core.Text{Content: fmt.Sprintf("集合数：%d · 图标总数：%d", len(s.ds.Sets()), s.ds.Total()),
					Style: core.Style{FontSize: 11, Color: core.RGB(0xa3a3a3)}},
				core.View{Class: "gap-1 flex-col", Children: []core.Node{
					core.Text{Content: "stdio 启动命令", Style: core.Style{FontSize: 10, Color: core.RGB(0x737373)}},
					core.Text{Content: "icons.exe --mcp", Style: core.Style{FontSize: 11,
						FontFamily: core.FontMonospace, Color: core.RGB(0xe5e5e5),
						Background: core.RGB(0x18181b), Padding: core.Symmetric(6, 8), CornerRadius: 6}},
				}},
				core.View{Class: "gap-1 flex-col", Children: []core.Node{
					core.Text{Content: "设置文件", Style: core.Style{FontSize: 10, Color: core.RGB(0x737373)}},
					core.Text{Content: settings.Path(), Style: core.Style{FontSize: 10, Color: core.RGB(0x737373)}},
				}},
			}},

			core.View{Class: "flex-row justify-end gap-2", Children: []core.Node{
				ui.Button{Text: "取消", OnPress: func() { s.onSaved(s.st.MCPPort) }},
				ui.Button{Text: "保存", Disabled: func() bool { return !portValid() }, OnPress: save},
			}},
		}},
	}}
}
