# Plan Creation Workflow

**Date:** 2025-02-11
**Context:** Creating implementation plans from design documents
**Bead:** wt-5l1

## Overview

This workflow transforms a validated design document into a bite-sized, TDD-compliant implementation plan that can be executed step-by-step. The key innovation is using multi-model consensus review to catch critical gaps before execution.

## Workflow Steps

### Step 1: Gather Context (Parallel Exploration)

**Goal:** Understand existing codebase patterns and conventions.

**Actions:**
- Read the design document thoroughly
- Use Serena/Glob to find similar existing commands (add.go, remove.go)
- Read key files to understand patterns:
  - CLI command structure (`internal/cli/add.go`, `remove.go`)
  - Service layer patterns (`internal/worktree/service.go`)
  - Git client interface (`internal/git/client_interface.go`)
  - Config structure (`internal/config/config.go`)
  - Test patterns (`*_test.go` files)

**What to look for:**
- Command registration pattern (`RegisterCommand()`)
- Service initialization pattern (`NewService(gitClient, cfg)`)
- Error handling style (`fmt.Errorf("operation: %w", err)`)
- Mock patterns for testing
- Hook execution patterns

### Step 2: Delegate Plan Creation to Subagent

**Goal:** Create detailed implementation plan with TDD steps.

**Command:**
```
Task tool with subagent_type=general-purpose
```

**Prompt Requirements:**
- Reference the design document path
- Include exact file paths for all changes
- Provide complete code snippets (not "add validation here")
- Cover all components: CLI, service, config, tests
- Follow TDD: write failing test → verify fails → implement → verify passes → commit
- Each step should be 2-5 minutes

**Plan Structure Required:**
```markdown
# [Feature Name] Implementation Plan

**Bead:** bead-id
**Date:** YYYY-MM-DD
**Status:** Implementation Plan
**Source:** [link to design]

## Overview
[Brief description]

## Implementation Steps

### Phase N: [Component Name]

**Step X: [Action]**

**File:** exact/path/to/file

**Action:** [Complete code snippet]

**Verification:**
```bash
# Command with expected result
```

---
```

### Step 3: Multi-Model Consensus Review

**Goal:** Catch critical gaps before execution using diverse perspectives.

**Tool:** `mcp__pal__consensus`

**Model Configuration:**
- 2+ models with different stances (for, against, neutral)
- Each model gets same prompt but different "stance" context

**Review Areas:**
1. TDD approach quality
2. Code quality / Go best practices
3. Error handling completeness
4. Hook execution correctness
5. Tmux integration
6. Step granularity (2-5 min)
7. Missing scenarios

**Prompt Template:**
```
Review the implementation plan at [path].

The plan implements [brief description].

Key areas to evaluate:
1. TDD approach: Are test-first steps well defined?
2. Code quality: Follows Go best practices?
3. Error handling: Edge cases covered?
4. [Specific to feature]
5. [Specific to feature]
6. Step granularity: Are steps bite-sized?
7. Missing anything: Critical gaps?

Please read the plan and provide your assessment.
```

**Expected Output:**
- Each model provides detailed feedback
- Look for issues mentioned by BOTH models (strong signal)
- Confidence score (7+/10 is good, <7 needs revision)

### Step 4: Apply Consensus Feedback

**Goal:** Fix critical issues identified by consensus review.

**Process:**
1. Extract agreed-upon issues from both models
2. Prioritize by severity:
   - **Critical:** Missing features from design, safety issues
   - **High:** Code quality, error handling
   - **Medium:** Step size, test coverage
   - **Low:** Nice-to-haves, future refactors
3. Delegate fixes to subagent (same as Step 2)

**Common Issues to Fix:**
- Missing features explicitly promised in design
- Import errors in code snippets
- Steps too large (>5 minutes)
- Missing safety checks
- Hook template variables not addressed
- Always using "force" variants of commands

**Update Plan:**
- Add "Consensus Review" section at top
- Document issues found and fixes applied
- Update step count and organization

### Step 5: Execution Choice

**Goal:** Offer execution options to user.

**Template:**
```
Plan complete and saved to `docs/plans/<filename>.md`. Two execution options:

1. **Subagent-Driven (this session)** - I dispatch fresh subagent per task, review between tasks, fast iteration

2. **Parallel Session (separate)** - Open new session with executing-plans, batch execution with checkpoints

Which approach?
```

**If Subagent-Driven:**
- Use `superpowers:subagent-driven-development` skill
- Stay in current session
- Fresh subagent per task + code review
- Report progress after each step

**If Parallel Session:**
- Guide user to open new session in worktree
- Use `superpowers:executing-plans` skill
- Batch execution with checkpoints

## Key Principles

### DRY (Don't Repeat Yourself)
- Reuse existing patterns instead of creating new ones
- Reference existing files rather than duplicating code
- Use shared helpers (ResolvePath, hook runners, etc.)

### YAGNI (You Aren't Gonna Need It)
- Implement ONLY what the design specifies
- No speculative features
- No "future-proofing" beyond reasonable error handling

