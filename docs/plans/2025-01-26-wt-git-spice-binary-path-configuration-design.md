# Git-Spice Binary Path Configuration - Design Document (v2)

## Problem Statement

The `gs` command alias used by git-spice conflicts with other common tools:
- **Ghostscript** - PDF/PostScript interpreter (very common)
- **ImageMagick** - Sometimes provides `gs` as well
- **git-spice** - Branch stacking tool that we need

**Key Insight**: Auto-detection at runtime is fragile and surprising. Detection should happen **once** during initialization, then be explicit in configuration.

## Requirements

1. **Explicit Configuration**: No runtime auto-detection; config must be set
2. **Init-Time Detection**: `wt init` detects git-spice and sets up config
3. **Clear Errors**: `wt stack` fails fast if git-spice not configured
4. **Health Checks**: `wt doctor` reports git-spice status accurately
5. **No Magic**: No environment variables, no fallback auto-detection at runtime

## Design: Config-Only with Init-Time Detection

### Configuration Structure

```yaml
# ~/.config/wt/config.yaml
spice:
  binary_path: "/usr/local/bin/git-spice"  # Required, populated by wt init
```

**Key points:**
- `spice.binary_path` is **required** for stack operations
- Empty/missing = error when running stack commands
- No environment variable support
- No runtime auto-detection

### Behavior Matrix

| Command | spice.binary_path set | spice.binary_path empty/missing |
|---------|----------------------|--------------------------------|
| `wt init` | Detects and populates | Detects and populates |
| `wt stack` | Uses configured path | **Error**: not configured |
| `wt doctor` | Checks configured path | Reports: not configured |
| `wt add/list/remove` | Ignored (git only) | Ignored (git only) |

### Implementation Structure

```go
// internal/config/config.go
type SpiceConfig struct {
    BinaryPath string `yaml:"binary_path"`
}

type Config struct {
    Global    GlobalConfig     `yaml:"global"`
    Hooks     HooksConfig      `yaml:"hooks"`
    Tmux      TmuxConfig       `yaml:"tmux"`
    Worktree  WorktreeConfig   `yaml:"worktree"`
    Spice     SpiceConfig      `yaml:"spice"`  // NEW
    Overrides []OverrideConfig `yaml:"project_overrides,omitempty"`
}
```

```go
// internal/spice/client.go
func NewClient(cfg *config.Config) (*Client, error) {
    // No auto-detection at runtime
    if cfg.Spice.BinaryPath == "" {
        return nil, fmt.Errorf("git-spice not configured. Run 'wt init' to set up git-spice integration")
    }

    // Verify the configured path exists and is executable
    if _, err := exec.LookPath(cfg.Spice.BinaryPath); err != nil {
        return nil, fmt.Errorf("git-spice binary not found at %s: %w", cfg.Spice.BinaryPath, err)
    }

    // Optional: verify it's actually git-spice
    if err := verifyGitSpice(cfg.Spice.BinaryPath); err != nil {
        return nil, fmt.Errorf("%s is not git-spice: %w", cfg.Spice.BinaryPath, err)
    }

    return &Client{gsPath: cfg.Spice.BinaryPath}, nil
}

func verifyGitSpice(path string) error {
    cmd := exec.Command(path, "--version")
    output, err := cmd.CombinedOutput()
    if err != nil {
        return fmt.Errorf("failed to run --version: %w", err)
    }
    if !strings.Contains(string(output), "git-spice") {
        return fmt.Errorf("version output doesn't contain 'git-spice'")
    }
    return nil
}
```

```go
// internal/cli/init.go - Modified wt init
func runInit(cmd *cobra.Command, args []string) {
    // ... existing setup ...

    // Detect git-spice and add to config
    gitSpicePath, err := detectGitSpice()
    if err != nil {
        fmt.Fprintf(cmd.ErrOrStderr(), "Warning: git-spice not found: %v\n", err)
        fmt.Fprintf(cmd.ErrOrStderr(), "Stacking features will not work.\n")
        fmt.Fprintf(cmd.ErrOrStderr(), "Install git-spice: cargo install git-spice\n")
        fmt.Fprintf(cmd.ErrOrStderr(), "Then re-run: wt init\n")
    } else {
        cfg.Spice.BinaryPath = gitSpicePath
        fmt.Fprintf(cmd.OutOrStdout(), "Detected git-spice at: %s\n", gitSpicePath)
    }

    // Save config
    if err := cfg.Save(configPath); err != nil {
        Fatal("Failed to save config: %v", err)
    }
}

func detectGitSpice() (string, error) {
    // Try "git-spice" first (most specific)
    if path, err := exec.LookPath("git-spice"); err == nil {
        if err := verifyGitSpice(path); err == nil {
            return path, nil
        }
    }

    // Try "gs" with verification
    if path, err := exec.LookPath("gs"); err == nil {
        if err := verifyGitSpice(path); err == nil {
            return path, nil
        }
    }

    return "", fmt.Errorf("git-spice not found in PATH (tried git-spice and gs)")
}
```

