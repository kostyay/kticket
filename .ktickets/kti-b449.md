---
id: kti-b449
status: closed
created: "2026-01-10T20:20:12Z"
type: task
priority: 1
assignee: kostyay
parent: kti-cd29
tests_passed: false
---
# Remove redundant sort in runClosed

list.go:138-141 - comment says already sorted but re-sorts anyway. Store.List() already sorts by created.

## Notes

**now**

Won't do: sort needed after filtering closed tickets
