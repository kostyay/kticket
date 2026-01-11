Use kt to implement ALL tasks automatically.

1. Run `kt ready` to see available tasks
2. Start with first task, `kt start <id>`
3. Implement the task
4. `kt close <id>` when done
5. Loop back to step 1 until `kt ready` shows no tasks

## Workflow
- Process tasks in priority order (top first)
- Do NOT stop after one task—continue until all complete
- Use `kt add-note` to log blockers if stuck

## kt reference
```sh
kt ready                    # show actionable tasks
kt show <id>                # view task details
kt start|close <id>         # workflow transitions
kt add-note <id> "text"     # log progress
kt ls --status=in_progress  # see active work
```
