// Package mcp serves the icon library over the Model Context Protocol:
// listing icon sets, searching icons, and reading or exporting a single
// icon's SVG/PNG. All tools are read-only.
package mcp

import (
	"context"
	"encoding/base64"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/signal"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"reacticons/internal/dataset"
	"reacticons/internal/exporter"
)

const (
	// searchLimit caps how many icons search_icons returns.
	searchLimit = 100
	// pngDefaultSize is the export size when the caller omits one.
	pngDefaultSize = 256
)

// New builds the MCP server with all tools registered. ds may be nil, in
// which case every tool reports the dataset as unavailable.
func New(ds *dataset.Dataset) *mcp.Server {
	server := mcp.NewServer(&mcp.Implementation{Name: "icons", Version: "1.0.0"}, nil)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "list_sets",
		Description: "列出所有图标集（如 fa6、ri、tb）、图标总数与 react-icons 版本。",
	}, listSets(ds))

	mcp.AddTool(server, &mcp.Tool{
		Name:        "search_icons",
		Description: "按关键词搜索图标（匹配图标名与关键词），可按图标集过滤。最多返回 100 条，total 为未截断的命中总数。",
	}, searchIcons(ds))

	mcp.AddTool(server, &mcp.Tool{
		Name:        "get_icon_svg",
		Description: "获取一个图标的原始 SVG 标记，可选注入颜色到根 svg 标签的 fill。",
	}, getIconSVG(ds))

	mcp.AddTool(server, &mcp.Tool{
		Name:        "export_icon_png",
		Description: "把图标渲染为 PNG 并以 base64 返回，可指定像素尺寸（默认 256）与颜色。",
	}, exportIconPNG(ds))

	return server
}

type setSummary struct {
	ID         string `json:"id" jsonschema:"图标集 id，如 fa6"`
	Name       string `json:"name" jsonschema:"图标集显示名，如 Font Awesome 6"`
	Count      int    `json:"count" jsonschema:"该集图标数量"`
	License    string `json:"license" jsonschema:"许可证名称"`
	ProjectURL string `json:"projectUrl" jsonschema:"项目主页"`
	LicenseURL string `json:"licenseUrl" jsonschema:"许可证链接"`
}

type listSetsResult struct {
	Version string       `json:"version" jsonschema:"react-icons 数据版本，如 5.7.0"`
	Total   int          `json:"total" jsonschema:"全部图标总数"`
	Sets    []setSummary `json:"sets" jsonschema:"图标集列表"`
}

func listSets(ds *dataset.Dataset) func(context.Context, *mcp.CallToolRequest, struct{}) (*mcp.CallToolResult, listSetsResult, error) {
	return func(_ context.Context, _ *mcp.CallToolRequest, _ struct{}) (*mcp.CallToolResult, listSetsResult, error) {
		if ds == nil {
			return errorResult("数据集不可用（内置图标数据加载失败）"), listSetsResult{}, nil
		}
		res := listSetsResult{
			Version: ds.Version(),
			Total:   ds.Total(),
			Sets:    make([]setSummary, 0, len(ds.Sets())),
		}
		for _, s := range ds.Sets() {
			res.Sets = append(res.Sets, setSummary{
				ID: s.ID, Name: s.Name, Count: s.Count,
				License: s.License, ProjectURL: s.ProjectURL, LicenseURL: s.LicenseURL,
			})
		}
		return nil, res, nil
	}
}

type searchIconsArgs struct {
	Query string `json:"query" jsonschema:"搜索关键词，如 beer"`
	Set   string `json:"set,omitempty" jsonschema:"可选，按图标集 id 过滤，如 fa6"`
}

type iconRef struct {
	Set  string `json:"set" jsonschema:"图标集 id"`
	Name string `json:"name" jsonschema:"图标名，如 FaBeer"`
}

type searchIconsResult struct {
	Total int       `json:"total" jsonschema:"未截断的命中总数"`
	Icons []iconRef `json:"icons" jsonschema:"命中的图标，最多 100 条"`
}

func searchIcons(ds *dataset.Dataset) func(context.Context, *mcp.CallToolRequest, searchIconsArgs) (*mcp.CallToolResult, searchIconsResult, error) {
	return func(_ context.Context, _ *mcp.CallToolRequest, in searchIconsArgs) (*mcp.CallToolResult, searchIconsResult, error) {
		if ds == nil {
			return errorResult("数据集不可用（内置图标数据加载失败）"), searchIconsResult{}, nil
		}
		if in.Set != "" {
			if _, ok := ds.Set(in.Set); !ok {
				return errorResult(fmt.Sprintf("图标集 %q 不存在，可用 list_sets 查看全部图标集", in.Set)), searchIconsResult{}, nil
			}
		}
		hits := ds.Search(in.Query, in.Set)
		total := len(hits)
		if total > searchLimit {
			hits = hits[:searchLimit]
		}
		icons := make([]iconRef, 0, len(hits))
		for _, ic := range hits {
			icons = append(icons, iconRef{Set: ic.Set, Name: ic.Name})
		}
		return nil, searchIconsResult{Total: total, Icons: icons}, nil
	}
}

