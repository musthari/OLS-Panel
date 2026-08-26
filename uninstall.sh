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
    ufw delete allow 8443/tcp || true
    ufw delete allow 8080/tcp || true
    echo "[Firewall] Panel ports removed from UFW rules."
fi

echo -e "\n${YELLOW}[4/4] Preserving OpenLiteSpeed & Databases...${NC}"
echo -e "${GREEN}Note: OpenLiteSpeed, MariaDB, and PHP packages were kept intact to protect your web data.${NC}"

echo -e "\n${GREEN}=================================================${NC}"
echo -e "${GREEN}     OLS Panel Successfully Uninstalled!         ${NC}"
echo -e "${GREEN}=================================================${NC}"
