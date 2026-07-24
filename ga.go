package main

import (
	"context"
	"fmt"
	"os"
	"strings"

	"golang.org/x/oauth2"
	admin "google.golang.org/api/analyticsadmin/v1alpha"
	data "google.golang.org/api/analyticsdata/v1beta"
	"google.golang.org/api/option"
)

// tokenSource builds an OAuth token source from the token oido-core injects.
func tokenSource() (option.ClientOption, error) {
	tok := os.Getenv("GOOGLE_ACCESS_TOKEN")
	if tok == "" {
		return nil, fmt.Errorf("not connected: GOOGLE_ACCESS_TOKEN is empty — open the extension settings and click Connect with Google")
	}
	return option.WithTokenSource(oauth2.StaticTokenSource(&oauth2.Token{AccessToken: tok})), nil
}

func dataService(ctx context.Context) (*data.Service, error) {
	opt, err := tokenSource()
	if err != nil {
		return nil, err
	}
	return data.NewService(ctx, opt)
}

func adminService(ctx context.Context) (*admin.Service, error) {
	opt, err := tokenSource()
	if err != nil {
		return nil, err
	}
	return admin.NewService(ctx, opt)
}

// property normalizes a bare ID ("123456") or a full name ("properties/123456").
func property(id string) string {
	id = strings.TrimSpace(id)
	if id == "" {
		return ""
	}
	if strings.HasPrefix(id, "properties/") {
		return id
	}
	return "properties/" + id
}

// splitList turns "a, b ,c" into ["a","b","c"], dropping empties.
func splitList(s string) []string {
	var out []string
	for _, p := range strings.Split(s, ",") {
		if p = strings.TrimSpace(p); p != "" {
			out = append(out, p)
		}
	}
	return out
}

// ListProperties returns the GA4 properties the connected account can access,
// grouped under their account, via the Admin API accountSummaries endpoint.
func ListProperties(ctx context.Context) ([]*admin.GoogleAnalyticsAdminV1alphaAccountSummary, error) {
	svc, err := adminService(ctx)
	if err != nil {
		return nil, err
	}
	res, err := svc.AccountSummaries.List().Do()
	if err != nil {
		return nil, err
	}
	return res.AccountSummaries, nil
}

// RunReport runs a core GA4 report. dimensions/metrics are comma-separated API
// names (e.g. "date,country" / "activeUsers,sessions"). Dates accept GA4 syntax
// like "2026-07-01", "7daysAgo", "today", "yesterday".
func RunReport(ctx context.Context, propertyID, startDate, endDate, dimensions, metrics string, limit int64) (*data.RunReportResponse, error) {
	svc, err := dataService(ctx)
	if err != nil {
		return nil, err
	}
	if startDate == "" {
		startDate = "7daysAgo"
	}
	if endDate == "" {
		endDate = "today"
	}
	if metrics == "" {
		metrics = "activeUsers,sessions"
	}
	if limit <= 0 {
		limit = 50
	}

	req := &data.RunReportRequest{
		DateRanges: []*data.DateRange{{StartDate: startDate, EndDate: endDate}},
		Limit:      limit,
	}
	for _, d := range splitList(dimensions) {
		req.Dimensions = append(req.Dimensions, &data.Dimension{Name: d})
	}
	for _, m := range splitList(metrics) {
		req.Metrics = append(req.Metrics, &data.Metric{Name: m})
	}
	return svc.Properties.RunReport(property(propertyID), req).Do()
}

// RunRealtimeReport reports on activity in the last 30 minutes.
func RunRealtimeReport(ctx context.Context, propertyID, dimensions, metrics string, limit int64) (*data.RunRealtimeReportResponse, error) {
	svc, err := dataService(ctx)
	if err != nil {
		return nil, err
	}
	if metrics == "" {
		metrics = "activeUsers"
	}
	if limit <= 0 {
		limit = 50
	}

	req := &data.RunRealtimeReportRequest{Limit: limit}
	for _, d := range splitList(dimensions) {
		req.Dimensions = append(req.Dimensions, &data.Dimension{Name: d})
	}
	for _, m := range splitList(metrics) {
		req.Metrics = append(req.Metrics, &data.Metric{Name: m})
	}
	return svc.Properties.RunRealtimeReport(property(propertyID), req).Do()
}

// GetMetadata lists the dimension and metric API names available for a property.
func GetMetadata(ctx context.Context, propertyID string) (*data.Metadata, error) {
	svc, err := dataService(ctx)
	if err != nil {
		return nil, err
	}
	return svc.Properties.GetMetadata(property(propertyID) + "/metadata").Do()
}

// ---------------------------------------------------------------------------
// Configuration (write) operations — require the analytics.edit OAuth scope.
// ---------------------------------------------------------------------------

// account normalizes a bare ID ("123") or full name ("accounts/123").
func account(id string) string {
	id = strings.TrimSpace(id)
	if id == "" || strings.HasPrefix(id, "accounts/") {
		return id
	}
	return "accounts/" + id
}

