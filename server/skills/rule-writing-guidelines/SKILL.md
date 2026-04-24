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
6. Validate syntax and apply reload/restart strategy from `bannin.yaml`.

## Rule Quality Criteria

- High signal: catches risky behavior tied to project threat model.
- Low noise: avoids known benign behavior, scheduled jobs, and trusted service accounts.
- Maintainable: clear names, stable identifiers, readable grouping, minimal duplication.
- Deployable: writes to the configured path and follows tool-native syntax.

## Output Contract

For each generated rules file:
- include metadata/comment headers
- include rationale comments for non-obvious conditions
- avoid deleting unrelated existing local rules unless required
- preserve operator override space
