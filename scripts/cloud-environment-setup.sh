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
  case ":${PATH}:" in
    *":${NPM_BIN}:"*) ;;
    *) export PATH="${NPM_BIN}:${PATH}" ;;
  esac

  # Codex setup and agent commands run in separate shells. Claude's cached
  # setup also retains files, not shell exports. Persist a user-local prefix;
  # /usr/local/bin is already on PATH when Claude runs this script as root.
  [[ "${NPM_PREFIX}" == "${HOME}/.local" ]] || return 0
  local path_line="export PATH=\"\$HOME/.local/bin:\$PATH\""
  touch "${HOME}/.bashrc"
  grep -Fqx "${path_line}" "${HOME}/.bashrc" || printf '\n%s\n' "${path_line}" >> "${HOME}/.bashrc"
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

install_python_tools() {
  log "installing Python validation dependencies"
  python3 -m pip install --user --disable-pip-version-check --quiet \
    pyyaml \
    voluptuous \
    yamllint
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
  python3 -c 'import voluptuous, yaml'
  yamllint --version
  shellcheck --version | head -n 1
  node --version
  go version
  backlog --version

  if command -v docker >/dev/null 2>&1; then
    docker --version
  else
    log "Docker is unavailable; image tests require a Docker-enabled cloud environment"
  fi
}

main() {
  cd "$(git rev-parse --show-toplevel)"
  persist_user_path
  install_os_tools
  install_python_tools
  install_backlog
  download_go_modules
  verify_tools
  log "complete"
}

main "$@"
