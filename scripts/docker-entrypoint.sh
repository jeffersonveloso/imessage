#!/bin/bash
set -euo pipefail

# If the first argument is a known subcommand, pass everything directly to the binary.
# This supports: docker run ... mautrix-imessage login -c /data/config.yaml
case "${1:-}" in
    login|check-restore|list-handles|carddav-setup)
        exec /usr/local/bin/mautrix-imessage-v2 "$@"
        ;;
esac

# Default: normal bridge startup with -c <config>
CONFIG="/data/config.yaml"
if [ "${1:-}" = "-c" ] && [ -n "${2:-}" ]; then
    CONFIG="$2"
fi

# Generate default config if it doesn't exist
if [ ! -f "$CONFIG" ]; then
    echo "Config not found at $CONFIG — generating default..."
    /usr/local/bin/mautrix-imessage-v2 -c "$CONFIG" -e 2>/dev/null || true
    echo ""
    echo "═══════════════════════════════════════════════════════════"
    echo "  Default config generated at $CONFIG"
    echo ""
    echo "  Edit the config before starting the bridge. At minimum:"
    echo ""
    echo "  For API-only mode (no Matrix):"
    echo "    network:"
    echo "      api:"
    echo "        enabled: true"
    echo "        listen: \"0.0.0.0:8080\""
    echo "        api_key: \"$(openssl rand -hex 32 2>/dev/null || echo 'GENERATE_WITH_openssl_rand_-hex_32')\""
    echo ""
    echo "  For Matrix bridge mode:"
    echo "    homeserver:"
    echo "      address: http://your-homeserver:8008"
    echo "      domain: your-domain.com"
    echo "    permissions:"
    echo "      \"@you:your-domain.com\": admin"
    echo ""
    echo "  Then restart the container."
    echo "═══════════════════════════════════════════════════════════"
    exit 0
fi

exec /usr/local/bin/mautrix-imessage-v2 "$@"
