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
