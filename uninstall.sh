#!/bin/bash
# =================================================================
# OLS Panel Uninstaller Script for Debian 12
# =================================================================

set -e

# Color definitions
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m'

echo -e "${RED}=================================================${NC}"
echo -e "${RED}         OLS Panel Uninstaller                   ${NC}"
echo -e "${RED}=================================================${NC}"

# Check Root
if [ "$EUID" -ne 0 ]; then
  echo -e "${RED}Error: Please run this script as root.${NC}"
  exit 1
fi

read -p "Are you sure you want to uninstall OLS Panel? [y/N]: " CONFIRM
if [[ "$CONFIRM" != "y" && "$CONFIRM" != "Y" ]]; then
    echo -e "${YELLOW}Uninstallation cancelled.${NC}"
    exit 0
fi

echo -e "\n${YELLOW}[1/4] Stopping and removing OLS Panel systemd service...${NC}"
if systemctl is-active --quiet ols-panel; then
    systemctl stop ols-panel
fi
if systemctl is-enabled --quiet ols-panel; then
    systemctl disable ols-panel
fi

if [ -f "/etc/systemd/system/ols-panel.service" ]; then
    rm -f /etc/systemd/system/ols-panel.service
    systemctl daemon-reload
fi

echo -e "\n${YELLOW}[2/4] Removing OLS Panel application files...${NC}"
if [ -d "/opt/ols-panel" ]; then
    rm -rf /opt/ols-panel
    echo "[Clean] /opt/ols-panel directory removed."
fi

echo -e "\n${YELLOW}[3/4] Resetting Firewall Rules...${NC}"
if command -v ufw >/dev/null 2>&1; then
    # Hapus aturan port 8080 dan 8443 berdasarkan kueri nomor aturan UFW
    for PORT in "8080" "8443"; do
        # Loop untuk menghapus semua aturan (IPv4 & IPv6) yang mengandung port tersebut
        while ufw status numbered | grep -q "$PORT"; do
            NUM=$(ufw status numbered | grep "$PORT" | tail -n1 | sed -E 's/\[ *([0-9]+)\].*/\1/')
            if [ -n "$NUM" ]; then
                echo "y" | ufw delete "$NUM" >/dev/null 2>&1
            else
                break
            fi
        done
    done
    echo "[Firewall] Panel ports successfully cleaned from UFW."
fi

echo -e "\n${YELLOW}[4/4] Preserving OpenLiteSpeed & Databases...${NC}"
echo -e "${GREEN}Note: OpenLiteSpeed, MariaDB, and PHP packages were kept intact to protect your web data.${NC}"

echo -e "\n${GREEN}=================================================${NC}"
echo -e "${GREEN}     OLS Panel Successfully Uninstalled!         ${NC}"
echo -e "${GREEN}=================================================${NC}"
