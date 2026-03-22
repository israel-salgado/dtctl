---
layout: docs
title: "DQL Queries"
---

dtctl provides a powerful interface for executing Dynatrace Query Language (DQL) queries directly from your terminal. Run ad-hoc queries inline, load them from files, use template variables, and stream live results.

## Simple Inline Queries

Pass a DQL query string directly as an argument:

```bash
dtctl query "fetch logs | limit 10"

dtctl query "fetch spans | limit 10"

dtctl query "fetch events | limit 10"

dtctl query "timeseries avg(dt.host.cpu.usage)"
```

## File-Based Queries

Store complex queries in `.dql` files and execute them with `-f`:

```bash
dtctl query -f queries/errors.dql
```

This keeps queries version-controlled and avoids shell-escaping issues.

## Stdin Input

Pipe queries or use heredocs to avoid shell quoting problems entirely:

```bash
# Heredoc
dtctl query <<'EOF'
fetch logs
| filter loglevel == "ERROR"
| summarize count = count(), by: {dt.entity.service}
| sort count desc
| limit 20
EOF

# Pipe from a file
cat queries/errors.dql | dtctl query

# Pipe from another command
echo 'fetch logs | limit 5' | dtctl query
```

### PowerShell Quoting

On Windows PowerShell, use here-strings to avoid escaping issues:

```powershell
# PowerShell here-string
dtctl query @'
fetch logs
| filter loglevel == "ERROR"
| limit 10
'@
```

## Template Queries

Use Go template syntax with `--set` to parameterize queries:

```bash
dtctl query "fetch logs | filter environment == '{{ .env }}' | limit {{ .n }}" \
  --set env=production --set n=50
```

Template variables work with both inline queries and file-based queries:

```bash
dtctl query -f queries/service-errors.dql --set service=checkout --set hours=24
```

## Output Formats

Control how results are displayed:

```bash
# Default table output
dtctl query "fetch logs | limit 10"

# JSON (for scripting and piping to jq)
dtctl query "fetch logs | limit 10" -o json

# YAML
dtctl query "fetch logs | limit 10" -o yaml

# CSV (for spreadsheets and data tools)
dtctl query "fetch logs | limit 10" -o csv
```

## Large Dataset Downloads

Control result size limits for bulk data extraction:

```bash
# Increase max records returned (default varies by query type)
dtctl query "fetch logs" --max-result-records 100000

# Increase max result payload size
dtctl query "fetch logs" --max-result-bytes 52428800

# Control how much data Grail scans (in GB)
dtctl query "fetch logs" --default-scan-limit-gbytes 500
```

## Additional Parameters

Fine-tune query execution with these options:

```bash
# Specify a time frame
dtctl query "fetch logs | limit 10" --timeframe "now()-2h"

# Set timezone and locale
dtctl query "fetch logs | limit 10" --timezone "America/New_York" --locale "en_US"

# Enable sampling for faster results on large datasets
dtctl query "fetch logs" --sampling-ratio 0.1

# Fetch execution metadata alongside results
dtctl query "fetch logs | limit 10" --metadata

# Preview mode (faster, approximate results)
dtctl query "fetch logs | limit 10" --preview
```

## Live Mode

Stream query results at a regular interval:

```bash
# Re-run every 5 seconds
dtctl query "fetch logs | filter loglevel == 'ERROR' | sort timestamp desc | limit 10" \
  --live --interval 5s
```

Press `Ctrl+C` to stop live mode.

## Query Warnings

DQL may emit warnings (e.g., result truncation, deprecated syntax). These are printed to **stderr** so they don't interfere with piped output:

```bash
# Warnings appear on stderr, results on stdout
dtctl query "fetch logs" -o json > results.json
# Any warnings are still visible in the terminal
```

## Query Verification

Validate DQL queries without executing them — useful for CI/CD pipelines and pre-commit hooks.

```bash
# Verify a query is syntactically valid
dtctl verify query "fetch logs | limit 10"

# Verify from a file
dtctl verify query -f queries/errors.dql

# Return the canonical (normalized) form of the query
dtctl verify query "fetch logs | limit 10" --canonical

# Treat warnings as errors (non-zero exit code)
dtctl verify query -f queries/errors.dql --fail-on-warn
```

### Exit Codes

| Code | Meaning |
|------|---------|
| `0`  | Query is valid |
| `1`  | Query has syntax or semantic errors |
| `2`  | Query is valid but has warnings (with `--fail-on-warn`) |

### CI/CD Integration

```bash
# In a CI pipeline — verify all .dql files before deploying
for f in queries/*.dql; do
  dtctl verify query -f "$f" --fail-on-warn || exit 1
done
```
