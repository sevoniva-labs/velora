#!/usr/bin/env bash
# Certbot deploy hook: atomically publish renewed certificates to the Edge.
set -euo pipefail
umask 077

RUNTIME_DIR="${VELORA_RUNTIME_DIR:-/opt/velora/prod/runtime}"
EDGE_CONTAINER="${VELORA_EDGE_CONTAINER:-velora-prod-edge}"
LINEAGE="${RENEWED_LINEAGE:-}"
DOMAINS="${RENEWED_DOMAINS:-}"
[[ -n "$LINEAGE" && -r "$LINEAGE/fullchain.pem" && -r "$LINEAGE/privkey.pem" ]] || { echo "error: renewed certificate lineage is unavailable" >&2; exit 1; }

primary="${DOMAINS%% *}"
case "$primary" in
  home.sevoniva.com|auth.sevoniva.com|demo.sevoniva.com) ;;
  *) echo "error: renewed certificate is not an approved Velora domain" >&2; exit 1 ;;
esac
openssl x509 -in "$LINEAGE/fullchain.pem" -noout -checkend 2592000 >/dev/null || { echo "error: renewed certificate validity is below 30 days" >&2; exit 1; }

target="$RUNTIME_DIR/certs/$primary"
mkdir -p "$target"
install -m 0644 "$LINEAGE/fullchain.pem" "$target/fullchain.pem.new"
install -m 0600 "$LINEAGE/privkey.pem" "$target/privkey.pem.new"
cp -p "$target/fullchain.pem" "$target/fullchain.pem.bak"
cp -p "$target/privkey.pem" "$target/privkey.pem.bak"
mv -f "$target/fullchain.pem.new" "$target/fullchain.pem"
mv -f "$target/privkey.pem.new" "$target/privkey.pem"

if ! docker exec "$EDGE_CONTAINER" nginx -t >/dev/null 2>&1; then
  mv -f "$target/fullchain.pem.bak" "$target/fullchain.pem"
  mv -f "$target/privkey.pem.bak" "$target/privkey.pem"
  echo "error: renewed certificate failed Edge validation; previous certificate restored" >&2
  exit 1
fi
rm -f "$target/fullchain.pem.bak" "$target/privkey.pem.bak"
docker exec "$EDGE_CONTAINER" nginx -s reload
echo "renewed certificate deployed: $primary"
