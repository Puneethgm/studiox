// Unit tests for encryptedAuthState.js — platform-security-hardening
// task 0.5 (and its 0.x unit-test follow-up). Uses Node's built-in test
// runner (node --test), no new test-framework dependency needed.
import { test } from 'node:test';
import assert from 'node:assert/strict';
import { mkdtemp, readFile, rm } from 'fs/promises';
import { tmpdir } from 'os';
import path from 'path';
import crypto from 'crypto';

import { encrypt, decrypt, keyFromBase64, useEncryptedMultiFileAuthState } from './encryptedAuthState.js';

test('keyFromBase64 accepts a 32-byte key and rejects anything else', () => {
  const good = Buffer.alloc(32, 7).toString('base64');
  const key = keyFromBase64(good);
  assert.equal(key.length, 32);

  const short = Buffer.alloc(16, 7).toString('base64');
  assert.throws(() => keyFromBase64(short), /32 bytes/);
});

test('encrypt/decrypt round-trips the exact plaintext', () => {
  const key = crypto.randomBytes(32);
  const plaintext = JSON.stringify({ hello: 'world', nested: { n: 1 } });
  const blob = encrypt(plaintext, key);
  assert.notEqual(blob, plaintext);
  assert.equal(decrypt(blob, key), plaintext);
});

test('decrypt fails on a tampered ciphertext', () => {
  const key = crypto.randomBytes(32);
  const blob = encrypt('{"a":1}', key);
  const tampered = Buffer.from(blob, 'base64');
  tampered[tampered.length - 1] ^= 0xff; // flip a bit inside the ciphertext
  assert.throws(() => decrypt(tampered.toString('base64'), key));
});

test('decrypt fails with the wrong key', () => {
  const key = crypto.randomBytes(32);
  const otherKey = crypto.randomBytes(32);
  const blob = encrypt('{"a":1}', key);
  assert.throws(() => decrypt(blob, otherKey));
});

test('useEncryptedMultiFileAuthState writes files that are not plaintext JSON on disk, and round-trips creds through saveCreds/reload', async (t) => {
  const key = crypto.randomBytes(32);
  const dir = await mkdtemp(path.join(tmpdir(), 'wa-web-auth-test-'));
  t.after(() => rm(dir, { recursive: true, force: true }));

  const { state, saveCreds } = await useEncryptedMultiFileAuthState(dir, key);
  state.creds.marker = 'this-is-a-secret-marker';
  await saveCreds();

  const onDisk = await readFile(path.join(dir, 'creds.json'), { encoding: 'utf-8' });
  assert.ok(!onDisk.includes('this-is-a-secret-marker'), 'the marker value must not appear in plaintext on disk');
  assert.throws(() => JSON.parse(onDisk), 'the on-disk bytes must not themselves be parseable JSON');

  const { state: reloaded } = await useEncryptedMultiFileAuthState(dir, key);
  assert.equal(reloaded.creds.marker, 'this-is-a-secret-marker');
});

test('useEncryptedMultiFileAuthState cannot be read back with the wrong key', async (t) => {
  // Mirrors Baileys' own useMultiFileAuthState: a file that fails to read
  // (missing, corrupt, or here, wrong key) is treated as "no data yet",
  // not a hard error — so the wrong key silently yields fresh
  // credentials rather than the original secret marker. Either way, the
  // original value must never come back without the right key.
  const key = crypto.randomBytes(32);
  const wrongKey = crypto.randomBytes(32);
  const dir = await mkdtemp(path.join(tmpdir(), 'wa-web-auth-test-'));
  t.after(() => rm(dir, { recursive: true, force: true }));

  const { state, saveCreds } = await useEncryptedMultiFileAuthState(dir, key);
  state.creds.marker = 'secret';
  await saveCreds();

  const { state: withWrongKey } = await useEncryptedMultiFileAuthState(dir, wrongKey);
  assert.notEqual(withWrongKey.creds.marker, 'secret');
});
