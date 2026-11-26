#!/bin/sh
# Resets WordPress and fills it with posts, pages, categories and comments.
set -eu
compose="docker compose -f $(dirname "$0")/compose.yml"

# drop the containers and their volumes, so every run starts from an identical database
$compose down -v --remove-orphans >/dev/null
$compose up -d --wait wordpress wordpress-db >/dev/null
$compose run --rm -T wp-cli sh -ec '
wp core install --url=http://127.0.0.1:8081 --title=bench --admin_user=admin --admin_password=admin --admin_email=admin@example.com --skip-email
wp rewrite structure "/%postname%/"
cats=
for i in 1 2 3 4 5; do
  id=$(wp term create category "Category $i" --porcelain)
  cats="$cats $id"
done
for i in $(seq 1 30); do
  cat=$(echo $cats | cut -d" " -f$(( (i - 1) % 5 + 1 )))
  wp post create --post_title="Post $i" --post_name="post-$i" --post_content="Post $i body" --post_status=publish --post_date=2026-01-01 --post_category="$cat" --porcelain
done
for i in $(seq 1 5); do
  wp post create --post_type=page --post_title="Page $i" --post_name="page-$i" --post_content="Page $i body" --post_status=publish --porcelain
done
wp comment generate --count=10 --post_id=1
' >/dev/null
