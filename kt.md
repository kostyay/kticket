## Task Tracking (kt)

Git-backed issue tracker for AI agents. Stores tickets as markdown files with YAML frontmatter in `.ktickets/`.

### Quick Reference
- `kt help` for full CLI reference
- Structure: epic > task > subtask (keep subtasks atomic)
- Start work: `kt ready` → pick top, execute, update status
- Creating: break features into testable chunks

### Common Commands

```sh
# Create tickets
kt create "title" -d "description" --parent <epic-id>
kt create "Feature X" -t feature -p 1

# List/query
kt ls --status=open       # List open tickets
kt ready                  # Open with deps resolved
kt blocked                # Open with unresolved deps
kt show <id>              # Display ticket details

# Workflow
kt start <id>             # Set to in_progress
kt pass <id>              # Mark tests passed
kt close <id>             # Set to closed
kt add-note <id> "text"   # Append timestamped note

# Dependencies
kt dep add <id> <dep-id>  # Add dependency
kt dep tree <id>          # Show dependency tree
kt link add <id> <id>     # Link tickets (symmetric)
```

### Create Options
- `-t, --type`: bug|feature|task|epic|chore (default: task)
- `-p, --priority`: 0-4, 0=highest (default: 2)
- `-d, --description`: Description text
- `--design`: Design notes
- `--acceptance`: Acceptance criteria
- `--tests`: Test requirements (requires `kt pass` before close)
- `--parent`: Parent ticket ID
- `--external-ref`: External reference (e.g., gh-123)

### Output Modes
- Terminal: human-readable text
- Piped/--json: JSON for scripting

### ID Format
Partial ID matching supported: `kt show a1b2` matches `kt-a1b2c3d4`.
