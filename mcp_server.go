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

// ---------------------------------------------------------------------------
// Configuration (write) tool args. Require the analytics.edit scope.
// Destructive tools (delete/archive) carry a `confirm` gate.
// ---------------------------------------------------------------------------

type CreatePropertyArgs struct {
	AccountID   string `json:"account_id" jsonschema:"Parent account ID, e.g. '123456' (use ga_list_properties to find accounts)"`
	DisplayName string `json:"display_name" jsonschema:"Property display name"`
	TimeZone    string `json:"time_zone" jsonschema:"IANA time zone, e.g. 'America/New_York' (required by GA4)"`
	Currency    string `json:"currency" jsonschema:"3-letter currency code (default 'USD')"`
	Industry    string `json:"industry" jsonschema:"Industry category enum, e.g. 'TECHNOLOGY' (optional)"`
}

type UpdatePropertyArgs struct {
	PropertyID  string `json:"property_id" jsonschema:"GA4 property ID"`
	DisplayName string `json:"display_name" jsonschema:"New display name (optional)"`
	TimeZone    string `json:"time_zone" jsonschema:"New IANA time zone (optional)"`
	Currency    string `json:"currency" jsonschema:"New 3-letter currency code (optional)"`
}

type CreateWebStreamArgs struct {
	PropertyID  string `json:"property_id" jsonschema:"GA4 property ID"`
	DisplayName string `json:"display_name" jsonschema:"Data stream display name"`
	DefaultURI  string `json:"default_uri" jsonschema:"Website URL, e.g. 'https://example.com'"`
}

type PropertyOnlyArgs struct {
	PropertyID string `json:"property_id" jsonschema:"GA4 property ID"`
}

type DeleteStreamArgs struct {
	StreamName string `json:"stream_name" jsonschema:"Full resource name 'properties/123/dataStreams/456' (use ga_list_data_streams)"`
	Confirm    bool   `json:"confirm" jsonschema:"Must be true to actually delete — destructive"`
}

type CreateCustomDimensionArgs struct {
	PropertyID    string `json:"property_id" jsonschema:"GA4 property ID"`
	DisplayName   string `json:"display_name" jsonschema:"Dimension display name"`
	ParameterName string `json:"parameter_name" jsonschema:"Event/user parameter name to map, e.g. 'plan_type'"`
	Scope         string `json:"scope" jsonschema:"'EVENT' (default) or 'USER'"`
	Description   string `json:"description" jsonschema:"Optional description"`
}

type ArchiveByNameArgs struct {
	Name    string `json:"name" jsonschema:"Full resource name of the item to archive/delete"`
	Confirm bool   `json:"confirm" jsonschema:"Must be true to proceed — destructive"`
}

type CreateCustomMetricArgs struct {
	PropertyID      string `json:"property_id" jsonschema:"GA4 property ID"`
	DisplayName     string `json:"display_name" jsonschema:"Metric display name"`
	ParameterName   string `json:"parameter_name" jsonschema:"Event parameter name to map, e.g. 'value'"`
	Scope           string `json:"scope" jsonschema:"'EVENT' (default)"`
	MeasurementUnit string `json:"measurement_unit" jsonschema:"'STANDARD' (default), 'CURRENCY', 'FEET', 'METERS', 'SECONDS', etc."`
	Description     string `json:"description" jsonschema:"Optional description"`
}

type CreateKeyEventArgs struct {
	PropertyID     string `json:"property_id" jsonschema:"GA4 property ID"`
	EventName      string `json:"event_name" jsonschema:"Event name to mark as key event (conversion), e.g. 'purchase'"`
	CountingMethod string `json:"counting_method" jsonschema:"'ONCE_PER_EVENT' (default) or 'ONCE_PER_SESSION'"`
}

type UpdateDataRetentionArgs struct {
	PropertyID         string `json:"property_id" jsonschema:"GA4 property ID"`
	EventDataRetention string `json:"event_data_retention" jsonschema:"Enum, e.g. 'TWO_MONTHS' or 'FOURTEEN_MONTHS' (optional)"`
	ResetOnNewActivity bool   `json:"reset_on_new_activity" jsonschema:"Reset user data retention timer on new activity"`
}

