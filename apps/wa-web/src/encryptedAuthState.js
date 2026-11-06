// Drop-in replacement for Baileys' own useMultiFileAuthState (see
// node_modules/@whiskeysockets/baileys/lib/Utils/use-multi-file-auth-state.js,
// which this file mirrors file-for-file) that encrypts every file's bytes
// before they touch disk and decrypts them on read. Baileys' own version
// writes plain JSON — credentials, pre-keys, identity keys, session keys
// — directly to the filesystem. Anyone with host or volume access could
// read a studio's live WhatsApp session straight off disk.
// platform-security-hardening task 0.5.
import { Mutex } from 'async-mutex';
import { mkdir, readFile, stat, unlink, writeFile } from 'fs/promises';
import { join } from 'path';
import crypto from 'crypto';
import { proto, initAuthCreds, BufferJSON } from '@whiskeysockets/baileys';

const ALGORITHM = 'aes-256-gcm';
const IV_LENGTH = 12; // 96-bit, the GCM-recommended nonce size

// encrypt returns iv‖authTag‖ciphertext, base64-encoded, so each file is a
// single opaque blob — no JSON structure or key names are visible without
// the key.
export function encrypt(plaintext, keyBuf) {
  const iv = crypto.randomBytes(IV_LENGTH);
  const cipher = crypto.createCipheriv(ALGORITHM, keyBuf, iv);
  const ciphertext = Buffer.concat([cipher.update(plaintext, 'utf8'), cipher.final()]);
  const authTag = cipher.getAuthTag();
  return Buffer.concat([iv, authTag, ciphertext]).toString('base64');
}

export function decrypt(blob, keyBuf) {
  const raw = Buffer.from(blob, 'base64');
  const iv = raw.subarray(0, IV_LENGTH);
  const authTag = raw.subarray(IV_LENGTH, IV_LENGTH + 16);
  const ciphertext = raw.subarray(IV_LENGTH + 16);
  const decipher = crypto.createDecipheriv(ALGORITHM, keyBuf, iv);
  decipher.setAuthTag(authTag);
  return Buffer.concat([decipher.update(ciphertext), decipher.final()]).toString('utf8');
}

// keyFromBase64 turns a 32-byte base64-encoded value (the same shape as
// the Go side's TOKEN_ENCRYPTION_KEY, generated with
// `openssl rand -base64 32`) into the raw key buffer aes-256-gcm needs.
export function keyFromBase64(b64) {
  const buf = Buffer.from(b64, 'base64');
  if (buf.length !== 32) {
    throw new Error(`WA_WEB_SESSION_ENCRYPTION_KEY must decode to 32 bytes, got ${buf.length}`);
  }
  return buf;
}

const fileLocks = new Map();
const getFileLock = (path) => {
  let mutex = fileLocks.get(path);
  if (!mutex) {
    mutex = new Mutex();
    fileLocks.set(path, mutex);
  }
  return mutex;
};

const fixFileName = (file) => file?.replace(/\//g, '__')?.replace(/:/g, '-');

export const useEncryptedMultiFileAuthState = async (folder, keyBuf) => {
  const writeData = async (data, file) => {
    const filePath = join(folder, fixFileName(file));
    const mutex = getFileLock(filePath);
    return mutex.acquire().then(async (release) => {
      try {
        const json = JSON.stringify(data, BufferJSON.replacer);
        await writeFile(filePath, encrypt(json, keyBuf));
      } finally {
        release();
      }
    });
  };

  const readData = async (file) => {
    try {
      const filePath = join(folder, fixFileName(file));
      const mutex = getFileLock(filePath);
      return await mutex.acquire().then(async (release) => {
        try {
          const blob = await readFile(filePath, { encoding: 'utf-8' });
          const json = decrypt(blob, keyBuf);
          return JSON.parse(json, BufferJSON.reviver);
        } finally {
          release();
        }
      });
    } catch {
      return null;
    }
  };

  const removeData = async (file) => {
    try {
      const filePath = join(folder, fixFileName(file));
      const mutex = getFileLock(filePath);
      return mutex.acquire().then(async (release) => {
        try {
          await unlink(filePath);
        } catch {
          // already gone — same no-op-on-missing behavior as Baileys' own version
        } finally {
          release();
        }
      });
    } catch {
      // no-op, matching Baileys' own removeData
    }
  };

  const folderInfo = await stat(folder).catch(() => {});
  if (folderInfo) {
    if (!folderInfo.isDirectory()) {
      throw new Error(`found something that is not a directory at ${folder}, either delete it or specify a different location`);
    }
  } else {
    await mkdir(folder, { recursive: true });
  }

  const creds = (await readData('creds.json')) || initAuthCreds();

  return {
    state: {
      creds,
      keys: {
        get: async (type, ids) => {
          const data = {};
          await Promise.all(
            ids.map(async (id) => {
              let value = await readData(`${type}-${id}.json`);
              if (type === 'app-state-sync-key' && value) {
                value = proto.Message.AppStateSyncKeyData.fromObject(value);
              }
              data[id] = value;
            }),
          );
          return data;
        },
        set: async (data) => {
          const tasks = [];
          for (const category in data) {
            for (const id in data[category]) {
              const value = data[category][id];
              const file = `${category}-${id}.json`;
              tasks.push(value ? writeData(value, file) : removeData(file));
            }
          }
          await Promise.all(tasks);
        },
      },
    },
    saveCreds: async () => writeData(creds, 'creds.json'),
  };
};
