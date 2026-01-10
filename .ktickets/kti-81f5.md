---
id: kti-81f5
status: closed
created: "2026-01-10T20:20:32Z"
type: chore
priority: 3
assignee: kostyay
parent: kti-cd29
tests_passed: false
---
# Consider DI for Store global

root.go:15 - global mutable Store makes testing harder. Consider dependency injection or context passing.

## Notes

**now**

Won't do: global store is fine for CLI