// mustConfirm returns an error result when a destructive op is not confirmed.
func mustConfirm(confirmed bool, what string) *mcp.CallToolResult {
	if confirmed {
		return nil
	}
	return textResult(fmt.Sprintf("Destructive: %s. Re-run with confirm=true to proceed.", what))
}

func (h *handler) CreateProperty(ctx context.Context, _ *mcp.CallToolRequest, a CreatePropertyArgs) (*mcp.CallToolResult, any, error) {
	if a.AccountID == "" || a.DisplayName == "" || a.TimeZone == "" {
		return errResult(fmt.Errorf("account_id, display_name and time_zone are required")), nil, nil
	}
	p, err := CreateProperty(ctx, a.AccountID, a.DisplayName, a.TimeZone, a.Currency, a.Industry)
	if err != nil {
		return errResult(err), nil, nil
	}
	id := strings.TrimPrefix(p.Name, "properties/")
	return textResult(fmt.Sprintf("Created property %s (%s), time zone %s, currency %s.", id, p.DisplayName, p.TimeZone, p.CurrencyCode)), nil, nil
}

func (h *handler) UpdateProperty(ctx context.Context, _ *mcp.CallToolRequest, a UpdatePropertyArgs) (*mcp.CallToolResult, any, error) {
	if a.PropertyID == "" {
		return errResult(fmt.Errorf("property_id is required")), nil, nil
	}
	p, err := UpdateProperty(ctx, a.PropertyID, a.DisplayName, a.TimeZone, a.Currency)
	if err != nil {
		return errResult(err), nil, nil
	}
	return textResult(fmt.Sprintf("Updated property %s: name=%s, tz=%s, currency=%s.", a.PropertyID, p.DisplayName, p.TimeZone, p.CurrencyCode)), nil, nil
}

func (h *handler) CreateWebStream(ctx context.Context, _ *mcp.CallToolRequest, a CreateWebStreamArgs) (*mcp.CallToolResult, any, error) {
	if a.PropertyID == "" || a.DisplayName == "" || a.DefaultURI == "" {
		return errResult(fmt.Errorf("property_id, display_name and default_uri are required")), nil, nil
	}
	ds, err := CreateWebDataStream(ctx, a.PropertyID, a.DisplayName, a.DefaultURI)
	if err != nil {
		return errResult(err), nil, nil
	}
	mid := ""
	if ds.WebStreamData != nil {
		mid = ds.WebStreamData.MeasurementId
	}
	return textResult(fmt.Sprintf("Created web stream %q\nName: %s\nMeasurement ID: %s", ds.DisplayName, ds.Name, mid)), nil, nil
}

func (h *handler) ListDataStreams(ctx context.Context, _ *mcp.CallToolRequest, a PropertyOnlyArgs) (*mcp.CallToolResult, any, error) {
	if a.PropertyID == "" {
		return errResult(fmt.Errorf("property_id is required")), nil, nil
	}
	streams, err := ListDataStreams(ctx, a.PropertyID)
	if err != nil {
		return errResult(err), nil, nil
	}
	if len(streams) == 0 {
		return textResult("No data streams."), nil, nil
	}
	var b strings.Builder
	b.WriteString("Name | Display Name | Type | Measurement ID\n")
	for _, s := range streams {
		mid := ""
		if s.WebStreamData != nil {
			mid = s.WebStreamData.MeasurementId
		}
		b.WriteString(fmt.Sprintf("%s | %s | %s | %s\n", s.Name, s.DisplayName, s.Type, mid))
	}
	return textResult(b.String()), nil, nil
}

func (h *handler) DeleteDataStream(ctx context.Context, _ *mcp.CallToolRequest, a DeleteStreamArgs) (*mcp.CallToolResult, any, error) {
	if a.StreamName == "" {
		return errResult(fmt.Errorf("stream_name is required")), nil, nil
	}
	if r := mustConfirm(a.Confirm, "delete data stream "+a.StreamName); r != nil {
		return r, nil, nil
	}
	if err := DeleteDataStream(ctx, a.StreamName); err != nil {
		return errResult(err), nil, nil
	}
	return textResult("Deleted data stream " + a.StreamName), nil, nil
}

