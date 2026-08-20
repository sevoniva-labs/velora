#!/usr/bin/env bash
set -euo pipefail
echo 'host replication all all trust' >> "$PGDATA/pg_hba.conf"
pg_ctl -D "$PGDATA" reload
