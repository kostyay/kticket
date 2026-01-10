```
    ██╗  ██╗████████╗██╗ ██████╗██╗  ██╗███████╗████████╗
    ██║ ██╔╝╚══██╔══╝██║██╔════╝██║ ██╔╝██╔════╝╚══██╔══╝
    █████╔╝    ██║   ██║██║     █████╔╝ █████╗     ██║
    ██╔═██╗    ██║   ██║██║     ██╔═██╗ ██╔══╝     ██║
    ██║  ██╗   ██║   ██║╚██████╗██║  ██╗███████╗   ██║
    ╚═╝  ╚═╝   ╚═╝   ╚═╝ ╚═════╝╚═╝  ╚═╝╚══════╝   ╚═╝
         git-backed issue tracker for ai agents
```

# kt

Git-backed issue tracker for AI agents.

Stores tickets as markdown files with YAML frontmatter in `.kticket/`. Designed for AI agents to easily search and manipulate without dumping large JSON blobs into context windows.

Set `KTICKET_DIR` environment variable to override the storage directory.

## Install

```sh
go install github.com/kostyay/kticket/cmd/kt@latest
```

## AI Agent Setup

Add to your project's `CLAUDE.md`:

```markdown
## Task Tracking (kt)
- `kt help` for CLI reference
- Structure: epic > task > subtask (keep subtasks atomic)
- Start work: `kt ready` → pick top, execute, update status
- Creating: break features into testable chunks (`kt create "title" -d "description" --parent <epic-id>`)
```

## Quick Start

```sh
# Create a ticket
kt create "Add user authentication" -t feature -p 1

# List open tickets
kt ls --status=open

# Start working on a ticket
kt start abc1  # partial ID matching works

# Add dependency
kt dep add abc1 def2

# Close when done
kt close abc1
```

## Commands

### Ticket Management

```sh
kt create [title]              # Create ticket, prints ID
  -d, --description            # Description text
  --design                     # Design notes
  --acceptance                 # Acceptance criteria
  --tests                      # Test requirements
  -t, --type                   # bug|feature|task|epic|chore (default: task)
  -p, --priority               # 0-4, 0=highest (default: 2)
  -a, --assignee               # Assignee (default: git user.name)
  --external-ref               # External reference (e.g., gh-123)
  --parent                     # Parent ticket ID

kt show <id>...                # Display ticket(s)
kt edit <id>                   # Open in $EDITOR
kt add-note <id> [text]        # Append timestamped note
```

### Status Changes

```sh
kt start <id>...               # Set to in_progress
kt close <id>...               # Set to closed (validates tests)
kt reopen <id>...              # Set to open
kt status <id> <status>        # Set arbitrary status
kt pass <id>...                # Mark tests as passed
```

### Dependencies & Links

```sh
kt dep add <id> <dep-id>       # Add dependency
kt dep rm <id> <dep-id>        # Remove dependency
kt dep tree [--full] <id>      # Show dependency tree

kt link add <id> <id> [id...]  # Link tickets (symmetric)
kt link rm <id> <target-id>    # Remove link
```

### Queries

```sh
kt ls [--status=X]             # List tickets
kt ready                       # Open/in_progress with deps resolved
kt blocked                     # Open/in_progress with unresolved deps
kt closed [--limit=N]          # Recently closed (default 20)
kt stats                       # Counts by status
kt query                       # Raw JSON output
```

## Output Modes

- **Terminal**: Human-readable text format
- **Piped/--json**: JSON format for scripting

```sh
# Auto-detects pipe, outputs JSON
kt ls | jq '.[0].id'

# Force JSON
kt --json stats
```

## Test Validation

Tickets with a `## Tests` section require `kt pass` before closing:

```sh
kt create "Feature X" --tests "- TestOne\n- TestTwo"
kt close abc1  # Error: tests not passed
kt pass abc1
kt close abc1  # Success
```

## Storage Format

Files stored in `.kticket/<id>.md`:

```markdown
---
id: kt-a1b2
status: in_progress
deps: [kt-c3d4]
created: 2026-01-09T10:30:00Z
type: feature
priority: 1
assignee: kostya
tests_passed: false
---
# Add user authentication

Description here.

## Design

Design notes.

## Acceptance Criteria

- Criterion 1
- Criterion 2

## Tests

- TestLoginSuccess
- TestLoginInvalidPassword

## Notes

**2026-01-09T14:00:00Z**

Note content.
```

## ID Format

IDs are generated from the project directory name:

- `my-project` → `mp-xxxx`
- `kticket` → `kti-xxxx`
- `foo-bar-baz` → `fbb-xxxx`

Partial ID matching is supported: `kt show a1b2` matches `kt-a1b2c3d4`.

## Inspired By

[beads](https://github.com/steveyegge/beads) by Steve Yegge

## License

MIT
