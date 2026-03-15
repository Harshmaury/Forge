# WORKFLOW-DELIVERY.md
# @version: 2.0.0
# @updated: 2026-03-16

---

## DROP FOLDER

Windows:  C:\Users\harsh\Downloads\engx-drop\
WSL2:     /mnt/c/Users/harsh/Downloads/engx-drop/

---

## ZIP NAMING

```
forge-<what>-<YYYYMMDD>-<HHMM>.zip
```

Examples: `forge-fix-workflow-transaction-20260316-2000.zip`
          `forge-phase4-ai-commands-20260316-0900.zip`

---

## ZIP STRUCTURE

Mirror the repo tree exactly. No wrapper folder.

---

## APPLY COMMAND

```bash
cd ~/workspace/projects/apps/forge && \
unzip -o /mnt/c/Users/harsh/Downloads/engx-drop/<ZIP>.zip -d . && \
go build ./... && \
git add <files> WORKFLOW-SESSION.md && \
git commit -m "<type>: <description>" && \
git push origin <branch>
```

`go build ./...` must pass before `git add`. Always.

---

## RULES

- WORKFLOW-SESSION.md travels in every zip
- Version bumps on every delivery
- One logical unit per zip
- Grep all import usages before removing any import
