#!/usr/bin/env bash
set -euo pipefail

BINARY_NAME="mini_monitor_server"
INSTALL_DIR="/usr/local/bin"
THIRD_PARTY_DIR="/usr/local/lib/${BINARY_NAME}/bin"
CONFIG_FILE="/etc/${BINARY_NAME}.yaml"
SERVICE_FILE="/etc/systemd/system/${BINARY_NAME}.service"
DATA_DIR="/var/lib/${BINARY_NAME}"
BASIC_CONFIG_TEMPLATE="${BINARY_NAME}.example.yaml"
FULL_CONFIG_TEMPLATE="${BINARY_NAME}.full.example.yaml"

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"

usage() {
    echo "Usage: sudo $0 {install|install-basic|install-full|uninstall}"
    echo ""
    echo "  install         Install ${BINARY_NAME} with the basic config (alias of install-basic)"
    echo "  install-basic   Install ${BINARY_NAME} with disk-backed standard config"
    echo "  install-full    Install ${BINARY_NAME} with bundled VictoriaMetrics/vmagent and persistent full config"
    echo "  uninstall       Uninstall ${BINARY_NAME} and clean files from both basic and full installs"
    exit 1
}

check_root() {
    if [[ $EUID -ne 0 ]]; then
        echo "Error: please run as root (sudo $0 $1)"
        exit 1
    fi
}

backup_config_if_needed() {
    if [[ -f "${CONFIG_FILE}" ]]; then
        backup="${CONFIG_FILE}.bak.$(date +%Y%m%d%H%M%S)"
        cp "${CONFIG_FILE}" "${backup}"
        echo "  [ok] backup config -> ${backup}"
    fi
}

install_config() {
    local template_file="$1"
    backup_config_if_needed
    cp "${SCRIPT_DIR}/${template_file}" "${CONFIG_FILE}"
    echo "  [ok] config  -> ${CONFIG_FILE} (from ${template_file})"
}

install_common() {
    local mode="$1"
    local require_third_party="$2"
    echo "Installing ${BINARY_NAME} (${mode}) ..."

    cp "${SCRIPT_DIR}/${BINARY_NAME}" "${INSTALL_DIR}/${BINARY_NAME}"
    chmod +x "${INSTALL_DIR}/${BINARY_NAME}"
    echo "  [ok] binary  -> ${INSTALL_DIR}/${BINARY_NAME}"

    if [[ -d "${SCRIPT_DIR}/third_party" ]]; then
        mkdir -p "${THIRD_PARTY_DIR}"
        command install -m 0755 "${SCRIPT_DIR}/third_party/"* "${THIRD_PARTY_DIR}/"
        echo "  [ok] third-party binaries -> ${THIRD_PARTY_DIR}"
    elif [[ "${require_third_party}" == "true" ]]; then
        echo "Error: full install requires bundled third-party binaries under ${SCRIPT_DIR}/third_party"
        exit 1
    else
        echo "  [skip] third-party binaries directory not found"
    fi

    cp "${SCRIPT_DIR}/${BINARY_NAME}.service" "${SERVICE_FILE}"
    echo "  [ok] service -> ${SERVICE_FILE}"

    mkdir -p "${DATA_DIR}"
    echo "  [ok] data dir -> ${DATA_DIR}"

    systemctl daemon-reload
    systemctl enable "${BINARY_NAME}"
    echo "  [ok] service enabled"
}

do_install_basic() {
    check_root install-basic
    install_common "basic" "false"
    install_config "${BASIC_CONFIG_TEMPLATE}"

    echo ""
    echo "Done! Next steps:"
    echo "  1. Edit config:  vi ${CONFIG_FILE}"
    echo "  2. Start:        systemctl start ${BINARY_NAME}"
    echo "  3. Status:       systemctl status ${BINARY_NAME}"
}

do_install_full() {
    check_root install-full
    install_common "full" "true"
    install_config "${FULL_CONFIG_TEMPLATE}"

    echo ""
    echo "Done! Next steps:"
    echo "  1. Review config: vi ${CONFIG_FILE}"
    echo "  2. Start:         systemctl start ${BINARY_NAME}"
    echo "  3. Status:        systemctl status ${BINARY_NAME}"
}

do_uninstall() {
    check_root uninstall
    echo "Uninstalling ${BINARY_NAME} ..."

    systemctl stop "${BINARY_NAME}" 2>/dev/null || true
    systemctl disable "${BINARY_NAME}" 2>/dev/null || true
    echo "  [ok] service stopped and disabled"

    rm -f "${INSTALL_DIR}/${BINARY_NAME}"
    rm -f "${SERVICE_FILE}"
    rm -rf "/usr/local/lib/${BINARY_NAME}"
    rm -f "${CONFIG_FILE}"
    rm -rf "${DATA_DIR}"
    systemctl daemon-reload
    echo "  [ok] binary, third-party binaries, config, service file and data directory removed"

    echo ""
    echo "Uninstall complete."
}

case "${1:-}" in
    install|install-basic) do_install_basic ;;
    install-full)          do_install_full ;;
    uninstall)             do_uninstall ;;
    *)                     usage ;;
esac
