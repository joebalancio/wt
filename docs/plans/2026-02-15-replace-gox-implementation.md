# Replace gox with Native Go Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Replace the archived `gox` dependency with native Go cross-compilation using `GOOS`/`GOARCH` environment variables.

**Architecture:** Shell loop in Makefile that iterates over platform targets, setting `GOOS`/`GOARCH` per iteration. No new files or code needed—pure Makefile/shell changes.

**Tech Stack:** GNU Make, Bash shell, Go toolchain (cross-compilation via environment variables)

---

## Task 1: Replace build-release Target

**Files:**
- Modify: `Makefile:77-81`

**Step 1: Edit Makefile - Replace build-release target**

Replace lines 77-81 with:

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

**What this does:**
- `${osarch%/*}` = shell parameter expansion: remove everything after `/` → gets OS
- `${osarch#*/}` = shell parameter expansion: remove everything before `/` → gets Arch
- `|| exit 1` = fail fast if any build fails
- `$$osarch` = double `$` escapes the variable in Make (single `$` is Make variable)

**Step 2: Test the new build-release target**

Run: `make build-release`

Expected output:
```
Building release binaries...
Building for linux/amd64...
Building for linux/arm64...
Building for darwin/amd64...
Building for darwin/arm64...
Release binaries built in bin/
```

**Step 3: Verify binaries were created**

Run: `ls -lh bin/`

Expected files:
- `wt_linux_amd64`
- `wt_linux_arm64`
- `wt_darwin_amd64`
- `wt_darwin_arm64`

**Step 4: Verify binaries are executable (optional sanity check)**

Run: `file bin/wt_linux_amd64`

Expected: `ELF 64-bit LSB executable, x86-64` (or similar)

**Step 5: Commit**

```bash
git add Makefile
git commit -m "feat: replace gox with native Go cross-compilation in build-release

- Use shell loop with GOOS/GOARCH environment variables
- Build for linux/amd64, linux/arm64, darwin/amd64, darwin/arm64
- Add progress echo statements and fail-fast error handling
- Preserves existing output naming convention"
```

---

## Task 2: Remove gox from tools Target

**Files:**
- Modify: `Makefile:104-110`

**Step 1: Edit Makefile - Remove gox installation line**

Replace lines 104-110 with:

```makefile
## tools: Install development tools
tools:
	@echo "Installing development tools..."
	@curl -sSfL https://raw.githubusercontent.com/golangci/golangci-lint/master/install.sh | sh -s -- -b $$(go env GOPATH)/bin
	@$(GO) install mvdan.cc/gofumpt@latest
	@echo "Tools installed successfully!"
```

**What changed:**
- Removed line: `@$(GO) install github.com/mitchellh/gox@latest`
- Updated golangci-lint URL to raw GitHub (more reliable than shortlink)

**Step 2: Test tools target works**

Run: `make tools`

Expected output:
```
Installing development tools...
Tools installed successfully!
```

**Step 3: Verify gox is not installed**

Run: `which gox` or `command -v gox`

Expected: No output (gox not found)

**Step 4: Commit**

```bash
git add Makefile
git commit -m "chore: remove gox from tools installation target

- gox no longer needed for cross-compilation
- Update golangci-lint URL to raw GitHub for reliability"
```

---

## Task 3: Update AGENTS.md Documentation

**Files:**
- Modify: `AGENTS.md:63`

**Step 1: Edit AGENTS.md - Remove gox from tools comment**

Find line 63:
```markdown
make tools                 # Install golangci-lint, gofumpt, gox
```

Replace with:
```markdown
make tools                 # Install golangci-lint, gofumpt
```

**Step 2: Verify the change looks correct in context**

The section should read:
```markdown
# Install development tools
make tools                 # Install golangci-lint, gofumpt
```

**Step 3: Commit**

```bash
git add AGENTS.md
git commit -m "docs: remove gox from tools documentation

gox is no longer needed or installed by make tools"
```

---

## Task 4: Full Integration Test

**Files:**
- Test: `Makefile` (verify targets work)

**Step 1: Clean build artifacts**

Run: `make clean`

Expected:
```
Cleaning...
```

**Step 2: Run build-release from scratch**

Run: `make build-release`

Expected: All 4 binaries built successfully with progress messages

**Step 3: Verify all binaries exist and are valid**

Run:
```bash
ls -lh bin/ | grep wt_
file bin/wt_*
```

Expected: 4 executables, correct file types for each platform

**Step 4: (Optional) Quick smoke test on local platform**

If on Linux:
```bash
./bin/wt_linux_amd64 --help
```

Expected: wt help output

**Step 5: Final verification - no gox reference remaining**

Run: `grep -r "gox" Makefile AGENTS.md *.md 2>/dev/null || echo "No gox references found"`

Expected: "No gox references found" (only matches in docs/plans/ are OK)

**Step 6: Final commit (if any adjustments needed)**

Only if issues found and fixed. Otherwise, work is complete.

---

## Testing Summary

After completing all tasks, verify:

| Test | Command | Expected |
|------|---------|----------|
| Build release | `make build-release` | 4 binaries created |
| List binaries | `ls bin/` | wt_linux_amd64, wt_linux_arm64, wt_darwin_amd64, wt_darwin_arm64 |
| Install tools | `make tools` | Success, no gox installed |
| No gox refs | `grep -r gox Makefile AGENTS.md` | No matches |
| Binary type | `file bin/wt_linux_amd64` | ELF 64-bit executable |

---

## Rollback Plan

If issues occur, revert commits:

```bash
git revert HEAD~2..HEAD  # Reverts the 3 commits above
```

Then reinstall gox: `go install github.com/mitchellh/gox@latest`

---

## References

- Design doc: `docs/plans/2026-02-15-replace-gox-native-go-design.md`
- Go cross-compilation: https://go.dev/doc/install/source#environment
