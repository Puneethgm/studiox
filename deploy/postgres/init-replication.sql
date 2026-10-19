-- Runs once, automatically, via docker-entrypoint-initdb.d on the PRIMARY
-- postgres service's first init (empty data dir only — never on restart,
-- same as any other initdb.d script).
--
-- Creates the `replicator` role that postgres_replica uses to
-- pg_basebackup/stream from this primary. The companion
-- 00-init-replication-hba.sh (same directory, named to run first
-- alphabetically) adds the pg_hba.conf line that lets that role actually
-- connect for replication.
--
-- The password comes from POSTGRES_REPLICATION_PASSWORD (set in
-- deploy/.env), not a literal here. Plain .sql initdb scripts don't get OS
-- env vars substituted automatically, so we pull it in via psql's backtick
-- shell-command substitution (psql inherits the container's environment),
-- then pass it through as a bound value with format(..., %L) rather than
-- string-concatenating it into the SQL text.
\set repl_password `echo "$POSTGRES_REPLICATION_PASSWORD"`

DO
$$
BEGIN
  IF NOT EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'replicator') THEN
    EXECUTE format('CREATE ROLE replicator WITH REPLICATION LOGIN PASSWORD %L', :'repl_password');
  END IF;
END
$$;
