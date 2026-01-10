---
id: kti-8b6e
status: closed
created: "2026-01-10T20:20:33Z"
type: task
priority: 3
assignee: kostyay
parent: kti-cd29
tests_passed: false
---
# Replace separatorRe with strings.FieldsFunc

store/id.go:12 - compiled regex for simple pattern. Could use strings.FieldsFunc instead.

## Notes

**now**

Fixed: Replaced separatorRe with strings.FieldsFunc
