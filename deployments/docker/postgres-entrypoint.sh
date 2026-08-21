#!/bin/sh
set -eu

tls_dir=/var/lib/postgresql/tls
install -d -o postgres -g postgres -m 0700 "$tls_dir"
install -o postgres -g postgres -m 0644 /run/secrets/postgres_tls_cert "$tls_dir/server.crt"
install -o postgres -g postgres -m 0600 /run/secrets/postgres_tls_key "$tls_dir/server.key"
install -o postgres -g postgres -m 0644 /run/secrets/postgres_tls_ca "$tls_dir/ca.crt"

exec /usr/local/bin/docker-entrypoint.sh "$@"
