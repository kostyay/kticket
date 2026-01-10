---
id: kti-a452
status: closed
deps:
- kti-b449
created: "2026-01-10T20:20:13Z"
type: task
priority: 1
assignee: kostyay
parent: kti-cd29
tests_passed: false
---
# Remove unused sort import

list.go:5 - sort import unused after removing redundant sort in runClosed.

## Notes

**now**

Invalid: sort IS used in list.go:163 (runClosed)