### TDD (Test-Driven Development)
- Write failing test FIRST
- Verify it fails
- Write minimal implementation
- Verify it passes
- Commit

### Bite-Sized Steps
- Each step: 2-5 minutes max
- One clear action per step
- Explicit verification command
- Clear expected result

## Common Patterns

### Command Pattern (`internal/cli/command.go`)
```go
func NewCommandCmd() *cobra.Command {
    var flagName type

    cmd := &cobra.Command{
        Use:   "command <args>",
        Short: "One line description",
        Long:  `Detailed description with examples.`,
        Args:  cobra.ExactArgs(1),
        Run: func(cmd *cobra.Command, args []string) {
            // Load dependencies
            ctx := context.Background()
            gitClient, err := git.NewClient()
            if err != nil {
                Fatal("Failed to create git client: %v", err)
            }

            cfg, err := loadConfigForCommand()
            if err != nil {
                Fatal("Failed to load config: %v", err)
            }

            svc, err := worktree.NewService(gitClient, cfg)
            if err != nil {
                Fatal("Failed to create service: %v", err)
            }

            // Execute operation
            if err := svc.Method(ctx, args[0], flagName); err != nil {
                Fatal("Failed: %v", err)
            }

            // Success output
            fmt.Fprintf(cmd.OutOrStdout(), "Success: %s\n", args[0])
        },
    }

    cmd.Flags().BoolVar(&flagName, "flag", false, "description")

    return cmd
}

func init() {
    RegisterCommand(NewCommandCmd())
}
```

### Service Method Pattern (`internal/worktree/service.go`)
```go
func (s *Service) Method(ctx context.Context, arg string, force bool) error {
    // 1. Validation
    if arg == "" {
        return fmt.Errorf("arg is required")
    }

    // 2. Git operation
    if err := s.git.GitMethod(ctx, arg); err != nil {
        return fmt.Errorf("git method: %w", err)
    }

    // 3. Hook execution (optional)
    if len(s.cfg.Hooks.OnHook) > 0 {
        runner := executor.NewHookRunner(path)
        if err := runner.RunHooks(ctx, s.cfg.Hooks.OnHook); err != nil {
            fmt.Fprintf(os.Stderr, "Warning: hooks failed: %v\n", err)
        }
    }

    return nil
}
```

### Git Client Pattern (`internal/git/worktree.go`)
```go
func (c *Client) Method(ctx context.Context, arg string) error {
    args := []string{"command", "--flag", arg}

    var stderr bytes.Buffer
    cmd := exec.CommandContext(ctx, c.gitPath, args...)
    cmd.Stderr = &stderr

    if err := cmd.Run(); err != nil {
        return fmt.Errorf("method: %w: %s", err, stderr.String())
    }
    return nil
}
```

### Test Pattern (`internal/*/*_test.go`)
```go
func TestService_Method(t *testing.T) {
    t.Run("success case", func(t *testing.T) {
        var called bool
        mock := &mockGitClient{
            methodFunc: func(_ context.Context, arg string) error {
                called = true
                return nil
            },
        }

        cfg := config.DefaultConfig()
        svc, err := NewService(mock, cfg)
        if err != nil {
            t.Fatalf("NewService() error = %v", err)
        }

        err = svc.Method(context.Background(), "test", false)
        if err != nil {
            t.Fatalf("Method() error = %v", err)
        }

        if !called {
            t.Error("expected method to be called")
        }
    })
}
```

## Example: wt done Command

This workflow was used to create the `wt done` command implementation plan:

1. **Design:** `docs/plans/2025-02-08-wt-done-command-design.md`
2. **Plan:** `docs/plans/2025-02-11-wt-done-command-implementation.md`
3. **Bead:** wt-5l1

**Consensus Findings:**
- Both models identified missing auto-commit (Critical)
- Both identified missing `--dry-run` flag (High)
- Both identified force delete issue (High)
- Both identified missing dirty worktree check (High)
- Total: 8 issues fixed before execution

**Result:**
- 13 steps → 16 steps (better granularity)
- Multiple compile errors fixed
- Safety issues addressed
- Plan ready for confident execution

## Reusability

This workflow can be applied to any feature implementation:

1. Start with validated design
2. Gather context from existing codebase
3. Create detailed TDD plan (delegate to subagent)
4. Review with multi-model consensus
5. Apply fixes (delegate to subagent)
6. Execute with subagent-driven development

The key insight: **diverse model perspectives catch issues that a single reviewer (human or AI) might miss.**

## Future Improvements

**Potential Automation:**
- Create `/plan` slash command that:
  1. Reads design document
  2. Gathers context automatically
  3. Creates initial plan
  4. Runs consensus review
  5. Applies feedback
  6. Offers execution choice

**Skill Structure:**
```yaml
name: create-implementation-plan
description: Create TDD implementation plan from design
parameters:
  - design_doc: Path to design document
  - bead_id: Associated bead identifier
```