type getIconSVGArgs struct {
	Set   string `json:"set" jsonschema:"图标集 id，如 fa6"`
	Name  string `json:"name" jsonschema:"图标名，如 FaBeer"`
	Color string `json:"color,omitempty" jsonschema:"可选，CSS 颜色注入根 svg 标签 fill，如 #ff0000"`
}

type getIconSVGResult struct {
	SVG string `json:"svg" jsonschema:"图标的 SVG 标记"`
}

func getIconSVG(ds *dataset.Dataset) func(context.Context, *mcp.CallToolRequest, getIconSVGArgs) (*mcp.CallToolResult, getIconSVGResult, error) {
	return func(_ context.Context, _ *mcp.CallToolRequest, in getIconSVGArgs) (*mcp.CallToolResult, getIconSVGResult, error) {
		if ds == nil {
			return errorResult("数据集不可用（内置图标数据加载失败）"), getIconSVGResult{}, nil
		}
		if _, ok := ds.Set(in.Set); !ok {
			return errorResult(fmt.Sprintf("图标集 %q 不存在，可用 list_sets 查看全部图标集", in.Set)), getIconSVGResult{}, nil
		}
		svg, err := ds.GetSVG(in.Set, in.Name)
		if err != nil {
			return errorResult(err.Error()), getIconSVGResult{}, nil
		}
		return nil, getIconSVGResult{SVG: exporter.ApplyColor(svg, in.Color)}, nil
	}
}

type exportIconPNGArgs struct {
	Set   string `json:"set" jsonschema:"图标集 id，如 fa6"`
	Name  string `json:"name" jsonschema:"图标名，如 FaBeer"`
	Size  int    `json:"size,omitempty" jsonschema:"可选，PNG 像素尺寸，默认 256"`
	Color string `json:"color,omitempty" jsonschema:"可选，CSS 颜色，如 #ff0000"`
}

type exportIconPNGResult struct {
	PNGBase64 string `json:"pngBase64" jsonschema:"PNG 文件内容的 base64 编码"`
	Size      int    `json:"size" jsonschema:"实际渲染的像素尺寸"`
}

func exportIconPNG(ds *dataset.Dataset) func(context.Context, *mcp.CallToolRequest, exportIconPNGArgs) (*mcp.CallToolResult, exportIconPNGResult, error) {
	return func(_ context.Context, _ *mcp.CallToolRequest, in exportIconPNGArgs) (*mcp.CallToolResult, exportIconPNGResult, error) {
		if ds == nil {
			return errorResult("数据集不可用（内置图标数据加载失败）"), exportIconPNGResult{}, nil
		}
		if _, ok := ds.Set(in.Set); !ok {
			return errorResult(fmt.Sprintf("图标集 %q 不存在，可用 list_sets 查看全部图标集", in.Set)), exportIconPNGResult{}, nil
		}
		svg, err := ds.GetSVG(in.Set, in.Name)
		if err != nil {
			return errorResult(err.Error()), exportIconPNGResult{}, nil
		}
		size := in.Size
		if size < 1 {
			size = pngDefaultSize
		}
		png, err := exporter.PNG(svg, size, in.Color)
		if err != nil {
			return errorResult(err.Error()), exportIconPNGResult{}, nil
		}
		return nil, exportIconPNGResult{
			PNGBase64: base64.StdEncoding.EncodeToString(png),
			Size:      size,
		}, nil
	}
}

func errorResult(text string) *mcp.CallToolResult {
	return &mcp.CallToolResult{
		IsError: true,
		Content: []mcp.Content{&mcp.TextContent{Text: text}},
	}
}

// ServeHTTP runs the MCP streamable HTTP server on addr (e.g.
// "127.0.0.1:8300") under /mcp until the process exits. It is meant to run
// in a goroutine next to the desktop UI.
func ServeHTTP(addr string, ds *dataset.Dataset) error {
	handler := mcp.NewStreamableHTTPHandler(func(*http.Request) *mcp.Server { return New(ds) }, nil)
	mux := http.NewServeMux()
	mux.Handle("/mcp", handler)
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		return fmt.Errorf("MCP HTTP 监听失败: %w", err)
	}
	fmt.Fprintf(os.Stderr, "[MCP] listening on http://%s/mcp\n", addr)
	return http.Serve(ln, mux)
}

// ServeStdio runs the MCP server over stdio for `--mcp` mode.
func ServeStdio(ds *dataset.Dataset) error {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()
	if ds != nil {
		fmt.Fprintf(os.Stderr, "icons MCP (stdio): %d icons, react-icons v%s\n", ds.Total(), ds.Version())
	}
	return New(ds).Run(ctx, &mcp.StdioTransport{})
}
