# shellcheck shell=bash
# ==============================================================================
# Shared option validators for the BroTEK Home Assistant Apps.
# ==============================================================================
# Source of truth: shared/lib/ha-validate.sh. Copies are generated into each
# App's rootfs by scripts/sync_shared_lib.py because the image build context is
# the App directory, so a Dockerfile cannot COPY from the repository root.
# Run that script after editing this file; CI runs it in --check mode.
#
# Every function here is a pure predicate: it returns 0 or 1 and prints nothing.
# Callers own their diagnostics, because the two Apps report failures
# differently (bashio::log.fatal plus bashio::exit.nok in Alloy, a local fatal
# helper in PDC) and their tests pin those exact strings.
#
# The one exception is ha_url_problem, which classifies rather than decides:
# Alloy distinguishes "contains a quote, backslash or whitespace" from the
# generic "is not a valid HTTP(S) URL", and that distinction has to survive.

# getent is used to confirm an IPv6 literal parses. Overridable for tests.
HA_GETENT_BIN="${HA_GETENT_BIN:-/usr/bin/getent}"

# A hostname or dotted domain, lowercase, at most 253 characters.
ha_valid_dns_name() {
    local value="$1"
    [[ ${#value} -le 253 ]] &&
        [[ "$value" =~ ^[a-z0-9]([a-z0-9-]{0,61}[a-z0-9])?(\.[a-z0-9]([a-z0-9-]{0,61}[a-z0-9])?)*$ ]]
}

# A single DNS label, lowercase, at most 63 characters.
ha_valid_dns_label() {
    local value="$1"
    [[ ${#value} -le 63 ]] &&
        [[ "$value" =~ ^[a-z0-9]([a-z0-9-]{0,61}[a-z0-9])?$ ]]
}

# A Go duration such as 5m, 1h30m or 1.5s. Bare 0 is accepted, as Go does.
ha_valid_duration() {
    local value="$1"
    [[ "$value" == 0 ]] ||
        [[ "$value" =~ ^(([0-9]+(\.[0-9]+)?|[0-9]*\.[0-9]+)(ns|us|µs|ms|s|m|h))+$ ]]
}

# A Go duration that is not zero. "0s" and "0" are both rejected.
ha_valid_positive_duration() {
    ha_valid_duration "$1" && [[ "$1" =~ [1-9] ]]
}

# A base-ten integer within an inclusive range. Leading zeroes are tolerated,
# which is why the comparison forces base ten rather than letting bash infer it.
ha_valid_integer_range() {
    local value="$1" minimum="$2" maximum="$3"
    [[ "$value" =~ ^[0-9]+$ ]] &&
        ((10#${value} >= minimum && 10#${value} <= maximum))
}

# The standard full and compressed IPv6 forms. An IPv4 dotted-decimal tail
# occupies two hextets, so validate it separately and substitute two hextets
# before applying the same structural check.
ha_valid_ipv6_literal() {
    local value="${1}" hex='[0-9A-Fa-f]{1,4}' pattern ipv4 octet
    if [[ "${value}" == *.* ]]; then
        [[ "${value}" == *:* ]] || return 1
        ipv4="${value##*:}"
        local -a octets
        IFS='.' read -r -a octets <<<"${ipv4}"
        [[ "${#octets[@]}" -eq 4 ]] || return 1
        for octet in "${octets[@]}"; do
            [[ "${octet}" =~ ^(0|[1-9][0-9]?|1[0-9]{2}|2[0-4][0-9]|25[0-5])$ ]] || return 1
        done
        value="${value%:*}:0:0"
    fi
    pattern="^(((${hex}:){7}${hex})|((${hex}:){1,7}:)|((${hex}:){1,6}:${hex})|((${hex}:){1,5}(:${hex}){1,2})|((${hex}:){1,4}(:${hex}){1,3})|((${hex}:){1,3}(:${hex}){1,4})|((${hex}:){1,2}(:${hex}){1,5})|(${hex}:((:${hex}){1,6}))|(:((:${hex}){1,7}|:)))$"
    [[ "${value}" =~ ${pattern} ]]
}

# Classify an HTTP(S) URL. Prints nothing when the value is acceptable (an empty
# value is acceptable, meaning "not configured"), "charset" when it carries a
# character that cannot survive interpolation into a quoted River string, and
# "invalid" for every structural problem. Callers turn the token into a message.
ha_url_problem() {
    local value="${1}"
    [ -n "${value}" ] || return 0
    case "${value}" in
        *'"'*|*[\\]*|*[[:space:]]*)
            printf '%s' charset
            return 0
            ;;
    esac

    local rest authority port="" unescaped ipv6_literal zone
    case "${value}" in
        http://*) rest="${value#http://}" ;;
        https://*) rest="${value#https://}" ;;
        *) printf '%s' invalid; return 0 ;;
    esac

    # Strip path, query and fragment, leaving the authority to validate. IPv6
    # literals must be bracketed so their colons cannot be mistaken for a port.
    authority="${rest%%[/?#]*}"
    local ipv6_authority='^\[([^]]+)\](:([0-9]+))?$'
    local host_authority='^[A-Za-z0-9._-]+(:([0-9]+))?$'
    if [[ "${authority}" =~ ${ipv6_authority} ]]; then
        ipv6_literal="${BASH_REMATCH[1]}"
        port="${BASH_REMATCH[3]}"
        # RFC 6874 requires the percent introducing an IPv6 zone identifier to
        # appear as %25 inside a URI. Zone IDs use URI-unreserved characters or
        # further percent escapes, which covers ordinary interface names.
        if [[ "${ipv6_literal}" == *%25* ]]; then
            zone="${ipv6_literal#*%25}"
            ipv6_literal="${ipv6_literal%%\%25*}"
            if [[ ! "${zone}" =~ ^([A-Za-z0-9._~-]|%[0-9A-Fa-f]{2})+$ ]]; then
                printf '%s' invalid
                return 0
            fi
        elif [[ "${ipv6_literal}" == *%* ]]; then
            printf '%s' invalid
            return 0
        fi
        if ! ha_valid_ipv6_literal "${ipv6_literal}"; then
            printf '%s' invalid
            return 0
        fi
    elif [[ "${authority}" =~ ${host_authority} ]]; then
        port="${BASH_REMATCH[2]}"
    else
        printf '%s' invalid
        return 0
    fi

    if [ -n "${port}" ] && { [ "${#port}" -gt 5 ] || ((10#${port} < 1 || 10#${port} > 65535)); }; then
        printf '%s' invalid
        return 0
    fi

    # Remove valid percent escapes one at a time; any percent left afterwards
    # begins an incomplete or non-hex escape.
    unescaped="${value}"
    while [[ "${unescaped}" =~ %[0-9A-Fa-f][0-9A-Fa-f] ]]; do
        unescaped="${unescaped/"${BASH_REMATCH[0]}"/}"
    done
    if [[ "${unescaped}" == *%* ]]; then
        printf '%s' invalid
        return 0
    fi
}

# A value that is safe to interpolate into a quoted River string. An empty value
# is safe; it renders as an empty string.
ha_valid_river_string() {
    local value="${1}"
    [ -n "${value}" ] || return 0
    [[ "${value}" != *'"'* && "${value}" != *\\* && ! "${value}" =~ [[:cntrl:]] ]]
}

# A host:port destination for an SSH remote-forward allowlist. The two OpenSSH
# sentinels are accepted verbatim, and either component may be the wildcard "*".
ha_valid_endpoint() {
    local endpoint="$1" host port label

    # PermitRemoteOpen's two sentinels are OpenSSH syntax, not host:port.
    [[ "$endpoint" == any || "$endpoint" == none ]] && return 0
    [[ "$endpoint" == */* ]] && return 1

    if [[ "$endpoint" =~ ^\[([0-9A-Fa-f:.]+)\]:(\*|[0-9]+)$ ]]; then
        host="${BASH_REMATCH[1]}"
        port="${BASH_REMATCH[2]}"
        [[ "$host" == *:* ]] || return 1
        "$HA_GETENT_BIN" ahosts "$host" >/dev/null 2>&1 || return 1
    elif [[ "$endpoint" == *:* ]]; then
        host="${endpoint%:*}"
        port="${endpoint##*:}"
        [[ "$host" != *:* && -n "$host" && "$port" =~ ^(\*|[0-9]+)$ ]] || return 1
        [[ "$host" != .* && "$host" != *. && "$host" != *..* ]] || return 1
        while IFS= read -r label; do
            [[ "$label" == '*' || "$label" =~ ^[[:alnum:]]([[:alnum:]-]*[[:alnum:]])?$ ]] || return 1
        done < <(tr '.' '\n' <<<"$host")
    else
        return 1
    fi

    [[ "$port" == '*' ]] || ha_valid_integer_range "$port" 1 65535
}
