# Oido Google Analytics (GA4)

Read and configure Google Analytics 4 via the Analytics Data & Admin APIs, using your own Google OAuth connection.

## Read tools

- `ga_list_properties` — list GA4 properties you can access. **Start here** to get a `property_id`.
- `ga_run_report` — core report over a date range. Args: `property_id`, `start_date`, `end_date`, `dimensions`, `metrics`, `limit`. Dimensions/metrics are comma-separated GA4 API names. Defaults: last 7 days, `activeUsers,sessions`.
- `ga_realtime_report` — activity in the last 30 minutes. Default metric `activeUsers`.
- `ga_get_metadata` — valid dimension/metric API names for a property. Use when unsure what to pass to `ga_run_report`.

## Configuration tools (write — need `analytics.edit`)

- `ga_create_property` / `ga_update_property` — create a property (needs `account_id`, `display_name`, `time_zone`) or patch name/tz/currency.
- `ga_create_web_stream` — create a web data stream; returns the **measurement ID** (`G-XXXX`) for gtag/GTM.
- `ga_list_data_streams` / `ga_delete_data_stream` — list streams (with measurement IDs) / delete one.
- `ga_create_custom_dimension` / `ga_list_custom_dimensions` / `ga_archive_custom_dimension` — EVENT or USER scope, mapped to an event/user parameter.
- `ga_create_custom_metric` / `ga_list_custom_metrics` / `ga_archive_custom_metric` — with a measurement unit (STANDARD, CURRENCY, …).
- `ga_create_key_event` / `ga_list_key_events` / `ga_delete_key_event` — mark event names as conversions.
- `ga_get_data_retention` / `ga_update_data_retention` — event data retention (`TWO_MONTHS`, `FOURTEEN_MONTHS`, …) + reset-on-activity.

**Destructive** tools (`ga_delete_*`, `ga_archive_*`) require `confirm: true` — without it they only report what would happen. GA4 archives (not deletes) custom dimensions/metrics.

## Dates

GA4 syntax: `YYYY-MM-DD`, `NdaysAgo` (e.g. `28daysAgo`), `yesterday`, `today`.

## Common names

- Metrics: `activeUsers`, `sessions`, `screenPageViews`, `engagementRate`, `bounceRate`, `conversions`, `totalRevenue`.
- Dimensions: `date`, `country`, `city`, `deviceCategory`, `sessionSource`, `sessionMedium`, `pagePath`, `landingPage`.

Call `ga_get_metadata` for the full list (including your custom dimensions/metrics).

## Setup

Requires a Google Cloud OAuth client (ID + secret) with the Analytics Data API and Analytics Admin API enabled, then "Connect with Google" in the extension settings. Scopes are `analytics.readonly` + `analytics.edit`. Adding the edit scope means **existing connections must reconnect** (re-consent) before configuration tools work.
