#!/bin/bash
# =================================================================
# Dynamic Firewall Setup for OLS Panel (IPv4 Only & Auto-Detect SSH)
# =================================================================

set -e

echo "[Firewall] Installing and configuring UFW..."
apt-get update -y
apt-get install -y ufw

# Disable IPv6 in UFW configuration
if [ -f "/etc/default/ufw" ]; then
    echo "[Firewall] Disabling IPv6 support in UFW..."
    sed -i 's/IPV6=yes/IPV6=no/' /etc/default/ufw
fi

# Set Default Policies
ufw default deny incoming
ufw default allow outgoing

# Auto-detect active SSH port
SSH_PORT=$(ss -tulpn | grep -E 'sshd|ssh' | awk '{print $5}' | awk -F':' '{print $NF}' | head -n1)

if [ -z "$SSH_PORT" ]; then
    SSH_PORT=$(grep -E "^Port " /etc/ssh/sshd_config | awk '{print $2}' | head -n1)
fi

SSH_PORT=${SSH_PORT:-22}

echo "[SSH] Detected active SSH Port: $SSH_PORT"

# Allow ports for IPv4
ufw allow 80/tcp comment 'HTTP Web Traffic'
ufw allow 443/tcp comment 'HTTPS Web Traffic'
ufw allow 7080/tcp comment 'OpenLiteSpeed Admin Console'
ufw allow 8080/tcp comment 'OLS Control Panel UI'
ufw allow "$SSH_PORT"/tcp comment 'Detected SSH Port'

echo "y" | ufw enable

echo "[Firewall] UFW configured successfully (IPv6 Disabled). Active SSH Port ($SSH_PORT) is allowed."