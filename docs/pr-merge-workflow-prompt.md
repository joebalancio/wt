# PR Merge Workflow Prompt for AI Coding Agents

Use this prompt when you need to create a PR, merge it, and properly sync the local repository.

---

## Prompt

```
You are helping complete a feature branch workflow. Follow these steps to push a feature branch, create a PR, merge it, and sync the local repository.

### Prerequisites
- You are on a feature branch with commits ready to merge
- Main branch is protected (cannot push directly)
- You have `gh` CLI installed and authenticated

### Step 1: Verify Current State

Check that the feature branch has unique commits compared to main:
```bash
git log main..HEAD --oneline
```

If no output, the branch has no unique commits. Either:
- Add commits to the branch first, OR
- The commit may already be on main locally (see Step 1b)

If the branch points to the same commit as main but main hasn't been pushed:
```bash
git rev-parse HEAD && git rev-parse main
```
Same hash = commit is on both. Push the feature branch to create a PR.

### Step 2: Push Feature Branch

Push with upstream tracking:
```bash
git push -u origin <feature-branch-name>
```

### Step 3: Create Pull Request

Create PR with a descriptive title and body:
```bash
gh pr create --title "<type>: <description>" --body "$(cat <<'EOF'
## Summary
- Bullet point summary of changes
- Include key modifications

## Test plan
- [ ] Test item 1
- [ ] Test item 2

🤖 Generated with [Claude Code](https://claude.com/claude-code)
EOF
)"
```

### Step 4: Check PR Status and CI

View PR details:
```bash
gh pr view <pr-number>
```

Check CI status:
```bash
gh pr checks <pr-number>
```

Wait for all checks to pass before merging.

### Step 5: Merge the PR

For squash merge (recommended for clean history):
```bash
gh pr merge <pr-number> --squash --delete-branch
```

Other merge options:
- `--merge` - Standard merge commit
- `--rebase` - Rebase and merge

### Step 6: Sync Local Repository

After squash merge, the local main branch will diverge from remote because squash creates a new commit hash. Reset local main:

```bash
git checkout main
git reset --hard origin/main
```

Clean up any remaining local branches:
```bash
# Check for deleted remote branches
git fetch --prune

# Delete local feature branch if it still exists
git branch -d <feature-branch-name> 2>/dev/null || git branch -D <feature-branch-name>
```

### Step 7: Verify Final State

Confirm everything is clean:
```bash
git status
git log --oneline -3
```

Expected: "Your branch is up to date with 'origin/main'" and the squash-merged commit is at HEAD.

---

## Troubleshooting

### "Diverging branches" warning after merge

This is expected after squash merge. The squash creates a new commit with different hash. Solution:
```bash
git checkout main
git reset --hard origin/main
```

### "Not possible to fast-forward" error

The `gh pr merge` command may show this warning. The PR still merges successfully on GitHub. Just sync local:
```bash
git checkout main
git reset --hard origin/main
```

### Feature branch not found when deleting

The `--delete-branch` flag in `gh pr merge` may have already deleted it locally. This is fine - no action needed.

### PR has no commits to merge

The feature branch and main point to the same commit. This happens when:
- Commit was made on main locally
- Branch was created after (points to same commit)

Solution: Just push the feature branch and create the PR. The PR will merge the "same" commit, which updates remote main.

---

## Quick Reference Commands

| Action | Command |
|--------|---------|
| Push branch | `git push -u origin <branch>` |
| Create PR | `gh pr create --title "..." --body "..."` |
| View PR | `gh pr view <number>` |
| Check CI | `gh pr checks <number>` |
| Squash merge | `gh pr merge <number> --squash --delete-branch` |
| Sync main | `git checkout main && git reset --hard origin/main` |
| Clean branches | `git fetch --prune && git branch -d <branch>` |
```

---

## Usage Example

**User request:** "Push the feature branch and create a PR, then merge it."

**Agent actions:**
1. `git push -u origin feat/my-feature`
2. `gh pr create --title "feat: add new feature" --body "..."`
3. `gh pr checks 12` (wait for green)
4. `gh pr merge 12 --squash --delete-branch`
5. `git checkout main && git reset --hard origin/main`
6. Confirm: `git status` shows "up to date with 'origin/main'"
