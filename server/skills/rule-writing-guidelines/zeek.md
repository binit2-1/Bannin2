# Zeek Rule Writing Guide

## Purpose

Write Zeek policy scripts that generate high-confidence notices aligned to your project behavior, while suppressing recurring benign events.

## Where To Write Rules/Scripts

- bannin policy script: `/usr/local/zeek/share/zeek/site/bannin.zeek`
- Site entrypoint (load file): `/usr/local/zeek/share/zeek/site/local.zeek`

Typical deployment step:
- ensure `@load ./bannin` (or equivalent) is in `local.zeek`

Why this placement:
- Zeek site policy scripts are designed for local customization and survive normal package upgrades better than editing upstream base scripts.

## Preferred Detection Approach

1. Use Zeek script logic and events first.
2. Use Notice framework for operator-facing alerts.
3. Use signatures only for narrow legacy cases (Zeek docs note signatures are not the preferred general detection path).

## Noise-Reduction Strategy

1. Bind detections to explicit context:
- service/protocol
- host roles
- directionality
- known maintenance traffic

2. Use `Notice::policy` hooks:
- set tuned notice actions
- suppress known-benign recurring notices using identifiers and suppression windows

3. Add clear local constants/sets for allowlisted hosts/domains and keep them easy to review.

## Example Pattern

```zeek
module bannin;

export {
  const allowed_backup_hosts: set[addr] = { 10.20.10.15 } &redef;
}

event zeek_init()
  {
  @load base/frameworks/notice
  }

event connection_established(c: connection)
  {
  if ( c$id$orig_h !in allowed_backup_hosts && c$id$resp_p == 22/tcp )
    {
    NOTICE([$note=Notice::ACTION_NEEDED,
            $msg=fmt("bannin: unexpected SSH destination %s", c$id$resp_h),
            $identifier=fmt("%s-%s", c$id$orig_h, c$id$resp_h)]);
    }
  }

event Notice::policy(n: Notice::Info)
  {
  if ( n$identifier == "known-benign-identifier" )
    n$suppress_for = 30min;
  }
```

## Validation and Deployment

1. Validate script availability and load state:
- `zeek -N`
2. Deploy policy changes:
- `zeekctl deploy`
3. Confirm notices are meaningful and suppression behavior is correct.

## Why These Choices

- Zeek documentation positions scripts + notice framework as the strongest path for contextual detection.
- Identifier-based suppression is safer than deleting detection logic because it preserves intentional visibility controls.
- Keeping local policy in site scripts keeps customization isolated and reviewable.
