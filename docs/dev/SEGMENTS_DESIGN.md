# Segments Feature Design

Full-featured segments support for dtctl: CRUD management + DQL query-time filtering.

Tracking issue: [#115 — Segments support when executing DQL queries](https://github.com/dynatrace-oss/dtctl/issues/115)

---

## Background

Segments are reusable, named filter definitions that logically structure observability data across the Dynatrace platform. They act as **query-time context** — similar to timeframes — where Grail evaluates only the segment includes relevant to the queried data type.

Key properties (from [Dynatrace Segments docs](https://docs.dynatrace.com/docs/manage/segments)):

- A segment has **includes** — rules referencing data types (logs, metrics, spans, entities, "all data types"). Includes are OR-combined within a segment.
- Segments support **variables** (primary + secondary) with values populated by DQL queries or static lists. Variables enable dynamic segments that cover similar instances in a single definition.
- **Visibility**: "unlisted" (owner-only in lists, default) vs "anyone in the environment". Visibility does not affect access — all segments are readable by anyone with `storage:filter-segments:read`.
- Multiple segments on a single query are **AND-combined** (intersection).
- **Limits**: 10 segments per query, 20 includes per segment, 10,000 segments per environment.

### API Types (from DQL Query SDK)

The query execution API (`ExecuteRequest.filterSegments`) accepts:

```
FilterSegments = Array<FilterSegment>

FilterSegment {
  id:        string                              (required)
  variables: Array<FilterSegmentVariableDefinition>
}

FilterSegmentVariableDefinition {
  name:   string          (required)
  values: Array<string>   (required)
}
```

### Management API

Base path: `/platform/storage/filter-segments/v1/filter-segments`

| Method | Path | Description |
|--------|------|-------------|
| GET | `/filter-segments` | List segments |
| GET | `/filter-segments/{id}` | Get segment by ID |
| POST | `/filter-segments` | Create segment |
| PUT | `/filter-segments/{id}` | Update segment |
| DELETE | `/filter-segments/{id}` | Delete segment |

Required scopes (already provisioned in `pkg/auth/oauth_flow.go`):
- `storage:filter-segments:read`
- `storage:filter-segments:write`
- `storage:filter-segments:delete`

---

## Part 1: Segment Management (CRUD)

### 1.1 Resource Handler

New package: `pkg/resources/segment/` (one package per resource, matching `bucket/`, `slo/`, `workflow/`, etc.).

**Data structures:**

```go
// FilterSegment is the read model for a Grail filter segment.
type FilterSegment struct {
    UID               string    `json:"uid" table:"UID"`
    Name              string    `json:"name" table:"NAME"`
    Description       string    `json:"description,omitempty" table:"DESCRIPTION,wide"`
    IsPublic          bool      `json:"isPublic" table:"PUBLIC"`
    Owner             string    `json:"owner,omitempty" table:"OWNER,wide"`
    Version           int       `json:"version,omitempty" table:"-"`
    IsReadyMade       bool      `json:"isReadyMade,omitempty" table:"-"`
    Includes          []Include `json:"includes,omitempty" table:"-"`
    Variables         *Variables `json:"variables,omitempty" table:"-"`
    AllowedOperations []string  `json:"allowedOperations,omitempty" table:"-"`
}

type Include struct {
    DataObject string `json:"dataObject"` // "logs", "spans", etc. Use "_all_data_object" for all.
    Filter     string `json:"filter"`
}

type Variables struct {
    Type  string `json:"type"`  // Variable type, e.g. "query"
    Value string `json:"value"` // Variable value, e.g. a DQL expression
}

type FilterSegmentList struct {
    FilterSegments []FilterSegment `json:"filterSegments"`
    TotalCount     int             `json:"totalCount,omitempty"`
}
```

> **Note**: These structs reflect the actual API response schema as confirmed during implementation. Key differences from the initial design: `Include.DataObject` (not `DataType`), `Variables.Type`/`Value` (not `Query`/`Columns`), no pagination (`FilterSegmentList` has no `NextPageKey`), and `Update` requires an `optimistic-locking-version` query param.

**Handler interface:**

```go
func NewHandler(c *client.Client) *Handler
func (h *Handler) List() (*FilterSegmentList, error)
func (h *Handler) Get(uid string) (*FilterSegment, error)
func (h *Handler) Create(data []byte) (*FilterSegment, error)
func (h *Handler) Update(uid string, version int, data []byte) error
func (h *Handler) Delete(uid string) error
func (h *Handler) GetRaw(uid string) ([]byte, error)    // For edit command
```

### 1.2 CLI Commands

| Command | File | Verb | Notes |
|---------|------|------|-------|
| `dtctl get segments [uid]` | `cmd/get_segments.go` | get | List all or get one |
| `dtctl describe segment <uid>` | `cmd/describe_segments.go` | describe | Rich detail view |
| `dtctl create segment -f segment.yaml` | `cmd/create_segments.go` | create | File-based |
| `dtctl edit segment <uid>` | `cmd/edit_segments.go` | edit | Opens in $EDITOR |
| `dtctl delete segment <uid>` | `cmd/get_segments.go` | delete | Safety check + confirm |
| `dtctl apply -f segment.yaml` | (modify `pkg/apply/`) | apply | Create-or-update |

### 1.3 Naming & Aliases

| Command | Use | Aliases |
|---------|-----|---------|
| get (list) | `segments` | `segment`, `seg`, `filter-segments`, `filter-segment` |
| get (single) | `segments <uid>` | (same as above, arg triggers single-get) |
| describe | `segment <uid>` | `seg`, `filter-segment` |
| create | `segment` | `seg`, `filter-segment` |
| edit | `segment <uid>` | `seg`, `filter-segment` |
| delete | `segment <uid>` | `seg`, `filter-segment` |

**Rationale**: Dynatrace's own docs and UI call them "segments" everywhere. The `filter-segment` prefix is an internal API path detail. Keeping `filter-segments` as an alias covers API-aware users.

### 1.4 Describe Output (table mode)

```
Name:          my-k8s-segment
UID:           abc123-def456
Description:   Filters data for Kubernetes cluster alpha
Public:        Yes
Owner:         user@example.invalid
Version:       3

Includes:
  DATA OBJECT         FILTER
  All data objects    k8s.cluster.name = "alpha"
  Logs                dt.system.bucket = "custom-logs"

Variables:
  Type:     query
  Value:    data record(ns="namespace-a"), record(ns="namespace-b")
```

### 1.5 Example YAML for create/apply

```yaml
name: my-k8s-segment
description: Filters data for Kubernetes cluster alpha
isPublic: true
includes:
  - dataObject: _all_data_object
    filter: 'k8s.cluster.name = "alpha"'
variables:
  type: query
  value: 'data record(ns="namespace-a"), record(ns="namespace-b")'
```

### 1.6 Registration Points

| File | Change |
|------|--------|
| `cmd/get.go` init() | `getCmd.AddCommand(getSegmentsCmd)`, `deleteCmd.AddCommand(deleteSegmentCmd)` |
| `cmd/describe.go` init() | `describeCmd.AddCommand(describeSegmentCmd)` |
| `cmd/create.go` init() | `createCmd.AddCommand(createSegmentCmd)` |
| `cmd/edit.go` init() | `editCmd.AddCommand(editSegmentCmd)` |
| `pkg/apply/applier.go` | Add `ResourceTypeSegment`, detection heuristic, `applySegment()` |
| `pkg/apply/result.go` | Add `SegmentApplyResult` |

---

## Part 2: Segments in DQL Query Execution

This is the core of issue [#115](https://github.com/dynatrace-oss/dtctl/issues/115).

### 2.1 API Integration — `pkg/exec/dql.go`

**New types:**

```go
// FilterSegmentRef identifies a segment and optional variable bindings for query execution.
type FilterSegmentRef struct {
    ID        string                    `json:"id"`
    Variables []FilterSegmentVariable   `json:"variables,omitempty"`
}

type FilterSegmentVariable struct {
    Name   string   `json:"name"`
    Values []string `json:"values"`
}
```

**Changes to existing types:**

```go
// Add to DQLQueryRequest:
FilterSegments []FilterSegmentRef `json:"filterSegments,omitempty"`

// Add to DQLExecuteOptions:
Segments []FilterSegmentRef
```

**Wiring in `ExecuteQueryWithOptions`:**

```go
if len(opts.Segments) > 0 {
    req.FilterSegments = opts.Segments
}
```

### 2.2 CLI Flags — `cmd/query.go`

Two new flags:

#### `--segment` / `-S` (repeatable string array)

Simple form for segments without variables. Can be specified multiple times.

```bash
# Single segment
dtctl query "fetch logs | limit 10" --segment my-segment-uid

# Multiple segments (AND-combined per Grail semantics)
dtctl query "fetch logs | limit 10" --segment seg-uid-1 --segment seg-uid-2

# Short form
dtctl query "fetch logs | limit 10" -S my-segment-uid
```

#### `--segments-file` (string path)

For segments that need variables or complex multi-segment configurations:

```bash
dtctl query "fetch logs | limit 10" --segments-file segments.yaml
```

**YAML schema** (mirrors the API's `FilterSegments` type exactly):

```yaml
- id: simple-segment-uid

- id: segment-with-variables
  variables:
    - name: host
      values: [HOST-0000000000000001, HOST-0000000000000002]

- id: segment-with-namespace
  variables:
    - name: ns
      values: [production, staging]
```

#### Combining `--segment` and `--segments-file`

Both flags can be used together. IDs from `--segment` are appended as variable-less entries. If the same segment ID appears in both, the `--segments-file` entry wins (it may carry variables). Duplicates by ID are deduplicated.

#### Validation

- **Max 10 segments per query** — validate client-side with a clear error message (matches [documented API limit](https://docs.dynatrace.com/docs/manage/segments/reference/segments-reference-limits)).
- `--segment ""` (empty string) is rejected.
- `--segments-file` must point to a readable file that parses as a YAML array of segment refs.

### 2.3 Name Resolution for `--segment`

Once the segment handler exists (Part 1), add segment support to `pkg/resources/resolver/`. This allows users to pass segment **names** (not just UIDs) to `--segment`:

```bash
# By UID (always works)
dtctl query "fetch logs" --segment abc123-def456

# By name (resolved via API)
dtctl query "fetch logs" --segment my-k8s-segment
```

Resolution follows the existing pattern: try exact UID first, fall back to name search via `List()`, error on ambiguity.

### 2.4 Agent Mode

When segments are used, the agent output envelope's `context` should reflect them:

```json
{
  "ok": true,
  "result": [...],
  "context": {
    "verb": "query",
    "segments": ["seg-uid-1", "seg-uid-2"],
    "suggestions": ["Use --segment to filter by segment"]
  }
}
```

---

## UX Design Decisions

| Decision | Choice | Rationale |
|----------|--------|-----------|
| Primary noun | `segment` / `segments` | Matches Dynatrace user-facing terminology. `filter-segment` kept as alias only. |
| Short alias | `seg` | Concise, unambiguous. |
| Query flag name | `--segment` / `-S` (repeatable) | `-s` conflicts with common flags. Repeatable is more shell-friendly than comma-separated (avoids quoting issues). |
| File flag name | `--segments-file` | Distinct from `--segment` to avoid confusion. Matches the issue's proposed UX. |
| Segment identification | UID-based with name resolution | Segments are UID-identified in the API. Name resolution (like workflows, documents) gives users convenience. |
| Combining flags | `--segment` + `--segments-file` merge | Power users can mix simple and complex cases. File entries win on ID conflict. |
| Max segments check | Client-side, 10 per query | Matches documented limit. Better UX than an opaque server-side 400 error. |
| Package location | `pkg/resources/segment/` | Per-resource package convention (not `grail/` which would mix multiple resources). |

---

## Implementation Phases

### Phase 1: Resource Handler + Read Commands

**Goal**: `dtctl get segments`, `dtctl describe segment <uid>`

| # | Task | Files |
|---|------|-------|
| 1 | Create segment handler with List, Get, GetRaw | `pkg/resources/segment/segment.go` |
| 2 | Create handler unit tests with mock server | `pkg/resources/segment/segment_test.go` |
| 3 | Create get command (list + get single) | `cmd/get_segments.go` |
| 4 | Create describe command with KV detail view | `cmd/describe_segments.go` |
| 5 | Register in get.go and describe.go | `cmd/get.go`, `cmd/describe.go` |
| 6 | Add watch support | `cmd/get_segments.go` (call `addWatchFlags`) |
| 7 | Add golden test cases using real structs | `pkg/output/golden_test.go` |

### Phase 2: Mutating Commands

**Goal**: `dtctl create/edit/delete/apply segment`

| # | Task | Files |
|---|------|-------|
| 1 | Add Create, Update, Delete to handler | `pkg/resources/segment/segment.go` |
| 2 | Add handler tests for mutations | `pkg/resources/segment/segment_test.go` |
| 3 | Create create command (file input, dry-run, safety) | `cmd/create_segments.go` |
| 4 | Create edit command (editor flow, ownership safety) | `cmd/edit_segments.go` |
| 5 | Add delete command (safety + confirmation) | `cmd/get_segments.go` |
| 6 | Register in create.go, edit.go | `cmd/create.go`, `cmd/edit.go` |
| 7 | Add apply support (detection + create-or-update) | `pkg/apply/applier.go` |
| 8 | Add SegmentApplyResult | `pkg/apply/result.go` |
| 9 | Update golden tests for apply results | `pkg/output/golden_test.go` |

### Phase 3: Query-Time Segments

**Goal**: `dtctl query "..." --segment <id>`, `dtctl query "..." --segments-file f.yaml`

| # | Task | Files |
|---|------|-------|
| 1 | Add FilterSegmentRef types | `pkg/exec/dql.go` |
| 2 | Add FilterSegments to DQLQueryRequest | `pkg/exec/dql.go` |
| 3 | Add Segments to DQLExecuteOptions | `pkg/exec/dql.go` |
| 4 | Wire segment options in ExecuteQueryWithOptions | `pkg/exec/dql.go` |
| 5 | Add `--segment` / `-S` repeatable flag | `cmd/query.go` |
| 6 | Add `--segments-file` flag | `cmd/query.go` |
| 7 | Implement segment parsing (YAML + merge + dedup) | `cmd/query.go` |
| 8 | Add client-side validation (max 10, no empty IDs) | `cmd/query.go` |
| 9 | Add unit tests for segment parsing and merging | `cmd/query_test.go` |
| 10 | Add shell completion for `--segment` | `cmd/query.go` |

### Phase 4: Integration & Polish

**Goal**: Name resolution, docs, E2E tests

| # | Task | Files |
|---|------|-------|
| 1 | Add segment name resolution to resolver | `pkg/resources/resolver/resolver.go` |
| 2 | Wire resolver into `--segment` flag processing | `cmd/query.go` |
| 3 | Add E2E tests | `test/e2e/segments_test.go` |
| 4 | Update implementation status | `docs/dev/IMPLEMENTATION_STATUS.md` |
| 5 | Update API design doc (uncomment + expand) | `docs/dev/API_DESIGN.md` |
| 6 | Update FUTURE_FEATURES.md (mark complete) | `docs/dev/FUTURE_FEATURES.md` |
| 7 | Update README resource table | `README.md` |
| 8 | Update CHANGELOG | `CHANGELOG.md` |

---

## Files Summary

### New Files

| File | Purpose |
|------|---------|
| `pkg/resources/segment/segment.go` | Handler + data structs |
| `pkg/resources/segment/segment_test.go` | Unit tests with mock server |
| `cmd/get_segments.go` | get + delete commands |
| `cmd/describe_segments.go` | describe command |
| `cmd/create_segments.go` | create command |
| `cmd/edit_segments.go` | edit command |
| `test/e2e/segments_test.go` | E2E tests |

### Modified Files

| File | Change |
|------|--------|
| `pkg/exec/dql.go` | Add FilterSegmentRef types, FilterSegments field in request + options |
| `cmd/query.go` | Add `--segment` / `-S`, `--segments-file` flags + parsing/validation |
| `cmd/query_test.go` | Tests for segment flag parsing and merging |
| `cmd/get.go` | Register getSegmentsCmd, deleteSegmentCmd |
| `cmd/describe.go` | Register describeSegmentCmd |
| `cmd/create.go` | Register createSegmentCmd |
| `cmd/edit.go` | Register editSegmentCmd |
| `pkg/apply/applier.go` | Add ResourceTypeSegment, detection heuristic, applySegment() |
| `pkg/apply/result.go` | Add SegmentApplyResult |
| `pkg/resources/resolver/resolver.go` | Add segment name-to-UID resolution |
| `pkg/output/golden_test.go` | Add segment golden test cases |
| `docs/dev/IMPLEMENTATION_STATUS.md` | Mark segments as implemented |
| `docs/dev/API_DESIGN.md` | Uncomment segment examples, add query examples |
| `docs/dev/FUTURE_FEATURES.md` | Mark segment tasks as complete |
| `README.md` | Add segments to resource table |
| `CHANGELOG.md` | Document new feature |
