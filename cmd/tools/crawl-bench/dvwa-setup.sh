#!/bin/sh
# Resets DVWA, logs in, and prints the session cookie as a request header for katana.
set -eu
base=${DVWA_URL:-http://127.0.0.1:4280}
jar=$(mktemp)
trap 'rm -f "$jar"' EXIT

docker compose -f "$(dirname "$0")/compose.yml" up -d --wait --force-recreate dvwa db >/dev/null

token() { curl -fsS -b "$jar" -c "$jar" "$base/$1" | sed -n "s/.*name='user_token' value='\([0-9a-f]*\)'.*/\1/p"; }

curl -fsS -b "$jar" -c "$jar" -o /dev/null --data "create_db=Create+%2F+Reset+Database&user_token=$(token setup.php)" "$base/setup.php"
curl -fsS -b "$jar" -c "$jar" -o /dev/null --data "username=admin&password=password&Login=Login&user_token=$(token login.php)" "$base/login.php"

session=$(awk '$6 == "PHPSESSID" {print $7}' "$jar")
curl -fsS -b "PHPSESSID=$session; security=low" "$base/index.php" | grep -q 'logout.php' || { echo "dvwa login failed" >&2; exit 1; }
echo "Cookie: PHPSESSID=$session; security=low"
