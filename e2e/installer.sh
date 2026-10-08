#!/usr/bin/env bash
# The test of install.sh, the script behind `curl -fsSL
# https://fusionaly.com/install | sudo bash`: on a fresh machine it installs
# Chasen and deploys the image of this commit, then this checks /up, the
# setup screen and a real login, the nightly updates, a second
# run that updates and keeps the data, and that a server of the older
# installer is left alone.
#
# It needs Docker, sudo, and the ports 80 and 443, and it changes the
# machine: run it on a CI runner or in a throwaway VM, not on your computer.
#   e2e/installer.sh
set -euo pipefail
cd "$(dirname "$0")/.."

registry=localhost:5000 # a registry with no login, like Docker Hub for a public image
image=$registry/fusionaly:e2e
host=fusionaly.localhost
email=owner@example.com
password=correct-horse-battery

pass() { echo "ok   - $*"; }
fail() { echo "FAIL - $*" >&2; exit 1; }
web() { curl -s -H "Host: $host" "$@"; } # through the proxy of Chasen
post() { web -o /dev/null -w '%{http_code} %{redirect_url}' -H "Origin: http://$host" "$@"; }
login() { post -d "email=$email" --data-urlencode "password=$1" http://127.0.0.1/login; }
# The container that runs: fusionaly, or fusionaly-next during a swap.
ctr() { docker ps --format '{{.Names}}' | grep -m1 -E '^fusionaly(-next)?$'; }
# The first account, made the way fnctl makes it, not through the setup screens.
create_owner() { docker exec "$(ctr)" /app/fnctl create-admin-user "$email" "$password" >/dev/null; }
install() { sudo FUSIONALY_IMAGE="$image" bash install.sh "$host"; } # the domain as the docs pass it: bash -s <domain>

docker rm -f fusionaly-e2e-registry >/dev/null 2>&1 || true
docker run -d --name fusionaly-e2e-registry -p 127.0.0.1:5000:5000 registry:2 >/dev/null
trap 'docker rm -f fusionaly-e2e-registry >/dev/null 2>&1 || true' EXIT

echo "--- the image of this commit"
docker build -q -t "$image" . >/dev/null
docker push -q "$image" >/dev/null

echo "--- the installer, on a fresh machine"
out="$(install 2>&1)" || { echo "$out"; fail "install.sh failed"; }
echo "$out" | tail -8

[[ "$(web -o /dev/null -w '%{http_code}' http://127.0.0.1/up)" == 200 ]] || fail "/up does not answer 200"
pass "/up answers 200"
status="$(sudo chasen-server status fusionaly)" # not in a pipe: grep -q stops early, and pipefail fails the status
grep -q "Updates:  each night" <<<"$status" || fail "the nightly updates are off"
pass "Fusionaly updates itself each night"

create_owner || fail "fnctl did not create the account"
[[ "$(login "$password")" == 302" http://127.0.0.1/admin" ]] || fail "the account does not log in: $(login "$password")"
pass "an account logs in"

echo "--- the installer again: an update that keeps the data"
again="$(install 2>&1)" || { echo "$again"; fail "the second run failed"; }
grep -q "Fusionaly is up to date" <<<"$again" || fail "the second run did not update: $again"
[[ "$(login "$password")" == 302" http://127.0.0.1/admin" ]] || fail "the login fails after the second run"
pass "a second run updates, and the data stays"

echo "--- a server of the older installer"
echo "0 3 * * * root /usr/local/bin/fusionaly update" | sudo tee /etc/cron.d/fusionaly-update >/dev/null
old="$(install 2>&1)"
sudo rm /etc/cron.d/fusionaly-update
grep -q "Nothing changed" <<<"$old" || fail "install.sh did not leave a server of the older installer alone: $old"
pass "install.sh leaves a server of the older installer alone"

echo "All checks passed."
