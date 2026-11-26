#!/bin/sh
# Resets WordPress and fills it with posts, pages, categories and comments.
set -eu
compose="docker compose -f $(dirname "$0")/compose.yml"

$compose up -d --wait --force-recreate wordpress wordpress-db >/dev/null
$compose run --rm -T wp-cli sh -ec '
wp core install --url=http://127.0.0.1:8081 --title=bench --admin_user=admin --admin_password=admin --admin_email=admin@example.com --skip-email
wp rewrite structure "/%postname%/"
wp term generate category --count=5
wp post generate --count=30 --post_date=2026-01-01
wp post generate --count=5 --post_type=page
wp comment generate --count=10 --post_id=1
' >/dev/null
