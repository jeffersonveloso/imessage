#!/bin/bash
set -euo pipefail

# If the first argument is a known subcommand, pass everything directly to the binary.
# This supports: docker run ... mautrix-imessage api-only -c /data/config.yaml
case "${1:-}" in
    api-only|login|check-restore|list-handles|carddav-setup)
        exec /usr/local/bin/mautrix-imessage-v2 "$@"
        ;;
esac

# Default: bridge startup with -c <config>
CONFIG="/data/config.yaml"
if [ "${1:-}" = "-c" ] && [ -n "${2:-}" ]; then
    CONFIG="$2"
fi

# Generate default config if it doesn't exist.
# Auto-patches the generated config for API-only Docker usage so
# the container is ready to start without manual editing.
if [ ! -f "$CONFIG" ]; then
    echo "Config not found at $CONFIG — generating default..."
    /usr/local/bin/mautrix-imessage-v2 -c "$CONFIG" -e 2>/dev/null || true

    API_KEY=$(openssl rand -hex 32 2>/dev/null || echo 'REPLACE_ME')
    WEBHOOK_SECRET=$(openssl rand -hex 32 2>/dev/null || echo '')

    # Patch the generated config for API-only Docker usage:
    #   - SQLite database at /data/
    #   - Admin permission for the login adapter
    #   - HTTP API enabled with generated keys
    sed -i \
        -e 's|type: postgres|type: sqlite3-fk-wal|' \
        -e 's|uri: postgres://user:password@host/database?sslmode=disable|uri: file:/data/mautrix-imessage.db?_txlock=immediate|' \
        -e 's|"@admin:example.com": admin|"@admin:api-only.local": admin|' \
        -e 's|enabled: false|enabled: true|' \
        -e "s|api_key: \"\"|api_key: \"$API_KEY\"|" \
        -e "s|webhook_secret: \"\"|webhook_secret: \"$WEBHOOK_SECRET\"|" \
        "$CONFIG"

    echo ""
    echo "═══════════════════════════════════════════════════════════"
    echo "  Config generated at $CONFIG (pre-configured for API-only)"
    echo ""
    echo "  API key: $API_KEY"
    echo ""
    echo "  The config is ready to use. Just restart the container:"
    echo "    docker start <container>"
    echo ""
    echo "  Optional: edit $CONFIG to set:"
    echo "    - network.api.webhook_url    (receive incoming events)"
    echo "    - network.api.api_key        (change the generated key)"
    echo "    - network.api.listen         (default: 0.0.0.0:8080)"
    echo ""
    echo "  Docs:  http://localhost:8080/api/v1/docs"
    echo "  Test:  curl -H 'Authorization: Bearer $API_KEY' \\"
    echo "           http://localhost:8080/api/v1/status"
    echo "═══════════════════════════════════════════════════════════"
    exit 0
fi

# Auto-detect mode from config.
# The default/unconfigured homeserver address is "http://example.localhost:8008".
# If it hasn't been changed, the user hasn't set up Matrix, so we start in
# api-only mode (which only requires api.enabled + permissions + database).
if grep -q 'address: http://example\.localhost' "$CONFIG" 2>/dev/null; then
    echo "Homeserver not configured — starting in API-only mode"
    exec /usr/local/bin/mautrix-imessage-v2 api-only "$@"
fi

exec /usr/local/bin/mautrix-imessage-v2 "$@"