// CreateProperty creates a GA4 property under an account. timeZone (e.g.
// "America/New_York") is required by the API; currency defaults to USD.
func CreateProperty(ctx context.Context, accountID, displayName, timeZone, currency, industry string) (*admin.GoogleAnalyticsAdminV1alphaProperty, error) {
	svc, err := adminService(ctx)
	if err != nil {
		return nil, err
	}
	if currency == "" {
		currency = "USD"
	}
	p := &admin.GoogleAnalyticsAdminV1alphaProperty{
		Parent:           account(accountID),
		DisplayName:      displayName,
		TimeZone:         timeZone,
		CurrencyCode:     currency,
		IndustryCategory: industry,
	}
	return svc.Properties.Create(p).Do()
}

// UpdateProperty patches display name / time zone / currency. Empty fields are
// left unchanged (only non-empty fields go into the update mask).
func UpdateProperty(ctx context.Context, propertyID, displayName, timeZone, currency string) (*admin.GoogleAnalyticsAdminV1alphaProperty, error) {
	svc, err := adminService(ctx)
	if err != nil {
		return nil, err
	}
	p := &admin.GoogleAnalyticsAdminV1alphaProperty{}
	var mask []string
	if displayName != "" {
		p.DisplayName = displayName
		mask = append(mask, "displayName")
	}
	if timeZone != "" {
		p.TimeZone = timeZone
		mask = append(mask, "timeZone")
	}
	if currency != "" {
		p.CurrencyCode = currency
		mask = append(mask, "currencyCode")
	}
	if len(mask) == 0 {
		return nil, fmt.Errorf("nothing to update: set at least one of display_name, time_zone, currency")
	}
	return svc.Properties.Patch(property(propertyID), p).UpdateMask(strings.Join(mask, ",")).Do()
}

// CreateWebDataStream creates a web data stream and returns it, including the
// measurement ID (G-XXXXXXX) needed to install gtag/GTM.
func CreateWebDataStream(ctx context.Context, propertyID, displayName, defaultURI string) (*admin.GoogleAnalyticsAdminV1alphaDataStream, error) {
	svc, err := adminService(ctx)
	if err != nil {
		return nil, err
	}
	ds := &admin.GoogleAnalyticsAdminV1alphaDataStream{
		DisplayName:   displayName,
		Type:          "WEB_DATA_STREAM",
		WebStreamData: &admin.GoogleAnalyticsAdminV1alphaDataStreamWebStreamData{DefaultUri: defaultURI},
	}
	return svc.Properties.DataStreams.Create(property(propertyID), ds).Do()
}

// ListDataStreams lists the data streams under a property.
func ListDataStreams(ctx context.Context, propertyID string) ([]*admin.GoogleAnalyticsAdminV1alphaDataStream, error) {
	svc, err := adminService(ctx)
	if err != nil {
		return nil, err
	}
	res, err := svc.Properties.DataStreams.List(property(propertyID)).Do()
	if err != nil {
		return nil, err
	}
	return res.DataStreams, nil
}

// DeleteDataStream deletes a data stream by full resource name
// ("properties/123/dataStreams/456"). Destructive.
func DeleteDataStream(ctx context.Context, streamName string) error {
	svc, err := adminService(ctx)
	if err != nil {
		return err
	}
	_, err = svc.Properties.DataStreams.Delete(strings.TrimSpace(streamName)).Do()
	return err
}

// CreateCustomDimension creates an event- or user-scoped custom dimension.
// scope is "EVENT" (default) or "USER".
func CreateCustomDimension(ctx context.Context, propertyID, displayName, parameterName, scope, description string) (*admin.GoogleAnalyticsAdminV1alphaCustomDimension, error) {
	svc, err := adminService(ctx)
	if err != nil {
		return nil, err
	}
	if scope == "" {
		scope = "EVENT"
	}
	cd := &admin.GoogleAnalyticsAdminV1alphaCustomDimension{
		DisplayName:   displayName,
		ParameterName: parameterName,
		Scope:         scope,
		Description:   description,
	}
	return svc.Properties.CustomDimensions.Create(property(propertyID), cd).Do()
}

// ListCustomDimensions lists a property's custom dimensions.
func ListCustomDimensions(ctx context.Context, propertyID string) ([]*admin.GoogleAnalyticsAdminV1alphaCustomDimension, error) {
	svc, err := adminService(ctx)
	if err != nil {
		return nil, err
	}
	res, err := svc.Properties.CustomDimensions.List(property(propertyID)).Do()
	if err != nil {
		return nil, err
	}
	return res.CustomDimensions, nil
}

// ArchiveCustomDimension archives a custom dimension by full resource name
// ("properties/123/customDimensions/456"). Destructive (GA4 has no delete).
func ArchiveCustomDimension(ctx context.Context, dimensionName string) error {
	svc, err := adminService(ctx)
	if err != nil {
		return err
	}
	_, err = svc.Properties.CustomDimensions.Archive(strings.TrimSpace(dimensionName), &admin.GoogleAnalyticsAdminV1alphaArchiveCustomDimensionRequest{}).Do()
	return err
}

