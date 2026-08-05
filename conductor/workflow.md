# Workflow Guidelines

## Development Cycle
1. **Spec & Plan**: Every track starts with a clear `spec.md` and `plan.md` in `conductor/tracks/<track_id>/`.
2. **Implementation & Testing**: Build tools and agents in Go (`pkg/`), verify with `go build ./...` and unit tests (`go test ./...`).
3. **Verification**: Run CLI/Console verification or test against example YAML configurations.
