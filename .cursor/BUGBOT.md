# Review Home Assistant App changes

## Software and operators

Home Assistant operators run Alloy, Private Data Source Connect, and Synthetic Monitoring through
these Apps. Trace startup through `config.yaml` and `Dockerfile`, then through the s6 services in
Alloy and PDC or the shared Go launcher for Synthetic Monitoring.
The Go ingress pages and shared MQTT reporter are executable source. Edit Synthetic Monitoring
variants through `synthetic_monitoring_shared/` and the common helper and reporter through `shared/`.

## Invariants

Preserve published option keys, App slugs, and slug-based MQTT identities. Missing `mqtt_enabled`
preserves default-on reporting. Explicit disable withdraws retained discovery and state and leaves
the reporter idle. Status pages distinguish unobserved metrics from observed zero. Keep the two
Synthetic Monitoring Dockerfiles aligned outside intentional variant differences.

## Sensitive changes

Trace Alloy configuration changes through local, Fleet-managed, hybrid, and break-glass startup.
Preserve upload and persisted settings bounds, the serialized merge and replacement, and safe-mode
recovery that archives the exact saved settings before resetting them. Check AppArmor and s6 changes
against real startup and allowed and denied access. Keep Supervisor credentials out of the PDC and
Synthetic Monitoring agent and UI environments while preserving the MQTT reporter's service access.

## Checks already in CI

Focus findings on behavioral regressions beyond format, YAML and shell lint, Go vet, and generated
file drift. Preserve the required `ci-success` result, its explicit no-change image exception, and
the publication queue that keeps pending version-changing pushes.

## Verification

Use `just check </dev/null` for source and contract checks. Exercise changed Alloy rendering with
`alloy/tests/generate-config.test.sh`, shared MQTT behavior from `shared/reporter/`, and Synthetic
Monitoring launcher and page behavior from their shared source. Use the Docker-backed `just ci`
checks for image startup or confinement changes, including real AppArmor enforcement where required.
