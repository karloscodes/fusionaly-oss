#!/bin/bash
# Installs Fusionaly on this server with Chasen (https://chasenhq.com):
# HTTPS, live backups of the database, and an update each night.
#   curl -fsSL https://fusionaly.com/install | sudo bash -s data.example.com
#
# The domain is the first argument, or FUSIONALY_DOMAIN. Without either, it
# asks. FUSIONALY_IMAGE deploys another image than karloscodes/fusionaly:latest,
# for a test. Run it again to update Fusionaly now.
#
# A Fusionaly Pro install of the older installer moves to the free Fusionaly with:
#   curl -fsSL https://fusionaly.com/install | sudo bash -s migrate-to-oss
#
# fusionaly.com/install serves this file from the main branch: a push here
# reaches people at once.
set -euo pipefail

RED='\033[0;31m'
GREEN='\033[0;32m'
NC='\033[0m'
image="${FUSIONALY_IMAGE:-karloscodes/fusionaly:latest}"

fail() { echo -e "${RED}Error: $*${NC}" >&2; exit 1; }

[ "$(id -u)" -eq 0 ] || fail "run it as root: curl -fsSL https://fusionaly.com/install | sudo bash -s data.example.com"
[ "$(uname -s)" = Linux ] || fail "Fusionaly installs on a Linux server. From your computer, deploy it with Chasen: https://fusionaly.com/docs/installation/"

# Fusionaly Pro to the free Fusionaly, on a server of the older installer: its
# own command does it, from the newest release, checked against its checksum.
migrate_to_oss() {
	case "$(uname -m)" in
		x86_64) arch=amd64 ;;
		aarch64 | arm64) arch=arm64 ;;
		*) fail "no build for $(uname -m)" ;;
	esac
	base="https://github.com/karloscodes/fusionaly-oss/releases/latest/download"
	tmp="$(mktemp -d)"
	curl -fsSL -o "$tmp/fusionaly" "$base/fusionaly-linux-$arch" || fail "cannot download the fusionaly command"
	curl -fsSL -o "$tmp/checksums.txt" "$base/checksums.txt" || fail "cannot download checksums.txt"
	expected="$(grep " fusionaly-linux-$arch\$" "$tmp/checksums.txt" | cut -d' ' -f1)"
	[ -n "$expected" ] && [ "$(sha256sum "$tmp/fusionaly" | cut -d' ' -f1)" = "$expected" ] || fail "the checksum of the fusionaly command does not match"
	install -m 755 "$tmp/fusionaly" /usr/local/bin/fusionaly
	rm -f "$tmp/fusionaly" "$tmp/checksums.txt"
	rmdir "$tmp"
	if [ -r /dev/tty ]; then
		/usr/local/bin/fusionaly migrate-to-oss </dev/tty
	else
		/usr/local/bin/fusionaly migrate-to-oss
	fi
}

if [ "${1:-}" = migrate-to-oss ] || [ "${1:-}" = migrate ]; then
	migrate_to_oss
	exit 0
fi

# A server that the older installer set up keeps it: it updates itself each
# night. This script never touches a Fusionaly that runs.
if [ -f /etc/cron.d/fusionaly-update ] || grep -qs '^ *fusionaly:' /etc/matcha/config.yml; then
	echo "This server runs Fusionaly from the older installer. It keeps updating itself each night,"
	echo "and its commands stay: fusionaly update, restore-db, change-admin-password."
	echo "Nothing changed. To move it to Chasen: https://fusionaly.com/docs/move-to-chasen/"
	exit 0
fi

# Fusionaly runs on Chasen already, also after a move from the older
# installer: deploy the newest image, keep the settings, and update it each
# night from now on.
if command -v chasen-server >/dev/null 2>&1 && chasen-server list 2>/dev/null | grep -q '^fusionaly '; then
	echo "Updating Fusionaly to the newest $image..."
	printf '{"image":"%s","keep_settings":true,"auto_update":true,"env":{}}\n' "$image" | chasen-server deploy fusionaly latest
	echo -e "${GREEN}Fusionaly is up to date.${NC}"
	exit 0
fi

domain="${1:-${FUSIONALY_DOMAIN:-}}"
if [ -z "$domain" ]; then
	[ -r /dev/tty ] || fail "no domain: curl -fsSL https://fusionaly.com/install | sudo bash -s data.example.com"
	echo "Tip: avoid 'analytics', 'tracking', 'stats', or 'telemetry' in the domain: ad blockers"
	echo "block those hostnames. A neutral one such as data.example.com is safer."
	read -r -p "Domain for Fusionaly (e.g. data.example.com): " domain </dev/tty
fi
domain="$(echo "$domain" | tr '[:upper:]' '[:lower:]')"
[[ "$domain" =~ ^[a-z0-9]([a-z0-9.-]*[a-z0-9])?$ && "$domain" == *.* ]] || fail "\"$domain\" is not a domain"
if [[ "$domain" =~ (analytics|tracking|stats|telemetry) ]]; then
	echo "Note: ad blockers often block hostnames with \"${BASH_REMATCH[1]}\". A neutral name such as data.example.com is safer."
fi

echo "Installing Chasen..."
curl -fsSL https://chasenhq.com/server | sh
chasen-server setup >/dev/null # it installs Docker when the server has none

echo "Deploying Fusionaly at $domain..."
printf '{"image":"%s","domain":"%s","auto_update":true,"env":{}}\n' "$image" "$domain" | chasen-server deploy fusionaly latest

echo
echo -e "${GREEN}Fusionaly runs at https://$domain${NC}"
echo "  Point an A record for $domain to this server: HTTPS comes on the first request."
echo "  Open it now and create your account: until you do, anyone who opens it can."
echo "  It updates itself each night, with a backup of the database first."
echo
echo "Manage it from your computer:"
echo "  curl -fsSL https://chasenhq.com/cli | sh"
echo "  chasen add server root@<this server>"
echo "  chasen -a fusionaly status        # also: logs, backups, restore, rollback"
