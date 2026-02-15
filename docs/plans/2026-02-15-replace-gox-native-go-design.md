# Replace gox with Native Go Cross-Compilation

**Status:** Design Approved
**Date:** 2026-02-15
**Author:** AI Design Session

## Overview

Replace the archived `gox` dependency with native Go cross-compilation using `GOOS`/`GOARCH` environment variables. This reduces external dependencies while simplifying the release build process.

## Motivation

- **Reduce dependencies:** `github.com/mitchellh/gox` is archived (last updated ~2020). While functional, removing it simplifies the toolchain.
- **Modernize:** Use idiomatic Go patterns for cross-compilation instead of a wrapper tool.
- **Simplify:** One less tool to install and maintain for contributors.

## Scope

**In scope:**
- Replace `gox` in `build-release` Makefile target
- Remove `gox` from `tools` Makefile target

**Out of scope:**
- Changing platform targets (keep: linux/amd64, linux/arm64, darwin/amd64, darwin/arm64)
- Parallel builds (sequential builds are acceptable; CI/CD handles parallelization)

## Design

### build-release Target Replacement

Replace lines 78-81 in `Makefile`:

```makefile
## build-release: Build release binaries for multiple platforms
build-release:
	@echo "Building release binaries..."
	@mkdir -p $(BUILD_DIR)
	@for osarch in linux/amd64 linux/arm64 darwin/amd64 darwin/arm64; do \
		echo "Building for $$osarch..."; \
		GOOS=$${osarch%/*} GOARCH=$${osarch#*/} \
			$(GO) build $(GOFLAGS) -o "$(BUILD_DIR)/$(BINARY_NAME)_$${osarch%/*}_$${osarch#*/}" ./$(CMD_DIR) || exit 1; \
	done
	@echo "Release binaries built in $(BUILD_DIR)/"
```

**Key details:**
- Shell parameter expansion: `${osarch%/*}` extracts OS, `${osarch#*/}` extracts architecture
- `$(GOFLAGS)` preserves existing `-v` verbose flag
- `|| exit 1` fails fast if any platform build fails
- Progress echo statements provide user feedback
- Output naming unchanged: `wt_linux_amd64`, `wt_linux_arm64`, `wt_darwin_amd64`, `wt_darwin_arm64`

### tools Target Update

Update line 108 in `Makefile`:

```makefile
## tools: Install development tools
tools:
	@echo "Installing development tools..."
	@curl -sSfL https://raw.githubusercontent.com/golangci/golangci-lint/master/install.sh | sh -s -- -b $$(go env GOPATH)/bin
	@$(GO) install mvdan.cc/gofumpt@latest
	@echo "Tools installed successfully!"
```

**Changes:**
- Removed `@$(GO) install github.com/mitchellh/gox@latest` line
- Updated golangci-lint URL to raw GitHub (more reliable than shortlink)

## Implementation Steps

1. Edit `Makefile` line 78-81: Replace `build-release` target
2. Edit `Makefile` line 108: Remove gox from `tools` target
3. Test locally: `make build-release` and verify 4 binaries are created
4. Update CLAUDE.md: Remove gox from development tools documentation

## Testing

```bash
# Verify build-release works
make build-release
ls -lh bin/
# Expected: wt_linux_amd64, wt_linux_arm64, wt_darwin_amd64, wt_darwin_arm64

# Verify tools target works without gox
make tools
```

## Alternatives Considered

| Option | Pros | Cons | Decision |
|--------|------|------|----------|
| Shell loop in Makefile | Simple, no new files | Shell syntax can be cryptic | **Selected** |
| Go build script | Clean, testable | Adds another artifact | Rejected: YAGNI |
| Makefile pattern rules | Make-idiomatic | Complex syntax | Rejected: Overkill |

## Risks

| Risk | Mitigation |
|------|------------|
| Shell syntax may confuse newcomers | Add inline comments explaining expansion |
| Sequential builds slower | Acceptable trade-off; builds done in CI/CD |

## References

- Go cross-compilation: https://go.dev/doc/install/source#environment
- Original gox repo: https://github.com/mitchellh/gox (archived)
