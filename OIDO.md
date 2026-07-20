# Oido Google Analytics (GA4)

Read-only access to Google Analytics 4 via the Analytics Data & Admin APIs, using your own Google OAuth connection.

## Tools

- `ga_list_properties` — list GA4 properties you can access. **Start here** to get a `property_id`.
- `ga_run_report` — core report over a date range. Args: `property_id`, `start_date`, `end_date`, `dimensions`, `metrics`, `limit`. Dimensions/metrics are comma-separated GA4 API names. Defaults: last 7 days, `activeUsers,sessions`.
- `ga_realtime_report` — activity in the last 30 minutes. Default metric `activeUsers`.
- `ga_get_metadata` — valid dimension/metric API names for a property. Use when unsure what to pass to `ga_run_report`.

## Dates

GA4 syntax: `YYYY-MM-DD`, `NdaysAgo` (e.g. `28daysAgo`), `yesterday`, `today`.

## Common names

- Metrics: `activeUsers`, `sessions`, `screenPageViews`, `engagementRate`, `bounceRate`, `conversions`, `totalRevenue`.
- Dimensions: `date`, `country`, `city`, `deviceCategory`, `sessionSource`, `sessionMedium`, `pagePath`, `landingPage`.

Call `ga_get_metadata` for the full list (including your custom dimensions/metrics).

## Setup

Requires a Google Cloud OAuth client (ID + secret) with the Analytics Data API and Analytics Admin API enabled, then "Connect with Google" in the extension settings. Scope is `analytics.readonly` — this plugin never writes.
