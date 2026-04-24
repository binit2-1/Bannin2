---
name: rule-writing-guidelines
description: Guidance for generating low-noise, tool-valid custom rules for Falco, Suricata, Wazuh, and Zeek using project-summary context and bannin.yaml deployment targets.
---

# Rule Writing Guidelines Skill

Use this skill when generating or updating security detection rules from a project summary.

## Goal

Produce production-ready, low-noise custom rules for one tool at a time:
- falco
- suricata
- wazuh
- zeek

## Required Inputs

- `toolname`: one of `falco | suricata | wazuh | zeek`
- `project-summary`: authoritative context about architecture, expected behavior, known noisy processes, approved network patterns, critical assets, and risk priorities

## Workflow

1. Read `bannin.yaml` first to get the canonical output path and validation/reload plan.
2. Read the tool-specific guide:
- `falco.md`
- `suricata.md`
- `wazuh.md`
- `zeek.md`
3. Translate project-summary into environment-specific allowlists and exception logic.
4. Prefer targeted detections with strong context over broad signatures.
5. Include concise rationale comments in generated rule files.
6. Return a complete draft for the server-owned deployment path.
7. Let the server validate syntax and apply reload/restart strategy from `bannin.yaml`.

## Rule Quality Criteria

- High signal: catches risky behavior tied to project threat model.
- Low noise: avoids known benign behavior, scheduled jobs, and trusted service accounts.
- Maintainable: clear names, stable identifiers, readable grouping, minimal duplication.
- Deployable: writes to the configured path and follows tool-native syntax.

## Authoring Discipline

- Prefer extending or overriding existing upstream logic when it preserves useful Falco macros/lists and reduces duplicate maintenance.
- Preserve load-order assumptions: custom files are typically loaded after default Falco rules, so overrides should be written with that ordering in mind.
- Keep actor, target, and context constraints explicit. Avoid suppressions that only name a process without also constraining path, directory, image, user, namespace, or similar context.
- Use reusable building blocks first: `list` for named value sets, `macro` for reusable boolean expressions, and `rule` for the final detection.
- When a tool supports structured exceptions, prefer them over long chains of ad hoc negative clauses if the exception shape is stable and narrow.
- Keep names stable. New `rule` names should be unique; `macro` and `list` names should be reusable and consistent over time.

## Output Contract

For each generated rules file:
- include metadata/comment headers
- include rationale comments for non-obvious conditions
- avoid deleting unrelated existing local rules unless required
- preserve operator override space
- return only the final rules file contents when asked for a draft
