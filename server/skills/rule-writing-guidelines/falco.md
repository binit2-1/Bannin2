# Falco Rule Writing Guide

## Purpose

Write Falco custom rules that detect suspicious runtime behavior without flooding operators with expected workload activity.

## Where To Write Rules

- Primary custom file: `/etc/falco/rules.d/bannin-rules.yaml`
- overwriting config to validate includes/load order: `/etc/falco/config.d/bannin.yaml`

Why this placement:
- Falco loads multiple rules files; keeping bannin rules in `rules.d` and local overrides in local files avoids modifying upstream default bundles directly.

## Rule Authoring Model

Falco rules are easiest to maintain when split into:
- `list`: reusable value sets (binaries, namespaces, users, images)
- `macro`: reusable boolean conditions
- `rule`: final detection with `condition`, `output`, `priority`, `tags`
- `exceptions`: structured allowlisting for known-benign combinations

Why this model:
- You get consistent condition reuse, lower duplication, and cleaner noise tuning over time.

## Noise-Reduction Strategy

1. Start from project-summary:
- expected container images
- known process trees
- maintenance jobs
- CI/CD automation

2. Encode known-benign patterns in `lists` and `exceptions`, not by weakening core detections.

3. Scope by runtime context:
- container vs host
- namespace
- user/process ancestry
- file path families

4. Prefer medium/high confidence conditions before high priority.

## Recommended Rule Layout

```yaml
- list: bannin_trusted_images
  items: [my-registry/payment-api, my-registry/worker]

- macro: bannin_expected_runtime
  condition: (container.image.repository in (bannin_trusted_images))

- rule: bannin Unexpected Sensitive File Read
  desc: Detect sensitive file reads outside expected runtime context.
  condition: open_read and fd.name startswith /etc/ssl and not bannin_expected_runtime
  output: >
    Sensitive file read outside expected image
    (user=%user.name proc=%proc.name file=%fd.name container=%container.id)
  priority: WARNING
  tags: [bannin, filesystem, tuning]
  exceptions:
    - name: known_debug_sessions
      fields: [proc.name, user.name]
      comps: [in, in]
      values:
        - [strace, root]
```

## Required Rule Hygiene

- Keep stable rule names (do not churn names unless behavior changes).
- Keep `desc` explicit and operational.
- Keep `output` actionable with key fields (`proc`, `user`, `container`, `fd`, `evt`).
- Group tags by function (`bannin`, `network`, `filesystem`, `container`, etc.).
- Use exception names that explain business reason.

## Validation and Deployment

1. Validate syntax and load:
- `Use Tool validateRules before restart`
2. Deploy:
- `Use Tool RestartService`
3. Observe first-hour hit rate and tune exceptions iteratively.

## Why These Choices

- Falco documentation emphasizes reusable structure (`lists`, `macros`, `rules`) and explicit exception handling for legitimate behavior.
- Using dedicated local/custom files preserves upgrade safety and keeps tuning diff small.
- Exception-first tuning avoids blinding core detection logic.
