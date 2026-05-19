---
name: git-commit
description: Stage all changes and commit with a semantic commit title and detailed body. Use when the user asks to commit, save work, or checkpoint progress. Auto-generates a conventional commit message by examining the diff.
---

# Git Commit with Semantic Messages

Stage all working changes, examine the diff, craft a semantic commit message,
and commit.

## Workflow

### 1. Stage everything

```bash
git add -A
```

### 2. Inspect the diff

```bash
git diff --staged --stat
```

Then read the full diff if needed:

```bash
git diff --staged
```

### 3. Craft the commit message

Use [Conventional Commits](https://www.conventionalcommits.org/) format:

```
type(scope): short title (≤72 chars, imperative mood, no period)

Detailed body explaining what changed and why. Wrap at 72 chars.
Can span multiple paragraphs. Reference issues or beads.
```

#### Types

| Type | When |
|------|------|
| `feat` | New feature or capability |
| `fix` | Bug fix |
| `docs` | Documentation only |
| `style` | Formatting, whitespace (not logic) |
| `refactor` | Code change that neither fixes nor adds |
| `perf` | Performance improvement |
| `test` | Adding or fixing tests |
| `chore` | Build, tooling, deps, maintenance |
| `ci` | CI/CD pipeline changes |
| `build` | Build system or external deps |
| `revert` | Reverting a previous commit |

#### Scope

The scope is optional but encouraged. Use a short noun describing the
affected module, package, or feature area:

- `feat(auth): add magic link login support`
- `fix(db): handle NULL assignee in item scan`
- `refactor(handlers): extract list rendering helper`
- `chore(deps): bump sqlite driver to v1.34.2`
- `docs(readme): add Docker deployment instructions`

#### Body

The body should cover (when relevant):
- **What changed** — which files, functions, or modules
- **Why** — the motivation or the problem it solves
- **How** — any notable implementation decisions
- **Side effects** — breaking changes, migrations, env vars
- **References** — bead IDs, issue numbers, PR links

### 4. Commit

Use two `-m` flags so Git separates title and body:

```bash
git commit -m "type(scope): short title" -m "Detailed body.
Multiple paragraphs if needed.

References: gabbie-123"
```

Do NOT use a single `-m` with everything jammed together — the title
and body must be separate for tools that parse the log.

### 5. Verify

```bash
git log -1 --format='%s%n%b'
```

## Examples

### Single feature

```bash
git commit -m "feat(lists): add collaborator management with HTMX" -m "New DB queries: AddCollaborator, RemoveCollaborator, GetCollaborators.
New handlers: POST /lists/{id}/collaborators, DELETE /lists/{id}/collaborators/{userId}.
Templates: _collaborator_list.html with owner-only remove buttons.
Only list owners can add or remove collaborators."
```

### Bug fix

```bash
git commit -m "fix(items): handle NULL due_date in IsOverdue check" -m "The IsOverdue helper panicked when due_date was NULL.
Added nil guard at the top of the method. Tested with all three
status values."
```

### Refactor

```bash
git commit -m "refactor(server): extract EitherAuthMiddleware" -m "Pulled the JWT-then-API-key fallback logic out of the route
registration into its own middleware. Simplifies RegisterRoutes
and makes the auth flow testable independently."
```

### Multi-scope commit

When changes span unrelated areas, consider splitting into multiple
commits first. If they're tightly coupled, use the most significant
scope or omit it:

```bash
git commit -m "feat: add kanban board with drag-and-drop" -m "New kanban.html template with three status columns.
HTML5 Drag and Drop API — no external libraries.
PUT /items/{id}/move handler with access control.
List/Kanban view toggle in list.html header."
```

## Anti-patterns

- ❌ `git commit -m "stuff"` — vague, useless for history
- ❌ `git commit -m "WIP"` — use `chore: save work in progress` instead
- ❌ `git commit -m "update files"` — say what files and why
- ❌ `git commit -m "fixed bug where the thing didn't work on the page"` — be specific
- ❌ Waiting to commit until work is "perfect" — commit early and often
