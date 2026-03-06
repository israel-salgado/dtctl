# dtctl CLI (v{{.Version}})

When the user wants to interact with Dynatrace, use dtctl:
- `dtctl get <resource>` to list resources
- `dtctl describe <resource> <id>` for details
- `dtctl apply -f <file>` to deploy (idempotent)
- `dtctl diff -f <file>` to preview changes before applying
- `dtctl query --query "<DQL>"` for ad-hoc queries
- `dtctl commands` for full command reference

Always use `--agent` flag for structured output.
Use `dtctl apply --dry-run` before applying changes.
