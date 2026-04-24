# Wazuh Rule Writing Guide

## Purpose

Create Wazuh custom rules that improve detection fidelity for your environment while reducing repetitive, low-value alerts.

## Where To Write Rules

- Custom local rules: `/var/ossec/etc/rules/local_rules.xml`
- Custom local decoders (if needed): `/var/ossec/etc/decoders/local_decoder.xml`

Why this placement:
- Wazuh documentation recommends local files because stock rules/decoders can be overwritten during upgrades.

## Rule Authoring Principles

1. Prefer correlation over duplication:
- use `if_sid`, `if_group`, and contextual fields to refine existing detections

2. Keep local IDs in safe custom range:
- use custom rule IDs in documented local range (commonly `100000-120000`)

3. Encode business context:
- trusted hosts
- expected service accounts
- scheduled maintenance windows

4. Use severity (`level`) intentionally:
- higher levels only for high-confidence indicators
- use `level="0"` for explicit suppressions of known benign matches

## Example Pattern

```xml
<group name="bannin,custom,authentication">
  <!-- Correlate repeated auth failures only when source is not allowlisted -->
  <rule id="100210" level="8">
    <if_group>authentication_failed</if_group>
    <srcip negate="yes">10.10.0.0/16</srcip>
    <description>bannin: repeated auth failures from non-trusted source</description>
    <frequency>6</frequency>
    <timeframe>120</timeframe>
  </rule>

  <!-- Suppress known noisy scanner account to prevent operator fatigue -->
  <rule id="100211" level="0">
    <if_sid>100210</if_sid>
    <user>svc_vuln_scanner</user>
    <description>bannin suppression: approved scanner account</description>
  </rule>
</group>
```

## Decoder Guidance

Add/modify decoders only when rule logic cannot reliably match fields from existing decoded logs.

Decoder best practice:
- extract stable fields with clear names
- avoid overfitting one-off strings
- test against representative log samples

## Validation and Deployment

1. Validate with:
- `/var/ossec/bin/wazuh-logtest`
2. Restart manager:
- `systemctl restart wazuh-manager`
3. Monitor local rule hit rates and false-positive trends.

## Why These Choices

- Wazuh docs emphasize local custom files and `wazuh-logtest`-based validation.
- Correlation (`if_sid`/`if_group`) minimizes duplicate alerts and keeps logic attached to existing rule semantics.
- Explicit suppressions at level 0 are safer than disabling core upstream rules.
