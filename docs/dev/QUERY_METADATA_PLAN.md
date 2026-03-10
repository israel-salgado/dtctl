# Query Metadata (`--metadata`) — Planning Document

**Status:** Implemented (Phases 1–4 complete)

## Problem Statement

`dtctl query` currently discards all DQL query metadata (execution time, scanned
records/bytes, query ID, analysis timeframe, contributions, etc.).  The only way
to see it today is `--debug`, which dumps the raw HTTP response body — unstructured
and noisy.

We need a `--metadata` (`-M`) flag on the `query` subcommand that surfaces this
metadata alongside the result records.  Because dtctl supports multiple output
formats, each format needs an appropriate strategy for separating **records** from
**metadata**.

---

## Scope

### In scope

| Item | Notes |
|------|-------|
| New `--metadata` / `-M` flag on `dtctl query` | String flag with `NoOptDefVal="all"`. Bare `-M` means all fields; `-M=field1,field2` selects specific fields; empty string means disabled |
| Phase 2 field selection | `--metadata=executionTimeMilliseconds,scannedRecords` selects only those fields |
| Field validation | `ParseMetadataFields()` rejects unknown/misspelled field names |
| Extend `QueryMetadata` struct | All 13 Grail metadata fields including `Contributions` |
| Adapt output for **table**, **wide**, **json**, **yaml**, **csv** | See format details below |
| Agent mode auto-enable | `--metadata` is implied (`"all"`) when `--agent`/`-A` is active |
| Zero-value preservation | Explicitly selected fields (e.g. `sampled=false`, `scannedDataPoints=0`) are included even when zero |
| Golden tests | 8 new golden files for metadata variants (JSON/YAML/table/CSV × all/filtered) |

### Out of scope

- Chart/sparkline/barchart/braille formats (metadata display doesn't add value
  to visualisations — if `--metadata` is passed with a chart format, print a
  stderr note and ignore).
- Live mode (`--live`) metadata — live mode warns on stderr about incompatible
  flags (`--metadata`, `--agent`, `--include-contributions`, `--dry-run`) and
  continues without them.

---

## Current Data Flow

```
cmd/query.go  →  DQLExecutor.ExecuteWithOptions()  →  printResults()
                                                          │
                                                          ├─ extracts records
                                                          ├─ extracts metadata (when --metadata is set)
                                                          ├─ creates Printer
                                                          └─ dispatches by format
```

`printResults()` (pkg/exec/dql.go):
1. Prints notifications to stderr.
2. Extracts `records` from the response.
3. Extracts metadata via `extractQueryMetadata()` when `opts.MetadataFields` is non-empty.
4. Passes records (and optionally metadata) to the selected printer/formatter.

---

## Metadata Shape (from Grail API)

```json
{
  "metadata": {
    "grail": {
      "canonicalQuery": "fetch logs\n| limit 3\n| fields timestamp",
      "timezone": "Z",
      "query": "fetch logs | limit 3 | fields timestamp",
      "scannedRecords": 42351,
      "dqlVersion": "V1_0",
      "scannedBytes": 2982690,
      "scannedDataPoints": 0,
      "analysisTimeframe": {
        "start": "2026-03-09T10:16:39.973805659Z",
        "end": "2026-03-09T12:16:39.973805659Z"
      },
      "locale": "und",
      "executionTimeMilliseconds": 47,
      "notifications": [],
      "queryId": "27c4daf9-2619-4ba1-b1ad-9e276c75a351",
      "sampled": false,
      "contributions": {
        "buckets": [
          {
            "name": "custom_sen_low_logs_platform_service_shared",
            "table": "logs",
            "scannedBytes": 2982690,
            "matchedRecordsRatio": 1.0
          }
        ]
      }
    }
  }
}
```

### Metadata fields

Field names use JSON struct tag names for `--metadata=field1,field2` selection:

