#!/bin/bash
# =================================================================
# Dynamic Firewall Setup for OLS Panel (Auto-Detect SSH Port)
# =================================================================

set -e

echo "[Firewall] Installing and configuring UFW..."
apt-get update -y
apt-get install -y ufw

# Set Default Policies
ufw default deny incoming
ufw default allow outgoing

# 1. Otomatis deteksi Port SSH aktif dari config sshd / systemctl
SSH_PORT=$(ss -tulpn | grep -E 'sshd|ssh' | awk '{print $5}' | awk -F':' '{print $NF}' | head -n1)

# Fallback jika ss tidak menemukan, baca dari /etc/ssh/sshd_config
if [ -z "$SSH_PORT" ]; then
    SSH_PORT=$(grep -E "^Port " /etc/ssh/sshd_config | awk '{print $2}' | head -n1)
fi

# Fallback default ke port 22 jika masih kosong
SSH_PORT=${SSH_PORT:-22}

echo "[SSH] Detected active SSH Port: $SSH_PORT"

# 2. Buka port web, OLS Admin Console, OLS Panel, dan Port SSH hasil deteksi
ufw allow 80/tcp comment 'HTTP Web Traffic'
ufw allow 443/tcp comment 'HTTPS Web Traffic'
ufw allow 7080/tcp comment 'OpenLiteSpeed Admin Console'
ufw allow 8080/tcp comment 'OLS Control Panel UI'
ufw allow "$SSH_PORT"/tcp comment 'Detected SSH Port'

# 3. Aktifkan UFW tanpa konfirmasi interaktif
echo "y" | ufw enable

echo "[Firewall] UFW configured successfully. Active SSH Port ($SSH_PORT) is allowed."