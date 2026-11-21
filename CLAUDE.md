# pingolin: Claude instructions

## Versioning and releases

- Always bump the version after implementing a change (before committing)
- Always run tests before committing (see Build section for test commands)
- Releases are built by CI, not locally. To cut vX.Y.Z:
  1. Write `RELEASE_NOTES.md` for the new version (it becomes the GitHub release body)
  2. Commit and push to main
  3. Push the tag: `git tag vX.Y.Z && git push origin vX.Y.Z`
- The `v*` tag triggers `.github/workflows/release.yml`, which runs goreleaser to build the binaries, `checksums.txt` and SBOMs, publishes the release, and attests the artifacts
- Never run `goreleaser release` locally and never use `gh release create`

## Build

- `make build` → `./pingolin` (local dev build, version injected via ldflags)
- `make test` → `go test -v -race -count=1 ./...`
- `make vet` → `go vet ./...`
- `make lint` → `golangci-lint run`
- Always set the version via ldflags: `go build -ldflags "-X main.version=vX.Y.Z"`
- Never use bare `go build` without ldflags: the binary will show "dev" as the version

## Documentation

- When making user-facing changes (new features, changed flags, new CLI modes, changed behavior), always update both README.md and llms.txt to reflect the changes

## Communication style

- Keep responses short and direct
- No emojis
