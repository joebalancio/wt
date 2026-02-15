# GitHub CI for PRs - Design

**Date:** 2026-02-15
**Status:** Approved

## Overview

Add GitHub Actions CI workflow to validate all pull requests before merge. The workflow runs two parallel jobs: fast feedback (build, lint, unit tests) and integration tests (git/tmux).

## Design Decisions

| Decision | Choice | Rationale |
|----------|--------|-----------|
| Coverage level | Standard + integration | Catches both code quality and real git/tmux issues |
| Release builds | Deferred | Will add when ready for binary releases |
| Tool installation | Use runner pre-installed | GitHub ubuntu-latest has git/tmux; simpler, faster |
| Trigger strategy | Every push to PR | Standard approach, early feedback |

## Workflow Structure

```
CI Workflow (on pull_request to main)
├── build-lint-test job (parallel)
│   ├── checkout
│   ├── setup Go 1.22
│   ├── download dependencies
│   ├── build
│   ├── lint (golangci-lint)
│   └── unit tests (with race detection)
│
└── integration job (parallel)
    ├── checkout
    ├── setup Go 1.22
    ├── verify git/tmux available
    └── integration tests (WT_INTEGRATION_TEST=1)
```

## Workflow File

Location: `.github/workflows/ci.yml`

```yaml
name: CI

on:
  pull_request:
    branches: [main]

jobs:
  build-lint-test:
    name: Build, Lint & Unit Tests
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4

      - name: Setup Go
        uses: actions/setup-go@v5
        with:
          go-version: '1.22'
          cache: true

      - name: Download dependencies
        run: go mod download

      - name: Build
        run: make build

      - name: Lint
        uses: golangci/golangci-lint-action@v6
        with:
          version: latest

      - name: Run unit tests
        run: make test

  integration:
    name: Integration Tests
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4

      - name: Setup Go
        uses: actions/setup-go@v5
        with:
          go-version: '1.22'
          cache: true

      - name: Verify git and tmux
        run: |
          git --version
          tmux -V

      - name: Run integration tests
        run: make test-integration
        env:
          WT_INTEGRATION_TEST: 1
```

## Notes

- tmux should work on GitHub ubuntu-latest without special configuration
- If tmux issues arise, add `tmux start-server` step or `TERM=xterm` environment
- Go module caching enabled for faster subsequent runs (~30s improvement)
- Both jobs run in parallel; total CI time expected ~3-4 minutes

## Future Considerations

- Add release builds when ready for binary distribution
- Consider adding `on: push: main` to run CI on main branch after merge
- Add branch protection rules requiring CI to pass before merge