// CreateCustomMetric creates a custom metric. scope defaults to "EVENT";
// measurementUnit defaults to "STANDARD" (see API for CURRENCY, FEET, etc.).
func CreateCustomMetric(ctx context.Context, propertyID, displayName, parameterName, scope, measurementUnit, description string) (*admin.GoogleAnalyticsAdminV1alphaCustomMetric, error) {
	svc, err := adminService(ctx)
	if err != nil {
		return nil, err
	}
	if scope == "" {
		scope = "EVENT"
	}
	if measurementUnit == "" {
		measurementUnit = "STANDARD"
	}
	cm := &admin.GoogleAnalyticsAdminV1alphaCustomMetric{
		DisplayName:     displayName,
		ParameterName:   parameterName,
		Scope:           scope,
		MeasurementUnit: measurementUnit,
		Description:     description,
	}
	return svc.Properties.CustomMetrics.Create(property(propertyID), cm).Do()
}

// ListCustomMetrics lists a property's custom metrics.
func ListCustomMetrics(ctx context.Context, propertyID string) ([]*admin.GoogleAnalyticsAdminV1alphaCustomMetric, error) {
	svc, err := adminService(ctx)
	if err != nil {
		return nil, err
	}
	res, err := svc.Properties.CustomMetrics.List(property(propertyID)).Do()
	if err != nil {
		return nil, err
	}
	return res.CustomMetrics, nil
}

// ArchiveCustomMetric archives a custom metric by full resource name. Destructive.
func ArchiveCustomMetric(ctx context.Context, metricName string) error {
	svc, err := adminService(ctx)
	if err != nil {
		return err
	}
	_, err = svc.Properties.CustomMetrics.Archive(strings.TrimSpace(metricName), &admin.GoogleAnalyticsAdminV1alphaArchiveCustomMetricRequest{}).Do()
	return err
}

// CreateKeyEvent marks an event name as a key event (conversion).
// countingMethod is "ONCE_PER_EVENT" (default) or "ONCE_PER_SESSION".
func CreateKeyEvent(ctx context.Context, propertyID, eventName, countingMethod string) (*admin.GoogleAnalyticsAdminV1alphaKeyEvent, error) {
	svc, err := adminService(ctx)
	if err != nil {
		return nil, err
	}
	if countingMethod == "" {
		countingMethod = "ONCE_PER_EVENT"
	}
	ke := &admin.GoogleAnalyticsAdminV1alphaKeyEvent{
		EventName:      eventName,
		CountingMethod: countingMethod,
	}
	return svc.Properties.KeyEvents.Create(property(propertyID), ke).Do()
}

// ListKeyEvents lists a property's key events.
func ListKeyEvents(ctx context.Context, propertyID string) ([]*admin.GoogleAnalyticsAdminV1alphaKeyEvent, error) {
	svc, err := adminService(ctx)
	if err != nil {
		return nil, err
	}
	res, err := svc.Properties.KeyEvents.List(property(propertyID)).Do()
	if err != nil {
		return nil, err
	}
	return res.KeyEvents, nil
}

// DeleteKeyEvent deletes a key event by full resource name
// ("properties/123/keyEvents/456"). Destructive.
func DeleteKeyEvent(ctx context.Context, keyEventName string) error {
	svc, err := adminService(ctx)
	if err != nil {
		return err
	}
	_, err = svc.Properties.KeyEvents.Delete(strings.TrimSpace(keyEventName)).Do()
	return err
}

// GetDataRetention returns a property's data retention settings.
func GetDataRetention(ctx context.Context, propertyID string) (*admin.GoogleAnalyticsAdminV1alphaDataRetentionSettings, error) {
	svc, err := adminService(ctx)
	if err != nil {
		return nil, err
	}
	return svc.Properties.GetDataRetentionSettings(property(propertyID) + "/dataRetentionSettings").Do()
}

// UpdateDataRetention sets event data retention duration and whether user data
// resets on new activity. eventDataRetention is a GA4 enum, e.g.
// "TWO_MONTHS", "FOURTEEN_MONTHS" (360 available on GA360).
func UpdateDataRetention(ctx context.Context, propertyID, eventDataRetention string, resetOnNewActivity bool) (*admin.GoogleAnalyticsAdminV1alphaDataRetentionSettings, error) {
	svc, err := adminService(ctx)
	if err != nil {
		return nil, err
	}
	s := &admin.GoogleAnalyticsAdminV1alphaDataRetentionSettings{
		ResetUserDataOnNewActivity: resetOnNewActivity,
		// resetUserDataOnNewActivity must be sent even when false, else it reads as "unset".
		ForceSendFields: []string{"ResetUserDataOnNewActivity"},
	}
	mask := []string{"resetUserDataOnNewActivity"}
	if eventDataRetention != "" {
		s.EventDataRetention = eventDataRetention
		mask = append(mask, "eventDataRetention")
	}
	name := property(propertyID) + "/dataRetentionSettings"
	return svc.Properties.UpdateDataRetentionSettings(name, s).UpdateMask(strings.Join(mask, ",")).Do()
}
