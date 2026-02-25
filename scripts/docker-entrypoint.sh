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

# Generate default config if it doesn't exist
if [ ! -f "$CONFIG" ]; then
    echo "Config not found at $CONFIG — generating default..."
    /usr/local/bin/mautrix-imessage-v2 -c "$CONFIG" -e 2>/dev/null || true

    API_KEY=$(openssl rand -hex 32 2>/dev/null || echo 'GENERATE_WITH_openssl_rand_-hex_32')

    echo ""
    echo "═══════════════════════════════════════════════════════════"
    echo "  Default config generated at $CONFIG"
    echo ""
    echo "  Edit the config before starting the container."
    echo ""
    echo "  ── API-only mode (no Matrix) ─────────────────────────"
    echo "  Set these in config.yaml:"
    echo ""
    echo "    database:"
    echo "      type: sqlite3-fk-wal"
    echo "      uri: file:/data/mautrix-imessage.db?_txlock=immediate"
    echo ""
    echo "    bridge:"
    echo "      permissions:"
    echo "        \"@admin:api-only.local\": admin"
    echo ""
    echo "    network:"
    echo "      api:"
    echo "        enabled: true"
    echo "        listen: \"0.0.0.0:8080\""
    echo "        api_key: \"$API_KEY\""
    echo ""
    echo "  Then restart the container."
    echo ""
    echo "  ── Matrix bridge mode ────────────────────────────────"
    echo "  Configure homeserver.address, homeserver.domain,"
    echo "  and bridge.permissions, then restart the container."
    echo ""
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
