---
id: kti-5cdb
status: closed
created: "2026-01-10T20:20:22Z"
type: task
priority: 2
assignee: kostyay
parent: kti-cd29
tests_passed: false
---
# Use slices stdlib for containsString/removeString

link.go:150-167 - replace custom helpers with slices.Contains and slices.DeleteFunc (Go 1.21+).

## Notes

**now**

Fixed: Replaced containsString/removeString with slices.Contains/DeleteFunc