func (h *handler) CreateCustomDimension(ctx context.Context, _ *mcp.CallToolRequest, a CreateCustomDimensionArgs) (*mcp.CallToolResult, any, error) {
	if a.PropertyID == "" || a.DisplayName == "" || a.ParameterName == "" {
		return errResult(fmt.Errorf("property_id, display_name and parameter_name are required")), nil, nil
	}
	cd, err := CreateCustomDimension(ctx, a.PropertyID, a.DisplayName, a.ParameterName, a.Scope, a.Description)
	if err != nil {
		return errResult(err), nil, nil
	}
	return textResult(fmt.Sprintf("Created custom dimension %s (%s, scope %s, param %s).", cd.Name, cd.DisplayName, cd.Scope, cd.ParameterName)), nil, nil
}

func (h *handler) ListCustomDimensions(ctx context.Context, _ *mcp.CallToolRequest, a PropertyOnlyArgs) (*mcp.CallToolResult, any, error) {
	if a.PropertyID == "" {
		return errResult(fmt.Errorf("property_id is required")), nil, nil
	}
	dims, err := ListCustomDimensions(ctx, a.PropertyID)
	if err != nil {
		return errResult(err), nil, nil
	}
	if len(dims) == 0 {
		return textResult("No custom dimensions."), nil, nil
	}
	var b strings.Builder
	b.WriteString("Name | Display Name | Scope | Parameter\n")
	for _, d := range dims {
		b.WriteString(fmt.Sprintf("%s | %s | %s | %s\n", d.Name, d.DisplayName, d.Scope, d.ParameterName))
	}
	return textResult(b.String()), nil, nil
}

func (h *handler) ArchiveCustomDimension(ctx context.Context, _ *mcp.CallToolRequest, a ArchiveByNameArgs) (*mcp.CallToolResult, any, error) {
	if a.Name == "" {
		return errResult(fmt.Errorf("name is required")), nil, nil
	}
	if r := mustConfirm(a.Confirm, "archive custom dimension "+a.Name); r != nil {
		return r, nil, nil
	}
	if err := ArchiveCustomDimension(ctx, a.Name); err != nil {
		return errResult(err), nil, nil
	}
	return textResult("Archived custom dimension " + a.Name), nil, nil
}

func (h *handler) CreateCustomMetric(ctx context.Context, _ *mcp.CallToolRequest, a CreateCustomMetricArgs) (*mcp.CallToolResult, any, error) {
	if a.PropertyID == "" || a.DisplayName == "" || a.ParameterName == "" {
		return errResult(fmt.Errorf("property_id, display_name and parameter_name are required")), nil, nil
	}
	cm, err := CreateCustomMetric(ctx, a.PropertyID, a.DisplayName, a.ParameterName, a.Scope, a.MeasurementUnit, a.Description)
	if err != nil {
		return errResult(err), nil, nil
	}
	return textResult(fmt.Sprintf("Created custom metric %s (%s, unit %s, param %s).", cm.Name, cm.DisplayName, cm.MeasurementUnit, cm.ParameterName)), nil, nil
}

func (h *handler) ListCustomMetrics(ctx context.Context, _ *mcp.CallToolRequest, a PropertyOnlyArgs) (*mcp.CallToolResult, any, error) {
	if a.PropertyID == "" {
		return errResult(fmt.Errorf("property_id is required")), nil, nil
	}
	mets, err := ListCustomMetrics(ctx, a.PropertyID)
	if err != nil {
		return errResult(err), nil, nil
	}
	if len(mets) == 0 {
		return textResult("No custom metrics."), nil, nil
	}
	var b strings.Builder
	b.WriteString("Name | Display Name | Unit | Parameter\n")
	for _, m := range mets {
		b.WriteString(fmt.Sprintf("%s | %s | %s | %s\n", m.Name, m.DisplayName, m.MeasurementUnit, m.ParameterName))
	}
	return textResult(b.String()), nil, nil
}

