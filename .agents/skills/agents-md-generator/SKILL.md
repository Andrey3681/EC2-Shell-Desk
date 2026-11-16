---
name: agents-md-generator
description: Maintains AGENTS.md and .agents/rules/ synchronized as packages, adapters, and UI modules evolve.
---

# AGENTS.md Maintenance Skill

Use this skill whenever new layers, adapters, tools, or architectural constraints are introduced.

## Guidelines
1. Keep `AGENTS.md` concise, high-signal, and strictly consistent with `.agents/rules/`.
2. Verify that any new package path mentioned in `AGENTS.md` actually exists in the repository.
3. Ensure the English-only code/docstrings rule, hexagonal dependency boundaries, and security invariants remain prominent and unambiguous.
4. Keep `estructura.md` as historical reference only once `docs/` and `AGENTS.md` are authoritative.