```go
// internal/cli/doctor.go - Modified wt doctor
func runDoctor(cmd *cobra.Command, args []string) {
    // ... existing checks ...

    fmt.Fprintln(cmd.OutOrStdout(), "\nGit-Spice:")
    if cfg.Spice.BinaryPath == "" {
        fmt.Fprintln(cmd.OutOrStdout(), "  Status: Not configured")
        fmt.Fprintln(cmd.OutOrStdout(), "  Fix: Run 'wt init' to detect and configure git-spice")
    } else {
        client, err := spice.NewClient(cfg)
        if err != nil {
            fmt.Fprintf(cmd.OutOrStdout(), "  Status: Error\n")
            fmt.Fprintf(cmd.OutOrStdout(), "  Configured: %s\n", cfg.Spice.BinaryPath)
            fmt.Fprintf(cmd.OutOrStdout(), "  Error: %v\n", err)
        } else {
            version, _ := client.GetVersion(context.Background())
            fmt.Fprintf(cmd.OutOrStdout(), "  Status: OK\n")
            fmt.Fprintf(cmd.OutOrStdout(), "  Path: %s\n", cfg.Spice.BinaryPath)
            fmt.Fprintf(cmd.OutOrStdout(), "  Version: %s\n", version)
        }
    }
}
```

```go
// internal/cli/stack.go - Modified wt stack
func NewStackCmd() *cobra.Command {
    cmd := &cobra.Command{
        Use:   "stack",
        Short: "Manage stacked branches",
        Long:  `Stack commands require git-spice to be installed and configured.`,
        Run: func(cmd *cobra.Command, args []string) {
            // Early check before any operations
            cfg, err := loadConfigForCommand()
            if err != nil {
                Fatal("Failed to load config: %v", err)
            }

            if cfg.Spice.BinaryPath == "" {
                Fatal(`git-spice is not configured.

Stacking features require git-spice.

To fix:
  1. Install git-spice: cargo install git-spice
  2. Run: wt init

This will detect and configure git-spice automatically.`)
            }

            // Verify it works
            client, err := spice.NewClient(cfg)
            if err != nil {
                Fatal("git-spice configuration error: %v\n\nRun 'wt doctor' for details.", err)
            }

            // ... continue with stack operations ...
        },
    }
    // ... subcommands ...
}
```

### Error Messages

**When spice.binary_path is empty:**
```
Error: git-spice is not configured.

Stacking features require git-spice.

To fix:
  1. Install git-spice: cargo install git-spice
  2. Run: wt init

This will detect and configure git-spice automatically.
```

**When configured path is invalid:**
```
Error: git-spice configuration error: binary not found at /usr/local/bin/git-spice

Configured path: /usr/local/bin/git-spice

To fix:
  1. Edit ~/.config/wt/config.yaml
  2. Update spice.binary_path to the correct location
  3. Or run: wt init (will re-detect)
```

**When configured path is not git-spice:**
```
Error: /usr/bin/gs is not git-spice: version output doesn't contain 'git-spice'

The configured path exists but is not git-spice (it might be Ghostscript).

To fix:
  1. Install git-spice: cargo install git-spice
  2. Run: wt init (will re-detect and update config)
```

### wt init Behavior

**Scenario 1: Git-spice installed, first-time init**
```bash
$ wt init
Created config file: ~/.config/wt/config.yaml
Detected git-spice at: /home/linuxbrew/.linuxbrew/bin/git-spice
Configuration saved successfully.

Run 'wt doctor' to verify your setup.
```

