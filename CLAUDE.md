# pingolin — Claude instructions

## Versioning and releases

- Always bump the version after implementing a change (before committing)
- "bump the version and commit" means: update the git tag (e.g. v0.3.0), commit all changes, tag
- Always run tests before committing (see Build section for test commands)

## Build

- `make build` → `./pingolin` (local dev build, version injected via ldflags)
- `make test` → `go test ./...`
- `make lint` → `golangci-lint run`
- Always set the version via ldflags: `go build -ldflags "-X main.version=vX.Y.Z"`
- Never use bare `go build` without ldflags — the binary will show "dev" as the version

## Documentation

- When making user-facing changes (new features, changed flags, new CLI modes, changed behavior), always update both README.md and llms.txt to reflect the changes

## Communication style

- Keep responses short and direct
- No emojis
