# OLS Panel

A minimalist, fast, and secure control panel tailored for OpenLiteSpeed web servers on Debian.

## Features
- **Low Resource Usage:** Automatic RAM-based MySQL & OpenLiteSpeed tuning (Presets for 1GB, 2GB, and 4GB+ RAM).
- **Embedded Security:** Custom SSH Port setup & automated UFW firewall configuration.
- **Native Debian 13 (Trixie) & 12 (Bookworm) Support.**
- **Ultra Lightweight UI:** Pure HTML/JS frontend coupled with a compiled Golang backend.

## Quick Installation (One-Line Installer)
Run this command as `root` on a clean Debian VPS:

```bash
bash <(curl -sSL [https://raw.githubusercontent.com/musthari/OLS-Panel/main/install.sh](https://raw.githubusercontent.com/musthari/OLS-Panel/main/install.sh))
