# Contributing

## Getting Started

- Fork the repository and create a focused branch.
- Keep changes scoped to one concern when possible.
- If you change behavior, update tests and docs in the same PR.

## Run Tests

Core module:

```bash
go test ./...
go test -race ./...
```

Adapters module:

```bash
cd adapters
go test ./...
go test -race ./...
```

## Run Lint

Core module:

```bash
golangci-lint run ./...
```

Adapters module:

```bash
cd adapters
golangci-lint run ./...
```

Examples module:

```bash
cd examples
golangci-lint run ./...
```

## Coding Expectations

- Keep APIs small, generic, and reusable.
- Preserve stdout-first behavior and no external-backend requirement.
- Keep defaults low-noise, secure, and production-safe.
- Avoid unnecessary allocations on hot paths.
- Keep adapters thin and avoid coupling framework concerns into core packages.

## Documentation Expectations

- Update README examples when exported behavior changes.
- Keep docs practical and usage-focused.
- Ensure examples reflect actual exported APIs.

## Issues and Pull Requests

- Open an issue for bugs, behavior regressions, or design proposals.
- In PRs, explain the problem, approach, and trade-offs.
- Add benchmark notes when changing hot-path behavior.

## Backward Compatibility

- Prefer additive changes.
- If a breaking change is necessary, call it out clearly and provide migration notes.
