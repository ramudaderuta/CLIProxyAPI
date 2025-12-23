---
name: git-commit-review
description: MUST use for any commit or PR submission request. Review staged diffs, specific commits, or pre-merge PRs to ensure correctness and scope discipline.
---

Use this skill to review **staged changes**, a **specific commit**, or a **PR before merge**. Keep it concise and evidence-based.

## 0) Pick the review type

Choose one and follow only that path:
- **Pre-commit (staged changes)**: review `git diff --staged` before creating a commit.
- **Commit review (specific commit)**: review an existing commit (default `HEAD`).
- **Pre-merge PR review**: review a PR/branch diff before merging.

If the user didn't specify, ask which type they want.

## 1) Required inputs (ask if missing)

### Pre-commit (staged)
1. `git status -sb`
2. `git diff --staged --stat`
3. `git diff --staged`
4. `git diff` (to detect unstaged spillover)

### Commit review (specific commit)
1. `git status -sb`
2. `git log -1 --oneline` (or commit SHA given by user)
3. `git show -1` (or `git show <sha>`)
4. Optional: issue/ticket description or acceptance criteria

### Pre-merge PR review
1. `git status -sb`
2. `git log --oneline --decorate -n 5`
3. `git diff <base>...<head>` (or PR diff from the user)
4. Optional: PR description or acceptance criteria

If any inputs are missing or partial, call it out explicitly.

## 2) Review goals (apply to all types)

- Correctness, edge cases, and error handling
- Security and data safety
- Performance risk in hot paths
- Maintainability and clarity
- Scope matches intent
- Tests/docs updated as needed

## 3) Review workflow (short + focused)

### A) Context check (fast)
- What is the stated intent?
- Is the change set aligned with that intent?
- Any unrelated or accidental changes?
- What minimal tests should cover this?

### B) High-level scan
- Design fit: matches patterns and architecture?
- Change hygiene: files in right place, no noise?
- Tests/docs: added or updated where needed?

### C) Line-by-line review
- Logic: edge cases, ordering, null/empty checks?
- Security: validation, trust boundaries, secrets?
- Performance: hot paths, loops, allocations?
- Maintainability: naming, complexity, comments?

### D) Decision
- Verdict: ready / comment / request changes
- Call out missing tests or follow-ups

## 4) Quick checks (use as needed)

### Scope & hygiene
- Only intended files changed
- No debug logs/TODOs
- No secrets/tokens/keys
- Config changes justified
- Can describe change in one sentence

### Security
- Inputs validated/sanitized
- Auth checks present if needed
- Errors don't leak sensitive info

### Performance
- No obvious hot-path regressions
- No N+1 queries
- Large data paginated/cached

### Testing
- Happy path covered
- Edge cases covered
- Tests deterministic

## 5) Comment labels

```markdown
[blocking] Must fix before commit/merge
[important] Should fix; discuss if disagree
[nit] Nice to have, not blocking
[suggestion] Alternative approach
[praise] Good work
```

## 6) Output format (use for all review types)

```markdown
## Review Summary
- Intent:
- Verdict: pass / mixed / fail

## Issues Found
- (file:line) short note
- None

## Fixes Applied
- (file:line) short rationale
- None

## Tests
- command - result
- Not run (why)
- Optional for small changes (state why)

## Risks / Follow-ups
- remaining concerns or next checks
```

## 7) Constraints (non-negotiable)

- Do not discard or overwrite uncommitted user changes.
- Do not rewrite history (no amend/rebase unless asked).
- Keep fixes minimal and within the reviewed scope.
- If validation is blocked, say why and propose next steps.
- Small changes do not require tests; state clearly when tests are skipped.
