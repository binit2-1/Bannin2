# Falco Rule Writing Guide

## Purpose

Write Falco custom rules that detect suspicious runtime behavior without flooding operators with expected workload activity.

## Where To Write Rules

- Primary custom file: `/etc/falco/rules.d/bannin-rules.yaml`
- Default local override file already loaded by Falco: `/etc/falco/falco_rules.local.yaml`
- Default Falco config that controls load order: `/etc/falco/falco.yaml`

Why this placement:
- Falco normally loads `/etc/falco/falco_rules.yaml`, then `/etc/falco/falco_rules.local.yaml`, then files in `/etc/falco/rules.d`.
- Keeping Bannin rules in `rules.d` avoids modifying vendor-managed default bundles directly.
- Because custom files are usually loaded after the default rules, overrides and appends can safely build on upstream lists, macros, and rules.

## Placement and Load-Order Rules

- Put more specific rules before more general rules when they share the same syscall family. Falco groups syscall rules by `evt.type` and then evaluates them in order.
- If you override or append to an upstream rule, assume your custom file must load after the upstream file.
- Prefer adding custom detections in `rules.d` and only use `falco_rules.local.yaml` when you intentionally want a compact local override file.

## Rule Authoring Model

Falco rules are easiest to maintain when split into:
- `list`: reusable value sets (binaries, namespaces, users, images)
- `macro`: reusable boolean conditions
- `rule`: final detection with `condition`, `output`, `priority`, `tags`
- `exceptions`: structured allowlisting for known-benign combinations

Why this model:
- You get consistent condition reuse, lower duplication, and cleaner noise tuning over time.

Visibility rules that matter:
- A `list` can only reference lists defined earlier.
- A `macro` can only reference macros defined earlier.
- A `macro` can reference any list.
- A `rule` can reference any macro.

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
- image repository
- working directory or executable path when relevant

4. Prefer medium/high confidence conditions before high priority.

5. Prefer rule-local structured `exceptions` when the safe allowlist shape is explicit and repeatable, such as actor plus target directory, process plus filename set, or image plus path family.

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

- Keep stable rule names. New rule names should start with an upper-case letter and be unique.
- Keep `list` and `macro` names in lowercase_with_underscores.
- Keep `desc` explicit and operational.
- End `desc` with a period.
- Keep `output` actionable with key fields (`proc`, `user`, `container`, `fd`, `evt`).
- Group tags by function (`bannin`, `network`, `filesystem`, `container`, etc.).
- Use exception names that explain business reason.
- Use the suggested Falco key order for rules: `rule`, `desc`, `condition`, `output`, `priority`, `tags`, then any extra keys.

## Condition Rules

- Every performance-sensitive rule should include a positive `evt.type` filter.
- Put the `evt.type` restriction at the beginning of the condition whenever possible.
- Do not use negative event-type matching such as `evt.type!=open`; it expands the rule across too many events and hurts performance.
- Avoid mixing unrelated event types in one rule. Only group close variants such as `open`, `openat`, and `openat2`.
- After the `evt.type` filter, place high-selectivity positive filters early so Falco can eliminate events quickly.
- Use `container` and other existing upstream macros when they match the needed semantics.
- Prefer `startswith` or `endswith` over `contains` when a prefix or suffix match is enough.
- Use parentheses aggressively for `or` branches and nested logic. More parentheses are safer than ambiguous precedence.
- Prefer `and not <positive subexpression>` over double-negation or negated `or` chains.

## Exceptions and Overrides

- Prefer overriding or appending existing upstream rules when the upstream rule already captures the core detection you want.
- Use `override` with `append` or `replace` intentionally. Do not use both on the same object.
- For rules, `append` is suited for extending `condition`, `output`, `desc`, `tags`, or `exceptions`.
- For rules, `replace` can also change `priority`, `enabled`, `warn_evttypes`, and `skip-if-unknown-filter`.
- Exceptions must stay narrow. A process-only exception is usually too broad; pair the actor with a directory, filename set, image, or equivalent target/context.
- When an exception uses `in`, the corresponding value should itself be a list.
- If the exception shape is stable but values vary by environment, define the exception schema first and let environment-specific values be appended later.

## Output and Priority

- Outputs should stay single-line and operator-friendly.
- Include the critical fields for triage: event type, process identity, user identity, target path or connection, and container context when applicable.
- Use Falco priorities as severity, not as ordering. Ordering is controlled by rule placement and file load order.
- Practical priority guidance:
  - writes or destructive state changes tend toward `ERROR`
  - sensitive reads tend toward `WARNING`
  - unexpected but less directly destructive behavior often fits `NOTICE`
  - noisier exploratory detections may need `DEBUG` or `INFORMATIONAL`

## Validation and Deployment

1. Validate syntax and load:
- server validates the generated draft before deployment
2. Deploy:
- server writes the validated file and restarts Falco
3. Observe first-hour hit rate and tune exceptions iteratively.

## Why These Choices

- Falco documentation emphasizes reusable structure (`lists`, `macros`, `rules`), load-order-aware overrides, and positive `evt.type` filtering for performance.
- Using dedicated local/custom files preserves upgrade safety and keeps tuning diff small.
- Structured exceptions let you encode legitimate behavior without weakening the core detection logic.
