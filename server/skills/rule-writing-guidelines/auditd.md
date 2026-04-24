# auditd Rule Writing Guide

## Purpose

Write persistent auditd rules that monitor high-value system behavior with precise keys and low operational noise.

## Where To Write Rules

- Primary custom file: `/etc/audit/rules.d/bannin.rules`
- Generated merged rules file: `/etc/audit/audit.rules`
- Main daemon config: `/etc/audit/auditd.conf`
- Primary event log: `/var/log/audit/audit.log`

Why this placement:
- `augenrules` merges files from `rules.d` into `audit.rules`.
- Keeping Bannin rules in a dedicated `.rules` file avoids editing vendor-managed defaults directly.
- Operators can reason about one local file when tuning or rolling back.

## Authoring Model

Prefer these persistent rule shapes:
- syscall rules using `-a always,exit`
- file/path watches using `-w`
- clear audit keys using `-k`

Each rule should answer:
- what action is being audited
- which subject or object is in scope
- why the event matters
- which `-k` key makes the event easy to search later

## Noise-Reduction Strategy

1. Start from project-summary evidence:
- expected service users
- approved executable paths
- expected write locations
- deployment and maintenance jobs

2. Keep scope explicit:
- path
- executable
- architecture
- user or auid
- permission set

3. Use distinct keys per intent:
- `bannin_exec`
- `bannin_sensitive_write`
- `bannin_priv_esc`

4. Avoid blanket watches on broad directories unless the summary proves they are critical and low volume.

## Recommended Rule Layout

```text
# Bannin auditd custom rules
# Watch unexpected execution of privileged helpers.
-a always,exit -F arch=b64 -S execve -F path=/usr/bin/sudo -F auid>=1000 -F auid!=unset -k bannin_priv_esc

# Watch writes to application secrets.
-w /opt/app/secrets -p wa -k bannin_sensitive_write
```

## Required Rule Hygiene

- One complete rule per line.
- Start each active rule with an auditd directive such as `-a`, `-w`, `-D`, `-b`, `-f`, or `-e`.
- Keep comment lines short and explain intent, not syntax.
- Every detection-oriented rule should include a stable `-k` key unless the directive type does not support it.
- Prefer absolute paths.

## Validation and Deployment

1. Server asks the daemon to validate the draft before writing.
2. Daemon returns detailed validation output when a rule line is malformed or when `augenrules --load` rejects the persisted file.
3. The server feeds that exact error back to the AI rule writer until the rules load cleanly.
4. Successful deployment writes `/etc/audit/rules.d/bannin.rules`, loads rules, and restarts `auditd`.

## Why These Choices

- auditd is strongest when rules are persistent, searchable by key, and tightly scoped to high-value actions.
- Narrow syscall and watch rules reduce log volume while keeping forensic value.
- Dedicated custom rule files make rollback and operator review straightforward.
