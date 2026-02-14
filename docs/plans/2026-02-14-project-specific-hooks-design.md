# Project-Specific Hooks Design

**Goal:** To allow users to define project-specific hooks in their `wt` configuration that are triggered based on the path of the worktree. This applies to all hook types: `on_worktree_create`, `on_worktree_remove`, and `on_worktree_done`.

**Architecture:** The implementation will modify the hook execution logic in `pkg/executor/hook_runner.go`. It will introduce logic to check for and apply matching project overrides from the configuration before executing any hook type. No new packages or dependencies are required.

**Configuration Schema:** See `configs/example.yaml` for the full structure. The relevant section:

```yaml
project_overrides:
  - match: "**/*rust*"        # Glob pattern matched against worktree path
    hooks:
      on_worktree_create:
        - run: "cargo fetch"
          cwd: "{worktree_path}"
      on_worktree_remove:
        - run: "rm -rf target"
          cwd: "{worktree_path}"
```

**Data Flow:**

1. A hook-triggering command is executed (`wt add`, `wt remove`, or `wt done`).
2. The `HookRunner` is invoked with the hook type and `worktreePath`.
3. Inside `HookRunner.Run()`:
    a. The `wt` configuration is loaded.
    b. A new list `hooksToRun` is initialized with the global hooks for the current hook type (e.g., `cfg.Hooks.OnWorktreeCreate`).
    c. The function iterates through each `override` in `cfg.ProjectOverrides`.
    d. For each `override`, `filepath.Match(override.Match, worktreePath)` checks if the glob pattern matches the worktree's absolute path.
    e. If a match is found, the hooks for the current hook type from `override.Hooks` are **appended** to `hooksToRun`.
    f. **All matching overrides are applied** (not just the first match).
    g. The final `hooksToRun` list is executed sequentially.
4. Each hook is executed with template variables expanded (e.g., `{worktree_path}`).

**Error Handling:**

*   **Runtime glob errors:** If `filepath.Match` returns an error (due to a malformed glob pattern), the error will be logged as a warning, and the process will continue without applying that specific override.
*   **Config validation:** Glob pattern validity is checked at config load time by `config.ValidateSchema()`. Malformed patterns (e.g., `**/[`) will cause `wt config validate` to fail with a descriptive error.
*   **Hook execution errors:** Continue to be handled by the existing `HookRunner`, which reports them as warnings without halting the command.

**Implementation Changes:**

1. `internal/config/config.go`:
   - Add `ProjectOverrides []ProjectOverride` to `Config` struct
   - Add `ValidateSchema()` check for glob pattern validity using `filepath.Match` with empty string
   - Update `ProjectOverride` struct with `Match string` and `Hooks HooksConfig`

2. `pkg/executor/hook_runner.go`:
   - Modify `Run()` to accept config and iterate over `ProjectOverrides`
   - Apply matching override hooks to all hook types

**Testing Plan:** Integration tests will verify:

| Test Case | Description |
|-----------|-------------|
| Global + Override | Both global and matching override hooks run in append order |
| Multiple Matches | All matching overrides apply, not just the first |
| All Hook Types | Overrides work for `create`, `remove`, and `done` hooks |
| Glob Validation | Malformed pattern fails `wt config validate` |
| No Match | Non-matching override does not apply hooks |
| Empty Overrides | Empty `project_overrides` list works correctly |
