## Description

<!-- What does this PR do? Link to the relevant issue if applicable. -->

Closes #

## Type of change

- [ ] Bug fix (non-breaking)
- [ ] New feature (non-breaking)
- [ ] Breaking change (requires major version bump)
- [ ] Documentation update
- [ ] CI/infrastructure change

## Checklist

- [ ] Tests added or updated for changed behavior
- [ ] All tests pass locally (`go test -race ./...` in affected modules)
- [ ] Lint passes (`golangci-lint run ./...`)
- [ ] Documentation updated (README, docs/, godoc)
- [ ] If API surface changed: `.api-snapshot.txt` updated (`go doc -all . > .api-snapshot.txt`)
- [ ] If new sub-module dependency: updated relevant `go.mod` and `go.sum`
- [ ] If breaking change: documented in `CHANGELOG.md` and bumps major version per [CONTRIBUTING.md](CONTRIBUTING.md)

## Testing

<!-- Describe how you tested this change. -->
