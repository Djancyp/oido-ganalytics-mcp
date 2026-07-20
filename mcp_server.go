package main

import (
	"context"
	"fmt"
	"log"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	data "google.golang.org/api/analyticsdata/v1beta"
)

type handler struct{}

type ListPropertiesArgs struct{}

type RunReportArgs struct {
	PropertyID string `json:"property_id" jsonschema:"GA4 property ID, e.g. '123456789' (use ga_list_properties to find it)"`
	StartDate  string `json:"start_date" jsonschema:"Start date, GA4 syntax: YYYY-MM-DD, 'NdaysAgo', 'yesterday', 'today' (default '7daysAgo')"`
	EndDate    string `json:"end_date" jsonschema:"End date, same syntax (default 'today')"`
	Dimensions string `json:"dimensions" jsonschema:"Comma-separated dimension API names, e.g. 'date,country' (optional)"`
	Metrics    string `json:"metrics" jsonschema:"Comma-separated metric API names, e.g. 'activeUsers,sessions' (default 'activeUsers,sessions')"`
	Limit      int64  `json:"limit" jsonschema:"Max rows (default 50)"`
}

type RunRealtimeArgs struct {
	PropertyID string `json:"property_id" jsonschema:"GA4 property ID"`
	Dimensions string `json:"dimensions" jsonschema:"Comma-separated realtime dimension names, e.g. 'country,unifiedScreenName' (optional)"`
	Metrics    string `json:"metrics" jsonschema:"Comma-separated realtime metric names (default 'activeUsers')"`
	Limit      int64  `json:"limit" jsonschema:"Max rows (default 50)"`
}

type GetMetadataArgs struct {
	PropertyID string `json:"property_id" jsonschema:"GA4 property ID"`
}

func textResult(s string) *mcp.CallToolResult {
	return &mcp.CallToolResult{Content: []mcp.Content{&mcp.TextContent{Text: s}}}
}

func errResult(err error) *mcp.CallToolResult {
	return &mcp.CallToolResult{Content: []mcp.Content{&mcp.TextContent{Text: "Error: " + err.Error()}}, IsError: true}
}

func (h *handler) ListProperties(ctx context.Context, _ *mcp.CallToolRequest, _ ListPropertiesArgs) (*mcp.CallToolResult, any, error) {
	accounts, err := ListProperties(ctx)
	if err != nil {
		return errResult(err), nil, nil
	}
	if len(accounts) == 0 {
		return textResult("No GA4 properties found for this account."), nil, nil
	}
	var b strings.Builder
	b.WriteString("GA4 Properties:\n\nProperty ID | Display Name | Account\n")
	for _, a := range accounts {
		for _, p := range a.PropertySummaries {
			// p.Property is "properties/123456789"; show the bare ID for convenience.
			id := strings.TrimPrefix(p.Property, "properties/")
			b.WriteString(fmt.Sprintf("%s | %s | %s\n", id, p.DisplayName, a.DisplayName))
		}
	}
	return textResult(b.String()), nil, nil
}

// formatRows renders GA4 report rows as a pipe-separated table.
func formatRows(dimHeaders []*data.DimensionHeader, metHeaders []*data.MetricHeader, rows []*data.Row, rowCount int64) string {
	var b strings.Builder
	var cols []string
	for _, d := range dimHeaders {
		cols = append(cols, d.Name)
	}
	for _, m := range metHeaders {
		cols = append(cols, m.Name)
	}
	b.WriteString(fmt.Sprintf("Rows: %d\n\n%s\n", rowCount, strings.Join(cols, " | ")))
	for _, r := range rows {
		var cells []string
		for _, dv := range r.DimensionValues {
			cells = append(cells, dv.Value)
		}
		for _, mv := range r.MetricValues {
			cells = append(cells, mv.Value)
		}
		b.WriteString(strings.Join(cells, " | ") + "\n")
	}
	return b.String()
}

func (h *handler) RunReport(ctx context.Context, _ *mcp.CallToolRequest, a RunReportArgs) (*mcp.CallToolResult, any, error) {
	if a.PropertyID == "" {
		return errResult(fmt.Errorf("property_id is required (use ga_list_properties to find it)")), nil, nil
	}
	res, err := RunReport(ctx, a.PropertyID, a.StartDate, a.EndDate, a.Dimensions, a.Metrics, a.Limit)
	if err != nil {
		return errResult(err), nil, nil
	}
	if len(res.Rows) == 0 {
		return textResult("No data for the given range."), nil, nil
	}
	return textResult(formatRows(res.DimensionHeaders, res.MetricHeaders, res.Rows, res.RowCount)), nil, nil
}

func (h *handler) RunRealtime(ctx context.Context, _ *mcp.CallToolRequest, a RunRealtimeArgs) (*mcp.CallToolResult, any, error) {
	if a.PropertyID == "" {
		return errResult(fmt.Errorf("property_id is required")), nil, nil
	}
	res, err := RunRealtimeReport(ctx, a.PropertyID, a.Dimensions, a.Metrics, a.Limit)
	if err != nil {
		return errResult(err), nil, nil
	}
	if len(res.Rows) == 0 {
		return textResult("No active users in the last 30 minutes."), nil, nil
	}
	return textResult(formatRows(res.DimensionHeaders, res.MetricHeaders, res.Rows, res.RowCount)), nil, nil
}

func (h *handler) GetMetadata(ctx context.Context, _ *mcp.CallToolRequest, a GetMetadataArgs) (*mcp.CallToolResult, any, error) {
	if a.PropertyID == "" {
		return errResult(fmt.Errorf("property_id is required")), nil, nil
	}
	md, err := GetMetadata(ctx, a.PropertyID)
	if err != nil {
		return errResult(err), nil, nil
	}
	var b strings.Builder
	b.WriteString("Dimensions (apiName — uiName):\n")
	for _, d := range md.Dimensions {
		b.WriteString(fmt.Sprintf("  %s — %s\n", d.ApiName, d.UiName))
	}
	b.WriteString("\nMetrics (apiName — uiName):\n")
	for _, m := range md.Metrics {
		b.WriteString(fmt.Sprintf("  %s — %s\n", m.ApiName, m.UiName))
	}
	return textResult(b.String()), nil, nil
}

// RunMCPServer registers tools and serves over stdio.
func RunMCPServer() {
	h := &handler{}
	server := mcp.NewServer(&mcp.Implementation{Name: "oido-ganalytics", Version: "1.0.0"}, nil)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "ga_list_properties",
		Description: "List the GA4 properties the connected Google account can access (property ID, name, account). Start here to get a property_id.",
	}, h.ListProperties)
	mcp.AddTool(server, &mcp.Tool{
		Name:        "ga_run_report",
		Description: "Run a GA4 report over a date range with dimensions and metrics. Defaults: last 7 days, activeUsers+sessions.",
	}, h.RunReport)
	mcp.AddTool(server, &mcp.Tool{
		Name:        "ga_realtime_report",
		Description: "Report on activity in the last 30 minutes (default metric activeUsers).",
	}, h.RunRealtime)
	mcp.AddTool(server, &mcp.Tool{
		Name:        "ga_get_metadata",
		Description: "List the dimension and metric API names available for a property — use to find valid names for ga_run_report.",
	}, h.GetMetadata)

	log.Println("Oido Google Analytics MCP Server starting on stdio...")
	if err := server.Run(context.Background(), &mcp.StdioTransport{}); err != nil {
		log.Fatalf("server error: %v", err)
	}
}
