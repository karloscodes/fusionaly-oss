#!/bin/bash
# Download GeoLite2-City database from MaxMind
# Requires: MAXMIND_LICENSE_KEY environment variable

set -e

GEOLITE_DB_PATH="${GEOLITE_DB_PATH:-internal-storage/GeoLite2-City.mmdb}"
GEOLITE_DB_DIR=$(dirname "$GEOLITE_DB_PATH")

if [ -z "$MAXMIND_LICENSE_KEY" ]; then
    echo "Error: MAXMIND_LICENSE_KEY environment variable is required"
    echo "Get your license key from https://www.maxmind.com/en/account"
    exit 1
fi

echo "Downloading GeoLite2-City database..."

# Create directory if it doesn't exist
mkdir -p "$GEOLITE_DB_DIR"

# Download the database
DOWNLOAD_URL="https://download.maxmind.com/app/geoip_download?edition_id=GeoLite2-City&license_key=${MAXMIND_LICENSE_KEY}&suffix=tar.gz"

# Create temp directory
TEMP_DIR=$(mktemp -d)
trap "rm -rf $TEMP_DIR" EXIT

# Download and extract
curl -sS "$DOWNLOAD_URL" -o "$TEMP_DIR/geolite.tar.gz"

# Extract the .mmdb file
tar -xzf "$TEMP_DIR/geolite.tar.gz" -C "$TEMP_DIR"

# Find and move the .mmdb file
MMDB_FILE=$(find "$TEMP_DIR" -name "*.mmdb" -type f | head -1)

if [ -z "$MMDB_FILE" ]; then
    echo "Error: Could not find .mmdb file in downloaded archive"
    exit 1
fi

mv "$MMDB_FILE" "$GEOLITE_DB_PATH"

echo "GeoLite2-City database downloaded to $GEOLITE_DB_PATH"