**Scenario 2: Git-spice not installed**
```bash
$ wt init
Created config file: ~/.config/wt/config.yaml
Warning: git-spice not found: git-spice not found in PATH (tried git-spice and gs)
Stacking features will not work.

Install git-spice: cargo install git-spice
Then re-run: wt init

Configuration saved successfully.
```

**Scenario 3: Re-running init (already configured)**
```bash
$ wt init
Config file already exists: ~/.config/wt/config.yaml
Detected git-spice at: /home/linuxbrew/.linuxbrew/bin/git-spice
Updated spice.binary_path in configuration.

Configuration saved successfully.
```

### wt doctor Output

**Git-spice configured and working:**
```
$ wt doctor
System:
  ✓ Go 1.22.2
  ✓ Git 2.43.0

Git-Spice:
  ✓ Status: OK
  ✓ Path: /home/linuxbrew/.linuxbrew/bin/git-spice
  ✓ Version: git-spice 0.6.3

Configuration:
  ✓ Config file loaded
  ✓ Worktree location: dedicated
```

**Git-spice not configured:**
```
$ wt doctor
System:
  ✓ Go 1.22.2
  ✓ Git 2.43.0

Git-Spice:
  ✗ Status: Not configured
  Fix: Run 'wt init' to detect and configure git-spice

Configuration:
  ✓ Config file loaded
  ✓ Worktree location: dedicated
```

**Git-spice configured but broken:**
```
$ wt doctor
System:
  ✓ Go 1.22.2
  ✓ Git 2.43.0

Git-Spice:
  ✗ Status: Error
  Configured: /usr/local/bin/git-spice
  Error: binary not found

To fix:
  1. Install git-spice: cargo install git-spice
  2. Run: wt init (will re-detect)
```

### wt stack Behavior

**Attempting stack without git-spice configured:**
```bash
$ wt stack
Error: git-spice is not configured.

Stacking features require git-spice.

To fix:
  1. Install git-spice: cargo install git-spice
  2. Run: wt init

This will detect and configure git-spice automatically.
```

**Attempting stack with invalid config:**
```bash
$ wt stack
Error: git-spice configuration error: /usr/bin/gs is not git-spice

The configured path exists but is not git-spice (it might be Ghostscript).

To fix:
  1. Install git-spice: cargo install git-spice
  2. Run: wt init (will re-detect and update config)

Run 'wt doctor' for more details.
```

### Configuration Examples

**After `wt init` with git-spice installed:**
```yaml
# ~/.config/wt/config.yaml
global:
  tmux_session_prefix: "wt-"

tmux:
  layout: main-vertical
  window_name: work
  attach_on_create: true

worktree:
  location: dedicated
  dedicated_path: ~/worktrees

spice:
  binary_path: "/home/linuxbrew/.linuxbrew/bin/git-spice"  # Populated by wt init
```

**After `wt init` without git-spice:**
```yaml
# ~/.config/wt/config.yaml
global:
  tmux_session_prefix: "wt-"

tmux:
  layout: main-vertical
  window_name: work
  attach_on_create: true

worktree:
  location: dedicated
  dedicated_path: ~/worktrees

spice:
  binary_path: ""  # Empty - stack commands will error
```

**Manual configuration (if user wants specific path):**
```yaml
spice:
  binary_path: "/custom/path/to/git-spice"
```

### Key Design Principles

1. **Explicit over Implicit**: Config file or fail; no runtime magic
2. **Detect Once**: `wt init` does the detection, not every command
3. **Fail Fast**: Stack commands error immediately if not configured
4. **Clear Guidance**: All errors include actionable fix instructions
5. **Doctor Knows**: `wt doctor` accurately reports configuration status

### Edge Cases

| Situation | Behavior |
|-----------|----------|
| First run, git-spice installed | `wt init` detects and sets `binary_path` |
| First run, git-spice not installed | `wt init` warns, leaves `binary_path` empty |
| Re-run `wt init` | Updates `binary_path` with current detection |
| User edits config to bad path | `wt stack` errors with clear message |
| User has `gs` = Ghostscript only | `wt init` won't set `binary_path`, `wt stack` errors |
| User moves git-spice install | Re-run `wt init` to update config |
| CI/CD with custom path | Set `binary_path` in config manually |

### Migration from Current Code

