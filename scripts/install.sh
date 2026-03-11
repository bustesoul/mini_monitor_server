#!/usr/bin/env bash
set -euo pipefail

BINARY_NAME="mini_monitor_server"
INSTALL_DIR="/usr/local/bin"
CONFIG_FILE="/etc/${BINARY_NAME}.yaml"
SERVICE_FILE="/etc/systemd/system/${BINARY_NAME}.service"
DATA_DIR="/var/lib/${BINARY_NAME}"

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"

usage() {
    echo "Usage: sudo $0 {install|uninstall}"
    echo ""
    echo "  install     Install ${BINARY_NAME} as a systemd service"
    echo "  uninstall   Stop, remove, and clean up ${BINARY_NAME}"
    exit 1
}

check_root() {
    if [[ $EUID -ne 0 ]]; then
        echo "Error: please run as root (sudo $0 $1)"
        exit 1
    fi
}

install() {
    check_root install
    echo "Installing ${BINARY_NAME} ..."

    cp "${SCRIPT_DIR}/${BINARY_NAME}" "${INSTALL_DIR}/${BINARY_NAME}"
    chmod +x "${INSTALL_DIR}/${BINARY_NAME}"
    echo "  [ok] binary  -> ${INSTALL_DIR}/${BINARY_NAME}"

    if [[ ! -f "${CONFIG_FILE}" ]]; then
        cp "${SCRIPT_DIR}/${BINARY_NAME}.example.yaml" "${CONFIG_FILE}"
        echo "  [ok] config  -> ${CONFIG_FILE} (from example)"
    else
        echo "  [skip] config already exists at ${CONFIG_FILE}"
    fi

    cp "${SCRIPT_DIR}/${BINARY_NAME}.service" "${SERVICE_FILE}"
    echo "  [ok] service -> ${SERVICE_FILE}"

    mkdir -p "${DATA_DIR}"
    echo "  [ok] data dir -> ${DATA_DIR}"

    systemctl daemon-reload
    systemctl enable "${BINARY_NAME}"
    echo "  [ok] service enabled"

    echo ""
    echo "Done! Next steps:"
    echo "  1. Edit config:  vi ${CONFIG_FILE}"
    echo "  2. Start:        systemctl start ${BINARY_NAME}"
    echo "  3. Status:       systemctl status ${BINARY_NAME}"
}

uninstall() {
    check_root uninstall
    echo "Uninstalling ${BINARY_NAME} ..."

    systemctl stop "${BINARY_NAME}" 2>/dev/null || true
    systemctl disable "${BINARY_NAME}" 2>/dev/null || true
    echo "  [ok] service stopped and disabled"

    rm -f "${INSTALL_DIR}/${BINARY_NAME}"
    rm -f "${SERVICE_FILE}"
    systemctl daemon-reload
    echo "  [ok] binary and service file removed"

    if [[ -f "${CONFIG_FILE}" ]]; then
        read -rp "  Remove config ${CONFIG_FILE}? [y/N]: " ans
        if [[ "${ans}" =~ ^[Yy]$ ]]; then
            rm -f "${CONFIG_FILE}"
            echo "  [ok] config removed"
        else
            echo "  [skip] config kept"
        fi
    fi

    if [[ -d "${DATA_DIR}" ]]; then
        read -rp "  Remove data directory ${DATA_DIR}? [y/N]: " ans
        if [[ "${ans}" =~ ^[Yy]$ ]]; then
            rm -rf "${DATA_DIR}"
            echo "  [ok] data directory removed"
        else
            echo "  [skip] data directory kept"
        fi
    fi

    echo ""
    echo "Uninstall complete."
}

case "${1:-}" in
    install)   install ;;
    uninstall) uninstall ;;
    *)         usage ;;
esac
