# Contributing to Loggos

Contributions are welcome. The following guidelines help keep the process
smooth for everyone.

## Getting Started

1. Fork the repository and clone your fork.
2. Copy `.env.example` to `.env` and adjust values if needed.
3. Run `go mod download` to fetch dependencies.
4. Run `go test ./...` to verify everything passes before making changes.

## Making Changes

1. Create a feature branch from `master` (`git checkout -b my-feature`).
2. Write clear, focused commits. Each commit should do one thing.
3. Add or update tests for any new or changed behaviour.
4. Run `go vet ./...` and `gofmt -l .` to check for issues before pushing.

## Pull Requests

1. Open a pull request against `master`.
2. Describe **what** the change does and **why** it is needed.
3. CI must pass (build, tests, lint) before a PR will be reviewed.

## Reporting Bugs

Open a GitHub issue with steps to reproduce the problem, expected behaviour,
and actual behaviour. Include the Go version and OS you are using.

## Security Issues

See [SECURITY.md](SECURITY.md) for how to report vulnerabilities privately.
