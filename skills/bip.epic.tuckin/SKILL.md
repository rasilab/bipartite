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

For each `ISSUE-EPIC-*.md` file in the repo root:

1. Identify the issue number (check file contents or ask)
2. Run the `updatedAt` conflict check:
   ```bash
   CURRENT_AT=$(gh issue view <number> --json updatedAt -q .updatedAt)
   ```
3. If no conflict, push:
   ```bash
   gh issue edit <number> --body-file ISSUE-EPIC-<short-desc>.md
   ```
4. If conflict detected, warn and skip — do not overwrite

Report which EPICs were pushed and which had conflicts.

### Step 2: Update clone status files

For each clone the orchestrator interacted with this session:

1. Read `.epic-config.json` for clone paths
2. Check if the clone's `.epic-status.json` is stale or missing
3. If the orchestrator has newer information (e.g., a clone finished,
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

Only update clones the orchestrator has direct knowledge about — do not
guess status for clones that have their own active sessions.

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
