#!/usr/bin/env bash
#
# Scripted round-trip check for backup.sh's encryption (platform-
# security-hardening task 6.4) — encrypts a known test file the same way
# backup.sh encrypts a dump, decrypts it back, and diffs against the
# original. Meant to run in CI or as a pre-deploy check, not just a
# one-off manual verification. Exits non-zero if the round-trip doesn't
# match.

set -euo pipefail

TMP_DIR="$(mktemp -d)"
trap 'rm -rf "${TMP_DIR}"' EXIT

export BACKUP_ENCRYPTION_KEY="${BACKUP_ENCRYPTION_KEY:-test-only-key-$(date +%s)}"

PLAIN="${TMP_DIR}/original.txt"
ENCRYPTED="${TMP_DIR}/original.txt.enc"
DECRYPTED="${TMP_DIR}/decrypted.txt"

printf 'backup encryption round-trip check %s\n' "$(date -u +%FT%TZ)" > "${PLAIN}"

openssl enc -aes-256-cbc -pbkdf2 -iter 100000 -salt \
  -pass env:BACKUP_ENCRYPTION_KEY \
  -in "${PLAIN}" -out "${ENCRYPTED}"

# The ciphertext must not contain the plaintext anywhere.
if grep -q "backup encryption round-trip" "${ENCRYPTED}" 2>/dev/null; then
  echo "FAIL: ciphertext contains recognizable plaintext" >&2
  exit 1
fi

openssl enc -d -aes-256-cbc -pbkdf2 -iter 100000 \
  -pass env:BACKUP_ENCRYPTION_KEY \
  -in "${ENCRYPTED}" -out "${DECRYPTED}"

if ! diff -q "${PLAIN}" "${DECRYPTED}" >/dev/null; then
  echo "FAIL: decrypted file does not match original" >&2
  exit 1
fi

echo "OK: backup encryption round-trip matches original"
