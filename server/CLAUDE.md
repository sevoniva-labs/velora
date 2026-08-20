@AGENTS.md

# Claude project instructions

Before editing, load `.claude/skills/forge-banking-scaffold/SKILL.md`. The mirrored Skill must remain byte-identical to `.agents/skills/forge-banking-scaffold/SKILL.md`; `make ai-governance` enforces this.

Treat `AGENTS.md` and `docs/ai-engineering-governance.md` as mandatory. If a request conflicts with them, surface the exact decision boundary instead of silently choosing a weaker implementation.
