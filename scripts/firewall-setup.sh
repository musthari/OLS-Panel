#!/bin/bash
# =================================================================
# Initial Firewall Setup for OLS Panel (Debian 12)
# =================================================================

set -e

echo "[Firewall] Installing and configuring UFW..."
apt-get update -y
apt-get install -y ufw

# Set default policies
ufw default deny incoming
ufw default allow outgoing
#!/bin/bash
# =================================================================
# Dynamic Firewall & SSH Port Setup for OLS Panel (Debian 12)
# =================================================================

set -e

# Default SSH Port if not provided
SSH_PORT=${1:-22}

echo "[Firewall] Installing and configuring UFW..."
apt-get update -y
apt-get install -y ufw

# Set default policies
ufw default deny incoming
ufw default allow outgoing

# Configure Custom SSH Port if changed from 22
if [ "$SSH_PORT" -ne 22 ]; then
    echo "[SSH] Changing SSH port to $SSH_PORT in /etc/ssh/sshd_config..."
    
    # Backup original sshd_config
    cp /etc/ssh/sshd_config /etc/ssh/sshd_config.bak
    
    # Update Port in sshd_config
    sed -i "s/#Port 22/Port $SSH_PORT/" /etc/ssh/sshd_config
    sed -i "s/Port 22/Port $SSH_PORT/" /etc/ssh/sshd_config
    
    # Restart SSH service safely
    systemctl restart ssh || systemctl restart sshd
fi

# Allow Web & Panel ports
ufw allow 80/tcp comment 'HTTP Web Traffic'
ufw allow 443/tcp comment 'HTTPS Web Traffic'
ufw allow 7080/tcp comment 'OpenLiteSpeed Admin Console'
ufw allow 8443/tcp comment 'OLS Control Panel UI'

# Allow the configured SSH port
ufw allow "$SSH_PORT"/tcp comment 'Configured SSH Port'

# Enable firewall without confirmation prompt
echo "y" | ufw enable

echo "[Firewall] UFW configured successfully. Active SSH Port: $SSH_PORT"
