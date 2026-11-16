---
name: readme-and-docs
description: Keeps README.md, CHANGELOG.md, CONTRIBUTING.md, docs/, and PlantUML diagrams synchronized with implementation.
---

# README & Documentation Writer Skill

Use this skill when updating documentation in the root or under `docs/`.

## Guidelines
1. **Audience Separation:**
   - `README.md`: Quickstart, features, security summary, Windows prerequisites, and `Taskfile.yml` commands.
   - `docs/*.md`: Deep technical specs (`arquitectura.md`, `diseno-solucion.md`, `seguridad-boveda.md`, `ui-ux-spec.md`, `verificacion.md`, `TODO.md`).
   - `CHANGELOG.md`: User-visible changes grouped under `[Unreleased]` (`Added`, `Changed`, `Fixed`, `Security`).
2. **Accuracy:** Only document commands, flags, file paths, and behaviors that exist in the repository.
3. **Diagrams:** Update `.puml` sources under `docs/diagramas/` whenever data flow or hexagonal layers change, then export `.png` via `task doc`.
