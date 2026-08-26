#!/bin/bash
# =================================================================
# OLS Panel Automated Installer for Debian 13 (Trixie) & 12 (Bookworm)
# =================================================================

set -e

RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m'

echo -e "${GREEN}=================================================${NC}"
echo -e "${GREEN}       OLS Panel Installer (Debian)             ${NC}"
echo -e "${GREEN}=================================================${NC}"

if [ "$EUID" -ne 0 ]; then
  echo -e "${RED}Error: Please run this script as root.${NC}"
  exit 1
fi

SCRIPT_DIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )" && pwd )"

if [ ! -d "$SCRIPT_DIR/scripts/configs" ]; then
    echo -e "${YELLOW}[Notice] Running via curl/remote. Downloading full repository...${NC}"
    apt-get update -y && apt-get install -y git
    rm -rf /tmp/OLS-Panel
    git clone https://github.com/musthari/OLS-Panel.git /tmp/OLS-Panel
    SCRIPT_DIR="/tmp/OLS-Panel"
    cd /tmp/OLS-Panel
fi

echo -e "\n${YELLOW}[1/6] Updating system packages...${NC}"
apt-get update -y && apt-get upgrade -y
apt-get install -y curl wget git build-essential golang-go mariadb-server ufw lsb-release gnupg2 ca-certificates net-tools

echo -e "\n${YELLOW}[2/6] Installing & Configuring OpenLiteSpeed Listeners (IPv4 Only)...${NC}"
if [ ! -d "/usr/local/lsws" ]; then
    OS_CODENAME=$(lsb_release -sc)
    wget -O /etc/apt/trusted.gpg.d/lst_repo.gpg http://rpms.litespeedtech.com/debian/lst_repo.gpg || \
    wget --no-check-certificate -O /etc/apt/trusted.gpg.d/lst_repo.gpg http://rpms.litespeedtech.com/debian/lst_repo.gpg

    echo "deb http://rpms.litespeedtech.com/debian/ ${OS_CODENAME} main" > /etc/apt/sources.list.d/openlitespeed.list
    apt-get update -y
    apt-get install -y openlitespeed lsphp82 lsphp82-mysql lsphp82-common lsphp82-curl lsphp82-opcache
fi

# Clean default listeners and recreate clean IPv4-only listeners without wildcard mapping
OLS_CONF="/usr/local/lsws/conf/httpd_config.conf"
if [ -f "$OLS_CONF" ]; then
    sed -i '/listener HTTP {/,/}/d' "$OLS_CONF"
    sed -i '/listener HTTPS {/,/}/d' "$OLS_CONF"

    cat << 'EOF' >> "$OLS_CONF"

listener HTTP {
  address                 0.0.0.0:80
  secure                  0
}

listener HTTPS {
  address                 0.0.0.0:443
  secure                  1
  keyFile                 /usr/local/lsws/admin/conf/webadmin.key
  certFile                /usr/local/lsws/admin/conf/webadmin.crt
}
EOF
fi

/usr/local/lsws/bin/lswsctrl restart

echo -e "\n${YELLOW}[3/6] Applying RAM-based MySQL Optimizations...${NC}"
TOTAL_RAM_KB=$(grep MemTotal /proc/meminfo | awk '{print $2}')
TOTAL_RAM_MB=$((TOTAL_RAM_KB / 1024))

echo "[Optimization] Total System RAM detected: ${TOTAL_RAM_MB} MB"

if [ "$TOTAL_RAM_MB" -le 1500 ]; then
    echo "[Optimization] VPS RAM <= 1.5GB detected. Applying 1GB preset..."
    CONFIG_FILE="$SCRIPT_DIR/scripts/configs/my-1gb.cnf"
elif [ "$TOTAL_RAM_MB" -le 3500 ]; then
    echo "[Optimization] VPS RAM <= 3.5GB detected. Applying 2GB preset..."
    CONFIG_FILE="$SCRIPT_DIR/scripts/configs/my-2gb.cnf"
else
    echo "[Optimization] High-RAM VPS detected. Applying 4GB+ preset..."
    CONFIG_FILE="$SCRIPT_DIR/scripts/configs/my-4gb.cnf"
fi

if [ -f "$CONFIG_FILE" ]; then
    cp "$CONFIG_FILE" /etc/mysql/mariadb.conf.d/50-server.cnf
    echo "[Optimization] Applied $(basename "$CONFIG_FILE") successfully."
else
    echo -e "${RED}[Warning] Configuration file $CONFIG_FILE not found! Skipping MariaDB tuning.${NC}"
fi

systemctl restart mariadb

echo -e "\n${YELLOW}[4/6] Configuring Firewall & Auto-detecting SSH...${NC}"
chmod +x "$SCRIPT_DIR/scripts/firewall-setup.sh"
"$SCRIPT_DIR/scripts/firewall-setup.sh"

echo -e "\n${YELLOW}[5/6] Building Backend & Setting Up Systemd...${NC}"
mkdir -p /opt/ols-panel
cp -r "$SCRIPT_DIR/backend" "$SCRIPT_DIR/frontend" "$SCRIPT_DIR/scripts" /opt/ols-panel/

cd /opt/ols-panel/backend

echo "[Go] Resolving module dependencies..."
go mod tidy

echo "[Go] Compiling backend binary..."
go build -o /opt/ols-panel/panel-backend main.go

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

ADMIN_USER="admin"
ADMIN_PASS=$(tr -dc A-Za-z0-9 </dev/urandom | head -c 16)

echo -e "\n${GREEN}=================================================${NC}"
echo -e "${GREEN}        INSTALLATION COMPLETED SUCCESSFULLY!      ${NC}"
echo -e "${GREEN}=================================================${NC}"
echo -e "Panel URL       : http://$(hostname -I | awk '{print $1}'):8080"
echo -e "Admin Username  : ${YELLOW}${ADMIN_USER}${NC}"
echo -e "Admin Password  : ${YELLOW}${ADMIN_PASS}${NC}"
echo -e "${GREEN}=================================================${NC}"