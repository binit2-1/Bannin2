# Suricata Rule Writing Guide

## Purpose

Generate Suricata rules that detect meaningful threats while controlling alert volume from known legitimate traffic patterns.

## Where To Write Rules

- Custom bannin rule file: `/etc/suricata/rules/bannin.rules`
- Common local include file: `/etc/suricata/rules/local.rules`
- Suricata config: `/etc/suricata/suricata.yaml`

Ensure `suricata.yaml` points to your rule files via:
- `default-rule-path`
- `rule-files`

Why this placement:
- Keeping custom rules in local files avoids collisions with vendor-fed rule updates and makes review/deployment deterministic.

## Rule Anatomy (Baseline)

A Suricata signature should have:
- clear protocol/flow scope
- precise content or behavior conditions
- informative `msg`
- stable `sid` and incremented `rev`
- useful metadata/classification

Example skeleton:

```text
alert http $HOME_NET any -> $EXTERNAL_NET any (
  msg:"bannin suspicious outbound API token exfil pattern";
  flow:established,to_server;
  http.method; content:"POST";
  http.uri; content:"/upload";
  content:"Authorization|3a 20|Bearer "; nocase;
  classtype:policy-violation;
  sid:9901001; rev:1;
)
```

## Noise-Reduction Strategy

1. Scope early:
- restrict source/destination network vars
- use app-layer keywords (`http.*`, `dns.*`, etc.) instead of raw content only

2. Use rate controls:
- `threshold`, `detection_filter`, or suppression strategy when same benign event repeats often

3. Prefer deterministic anchors:
- URI prefix + method + header patterns together
- flow direction and state constraints

4. Avoid broad one-token signatures that match common business traffic.

## Change Management

- Keep `sid` stable for an alert concept.
- Bump `rev` when logic changes.
- Keep messages operational and searchable.
- Record rationale for every suppress/threshold decision.

## Validation and Deployment

1. Syntax/config test:
- `suricata -T -c /etc/suricata/suricata.yaml`
2. Apply updates:
- `suricatasc -c reload-rules` (preferred when supported)
3. Track hit quality for at least one normal traffic cycle.

## Why These Choices

- Suricata docs stress central config (`suricata.yaml`) rule-file loading and explicit thresholding/suppression controls.
- Using app-layer keywords plus directional flow sharply improves precision versus raw payload-only matching.
- Stable `sid/rev` handling preserves historical analytics integrity.
