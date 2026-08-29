#!/usr/bin/env bash
# LOCAL AGENT WARNING: If you are not a Codex or Claude Code cloud agent, you MUST NOT
# execute this script. It changes the host toolchain and is only intended for cloud setup.
# Configure the environment's setup command as: bash scripts/cloud-environment-setup.sh

set -Eeuo pipefail

readonly BACKLOG_VERSION="1.50.1"
if ((EUID == 0)); then
  readonly NPM_PREFIX="/usr/local"
else
  readonly NPM_PREFIX="${HOME}/.local"
fi
readonly NPM_BIN="${NPM_PREFIX}/bin"

log() {
  printf 'cloud setup: %s\n' "$*"
}

persist_user_path() {
  mkdir -p "${NPM_BIN}"

  # Add Python user-base bin directory to PATH
  local python_user_bin
  python_user_bin="$(python3 -m site --user-base)/bin"
  mkdir -p "${python_user_bin}"

  # Update current shell PATH with both NPM and Python user bins
  case ":${PATH}:" in
    *":${NPM_BIN}:"*) ;;
    *) export PATH="${NPM_BIN}:${PATH}" ;;
  esac
  case ":${PATH}:" in
    *":${python_user_bin}:"*) ;;
    *) export PATH="${python_user_bin}:${PATH}" ;;
  esac

  # Codex setup and agent commands run in separate shells. Claude's cached
  # setup also retains files, not shell exports. Persist a user-local prefix;
  # /usr/local/bin is already on PATH when Claude runs this script as root.
  if [[ "${NPM_PREFIX}" == "${HOME}/.local" ]]; then
    local path_line="export PATH=\"\$HOME/.local/bin:\$(python3 -m site --user-base)/bin:\$PATH\""
    touch "${HOME}/.bashrc"
    grep -Fqx "${path_line}" "${HOME}/.bashrc" || printf '\n%s\n' "${path_line}" >> "${HOME}/.bashrc"
  else
    # When running as root, persist Python user-base bin to PATH
    local python_path_line="export PATH=\"\$(python3 -m site --user-base)/bin:\$PATH\""
    touch "${HOME}/.bashrc"
    grep -Fqx "${python_path_line}" "${HOME}/.bashrc" || printf '\n%s\n' "${python_path_line}" >> "${HOME}/.bashrc"
  fi
}

install_os_tools() {
  local -a missing=()
  command -v jq >/dev/null 2>&1 || missing+=(jq)
  command -v shellcheck >/dev/null 2>&1 || missing+=(shellcheck)

  ((${#missing[@]} == 0)) && return

  log "installing OS tools: ${missing[*]}"
  local -a privilege=()
  ((EUID == 0)) || privilege=(sudo)
  "${privilege[@]}" apt-get update
  "${privilege[@]}" env DEBIAN_FRONTEND=noninteractive \
    apt-get install --yes --no-install-recommends "${missing[@]}"
}

# Nothing is pip-installed. The six scripts that need a third-party import carry
# a PEP 723 header and are run with `uv run`; yamllint is run with `uvx`. Both
# resolve on first use, so this only has to prove uv is present and warm its
# cache while the network is known good, rather than failing mid-gate later.
verify_uv() {
  command -v uv >/dev/null 2>&1 || {
    log "uv is not installed; enable it in the cloud environment image"
    return 1
  }

  log "warming the uv cache for the gate's Python dependencies"
  uv run tests/app_contract_test.py >/dev/null
  uv run alloy/tests/config-schema.test.py >/dev/null
  uvx yamllint --version >/dev/null
}

install_backlog() {
  if command -v backlog >/dev/null 2>&1 && [[ "$(backlog --version)" == *"${BACKLOG_VERSION}"* ]]; then
    return
  fi

  log "installing Backlog.md ${BACKLOG_VERSION}"
  npm install --global --prefix "${NPM_PREFIX}" "backlog.md@${BACKLOG_VERSION}"
}

download_go_modules() {
  command -v go >/dev/null 2>&1 || {
    log "Go is not installed; enable it in the cloud environment"
    return 1
  }

  log "downloading Go module dependencies"
  while IFS= read -r -d '' module; do
    (cd "$(dirname "${module}")" && go mod download)
  done < <(find . -name go.mod -not -path './.git/*' -print0)
}

verify_tools() {
  log "verifying toolchain"
  uv --version
  uvx yamllint --version
  shellcheck --version | head -n 1
  node --version
  go version
  backlog --version

  if command -v docker >/dev/null 2>&1; then
    docker info >/dev/null 2>&1 || {
      log "Docker CLI is installed but daemon is not accessible; image tests require a Docker-enabled cloud environment"
      return 0
    }
    docker --version
  else
    log "Docker is unavailable; image tests require a Docker-enabled cloud environment"
  fi
}

main() {
  cd "$(git rev-parse --show-toplevel)"
  persist_user_path
  install_os_tools
  verify_uv
  install_backlog
  download_go_modules
  verify_tools
  log "complete"
}

main "$@"
