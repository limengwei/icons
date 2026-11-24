// Command mcpsmoke is a one-off MCP client smoke test against the running
// icons HTTP server. It is a dev tool, not part of the app.
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func main() {
	url := flag.String("url", "http://127.0.0.1:8300/mcp", "streamable HTTP endpoint")
	flag.Parse()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	transport := &mcp.StreamableClientTransport{Endpoint: *url}
	client := mcp.NewClient(&mcp.Implementation{Name: "smoke", Version: "0"}, nil)
	sess, err := client.Connect(ctx, transport, nil)
	if err != nil {
		fmt.Fprintln(os.Stderr, "connect:", err)
		os.Exit(1)
	}
	defer sess.Close()

	tools, err := sess.ListTools(ctx, nil)
	if err != nil {
		fmt.Fprintln(os.Stderr, "list tools:", err)
		os.Exit(1)
	}
	for _, t := range tools.Tools {
		fmt.Println("tool:", t.Name)
	}

	call := func(name string, args map[string]any, out any) {
		res, err := sess.CallTool(ctx, &mcp.CallToolParams{Name: name, Arguments: args})
		if err != nil {
			fmt.Fprintln(os.Stderr, name+":", err)
			os.Exit(1)
		}
		if res.IsError {
			fmt.Fprintln(os.Stderr, name+": tool error:", res.Content)
			os.Exit(1)
		}
		raw, _ := json.Marshal(res.StructuredContent)
		if err := json.Unmarshal(raw, out); err != nil {
			fmt.Fprintln(os.Stderr, name+": unmarshal:", err)
			os.Exit(1)
		}
	}

	var sr struct {
		Total int `json:"total"`
		Icons []struct {
			Set  string `json:"set"`
			Name string `json:"name"`
		} `json:"icons"`
	}
	call("search_icons", map[string]any{"query": "beer"}, &sr)
	fmt.Printf("search beer: total=%d first=%s/%s\n", sr.Total, sr.Icons[0].Set, sr.Icons[0].Name)

	var gr struct {
		SVG string `json:"svg"`
	}
	call("get_icon_svg", map[string]any{"set": "go", "name": "GoZap", "color": "#FF5722"}, &gr)
	fmt.Printf("svg: %.60s... len=%d\n", gr.SVG, len(gr.SVG))

	var pr struct {
		PNGBase64 string `json:"pngBase64"`
		Size      int    `json:"size"`
	}
	call("export_icon_png", map[string]any{"set": "go", "name": "GoZap", "size": 64}, &pr)
	fmt.Printf("png: size=%d base64len=%d\n", pr.Size, len(pr.PNGBase64))

	var ls struct {
		Version string `json:"version"`
		Total   int    `json:"total"`
		Sets    []struct {
			ID string `json:"id"`
		} `json:"sets"`
	}
	call("list_sets", map[string]any{}, &ls)
	fmt.Printf("sets: version=%s total=%d count=%d\n", ls.Version, ls.Total, len(ls.Sets))
	fmt.Println("SMOKE_OK")
}
