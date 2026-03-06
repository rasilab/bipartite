---
name: bip.epic.tuckin
description: Persist orchestrator state before context reset
---

# /bip.epic.tuckin

Flush orchestrator state to durable storage before a context reset or
session end. Run this when context is getting long or before stopping.

## Usage

```
/bip.epic.tuckin
```

## Workflow

### Step 1: Push EPIC body edits

EPIC bodies live as `ISSUE-EPIC-<N>.md` files in the repo root (gitignored).
These are the working copies — edit them locally, then push to GitHub.

For each `ISSUE-EPIC-*.md` file in the repo root:

1. Extract the issue number from the filename (`ISSUE-EPIC-284.md` → 284)
2. Push to GitHub:
   ```bash
   gh issue edit <number> --body-file ISSUE-EPIC-<N>.md
   ```

During the session, always edit the local file first, then push. This
ensures the local file is the source of truth and survives context resets.

If editing EPIC bodies mid-session (not just at tuckin), follow the same
pattern: edit the local `ISSUE-EPIC-<N>.md`, then push with `gh issue edit`.

Report which EPICs were pushed.

### Step 2: Update clone status files

Read `clone_root` from `.epic-config.json`.

For each clone the orchestrator interacted with this session:

1. Check if the clone's `.epic-status.json` is stale or missing
2. If the orchestrator has newer information (e.g., a clone finished,
   got blocked, or changed phase), update the file:
   ```bash
   CLONE_ROOT=$(jq -r .clone_root .epic-config.json)
   cat > "$CLONE_ROOT/<clone>/.epic-status.json" << 'EOF'
   {
     "issue": <N>,
     "title": "<title>",
     "phase": "<current phase>",
     "summary": "<what happened>",
     "updated_at": "<ISO 8601 timestamp>",
     "blockers": []
   }
   EOF
   ```

Update clones the orchestrator has direct knowledge about. This includes:
- Clones whose PRs were merged this session (even if the clone's own
  session already exited)
- Clones the orchestrator observed finishing via tmux or poll

Do not guess status for clones with active sessions that may have
progressed beyond what the orchestrator last observed.

### Step 3: Write MEMORY.md

Update the project's `MEMORY.md` (at the auto-memory path) with
orchestrator-specific context that isn't captured elsewhere:

- Decisions made this session and their rationale
- Patterns noticed (e.g., "clone X is slow", "issue Y depends on Z")
- Anything the next session should know that isn't in EPIC bodies or
  status files

**Do not duplicate** information already in EPIC bodies or status files.
Focus on the "why" and "what's next" that would be lost.

### Step 4: Report

Print a summary:

```
## Tuckin Complete

- EPICs pushed: i281, i295
- EPICs skipped (conflict): i310
- Clone status updated: cedar, oak
- MEMORY.md updated: yes

Safe to reset context.
```
