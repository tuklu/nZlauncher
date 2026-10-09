# Contributing

Build, code layout, how to add a provider and the release steps are in
[docs/development.md](docs/development.md).

The short version:

- branch off `dev`, open the PR against `dev`
- conventional commits (`feat:`, `fix:`, `refactor:`, `docs:`,
  `chore:`, `test:`), one change per commit
- `go vet ./...` and `go build ./...` have to pass, CI checks both
- ask before adding a dependency
- if it can be a `providers.d` script instead of Go code, it probably
  should be. See [docs/plugins.md](docs/plugins.md)
