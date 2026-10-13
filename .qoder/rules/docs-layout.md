---
trigger: glob
globs: "docs/**"
---

# Docs folder

Project documents live in `docs/`.
Keep them few and current: a stale or duplicate document misleads the next session.

- **Role documents**, checked against the code and citing `file:line`:
  `ARCHITECTURE.md` (how it is built), `STATUS.md` (where it stands),
  `OPEN_ITEMS.md` (defects, debt, pending decisions).
- **One architecture entry point.** A large project splits it into files that `ARCHITECTURE.md` links to;
  never keep a parallel summary, overview or quick-look.
- **Self-explanatory project documents are fine**, such as `DATABASE.md`, `ROUTING.md`, `API.md`,
  `TROUBLESHOOTING.md`, `USING.md`, `NEXT_STEPS.md`.
  Update the existing one; do not add a sibling.
  One that carries guidance gets a row in the `CLAUDE.md` / `AGENTS.md` map.
- **Reports** (analyses, reviews, findings, run reports) go in `docs/reports/`.
  They are many and age fast, so they are gitignored.
  Do not read them as context unless pointed at one, and write a lasting conclusion into the role document it belongs to.
- **Aged material** moves to `docs/retired/`, or an existing `docs/archived/`; never the trash.

## Incorrect

```text
docs/ARCHITECTURE.md
docs/ARCHITECTURE_SUMMARY.md       parallel copy, drifts
docs/BUG_FIX_SUMMARY.md            a report at the root
docs/RUN_REPORT_2026-04-10.md      a report at the root
```

## Correct

```text
docs/ARCHITECTURE.md               links to architecture/auth.md
docs/architecture/auth.md
docs/TROUBLESHOOTING.md
docs/reports/2026-04-10-run-report.md
```