func (h *handler) ArchiveCustomMetric(ctx context.Context, _ *mcp.CallToolRequest, a ArchiveByNameArgs) (*mcp.CallToolResult, any, error) {
	if a.Name == "" {
		return errResult(fmt.Errorf("name is required")), nil, nil
	}
	if r := mustConfirm(a.Confirm, "archive custom metric "+a.Name); r != nil {
		return r, nil, nil
	}
	if err := ArchiveCustomMetric(ctx, a.Name); err != nil {
		return errResult(err), nil, nil
	}
	return textResult("Archived custom metric " + a.Name), nil, nil
}

func (h *handler) CreateKeyEvent(ctx context.Context, _ *mcp.CallToolRequest, a CreateKeyEventArgs) (*mcp.CallToolResult, any, error) {
	if a.PropertyID == "" || a.EventName == "" {
		return errResult(fmt.Errorf("property_id and event_name are required")), nil, nil
	}
	ke, err := CreateKeyEvent(ctx, a.PropertyID, a.EventName, a.CountingMethod)
	if err != nil {
		return errResult(err), nil, nil
	}
	return textResult(fmt.Sprintf("Created key event %s (event %q, counting %s).", ke.Name, ke.EventName, ke.CountingMethod)), nil, nil
}

func (h *handler) ListKeyEvents(ctx context.Context, _ *mcp.CallToolRequest, a PropertyOnlyArgs) (*mcp.CallToolResult, any, error) {
	if a.PropertyID == "" {
		return errResult(fmt.Errorf("property_id is required")), nil, nil
	}
	kes, err := ListKeyEvents(ctx, a.PropertyID)
	if err != nil {
		return errResult(err), nil, nil
	}
	if len(kes) == 0 {
		return textResult("No key events."), nil, nil
	}
	var b strings.Builder
	b.WriteString("Name | Event | Counting | Deletable\n")
	for _, k := range kes {
		b.WriteString(fmt.Sprintf("%s | %s | %s | %t\n", k.Name, k.EventName, k.CountingMethod, k.Deletable))
	}
	return textResult(b.String()), nil, nil
}

func (h *handler) DeleteKeyEvent(ctx context.Context, _ *mcp.CallToolRequest, a ArchiveByNameArgs) (*mcp.CallToolResult, any, error) {
	if a.Name == "" {
		return errResult(fmt.Errorf("name is required")), nil, nil
	}
	if r := mustConfirm(a.Confirm, "delete key event "+a.Name); r != nil {
		return r, nil, nil
	}
	if err := DeleteKeyEvent(ctx, a.Name); err != nil {
		return errResult(err), nil, nil
	}
	return textResult("Deleted key event " + a.Name), nil, nil
}

func (h *handler) GetDataRetention(ctx context.Context, _ *mcp.CallToolRequest, a PropertyOnlyArgs) (*mcp.CallToolResult, any, error) {
	if a.PropertyID == "" {
		return errResult(fmt.Errorf("property_id is required")), nil, nil
	}
	s, err := GetDataRetention(ctx, a.PropertyID)
	if err != nil {
		return errResult(err), nil, nil
	}
	return textResult(fmt.Sprintf("Event data retention: %s\nReset on new activity: %t", s.EventDataRetention, s.ResetUserDataOnNewActivity)), nil, nil
}

func (h *handler) UpdateDataRetention(ctx context.Context, _ *mcp.CallToolRequest, a UpdateDataRetentionArgs) (*mcp.CallToolResult, any, error) {
	if a.PropertyID == "" {
		return errResult(fmt.Errorf("property_id is required")), nil, nil
	}
	s, err := UpdateDataRetention(ctx, a.PropertyID, a.EventDataRetention, a.ResetOnNewActivity)
	if err != nil {
		return errResult(err), nil, nil
	}
	return textResult(fmt.Sprintf("Updated retention: %s, reset on new activity %t.", s.EventDataRetention, s.ResetUserDataOnNewActivity)), nil, nil
}

