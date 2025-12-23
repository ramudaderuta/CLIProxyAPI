---
name: git-upstream-merge
description: MUST use when asked to merge upstream changes. Fetch, merge, resolve conflicts, and report clearly before any commit/push.
---

Use this skill to merge an upstream remote into the current repo with careful conflict handling and a clear report.

## 0) Required inputs (ask if missing)

- Upstream remote name (default: `upstream`)
- Upstream branch (default: `main` or `master`)
- Local branch (default: current)
- Whether to **commit** or **push** after merge (default: ask)

## 1) Preconditions (stop and ask if not met)

- Working tree must be clean (`git status -sb`).
- Upstream remote must exist (`git remote -v`).

If the tree is dirty or upstream is missing, pause and ask how to proceed.

## 2) Merge workflow (minimal, in order)

### A) Prepare
- `git status -sb`
- `git remote -v`
- `git branch --show-current`

### B) Fetch + inspect
- `git fetch <upstream> <branch>`
- `git log --oneline HEAD..<upstream>/<branch>`

### C) Merge
- `git merge <upstream>/<branch>`
- If clean merge: skip to step E.
- If conflicts: continue to step D.

### D) Conflict resolution (only conflicted files)
- Identify conflicts via `git status`.
- For each conflicted file:
  - Read the conflicting regions and surrounding context.
  - Resolve using the smallest consistent merge.
  - Remove all conflict markers.
  - `git add <file>` once resolved.

Resolution priorities:
- **Imports/deps**: merge unique entries, remove duplicates, keep order.
- **Config**: prefer newer versions; flag conflicts needing confirmation.
- **Logic changes**: preserve both when compatible; otherwise prefer upstream and flag for review.
- **Docs/comments**: merge if additive; avoid duplicates.

If resolution is unclear, stop and ask instead of guessing.

### E) Validate (lightweight)
- Run the smallest relevant test/build only if requested or if changes are risky.
- Otherwise, state tests were skipped and why.

### F) Finish (only if user asked)
- Commit merge **only** if explicitly requested.
- Push **only** if explicitly requested.

## 3) Output format

```markdown
## Merge Summary
- Status: success / conflicts resolved / blocked
- Upstream: <remote>/<branch>
- Local branch: <branch>
- Commits merged: <count or list>

## Conflict Resolution
- (file) short resolution note
- None

## Validation
- command - result
- Not run (why)

## Risks / Follow-ups
- remaining concerns or manual checks
```

## 4) Constraints (non-negotiable)

- Do not discard or overwrite uncommitted user changes.
- Do not rewrite history (no rebase/squash unless asked).
- Do not commit or push unless explicitly requested.
- Keep changes minimal and scoped to conflict resolution.
- If blocked, report clearly and propose next steps.