| Field | Human-friendly label (table/wide) |
|-------|----------------------------------|
| `executionTimeMilliseconds` | Execution time |
| `scannedRecords` | Scanned records |
| `scannedBytes` | Scanned bytes |
| `scannedDataPoints` | Scanned data points |
| `sampled` | Sampled |
| `queryId` | Query ID |
| `dqlVersion` | DQL version |
| `analysisTimeframe` | Analysis timeframe |
| `canonicalQuery` | Canonical query |
| `query` | Query |
| `timezone` | Timezone |
| `locale` | Locale |
| `contributions` | Contributions |

### Note on wide vs. table for DQL queries

For DQL query results (`[]map[string]interface{}`), the `wide` flag has **no
effect** on the records table — `printMaps()` in `table.go` always shows all
map keys regardless of the `wide` field. The `wide` distinction only applies
to struct-based resources via `table:"HEADER,wide"` tags.

Therefore, the metadata footer is **identical** for both `table` and `wide`
output when used with `dtctl query`. Both show the same metadata fields.

---

## Format-Specific Implementation

### 1. JSON (`-o json --metadata`)

**Strategy:** Add `metadata` as a sibling key to `records`.

```json
{
  "records": [
    {"timestamp": "2026-03-09T12:15:00Z"},
    {"timestamp": "2026-03-09T12:14:00Z"}
  ],
  "metadata": {
    "executionTimeMilliseconds": 47,
    "scannedRecords": 42351,
    "scannedBytes": 2982690,
    "queryId": "27c4daf9-2619-4ba1-b1ad-9e276c75a351",
    "analysisTimeframe": {
      "start": "2026-03-09T10:16:39.973805659Z",
      "end": "2026-03-09T12:16:39.973805659Z"
    }
  }
}
```

Without `--metadata` (current behavior):
```json
{
  "records": [
    {"timestamp": "2026-03-09T12:15:00Z"}
  ]
}
```

**Implementation:** `printResults()` uses `MetadataToMap(meta, fields)` to build
the metadata value, then passes `map[string]interface{}{"records": records, "metadata": metaValue}`
to `JSONPrinter.Print()`.

**Zero-value handling:** When specific fields are selected, `MetadataToMap()`
returns a `map[string]interface{}` that always includes the requested fields
even when their values are zero (`false`, `0`, `""`). When all fields are
requested (`"all"`), it returns the struct pointer directly, letting
`json:",omitempty"` tags suppress zero values for cleaner output.

---

### 2. YAML (`-o yaml --metadata`)

**Strategy:** Same structural approach as JSON. `metadata` is a top-level key
beside `records`.

```yaml
records:
  - timestamp: "2026-03-09T12:15:00Z"
  - timestamp: "2026-03-09T12:14:00Z"
metadata:
  executionTimeMilliseconds: 47
  scannedRecords: 42351
  scannedBytes: 2982690
  queryId: 27c4daf9-2619-4ba1-b1ad-9e276c75a351
  analysisTimeframe:
    start: "2026-03-09T10:16:39.973805659Z"
    end: "2026-03-09T12:16:39.973805659Z"
```

**Implementation:** Same as JSON — uses `MetadataToMap()` and passes the result
to `YAMLPrinter.Print()`.

**Note:** `QueryMetadata` struct has both `json:"...,omitempty"` and
`yaml:"...,omitempty"` tags for consistent zero-value behavior across formats.

---

### 3. Table / Wide (`-o table --metadata`, `-o wide --metadata`) — default format

**Strategy:** Print the normal records table to stdout, then print a
metadata summary **below the table** as a styled footer block, separated by
an empty line.

