#!/bin/bash

# Script to install Fusionaly from GitHub releases
# Run as: curl -fsSL https://fusionaly.com/install | sudo bash

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
NC='\033[0m' # No Color

# Configuration
GITHUB_REPO="karloscodes/fusionaly-oss"
INSTALL_DIR="/usr/local/bin"

run_installer() {
    # Verify running as root
    if [ "$(id -u)" -ne 0 ]; then
        echo -e "${RED}Error: This script requires root privileges. Use 'sudo su' and then re-run the installation command.${NC}"
        exit 1
    fi

    # Detect architecture
    ARCH=$(uname -m)
    case "$ARCH" in
        x86_64)
            ARCH="amd64"
            ;;
        aarch64|arm64)
            ARCH="arm64"
            ;;
        *)
            echo -e "${RED}Unsupported architecture: $ARCH. Only amd64 and arm64 are supported.${NC}"
            exit 1
            ;;
    esac

    BINARY_PATH="$INSTALL_DIR/fusionaly"
    TEMP_FILE="/tmp/fusionaly-$ARCH"

    # Install dependencies if needed
    NEED_UPDATE=false
    if ! command -v jq >/dev/null 2>&1; then
        NEED_UPDATE=true
    fi
    if ! command -v file >/dev/null 2>&1; then
        NEED_UPDATE=true
    fi
    if [ "$NEED_UPDATE" = true ]; then
        apt-get update -qq > /dev/null 2>&1
    fi
    if ! command -v jq >/dev/null 2>&1; then
        apt-get install -y -qq jq > /dev/null 2>&1 || {
            echo -e "${RED}Error: Failed to install jq. This script requires jq to parse GitHub API responses.${NC}"
            exit 1
        }
    fi
    if ! command -v file >/dev/null 2>&1; then
        apt-get install -y -qq file > /dev/null 2>&1 || {
            echo -e "${RED}Error: Failed to install 'file'. Binary verification will be skipped.${NC}"
        }
    fi

    # Fetch the latest release information
    echo "Fetching latest release information..."
    RELEASE_INFO=$(curl -fsSL "https://api.github.com/repos/$GITHUB_REPO/releases/latest")

    # Check for rate limit or other API errors
    if echo "$RELEASE_INFO" | grep -q "API rate limit exceeded"; then
        echo -e "${RED}Error: GitHub API rate limit exceeded. Please try again later.${NC}"
        exit 1
    fi

    if echo "$RELEASE_INFO" | grep -q "Not Found"; then
        echo -e "${RED}Error: No releases found in $GITHUB_REPO.${NC}"
        exit 1
    fi

    # Extract the latest version
    LATEST_VERSION=$(echo "$RELEASE_INFO" | jq -r '.tag_name' | sed 's/^v//')

    if [ -z "$LATEST_VERSION" ]; then
        echo -e "${RED}Error: Could not determine latest version.${NC}"
        exit 1
    fi

    echo "Latest version: $LATEST_VERSION"

    # Look for the correct asset
    ASSET_NAME="fusionaly-linux-$ARCH"
    if ! echo "$RELEASE_INFO" | jq -r '.assets[].name' | grep -q "$ASSET_NAME"; then
        echo -e "${RED}Error: No binary found for $ARCH in release v$LATEST_VERSION.${NC}"
        echo "Available assets:"
        echo "$RELEASE_INFO" | jq -r '.assets[].name'
        exit 1
    fi

    # Construct the download URL
    BINARY_URL="https://github.com/$GITHUB_REPO/releases/download/v$LATEST_VERSION/$ASSET_NAME"
    echo "Download URL: $BINARY_URL"

    # Download the binary
    echo "Downloading Fusionaly v$LATEST_VERSION for $ARCH..."
    curl -L --fail --progress-bar -o "$TEMP_FILE" "$BINARY_URL" || {
        echo -e "${RED}Error: Failed to download binary.${NC}"
        rm -f "$TEMP_FILE"
        exit 1
    }

    # Verify the download
    if [ ! -s "$TEMP_FILE" ]; then
        echo -e "${RED}Error: Downloaded file is empty.${NC}"
        rm -f "$TEMP_FILE"
        exit 1
    fi

    # Check file type
    if command -v file >/dev/null 2>&1; then
        FILE_TYPE=$(file -b "$TEMP_FILE" | cut -d',' -f1-2)
        echo "Verifying file: $FILE_TYPE"
        if ! echo "$FILE_TYPE" | grep -q "ELF"; then
            echo -e "${RED}Error: Downloaded file is not a valid binary.${NC}"
            rm -f "$TEMP_FILE"
            exit 1
        fi
    fi

    # Install the binary
    echo "Installing to $BINARY_PATH..."
    mv "$TEMP_FILE" "$BINARY_PATH" && chmod +x "$BINARY_PATH" || {
        echo -e "${RED}Error: Failed to install binary.${NC}"
        rm -f "$TEMP_FILE"
        exit 1
    }

    # Run the installer interactively
    echo -e "${GREEN}Running Fusionaly installer...${NC}"
    if "$BINARY_PATH" install; then
        echo -e "${GREEN}Installation complete!${NC}"
    else
        INSTALL_EXIT_CODE=$?
        echo -e "${RED}Installation failed with exit code $INSTALL_EXIT_CODE.${NC}"
        exit $INSTALL_EXIT_CODE
    fi
}

# Handle piped execution (curl | sudo bash)
# When piped, stdin is not a TTY, so we need to re-exec with /dev/tty
if [ ! -t 0 ]; then
    echo "Detected piped execution. Creating temporary installer for interactive mode..."
    TEMP_SCRIPT=$(mktemp /tmp/fusionaly-install-XXXXXX.sh)

    # Export the function and variables to a temp script
    {
        echo '#!/bin/bash'
        echo "RED='$RED'"
        echo "GREEN='$GREEN'"
        echo "NC='$NC'"
        echo "GITHUB_REPO='$GITHUB_REPO'"
        echo "INSTALL_DIR='$INSTALL_DIR'"
        declare -f run_installer
        echo 'run_installer'
    } > "$TEMP_SCRIPT"

    chmod +x "$TEMP_SCRIPT"
    bash "$TEMP_SCRIPT" < /dev/tty
    EXIT_CODE=$?
    rm -f "$TEMP_SCRIPT"
    exit $EXIT_CODE
fi

# Run the installer
run_installer
