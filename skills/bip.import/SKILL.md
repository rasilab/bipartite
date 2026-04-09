---
name: bip.import
description: Import Paperpile or Zotero library into bip, rebuild database, and clean up.
---

# Import Reference Library

Import a Paperpile JSON or Zotero Better BibTeX JSON export into the bip reference library.

## Usage

```bash
/bip.import              # Auto-detect latest export in ~/Downloads
/bip.import ~/path.json  # Explicit file path
```

## Paperpile Workflow

### 1. Find the export file

If no path argument given, look for `~/Downloads/Paperpile*.json` or `~/Downloads/*.json`:

```bash
ls -lt ~/Downloads/Paperpile*.json 2>/dev/null || ls -lt ~/Downloads/*.json
```

If multiple files match, ask the user which one.

### 2. Dry-run import

Always dry-run first to show what will change:

```bash
bip import --format paperpile "<file>" --dry-run --human
```

Report the counts (new, updated, skipped) to the user. The skipped entries are typically old Paperpile records missing year or author — expected and not actionable.

### 3. Import

```bash
bip import --format paperpile "<file>" --human
```

### 4. Rebuild database

```bash
bip rebuild --human
```

### 5. Delete the export file

```bash
rm "<file>"
```

### 6. Report

Summarize: new refs added, total count, file deleted.

## Zotero Workflow

### Prerequisites

- Better BibTeX plugin installed in Zotero
- `pdf_root` configured to Zotero storage directory: `bip config set pdf_root ~/Zotero/storage`

### 1. Export from Zotero

In Zotero: File > Export Library > Format: "Better BibTeX JSON" > OK

### 2. Dry-run import

```bash
bip import --format zotero "<file>" --dry-run --human
```

### 3. Import

```bash
bip import --format zotero "<file>" --human
```

### 4. Rebuild database

```bash
bip rebuild --human
```

### 5. Delete the export file

```bash
rm "<file>"
```

### 6. Report

Summarize: new refs added, total count, file deleted.