```
TIMESTAMP
2026-03-09T12:15:00Z
2026-03-09T12:14:00Z

--- Query Metadata ---
Execution time:     47ms
Scanned records:    42,351
Scanned bytes:      2.8 MB
Scanned data pts:   0
Analysis window:    2026-03-09T10:16:39Z → 2026-03-09T12:16:39Z
Query ID:           27c4daf9-2619-4ba1-b1ad-9e276c75a351
DQL version:        V1_0
Canonical query:    fetch logs | limit 3 | fields timestamp
Query:              fetch logs | limit 3 | fields timestamp
Timezone:           Z
Locale:             und
Sampled:            no
Contributions:
  custom_sen_low_logs_platform_service_shared (logs)
    scanned: 2.8 MB, matched: 100.0%
```

**Implementation:** `FormatMetadataFooter()` in `pkg/output/metadata.go`.
Uses `hasField()` internally to skip lines for fields not in the selection
when using `--metadata=field1,field2`.

**Design choices:**
- Footer goes to **stdout** (not stderr) so it can be captured alongside the table.
- Human-friendly formatting: bytes → "2.8 MB", milliseconds → "47ms", booleans →
  "yes"/"no", numbers → comma-separated thousands.
- The `--- Query Metadata ---` header uses ANSI bold when color is enabled.

---

### 4. CSV (`-o csv --metadata`)

**Strategy:** Write metadata as comment lines (`#`-prefixed) **above** the
CSV data.

```csv
# Query Metadata
# execution_time_ms: 47
# scanned_records: 42351
# scanned_bytes: 2982690
# scanned_data_points: 0
# analysis_start: 2026-03-09T10:16:39.973805659Z
# analysis_end: 2026-03-09T12:16:39.973805659Z
# query_id: 27c4daf9-2619-4ba1-b1ad-9e276c75a351
# dql_version: V1_0
# canonical_query: fetch logs | limit 3 | fields timestamp
# query: fetch logs | limit 3 | fields timestamp
# timezone: Z
# locale: und
# sampled: false
# contributions: custom_sen_low_logs_platform_service_shared (logs, 2982690 bytes, 100.0% matched)
timestamp
2026-03-09T12:15:00Z
2026-03-09T12:14:00Z
```

**Implementation:** `FormatMetadataCSVComments()` in `pkg/output/metadata.go`.
Uses `hasField()` internally for field selection filtering.

Single-stream output; `grep -v '^#'` strips metadata for strict RFC 4180 parsers.

---

### 5. Agent Envelope (`--agent`)

**How it actually works:** In agent mode, `--metadata` is automatically set to
`"all"` (unless explicitly overridden). The DQL executor creates a `JSONPrinter`
(because agent mode implies `--plain` which forces JSON output), and prints
`{"records": [...], "metadata": {...}}` directly as the result payload.

The agent envelope wraps this as:

```json
{
  "ok": true,
  "result": {
    "records": [...],
    "metadata": {
      "executionTimeMilliseconds": 47,
      "scannedRecords": 42351,
      ...
    }
  },
  "context": {
    "verb": "query",
    "resource": "dql",
    "suggestions": [...]
  }
}
```

**Key architectural note:** Metadata lives inside the `result` payload, **not**
in the `context` envelope. The `ResponseContext` struct does not have a
`QueryMetadata` field — this was considered but removed as dead code because
the DQL executor bypasses the agent envelope's context enrichment entirely.

---

### 6. Chart Formats (sparkline, barchart, braille)

**Strategy:** Ignore `--metadata` for chart formats. If both are specified,
print a short notice to stderr: `Note: --metadata is not supported with chart
output formats` and proceed with the chart.

---

## Struct & Function Summary

### `QueryMetadata` struct (pkg/output/metadata.go)

