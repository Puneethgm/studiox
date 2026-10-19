#!/bin/sh
#
# Runs once, automatically, via docker-entrypoint-initdb.d on the PRIMARY
# postgres service's first init (empty data dir only — never on restart).
#
# Allows the `replicator` role (created by the sibling 00-init-replication.sql
# in this same directory) to open a physical replication connection from
# postgres_replica.
#
# This is needed because pg_hba.conf's `database` field keyword `all` does
# NOT match replication connections — Postgres treats `replication` as a
# distinct pseudo-database that must be named explicitly. Without this
# line, the default `host all all all <method>` entry the official
# postgres image writes for POSTGRES_HOST_AUTH_METHOD is not enough, and
# postgres_replica's pg_basebackup fails with:
#   "no pg_hba.conf entry for replication connection"
#
# `md5` here authenticates correctly against both md5- and scram-sha-256-
# hashed passwords (the server negotiates), so it works regardless of this
# image's password_encryption default.
set -e

echo "host replication replicator all md5" >> "${PGDATA:-/var/lib/postgresql/data}/pg_hba.conf"
