Create an epic and bite-sized tasks for this plan.

## kt reference
Tickets in `.ktickets/` (markdown+YAML). Hierarchy: epic>task>subtask.

```sh
kt create "title" -d "desc" --parent <id>  # -t bug|feature|task|epic|chore -p 0-4
kt ls [--status=open] [--parent=<id>]      # or: kt ready, kt blocked
kt show <id>                               # partial ID ok: a1b2 → kt-a1b2c3d4
kt start|pass|close <id>                   # workflow transitions
kt dep add|rm|tree <id> [dep-id]
```

Create flags: `--design --acceptance --tests --external-ref`
