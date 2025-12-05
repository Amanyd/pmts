#!/bin/sh
set -e

API_KEY="${DATACAT_KEY}"
API_URL="${DATACAT_URL:-http://localhost:8080/api/ingest}"

if [ -z "$API_KEY" ]; then
    echo "Error: DATACAT_KEY is missing."
    echo "Usage: curl ... | DATACAT_KEY=sk_123 sh"
    exit 1
fi

echo "Installing DataCat Agent..."

ARCH=$(uname -m)
case $ARCH in
    x86_64)
        BINARY_URL="http://localhost:5173/agent-linux" 
        ;;
    aarch64)
        echo "ARM64 detected. Assuming agent-linux is compatible or using fallback..."
        BINARY_URL="http://localhost:5173/agent-linux"
        ;;
    *)
        echo "Unsupported architecture: $ARCH"
        exit 1
        ;;
esac

echo "Downloading agent..."
if ! curl -sfL "$BINARY_URL" -o /usr/local/bin/datacat-agent; then
    echo "Failed to download agent. Check if the server is reachable."
    exit 1
fi

chmod +x /usr/local/bin/datacat-agent

echo "Configuring systemd service..."
cat <<EOF > /etc/systemd/system/datacat.service
[Unit]
Description=DataCat Metrics Agent
After=network.target

[Service]
Type=simple
User=root
ExecStart=/usr/local/bin/datacat-agent --key=$API_KEY --target=$API_URL --scrape=http://localhost:3000/metrics
Restart=always
RestartSec=5s

[Install]
WantedBy=multi-user.target
EOF

echo "Starting DataCat..."
systemctl daemon-reload
systemctl enable datacat
systemctl restart datacat

echo "Installation Complete! DataCat is running in the background."