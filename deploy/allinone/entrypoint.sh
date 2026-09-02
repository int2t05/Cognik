#!/bin/bash
set -e

# OpsMind All-in-One entrypoint
# 首次启动：initdb → 建库 → supervisord
# 后续启动：直接 supervisord

PGDATA="${PGDATA:-/data/postgresql/pgdata}"
PG_BIN="/usr/lib/postgresql/18/bin"
PG_USER="${POSTGRES_USER:-opsmind}"
PG_PASS="${POSTGRES_PASSWORD:-opsmind_dev}"
PG_DB="${POSTGRES_DB:-opsmind}"

log() { echo "[entrypoint] $(date +%H:%M:%S) $*"; }

init_pgdata() {
    if [ -f "${PGDATA}/PG_VERSION" ]; then
        log "PGDATA already initialized, skip initdb"
        return 0
    fi
    log "First run: initdb..."
    mkdir -p "$(dirname "$PGDATA")"
    chown -R postgres:postgres "$(dirname "$PGDATA")"
    su - postgres -c "${PG_BIN}/initdb -D ${PGDATA} \
        --auth-host=scram-sha-256 --auth-local=peer \
        --encoding=UTF-8 --locale=C"
    # 允许本地 TCP 密码连接
    cat >> "${PGDATA}/pg_hba.conf" <<'HBA'
host all all 127.0.0.1/32 scram-sha-256
host all all ::1/128 scram-sha-256
HBA
    log "initdb done"
}

init_database() {
    log "Starting temp PostgreSQL..."
    su - postgres -c "${PG_BIN}/pg_ctl -D ${PGDATA} -l /tmp/pg-init.log start"
    for i in $(seq 1 30); do
        su - postgres -c "${PG_BIN}/pg_isready -q" 2>/dev/null && break
        sleep 1
    done

    echo "ALTER USER postgres PASSWORD '${PG_PASS}';" | su - postgres -c "${PG_BIN}/psql"
    su - postgres -c "${PG_BIN}/psql -tAc \
        \"SELECT 1 FROM pg_roles WHERE rolname='${PG_USER}';\"" | grep -q 1 || \
        echo "CREATE USER ${PG_USER} WITH PASSWORD '${PG_PASS}' SUPERUSER;" | su - postgres -c "${PG_BIN}/psql"
    su - postgres -c "${PG_BIN}/psql -tAc \
        \"SELECT 1 FROM pg_database WHERE datname='${PG_DB}';\"" | grep -q 1 || \
        su - postgres -c "${PG_BIN}/psql -c \"CREATE DATABASE ${PG_DB} OWNER ${PG_USER};\""

    log "Database ready (seed data loads via AutoSeed on server start)"
    su - postgres -c "${PG_BIN}/pg_ctl -D ${PGDATA} stop"
}

log "=== OpsMind All-in-One Starting ==="
init_pgdata
init_database
log "Launching supervisord..."
exec /usr/bin/supervisord -c /etc/supervisor/supervisord.conf -n
