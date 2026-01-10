---
id: kti-f6a9
status: closed
created: "2026-01-10T20:20:22Z"
type: task
priority: 2
assignee: kostyay
parent: kti-cd29
tests_passed: false
---
# Fix timestamp handling in runAddNote

show.go:152-155 - uses context value with string 'now' fallback. Use actual time.Now().UTC().Format(time.RFC3339).

## Notes

**now**

Fixed: runAddNote now uses time.Now().UTC().Format(time.RFC3339)