// RunMCPServer registers tools and serves over stdio.
func RunMCPServer() {
	h := &handler{}
	server := mcp.NewServer(&mcp.Implementation{Name: "oido-ganalytics", Version: "1.1.0"}, nil)

	// Read tools.
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

	// Configuration (write) tools — require the analytics.edit scope.
	mcp.AddTool(server, &mcp.Tool{
		Name:        "ga_create_property",
		Description: "Create a new GA4 property under an account. Requires account_id, display_name, time_zone.",
	}, h.CreateProperty)
	mcp.AddTool(server, &mcp.Tool{
		Name:        "ga_update_property",
		Description: "Update a GA4 property's display name, time zone, and/or currency.",
	}, h.UpdateProperty)
	mcp.AddTool(server, &mcp.Tool{
		Name:        "ga_create_web_stream",
		Description: "Create a web data stream and return its measurement ID (G-XXXX) for gtag/GTM install.",
	}, h.CreateWebStream)
	mcp.AddTool(server, &mcp.Tool{
		Name:        "ga_list_data_streams",
		Description: "List a property's data streams with their measurement IDs.",
	}, h.ListDataStreams)
	mcp.AddTool(server, &mcp.Tool{
		Name:        "ga_delete_data_stream",
		Description: "Delete a data stream by full resource name. Destructive — requires confirm=true.",
	}, h.DeleteDataStream)
	mcp.AddTool(server, &mcp.Tool{
		Name:        "ga_create_custom_dimension",
		Description: "Create a custom dimension (EVENT or USER scope) mapping an event/user parameter.",
	}, h.CreateCustomDimension)
	mcp.AddTool(server, &mcp.Tool{
		Name:        "ga_list_custom_dimensions",
		Description: "List a property's custom dimensions.",
	}, h.ListCustomDimensions)
	mcp.AddTool(server, &mcp.Tool{
		Name:        "ga_archive_custom_dimension",
		Description: "Archive a custom dimension by full resource name. Destructive — requires confirm=true.",
	}, h.ArchiveCustomDimension)
	mcp.AddTool(server, &mcp.Tool{
		Name:        "ga_create_custom_metric",
		Description: "Create a custom metric mapping an event parameter, with a measurement unit.",
	}, h.CreateCustomMetric)
	mcp.AddTool(server, &mcp.Tool{
		Name:        "ga_list_custom_metrics",
		Description: "List a property's custom metrics.",
	}, h.ListCustomMetrics)
	mcp.AddTool(server, &mcp.Tool{
		Name:        "ga_archive_custom_metric",
		Description: "Archive a custom metric by full resource name. Destructive — requires confirm=true.",
	}, h.ArchiveCustomMetric)
	mcp.AddTool(server, &mcp.Tool{
		Name:        "ga_create_key_event",
		Description: "Mark an event name as a key event (conversion). counting_method ONCE_PER_EVENT (default) or ONCE_PER_SESSION.",
	}, h.CreateKeyEvent)
	mcp.AddTool(server, &mcp.Tool{
		Name:        "ga_list_key_events",
		Description: "List a property's key events (conversions).",
	}, h.ListKeyEvents)
	mcp.AddTool(server, &mcp.Tool{
		Name:        "ga_delete_key_event",
		Description: "Delete a key event by full resource name. Destructive — requires confirm=true.",
	}, h.DeleteKeyEvent)
	mcp.AddTool(server, &mcp.Tool{
		Name:        "ga_get_data_retention",
		Description: "Get a property's event data retention settings.",
	}, h.GetDataRetention)
	mcp.AddTool(server, &mcp.Tool{
		Name:        "ga_update_data_retention",
		Description: "Set event data retention duration (e.g. TWO_MONTHS, FOURTEEN_MONTHS) and reset-on-activity.",
	}, h.UpdateDataRetention)

	log.Println("Oido Google Analytics MCP Server starting on stdio...")
	if err := server.Run(context.Background(), &mcp.StdioTransport{}); err != nil {
		log.Fatalf("server error: %v", err)
	}
}