```go
type QueryMetadata struct {
    ExecutionTimeMilliseconds int64              `json:"executionTimeMilliseconds,omitempty" yaml:"executionTimeMilliseconds,omitempty"`
    ScannedRecords            int64              `json:"scannedRecords,omitempty"            yaml:"scannedRecords,omitempty"`
    ScannedBytes              int64              `json:"scannedBytes,omitempty"              yaml:"scannedBytes,omitempty"`
    ScannedDataPoints         int64              `json:"scannedDataPoints,omitempty"         yaml:"scannedDataPoints,omitempty"`
    Sampled                   bool               `json:"sampled,omitempty"                   yaml:"sampled,omitempty"`
    QueryID                   string             `json:"queryId,omitempty"                   yaml:"queryId,omitempty"`
    DQLVersion                string             `json:"dqlVersion,omitempty"                yaml:"dqlVersion,omitempty"`
    Query                     string             `json:"query,omitempty"                     yaml:"query,omitempty"`
    CanonicalQuery            string             `json:"canonicalQuery,omitempty"             yaml:"canonicalQuery,omitempty"`
    Timezone                  string             `json:"timezone,omitempty"                   yaml:"timezone,omitempty"`
    Locale                    string             `json:"locale,omitempty"                     yaml:"locale,omitempty"`
    AnalysisTimeframe         *AnalysisTimeframe `json:"analysisTimeframe,omitempty"          yaml:"analysisTimeframe,omitempty"`
    Contributions             *Contributions     `json:"contributions,omitempty"              yaml:"contributions,omitempty"`
}
```

### `MetadataFields` in `DQLExecuteOptions` (pkg/exec/dql.go)

```go
type DQLExecuteOptions struct {
    // ... existing fields ...
    MetadataFields []string  // parsed field names, or ["all"] for all fields
}
```

### Key functions in `pkg/output/metadata.go`

| Function | Purpose |
|----------|---------|
| `ParseMetadataFields(input string) ([]string, error)` | Parses comma-separated field names, validates against known fields, returns error with suggestions for unknown fields (single-line error format with semicolon-space separator) |
| `MetadataToMap(meta *QueryMetadata, fields []string) interface{}` | Returns a `map[string]interface{}` for explicit field selection (preserves zero values) or the struct pointer for `"all"` (lets `omitempty` clean up zeros) |
| `FormatMetadataFooter(meta *QueryMetadata, fields []string) string` | Formats human-friendly footer for table/wide output. Uses `hasField()` internally to filter by selected fields |
| `FormatMetadataCSVComments(meta *QueryMetadata, fields []string) string` | Formats `#`-prefixed comment lines for CSV output. Uses `hasField()` internally to filter by selected fields |
| `FilterMetadata(meta *QueryMetadata, fields []string) *QueryMetadata` | Creates a copy with only selected fields populated. Exists as a utility but is **not called from the main code path** — `MetadataToMap()` handles JSON/YAML, and footer/CSV functions handle their own filtering |

### Removed code

| Item | Reason |
|------|--------|
| `ResponseContext.QueryMetadata` field | Agent mode uses `JSONPrinter` directly; metadata goes into the result payload, not the context envelope |
| `SetQueryMetadata()` method on `AgentPrinter` | Same reason — dead code that was never reached |

---

## Implementation Phases (all complete)

### Phase 1: Core feature ✅
1. Added `Contributions`, `BucketContribution`, `AnalysisTimeframe` structs.
2. Created `QueryMetadata` struct with all 13 fields in `pkg/output/metadata.go`.
3. Added `MetadataFields []string` to `DQLExecuteOptions`.
4. Added `--metadata` / `-M` string flag to `cmd/query.go` with `NoOptDefVal="all"`.
5. Wired flag value through `ParseMetadataFields()` into `DQLExecuteOptions.MetadataFields`.
6. Implemented `extractQueryMetadata()` in `pkg/exec/dql.go`.
7. Updated `printResults()` to include metadata in all output formats.
8. Created formatting helpers: `FormatMetadataFooter()`, `FormatMetadataCSVComments()`.
9. Agent mode auto-enables `--metadata` to `"all"`.

