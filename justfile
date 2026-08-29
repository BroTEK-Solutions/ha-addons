set shell := ["bash", "-euo", "pipefail", "-c"]

# Go modules this repository OWNS. Everything under alloy/reporter,
# grafana_pdc/reporter, grafana_sm/ and grafana_sm_browser/ is a generated copy
# written by scripts/sync_shared_lib.py or
# scripts/sync_synthetic_monitoring_variants.py; formatting or linting those
# reports every finding twice and is reverted by the next `just gen`.
go_source_modules := "alloy/ui grafana_pdc/ui shared/reporter synthetic_monitoring_shared/launcher synthetic_monitoring_shared/ui"

# Show the task surface.
default:
    @just --list

# Download Go module dependencies and verify the host toolchain is complete.
[script('bash')]
setup:
    set -euo pipefail
    for module in {{ go_source_modules }}; do
        (cd "$module" && go mod download)
    done
    missing=()
    for tool in python3 node go gofmt yamllint shellcheck; do
        command -v "$tool" >/dev/null 2>&1 || missing+=("$tool")
    done
    python3 -c 'import voluptuous, yaml' >/dev/null 2>&1 || missing+=("python3 modules pyyaml+voluptuous")
    if ((${#missing[@]})); then
        printf 'setup: missing from this machine: %s\n' "${missing[*]}" >&2
        printf 'setup: cloud agents run `bash scripts/cloud-environment-setup.sh` instead\n' >&2
        exit 1
    fi
    command -v docker >/dev/null 2>&1 || printf 'setup: docker absent - the image lanes in `just ci` will fail\n' >&2

# Format Go sources and this justfile in place, then refresh generated copies.
[group('check')]
fmt: && gen
    gofmt -w {{ go_source_modules }}
    just --fmt

# Verify formatting without mutating files.
[group('check')]
[no-exit-message]
fmt-check:
    just --fmt --check
    unformatted="$(gofmt -l {{ go_source_modules }})"; if [ -n "$unformatted" ]; then printf 'unformatted Go sources:\n%s\n' "$unformatted" >&2; exit 1; fi

# Lint every YAML file and every tracked shell script, including the s6 services.
[group('check')]
[no-exit-message]
lint:
    yamllint --strict .
    shellcheck -x --source-path=SCRIPTDIR \
        shared/lib/ha-validate.sh \
        tests/shared_validate_lib_test.sh \
        alloy/rootfs/usr/share/alloy/generate-config.sh \
        alloy/rootfs/etc/s6-overlay/s6-rc.d/init-alloy/run \
        alloy/rootfs/etc/s6-overlay/s6-rc.d/alloy/run \
        alloy/rootfs/etc/s6-overlay/s6-rc.d/alloy/finish \
        alloy/rootfs/etc/s6-overlay/s6-rc.d/alloy-reporter/run \
        alloy/rootfs/etc/s6-overlay/s6-rc.d/alloy-reporter/finish \
        alloy/tests/generate-config.test.sh \
        alloy/tests/init-alloy.test.sh \
        grafana_pdc/rootfs/etc/s6-overlay/s6-rc.d/init-grafana-pdc/run \
        grafana_pdc/rootfs/etc/s6-overlay/s6-rc.d/grafana-pdc/run \
        grafana_pdc/rootfs/etc/s6-overlay/s6-rc.d/grafana-pdc/finish \
        grafana_pdc/rootfs/etc/s6-overlay/s6-rc.d/grafana-pdc-reporter/run \
        grafana_pdc/rootfs/etc/s6-overlay/s6-rc.d/grafana-pdc-reporter/finish \
        grafana_pdc/rootfs/etc/s6-overlay/s6-rc.d/grafana-pdc-ui/run \
        grafana_pdc/rootfs/etc/s6-overlay/s6-rc.d/grafana-pdc-ui/finish \
        grafana_pdc/tests/service.test.sh \
        grafana_pdc/tests/image-smoke.test.sh \
        grafana_pdc/tests/fixtures/bashio \
        grafana_pdc/tests/fixtures/fake-pdc

# Rewrite every generated copy from its source tree.
[group('gen')]
gen:
    python3 scripts/sync_shared_lib.py
    python3 scripts/sync_synthetic_monitoring_variants.py

# Fail when a generated copy has drifted from its source.
[group('gen')]
[no-exit-message]
gen-check:
    python3 scripts/sync_shared_lib.py --check
    python3 scripts/sync_synthetic_monitoring_variants.py --check

# Run every test that needs only the language toolchain.
[group('check')]
test: test-repo test-pdc test-alloy test-sm

# Run repository contract tests, the shared validator library, and the MQTT reporter.
[group('check')]
[no-exit-message]
test-repo:
    bash tests/shared_validate_lib_test.sh
    (cd shared/reporter && go test ./...)
    python3 tests/app_metadata_contract_test.py
    python3 tests/app_contract_test.py
    python3 tests/app_version_changed_test.py
    python3 tests/renovate_config_contract_test.py
    python3 tests/repository_workflow_contract_test.py
    python3 tests/synthetic_monitoring_variants_test.py

# Run Grafana PDC's schema, image-contract, and status-page tests.
[group('check')]
[no-exit-message]
test-pdc:
    python3 grafana_pdc/tests/config-schema.test.py
    python3 grafana_pdc/tests/image-contract.test.py
    (cd grafana_pdc/ui && go test ./...)

# Run Grafana PDC's Docker-backed service and complete-image tests.
[group('check')]
[no-exit-message]
test-pdc-image:
    ha_base="$(awk '/^FROM ghcr.io\/home-assistant\/base:/ { print $2; exit }' grafana_pdc/Dockerfile)"; \
        test -n "$ha_base"; \
        docker run --rm -e BASHIO_BIN=/usr/bin/bashio -v "${PWD}:/w" -w /w \
        "$ha_base" bash grafana_pdc/tests/service.test.sh
    docker build --tag local/ha-grafana-pdc:smoke grafana_pdc
    bash grafana_pdc/tests/image-smoke.test.sh local/ha-grafana-pdc:smoke

# Run Alloy's schema, image-contract, UI, browser-state, and generator tests.
[group('check')]
[no-exit-message]
test-alloy:
    python3 alloy/tests/config-schema.test.py
    python3 alloy/tests/image-contract.test.py
    (cd alloy/ui && go test ./...)
    node alloy/tests/ui-static.test.mjs
    bash alloy/tests/generate-config.test.sh

# Run Alloy's Docker-backed complete-image smoke test.
[group('check')]
[no-exit-message]
test-alloy-image:
    docker build --tag local/ha-alloy:smoke alloy
    bash alloy/tests/image-smoke.test.sh local/ha-alloy:smoke

# Run Alloy's initialization-service test inside its pinned base image.
[group('check')]
[no-exit-message]
test-alloy-init:
    alloy_base="$(awk -F= '/^ARG BUILD_FROM=ghcr.io\/home-assistant\/base-debian:/ { print $2; exit }' alloy/Dockerfile)"; \
        test -n "$alloy_base"; \
        docker run --rm -v "${PWD}:/w" -w /w \
        "$alloy_base" bash alloy/tests/init-alloy.test.sh

# Run the shared Synthetic Monitoring launcher tests.
[group('check')]
[no-exit-message]
test-sm:
    (cd synthetic_monitoring_shared/launcher && go test ./...)

# Build and smoke-test both Docker-backed Synthetic Monitoring images.
[group('check')]
[no-exit-message]
test-sm-image:
    docker build --tag local/ha-grafana-sm:smoke grafana_sm
    docker build --tag local/ha-grafana-sm-browser:smoke grafana_sm_browser
    python3 tests/synthetic_monitoring_image_smoke_test.py local/ha-grafana-sm:smoke standard
    python3 tests/synthetic_monitoring_image_smoke_test.py local/ha-grafana-sm-browser:smoke browser

# Run the toolchain-only pre-commit gate.
[group('check')]
check: fmt-check lint gen-check test

# Docker daemon required by test-pdc-image, test-alloy-image, test-alloy-init, and test-sm-image.
# Run the CI-equivalent gate.
[group('check')]
ci: check test-pdc-image test-alloy-image test-alloy-init test-sm-image

# Compile every source Go module without keeping artifacts.
[group('build')]
build:
    for module in {{ go_source_modules }}; do (cd "$module" && go build ./...); done

# Build one App's complete image locally.
[group('build')]
image app tag="local/ha-dev:smoke":
    docker build --tag '{{ tag }}' '{{ app }}'

# Delete the local `go build` outputs listed in .gitignore.
[group('dev')]
clean:
    rm -f alloy/ui/alloy-ui grafana_pdc/ui/grafana-pdc-ui shared/reporter/ha-reporter synthetic_monitoring_shared/launcher/synthetic-monitoring-launcher