**Current state:**
- `spice.NewClient()` auto-detects on every call
- No config support
- Fails cryptically if `gs` is Ghostscript

**Migration:**
1. Add `SpiceConfig` to config struct
2. Modify `wt init` to detect and populate `binary_path`
3. Modify `spice.NewClient()` to require config
4. Update all callers to pass config
5. Add `wt doctor` git-spice check
6. Update `wt stack` to validate config before operations

**Breaking changes:**
- Existing users need to run `wt init` after upgrading
- If they don't, stack commands will fail with helpful error
- This is acceptable pre-1.0

### Testing Strategy

1. **Unit tests** for `detectGitSpice()` function
2. **Unit tests** for `verifyGitSpice()` function
3. **Integration tests** for `wt init` with/without git-spice
4. **Integration tests** for `wt doctor` output parsing
5. **Integration tests** for `wt stack` error conditions
6. **Manual tests** on system with Ghostscript installed

---

## Implementation Tasks

1. **Config structure**
   - [ ] Add `SpiceConfig` struct to `internal/config/config.go`
   - [ ] Add `Spice` field to main `Config` struct
   - [ ] Update `DefaultConfig()` to include empty `Spice` config

2. **Detection logic**
   - [ ] Add `detectGitSpice()` function (tries git-spice, then verified gs)
   - [ ] Add `verifyGitSpice()` function (runs --version, checks output)
   - [ ] Add unit tests for detection

3. **wt init command**
   - [ ] Modify `runInit()` to call `detectGitSpice()`
   - [ ] Populate `cfg.Spice.BinaryPath` with detected path
   - [ ] Print warning if git-spice not found
   - [ ] Print success message with detected path
   - [ ] Test init with git-spice installed
   - [ ] Test init without git-spice

4. **Spice client**
   - [ ] Modify `NewClient()` to accept `*config.Config`
   - [ ] Return error if `BinaryPath` is empty
   - [ ] Verify configured path exists and is executable
   - [ ] Verify configured binary is actually git-spice
   - [ ] Update error messages to be actionable

5. **Update callers**
   - [ ] Update `internal/stack/service.go` to pass config to spice client
   - [ ] Update `internal/cli/stack.go` to load config
   - [ ] Update `internal/cli/init.go` to use new config structure
   - [ ] Update tests to include spice config

6. **wt doctor command**
   - [ ] Add git-spice section to doctor output
   - [ ] Check if `binary_path` is set
   - [ ] Try to create client and get version
   - [ ] Print status (OK/Error/Not configured)
   - [ ] Include fix suggestions for each state

7. **wt stack command**
   - [ ] Add early check for `spice.binary_path` before operations
   - [ ] Error with helpful message if not configured
   - [ ] Verify client creation succeeds before proceeding
   - [ ] Test stack command without config

8. **Documentation**
   - [ ] Update README with git-spice setup instructions
   - [ ] Update configs/example.yaml with spice section
   - [ ] Add troubleshooting section for git-spice issues
   - [ ] Document error messages and their solutions

9. **Integration tests**
   - [ ] Test full workflow: install git-spice → wt init → wt stack
   - [ ] Test error case: no git-spice → wt init → wt stack (should fail)
   - [ ] Test re-init: wt init → move git-spice → wt init → wt stack

---

## Summary of Changes from v1

| Aspect | v1 (Hybrid) | v2 (Config-Only) |
|--------|-------------|------------------|
| Environment variable | `WT_SPICE_PATH` | **Removed** |
| Runtime auto-detect | Yes (fallback) | **No** |
| When detection happens | Every client creation | **Only during `wt init`** |
| Empty config behavior | Falls back to auto-detect | **Error** |
| `wt init` behavior | Creates config only | **Creates + detects git-spice** |
| `wt doctor` | Basic check | **Full health report** |
| `wt stack` validation | Implicit | **Explicit early check** |

**Rationale for v2 simplification:**
- More predictable behavior (no hidden runtime detection)
- Easier to debug (config is explicit)
- Better error messages (know exactly what's configured)
- Simpler mental model (init sets up everything)
- Still user-friendly (init does the hard work)

---

## Sign-off

**Approach:** Config-Only with Init-Time Detection
**Complexity:** Low (simpler than v1)
**Breaking Changes:** Yes (need to re-run init after upgrade)
**Migration Required:** Run `wt init` after upgrade
