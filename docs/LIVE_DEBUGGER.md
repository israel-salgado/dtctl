# Live Debugger Guide

This guide explains how to use Dynatrace Live Debugger features from `dtctl`.

## Overview

The current Live Debugger flow in `dtctl` supports:

- configuring workspace filters with `dtctl update breakpoint --filters ...`
- creating breakpoints with `dtctl create breakpoint File.java:line`
- listing breakpoints with `dtctl get breakpoints`
- describing breakpoint status with `dtctl describe <id|filename:line>`
- updating breakpoints with `dtctl update breakpoint ...`
- deleting breakpoints with `dtctl delete breakpoint ...`
- viewing decoded snapshot output with `dtctl query ... -o snapshot`

`dtctl` resolves or creates a Live Debugger workspace for the current project path, so commands operate on the workspace associated with the directory you run them from.

## Prerequisites

Before using Live Debugger commands:

1. Configure a Dynatrace context with `dtctl config set-context`
2. Authenticate with OAuth using `dtctl auth login`
3. Run Live Debugger commands from the project directory you want associated with the workspace

### Authentication note

Live Debugger breakpoint operations currently require OAuth authentication.
The `dev-obs:breakpoints:set` scope is supported via `dtctl auth login`, but is not currently supported when authenticating with API tokens (for example via `dtctl config set-credentials`).

## 1. Configure workspace filters

Use `dtctl update breakpoint --filters` to target the runtime instances you want Live Debugger to apply to.

```bash
dtctl update breakpoint --filters k8s.namespace.name:prod
```

Multiple filters are supported as comma-separated `key:value` pairs.
`key=value` is also supported for compatibility.

```bash
dtctl update breakpoints --filters k8s.namespace.name:prod,dt.entity.host:HOST-123
dtctl update breakpoint --filters k8s.namespace.name=prod,dt.entity.host=HOST-123
```

### Notes

- `--filters` is required for `dtctl update breakpoint`
- filter values are mapped to the Live Debugger workspace filter set payload
- repeated keys are supported
- in verbose/debug mode, raw GraphQL responses are printed for troubleshooting

## 2. Create a breakpoint

Create a breakpoint by file and line number:

```bash
dtctl create breakpoint OrderController.java:306
```

### Rules

- the expected format is `File.java:line`
- the line number must be a positive integer
- `--dry-run` is supported

Example:

```bash
dtctl create breakpoint OrderController.java:306 --dry-run
```

## 3. List breakpoints

List breakpoints in the current workspace:

```bash
dtctl get breakpoints
```

Default output is a table with:

- breakpoint ID
- filename
- line number
- active state

Structured output is also supported:

```bash
dtctl get breakpoints -o json
dtctl get breakpoints -o yaml
```

## 4. Describe breakpoint status

Inspect the current status of a breakpoint by ID or source location:

```bash
dtctl describe dtctl-rule-123
```

```bash
dtctl describe OrderController.java:306
```

The command uses `GetRuleStatusBreakdown` and summarizes:

- enabled/disabled state
- overall status
- active and pending rooks
- warnings and errors
- controller warnings and errors
- backend tips

Structured output is supported:

```bash
dtctl describe OrderController.java:306 -o json
dtctl describe OrderController.java:306 -o yaml
```

## 5. Update breakpoints

Update a breakpoint condition:

```bash
dtctl update breakpoint OrderController.java:306 --condition "value>othervalue"
```

Enable or disable a breakpoint:

```bash
dtctl update breakpoint OrderController.java:306 --enabled true
dtctl update breakpoint OrderController.java:306 --enabled false
```

### Notes

- identifiers can be either a mutable breakpoint ID or `filename:line`
- source locations resolve all matching breakpoints in the current workspace
- `--dry-run` is supported

## 6. Delete breakpoints

Delete a breakpoint by ID:

```bash
dtctl delete breakpoint dtctl-rule-123
```

Delete all breakpoints at a source location:

```bash
dtctl delete breakpoint OrderController.java:306
```

Delete all breakpoints in the current workspace:

```bash
dtctl delete breakpoint --all
```

### Delete behavior

- delete commands require confirmation by default
- `-y` or `--yes` skips confirmation
- `--dry-run` shows what would be deleted

Examples:

```bash
dtctl delete breakpoint --all -y
dtctl delete breakpoint OrderController.java:306 --dry-run
```

## 7. View decoded snapshots

Live Debugger snapshot data can be viewed through query output using `-o snapshot`.

Example:

```bash
dtctl query "fetch application.snapshots | sort timestamp desc | limit 5" -o snapshot
```

The snapshot formatter enriches each record with a decoded `parsed_snapshot` field built from:

- `snapshot.data`
- `snapshot.string_map`

This makes snapshot output readable directly from the CLI without manual protobuf decoding.

## Output and troubleshooting

### Default behavior

- successful mutating commands are quiet by default
- listing and describe commands use human-readable output by default

### Verbose and debug modes

Use `-v` or `--debug` when you want raw GraphQL payloads for troubleshooting.

### Structured output

Use `-o json` or `-o yaml` when you want automation-friendly output.

## Safety and dry-run

Live Debugger mutating commands follow `dtctl` safety conventions:

- filter updates use update safety checks
- breakpoint creation uses create safety checks
- edit operations use update safety checks
- delete operations use delete safety checks
- destructive delete operations support confirmation bypass with `-y`
- supported mutating commands provide `--dry-run`

## Example workflow

```bash
# Target a workload
dtctl update breakpoint --filters k8s.namespace.name:prod

# Create a breakpoint
dtctl create breakpoint OrderController.java:306

# List current breakpoints
dtctl get breakpoints

# Inspect status
dtctl describe OrderController.java:306

# Update condition
dtctl update breakpoint OrderController.java:306 --condition "orderId != null"

# Disable the breakpoint
dtctl update breakpoint OrderController.java:306 --enabled false

# View snapshots
dtctl query "fetch application.snapshots | sort timestamp desc | limit 5" -o snapshot

# Delete the breakpoint
dtctl delete breakpoint OrderController.java:306
```
