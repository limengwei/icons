// Command reacticons is icons: a desktop browser over the react-icons icon
// library, with an embedded MCP server for external clients. Run with --mcp
// for a standalone stdio MCP server.
package main

import (
	"fmt"
	"os"

	"github.com/nsgo-dev/nsgo"
	"github.com/nsgo-dev/nsgo/core"
	"github.com/nsgo-dev/nsgo/desktop"

	"reacticons/internal/dataset"
	"reacticons/internal/mcp"
	"reacticons/internal/settings"
	"reacticons/internal/ui"
)

func main() {
	ds, dsErr := dataset.Open()

	if len(os.Args) > 1 && os.Args[1] == "--mcp" {
		if dsErr != nil {
			fmt.Fprintf(os.Stderr, "数据集不可用: %v\n", dsErr)
			os.Exit(1)
		}
		if err := mcp.ServeStdio(ds); err != nil {
			fmt.Fprintf(os.Stderr, "MCP stdio: %v\n", err)
			os.Exit(1)
		}
		return
	}

	if dsErr != nil {
		fmt.Fprintf(os.Stderr, "[数据集] %v（图标浏览与 MCP 工具不可用）\n", dsErr)
	}

	cfg, _ := settings.Load() // 容错：失败时返回默认值
	port := cfg.MCPPort

	// Embedded MCP HTTP server for external clients.
	go func() {
		if err := mcp.ServeHTTP(fmt.Sprintf("127.0.0.1:%d", port), ds); err != nil {
			fmt.Fprintf(os.Stderr, "[MCP] %v\n", err)
		}
	}()

	nsgo.Run(func(app *core.Scope) {
		desktop.OpenWindow(app, desktop.WindowOptions{
			Title: "icons · react-icons 图标库", Width: 1280, Height: 800, Key: "main",
		}, func(cx *core.Scope) core.Node {
			if ds == nil {
				return core.View{Class: "w-full h-full flex-row items-center justify-center bg-neutral-950 text-neutral-100", Children: []core.Node{
					core.Text{Content: fmt.Sprintf("icons · 数据集加载失败: %v", dsErr), Style: core.Style{FontSize: 14, Color: core.RGB(0xa3a3a3)}},
				}}
			}
			return ui.Main(cx, &ui.Deps{DS: ds, Port: port})
		})
	}, nsgo.Appearance(core.AppearanceDark))
}
