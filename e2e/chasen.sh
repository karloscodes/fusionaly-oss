#!/usr/bin/env bash
# The install test of the Chasen path, like a new user runs it: a fresh
# machine gets chasen-server and the chasen CLI from their releases, deploys
# the image of this commit with one line and no chasen.yml, and checks that
# Fusionaly works: /up, the setup screen for the first account, a real login, the
# data across a second deploy, a restart, and a backup.
#
# It needs Docker, sudo, and the ports 80 and 443, and it changes the
# machine: run it on a CI runner or in a throwaway VM, not on your computer.
#   e2e/chasen.sh
set -euo pipefail
cd "$(dirname "$0")/.."

registry=localhost:5000 # a registry with no login, like Docker Hub for a public image
image=$registry/fusionaly
host=fusionaly.localhost
email=owner@example.com
password=correct-horse-battery

pass() { echo "ok   - $*"; }
fail() { echo "FAIL - $*" >&2; exit 1; }
web() { curl -s -H "Host: $host" "$@"; } # through the proxy of Chasen
app() { chasen -a fusionaly "$@"; }      # the app is named after the image
# Where a form sends the browser: the home page when it works, a form again when not.
post() { web -o /dev/null -w '%{http_code} %{redirect_url}' -H "Origin: http://$host" "$@"; }
login() { post -d "email=$email" --data-urlencode "password=$1" http://127.0.0.1/login; }
# The container that runs: fusionaly, or fusionaly-next during a swap.
ctr() { docker ps --format '{{.Names}}' | grep -m1 -E '^fusionaly(-next)?$'; }
# The first account, made the way fnctl makes it, not through the setup screens.
create_owner() { docker exec "$(ctr)" /app/fnctl create-admin-user "$email" "$password" >/dev/null; }

docker rm -f fusionaly-e2e-registry >/dev/null 2>&1 || true
docker run -d --name fusionaly-e2e-registry -p 127.0.0.1:5000:5000 registry:2 >/dev/null
trap 'docker rm -f fusionaly-e2e-registry >/dev/null 2>&1 || true' EXIT

echo "--- the image of this commit"
docker build -q -t "$image:e2e" . >/dev/null
docker push -q "$image:e2e" >/dev/null

echo "--- Chasen, from its releases"
curl -fsSL https://chasenhq.com/server | sudo sh
# The setup prints the token of the server: keep it out of the log of CI.
sudo chasen-server setup --domain localhost >/tmp/chasen-setup.log 2>&1 || { cat /tmp/chasen-setup.log; fail "chasen-server setup"; }
mkdir -p "$HOME/.local/bin"
export PATH="$HOME/.local/bin:$PATH"
curl -fsSL https://chasenhq.com/cli | CHASEN_INSTALL_DIR="$HOME/.local/bin" sh
CHASEN_TOKEN="$(sudo chasen-server token)"
export CHASEN_URL=http://api.localhost CHASEN_TOKEN CHASEN_NO_UPDATE_CHECK=1
cd "$(mktemp -d)" # nothing of the repository counts: the deploy is the image alone

echo "--- the one line of the docs"
chasen deploy "$image:e2e" --domain "$host" --auto-update

[[ "$(web -o /dev/null -w '%{http_code}' http://127.0.0.1/up)" == 200 ]] || fail "/up does not answer 200"
pass "/up answers 200"
status="$(app status)" # not in a pipe: grep -q stops early, and pipefail fails the status
grep -q "Updates:  each night" <<<"$status" || fail "chasen status does not show the nightly updates"
pass "auto-update is on"

[[ "$(web -o /dev/null -w '%{redirect_url}' http://127.0.0.1/)" == */setup ]] || fail "a fresh Fusionaly does not ask for the first account"
pass "a fresh Fusionaly asks for the first account"
create_owner || fail "fnctl did not create the account"
[[ "$(login "$password")" == 302" http://127.0.0.1/admin" ]] || fail "the login does not go to the home page: $(login "$password")"
[[ "$(login wrong-password)" != 302" http://127.0.0.1/admin" ]] || fail "a wrong password logs in"
pass "the account logs in, a wrong password does not"
[[ -z "$(app run find /app/storage -name '*.log')" ]] || fail "log files in the storage: logs go to stdout and /app/logs"
pass "no log files in the storage"

echo "--- a new version keeps the data"
docker build -q -t "$image:e2e-next" - >/dev/null <<EOF
FROM $image:e2e
LABEL e2e=next
EOF
docker push -q "$image:e2e-next" >/dev/null
chasen deploy "$image:e2e-next" --domain "$host"

[[ "$(login "$password")" == 302" http://127.0.0.1/admin" ]] || fail "the login fails after the second deploy: the data is lost"
pass "the second deploy keeps the account"

echo "--- restart and backup"
app restart
[[ "$(web -o /dev/null -w '%{http_code}' http://127.0.0.1/up)" == 200 ]] || fail "/up does not answer after a restart"
pass "a restart keeps it up"
app backup
backups="$(app backups)"
grep -q "Z  server" <<<"$backups" || fail "no backup on the server: $backups"
pass "the backup is on the server"

echo "All checks passed."
