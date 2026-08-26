#!/bin/bash
# =================================================================
# OLS Panel Automated Installer for Debian 12
# =================================================================

set -e

# Color definitions
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m'

echo -e "${GREEN}=================================================${NC}"
echo -e "${GREEN}       OLS Panel Installer (Debian 12)           ${NC}"
echo -e "${GREEN}=================================================${NC}"

# Check Root
if [ "$EUID" -ne 0 ]; then
  echo -e "${RED}Error: Please run this script as root.${NC}"
  exit 1
fi

# Prompt for Custom SSH Port
read -p "Enter desired SSH Port [Default: 22]: " SSH_PORT
SSH_PORT=${SSH_PORT:-22}

echo -e "\n${YELLOW}[1/6] Updating system packages...${NC}"
apt-get update -y && apt-get upgrade -y
apt-get install -y curl wget git build-essential golang-go mariadb-server ufw

echo -e "\n${YELLOW}[2/6] Installing OpenLiteSpeed...${NC}"
if [ ! -d "/usr/local/lsws" ]; then
    wget --no-check-certificate -O - https://repo.litespeedtech.com/res/install.sh | bash
    apt-get install -y openlitespeed lsphp82 lsphp82-mysql lsphp82-common lsphp82-curl lsphp82-opcache
fi

echo -e "\n${YELLOW}[3/6] Applying RAM-based MySQL Optimizations...${NC}"
TOTAL_RAM=$(free -m | awk '/^Mem:/{print $2}')

if [ "$TOTAL_RAM" -le 1500 ]; then
    echo "[Optimization] VPS RAM <= 1.5GB detected. Applying 1GB preset..."
    cp scripts/configs/my-1gb.cnf /etc/mysql/mariadb.conf.d/50-server.cnf
elif [ "$TOTAL_RAM" -le 3500 ]; then
    echo "[Optimization] VPS RAM <= 3.5GB detected. Applying 2GB preset..."
    cp scripts/configs/my-2gb.cnf /etc/mysql/mariadb.conf.d/50-server.cnf
else
    echo "[Optimization] High-RAM VPS detected. Applying 4GB+ preset..."
    cp scripts/configs/my-4gb.cnf /etc/mysql/mariadb.conf.d/50-server.cnf
fi

systemctl restart mariadb

echo -e "\n${YELLOW}[4/6] Configuring Firewall & SSH Port...${NC}"
chmod +x scripts/firewall-setup.sh
./scripts/firewall-setup.sh "$SSH_PORT"

echo -e "\n${YELLOW}[5/6] Building Backend & Setting Up Systemd...${NC}"
mkdir -p /opt/ols-panel
cp -r backend frontend /opt/ols-panel/

cd /opt/ols-panel/backend
go build -o /opt/ols-panel/panel-backend main.go

# Create Systemd Service
cat << 'EOF' > /etc/systemd/system/ols-panel.service
[Unit]
Description=OLS Lite Control Panel Service
After=network.target lsws.service mariadb.service

[Service]
Type=simple
User=root
WorkingDirectory=/opt/ols-panel/backend
ExecStart=/opt/ols-panel/panel-backend
Restart=always
RestartSec=5

[Install]
WantedBy=multi-user.target
EOF

systemctl daemon-reload
systemctl enable ols-panel
systemctl restart ols-panel

# Generate Random Admin Password
ADMIN_USER="admin"
ADMIN_PASS=$(tr -dc A-Za-z0-9 </dev/urandom | head -c 16)

echo -e "\n${GREEN}=================================================${NC}"
echo -e "${GREEN}        INSTALLATION COMPLETED SUCCESSFULLY!      ${NC}"
echo -e "${GREEN}=================================================${NC}"
echo -e "Panel URL       : http://$(hostname -I | awk '{print $1}'):8080"
echo -e "Admin Username  : ${YELLOW}${ADMIN_USER}${NC}"
echo -e "Admin Password  : ${YELLOW}${ADMIN_PASS}${NC}"
echo -e "Configured SSH  : Port ${YELLOW}${SSH_PORT}${NC}"
echo -e "${GREEN}=================================================${NC}"