### Phase 2: Field selection ✅
1. Changed `--metadata` from boolean to string flag with `NoOptDefVal="all"`.
2. Implemented `ParseMetadataFields()` with validation against known field names.
3. Added `hasField()` helper for filtering in footer/CSV formatters.
4. Added `FilterMetadata()` utility function.

### Phase 3: Quality pass ✅
1. Fixed omitempty bug — added `MetadataToMap()` for zero-value preservation.
2. Removed dead code — `SetQueryMetadata()` and `ResponseContext.QueryMetadata`.
3. Fixed error message format — `\n` → `"; "` per Go convention.
4. Added YAML struct tags (`yaml:"...,omitempty"`) alongside JSON tags.
5. Removed redundant `FilterMetadata()` call from `printResults()`.

### Phase 4: Tests ✅
1. Unit tests for all formatting helpers.
2. Unit tests for `MetadataToMap()` — nil, all, empty, zero-value preservation (13 subtests).
3. End-to-end JSON and YAML printer tests with zero values.
4. Nil metadata path test in `dql_test.go`.
5. Edge case tests — misspelled fields, mixed valid/invalid, empty values, only commas.
6. Golden tests — 8 new golden files (JSON/YAML/table/CSV × all/filtered).

### Phase 5: Shell completion and live mode warnings ✅
1. Added `metadataFieldCompletion()` function supporting comma-separated value completion.
2. Registered completion via `RegisterFlagCompletionFunc("metadata", ...)`.
3. Completion excludes already-selected fields and supports partial matching.
4. Added stderr warnings for incompatible flags in live mode: `--metadata`, `--agent`, `--include-contributions`, `--dry-run`.
5. Added 8 unit tests for completion function (empty, partial, after-comma, multi-selected, no-match, etc.).

---

## Decisions (resolved)

1. **Agent mode default:** `--metadata` is implied (`"all"`) when `--agent` is active.
2. **CSV comment style:** `#`-prefixed comment lines above CSV data.
3. **Table footer placement:** stdout (not stderr).
4. **Wide vs. table:** Functionally identical for DQL queries — same metadata footer.
5. **Field selection:** `--metadata=field1,field2` selects specific fields. `--metadata` alone means all fields.
6. **Zero-value preservation:** `MetadataToMap()` returns a map for explicit selection (zero values preserved) vs. struct for "all" (`omitempty` applies).
7. **Error message format:** Single-line with `"; "` separator, per Go convention.
8. **No auto `--include-contributions`:** User must explicitly pass `--include-contributions` to get contribution data from the API; `--metadata` alone does not trigger it.
9. **Agent envelope structure:** Metadata goes into the `result` payload (via `JSONPrinter`), not into `context`. `ResponseContext.QueryMetadata` was removed as dead code.
10. **All 13 fields in `--help`:** All field names are listed in the flag description for discoverability.
11. **Live mode:** Incompatible flags (`--metadata`, `--agent`, `--include-contributions`, `--dry-run`) produce a stderr warning and are otherwise ignored.
12. **Shell completion:** Supports comma-separated field selection with already-used field exclusion.

---

## Key Files

| File | Purpose |
|------|---------|
| `cmd/query.go` | Flag registration, metadata parsing, agent-mode auto-enable, shell completion, live mode warnings |
| `cmd/flags_test.go` | Completion function tests, flag default tests |
| `pkg/exec/dql.go` | `extractQueryMetadata()`, `printResults()` metadata dispatch |
| `pkg/output/metadata.go` | `QueryMetadata` struct, `ParseMetadataFields`, `MetadataToMap`, `FormatMetadataFooter`, `FormatMetadataCSVComments`, `FilterMetadata` |
| `pkg/output/metadata_test.go` | Comprehensive tests for all metadata functions |
| `pkg/exec/dql_test.go` | Nil metadata path test |
| `pkg/output/golden_test.go` | Golden test cases with `metadataFixture()` |
| `pkg/output/testdata/golden/query/dql-metadata-*.golden` | 8 golden files |
