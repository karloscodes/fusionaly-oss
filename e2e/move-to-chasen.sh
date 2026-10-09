#!/usr/bin/env bash
# The test of moving a server of the older installer to Chasen: it sets one
# up with the released fusionaly binary, as the install line did before, then
# runs the steps of the docs with the image of this commit, and checks that
# the account, the session key, and the domain stay, and that Chasen updates
# Fusionaly each night from then on.
#
# It needs Docker, sudo, and the ports 80 and 443, and it changes the
# machine: run it on a CI runner or in a throwaway VM, not on your computer.
#   e2e/move-to-chasen.sh
set -euo pipefail
cd "$(dirname "$0")/.."

registry=localhost:5000 # a registry with no login, like Docker Hub for a public image
image=$registry/fusionaly:e2e
host=fusionaly.localhost
email=owner@example.com
password=correct-horse-battery

pass() { echo "ok   - $*"; }
fail() { echo "FAIL - $*" >&2; exit 1; }
web() { curl -s -H "Host: $host" "$@"; }
post() { web -o /dev/null -w '%{http_code} %{redirect_url}' -H "Origin: http://$host" "$@"; }
login() { post -d "email=$email" --data-urlencode "password=$1" http://127.0.0.1/login; }
# The container that runs: fusionaly, or fusionaly-next during a swap.
ctr() { docker ps --format '{{.Names}}' | grep -m1 -E '^fusionaly(-next)?$'; }
# The first account, made the way fnctl makes it, not through the setup screens.
create_owner() { docker exec "$(ctr)" /app/fnctl create-admin-user "$email" "$password" >/dev/null; }
# The private key of the container that runs: fusionaly, or fusionaly-next after a swap.
key() { docker exec "$(ctr)" printenv PRIVATE_KEY; }

docker rm -f fusionaly-e2e-registry >/dev/null 2>&1 || true
docker run -d --name fusionaly-e2e-registry -p 127.0.0.1:5000:5000 registry:2 >/dev/null
trap 'docker rm -f fusionaly-e2e-registry >/dev/null 2>&1 || true' EXIT

echo "--- the image of this commit"
docker build -q -t "$image" . >/dev/null
docker push -q "$image" >/dev/null

echo "--- a server of the older installer, with the released image"
case "$(uname -m)" in x86_64) arch=amd64 ;; aarch64) arch=arm64 ;; *) fail "no build for $(uname -m)" ;; esac
curl -fsSL -o /tmp/fusionaly "https://github.com/karloscodes/fusionaly-oss/releases/latest/download/fusionaly-linux-$arch"
sudo install -m 755 /tmp/fusionaly /usr/local/bin/fusionaly
printf '%s\ny\n' "$host" | sudo fusionaly install >/tmp/old-install.log 2>&1 || { cat /tmp/old-install.log; fail "the older installer failed"; }
for _ in $(seq 30); do [[ "$(web -o /dev/null -w '%{http_code}' http://127.0.0.1/_health)" == 200 ]] && break; sleep 2; done
create_owner || fail "fnctl did not create the account on the older install"
[[ "$(login "$password")" == 302" http://127.0.0.1/admin" ]] || fail "the older install does not log in"
[[ -f /etc/cron.d/fusionaly-update ]] || fail "the older installer made no nightly update"
old_key="$(key)"
pass "the older installer runs Fusionaly, and its account logs in"

echo "--- the move, as the docs say"
sudo fusionaly update >/tmp/old-update.log 2>&1 || { cat /tmp/old-update.log; fail "fusionaly update failed"; }
curl -fsSL https://chasenhq.com/server | sudo sh >/dev/null
sudo chasen-server setup >/dev/null
sudo chasen-server adopt fusionaly
sudo rm /etc/cron.d/fusionaly-update /usr/local/bin/fusionaly
sudo FUSIONALY_IMAGE="$image" bash install.sh

[[ "$(web -o /dev/null -w '%{http_code}' http://127.0.0.1/up)" == 200 ]] || fail "/up does not answer after the move"
pass "/up answers after the move"
[[ "$(login "$password")" == 302" http://127.0.0.1/admin" ]] || fail "the account does not log in after the move"
pass "the account stays"
status="$(sudo chasen-server status fusionaly)" # not in a pipe: grep -q stops early, and pipefail fails the status
[[ -n "$old_key" && "$(key)" == "$old_key" ]] || fail "the session key changed: every login ends"
pass "the session key stays, so the logins stay"
grep -q "Updates:  each night" <<<"$status" || fail "the nightly updates of Chasen are off"
[[ ! -f /etc/cron.d/fusionaly-update ]] || fail "the nightly update of the older installer is still there"
pass "Chasen updates it each night, and the older nightly update is gone"
grep -qE "^URL: +.*$host" <<<"$status" || fail "the domain changed: $status"
pass "the domain stays"

echo "--- the install line on the moved server: an update"
again="$(sudo FUSIONALY_IMAGE="$image" bash install.sh 2>&1)" || { echo "$again"; fail "the install line failed on the moved server"; }
grep -q "Fusionaly is up to date" <<<"$again" || fail "the install line did not update the moved server: $again"
[[ "$(login "$password")" == 302" http://127.0.0.1/admin" ]] || fail "the login fails after the update"
pass "the install line updates the moved server"

echo "All checks passed."
