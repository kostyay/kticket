# Changelog

## feat/kt-version

`kt wait <id>` blocks until a ticket reaches `closed` status, polling every
2 seconds with a stderr heartbeat every 30 seconds (#8). Useful for scripting
and agent workflows that need to pause until a dependency ticket is resolved.
`make release` replicates the CI release workflow locally — it runs lint and
tests, auto-bumps the patch version tag, pushes it, and runs goreleaser to
create the GitHub release with changelog and cross-platform binaries.
