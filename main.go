// Command reacticons is icons: a desktop browser over the react-icons icon
// library plus curated Iconify collections, with an embedded MCP server for
// external clients. Run with --mcp for a standalone stdio MCP server.
package main

import (
	_ "embed"
	"fmt"
	"net"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"time"

	"github.com/nsgo-dev/nsgo"
	"github.com/nsgo-dev/nsgo/core"
	"github.com/nsgo-dev/nsgo/desktop"

	"reacticons/internal/dataset"
	"reacticons/internal/mcp"
	"reacticons/internal/settings"
	"reacticons/internal/ui"
)

//go:embed assets/IconWindows.png
var trayIcon []byte

// singleInstanceAddr is the local lock the GUI holds to stay a single
// instance: the running process listens on it. It is deliberately not the
// MCP port.
const (
	singleInstanceAddr = "127.0.0.1:9395"
	singleInstancePort = "9395"
)

// acquireSingleInstance takes the single-instance lock. A previous instance
// still holding it (hiding in the tray) is killed, so the newest launch
// always wins and shows its own window. release frees the lock for an
// in-app restart.
func acquireSingleInstance() (release func()) {
	ln, err := net.Listen("tcp", singleInstanceAddr)
	if err != nil {
		killPreviousInstance()
		for i := 0; i < 30 && err != nil; i++ {
			time.Sleep(100 * time.Millisecond)
			ln, err = net.Listen("tcp", singleInstanceAddr)
		}
		if err != nil {
			// A foreign program squats the port; run without the lock.
			fmt.Fprintf(os.Stderr, "[单实例] %s 被其他程序占用，跳过单实例保护\n", singleInstanceAddr)
			return func() {}
		}
	}
	return func() { _ = ln.Close() }
}

// killPreviousInstance force-kills the process listening on the singleton
// port — the previous GUI instance (likely hiding in the tray).
func killPreviousInstance() {
	out, err := exec.Command("netstat", "-ano", "-p", "tcp").Output()
	if err != nil {
		return
	}
	self := os.Getpid()
	for _, line := range strings.Split(string(out), "\n") {
		if !strings.Contains(line, "LISTENING") {
			continue
		}
		f := strings.Fields(strings.TrimSpace(line))
		if len(f) < 5 || !strings.HasSuffix(f[1], ":"+singleInstancePort) {
			continue
		}
		if pid, err := strconv.Atoi(f[4]); err == nil && pid != self {
			if p, err := os.FindProcess(pid); err == nil {
				_ = p.Kill()
			}
		}
	}
}

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

	// Only one GUI instance: the newest launch kills the previous one
	// (running in the tray) and takes over the lock. stdio --mcp runs do
	// not take the lock.
	releaseSingleton := acquireSingleInstance()

	// Embedded MCP HTTP server for external clients.
	go func() {
		if err := mcp.ServeHTTP(fmt.Sprintf("127.0.0.1:%d", port), ds); err != nil {
			fmt.Fprintf(os.Stderr, "[MCP] %v\n", err)
		}
	}()

	nsgo.Run(func(app *core.Scope) {
		win := desktop.OpenWindow(app, desktop.WindowOptions{
			Title: "icons · UI图标库", Width: 1280, Height: 800, Key: "main",
		}, func(cx *core.Scope) core.Node {
			if ds == nil {
				return core.View{Class: "w-full h-full flex-row items-center justify-center bg-neutral-950 text-neutral-100", Children: []core.Node{
					core.Text{Content: fmt.Sprintf("icons · 数据集加载失败: %v", dsErr), Style: core.Style{FontSize: 14, Color: core.RGB(0xa3a3a3)}},
				}}
			}
			return ui.Main(cx, &ui.Deps{DS: ds, Port: port, ReleaseSingleton: releaseSingleton})
		})

		// Closing the window hides it to the tray instead of quitting, so the
		// embedded MCP server keeps serving; the tray menu shows or quits.
		win.OnClosing(app, func(c *core.WindowClosing) {
			c.Cancel()
			win.Hide()
		})
		desktop.NewTray(app, desktop.TrayOptions{
			Icon:    trayIcon,
			Tooltip: fmt.Sprintf("icons · MCP :%d", port),
			OnClick: win.Show,
			Menu: func() []core.MenuItem {
				return []core.MenuItem{
					{Label: "显示窗口", OnSelect: win.Show},
					{Separator: true},
					{Label: "退出", OnSelect: app.Quit},
				}
			},
		})
	}, nsgo.Appearance(core.AppearanceDark))
}
