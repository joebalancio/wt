# Design: Remove Deprecated project_overrides

**Date:** 2026-02-16
**Issue:** wt-xjh
**Status:** Approved

## Summary

Remove the deprecated `project_overrides` configuration feature that was replaced by project-local `.wt.yaml` files.

## Files to Modify

| File | Change |
|------|--------|
| `internal/config/config.go` | Remove `OverrideConfig` struct and `Overrides` field |
| `internal/cli/cli_config_parser.go` | Update error message to remove `project_overrides` reference |
| `docs/usage.md` | Remove deprecated documentation section |

## Code Changes

### internal/config/config.go

Remove the `Overrides` field from the `Config` struct:
```go
// REMOVE: Overrides []OverrideConfig `yaml:"project_overrides,omitempty"`
```

Remove the entire `OverrideConfig` struct:
```go
// REMOVE ENTIRELY:
type OverrideConfig struct {
    Match string      `yaml:"match"`
    Hooks HooksConfig `yaml:"hooks,omitempty"`
}
```

### internal/cli/cli_config_parser.go

Update the error message:
```go
// BEFORE:
return fmt.Errorf("key %q not supported for CLI manipulation\n       Edit config file directly to modify hooks or project_overrides", key)

// AFTER:
return fmt.Errorf("key %q not supported for CLI manipulation\n       Edit config file directly to modify hooks", key)
```

### docs/usage.md

Remove lines 202-211 (deprecated project_overrides section).
Remove line 321 (deprecation notice in notes).

## Verification

1. `make build` - No compilation errors
2. `make test` - All tests pass
3. `make lint` - No lint warnings
4. `wt config list` and `wt config validate` - Commands work correctly

## Notes

- No test files reference these structures
- Historical plan documents in `docs/plans/` are NOT modified (historical accuracy)
- `.wt.yaml.example` already uses the new approach and requires no changes
