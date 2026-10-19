package studios

import (
	"archive/zip"
	"bytes"
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/projectx/api/internal/platform/secrets"
)

// archiveTables lists every studio-scoped table (besides studios itself)
// included in a full-data export, matching init.md §4.5's "studio info,
// users, leads/contacts, conversations, campaigns" requirement at the level
// this codebase actually has tables for today. This is NOT an export of
// every studio-scoped table that exists in the system (settings/knowledge
// base live as columns on studios itself and are covered by studios.json;
// channel credentials, broadcast campaigns, decision trees, etc. are not
// included in this v1) — it is the set explicitly called out in the spec,
// kept short and honest rather than padded out with a guessed larger list.
var archiveTables = []string{"users", "leads", "campaigns", "conversations", "messages"}

type archiveManifestEntry struct {
	Table string `json:"table"`
	Rows  int    `json:"rows"`
}

// BuildArchive queries every studio-scoped table and produces an in-memory
// zip archive: one JSON file per table (a JSON array of row objects, each
// row produced by Postgres's own row_to_json so this code never has to
// hand-maintain a column list that can drift out of sync with migrations),
// plus manifest.json (table name -> row count) and files.json.
//
// Binary files are NOT embedded in this v1. The only uploaded file this
// codebase actually tracks per-studio today is the studio's logo
// (studios.logo_url) — knowledge-base "files" are stored as text content
// inline in the studios table already (see KnowledgeBaseFile), and
// social-post media is referenced by URL on individual posts, not tracked
// in any studio-scoped manifest table. files.json records the logo URL, if
// set, as a reference only — the image bytes are not downloaded or
// embedded. This is a deliberate, explicitly-flagged limitation, not a
// silent gap: a future pass that wants real binary export should extend
// this function, not assume it already happens.
func BuildArchive(ctx context.Context, pool *pgxpool.Pool, studioID uuid.UUID) ([]byte, error) {
	buf := &bytes.Buffer{}
	zw := zip.NewWriter(buf)

	manifest := make([]archiveManifestEntry, 0, len(archiveTables)+1)

	studioRows, err := queryTableRowsJSON(ctx, pool, "studios", "id", studioID)
	if err != nil {
		_ = zw.Close()
		return nil, fmt.Errorf("export studios: %w", err)
	}
	if err := writeZipJSON(zw, "studios.json", studioRows); err != nil {
		_ = zw.Close()
		return nil, err
	}
	manifest = append(manifest, archiveManifestEntry{Table: "studios", Rows: len(studioRows)})

	var logoURL string
	if len(studioRows) > 0 {
		var row map[string]json.RawMessage
		if err := json.Unmarshal(studioRows[0], &row); err == nil {
			if raw, ok := row["logo_url"]; ok {
				_ = json.Unmarshal(raw, &logoURL)
			}
		}
	}

	for _, table := range archiveTables {
		rows, err := queryTableRowsJSON(ctx, pool, table, "studio_id", studioID)
		if err != nil {
			_ = zw.Close()
			return nil, fmt.Errorf("export %s: %w", table, err)
		}
		if err := writeZipJSON(zw, table+".json", rows); err != nil {
			_ = zw.Close()
			return nil, err
		}
		manifest = append(manifest, archiveManifestEntry{Table: table, Rows: len(rows)})
	}

	if err := writeZipJSON(zw, "manifest.json", manifest); err != nil {
		_ = zw.Close()
		return nil, err
	}

	filesManifest := map[string]any{
		"note":    "Only the studio logo URL is recorded here; the file itself is not downloaded or embedded in this archive (v1 limitation — see BuildArchive's doc comment).",
		"logoUrl": logoURL,
	}
	if err := writeZipJSON(zw, "files.json", filesManifest); err != nil {
		_ = zw.Close()
		return nil, err
	}

	if err := zw.Close(); err != nil {
		return nil, fmt.Errorf("close zip: %w", err)
	}
	return buf.Bytes(), nil
}

// queryTableRowsJSON reads every row of `table` matching `column = id` and
// returns each as a raw JSON object (via Postgres's row_to_json), so the
// archive's column lists are always exactly whatever the live schema has —
// never a hand-maintained list that can silently fall behind a migration.
// table/column are always one of the fixed literals passed in by
// BuildArchive above, never caller/user input, so building the query via
// fmt.Sprintf here is safe.
func queryTableRowsJSON(ctx context.Context, pool *pgxpool.Pool, table, column string, id uuid.UUID) ([]json.RawMessage, error) {
	query := fmt.Sprintf(`SELECT row_to_json(t) FROM %s t WHERE %s = $1`, table, column)
	rows, err := pool.Query(ctx, query, id)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make([]json.RawMessage, 0)
	for rows.Next() {
		var raw json.RawMessage
		if err := rows.Scan(&raw); err != nil {
			return nil, err
		}
		out = append(out, raw)
	}
	return out, rows.Err()
}

func writeZipJSON(zw *zip.Writer, name string, v any) error {
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal %s: %w", name, err)
	}
	w, err := zw.Create(name)
	if err != nil {
		return fmt.Errorf("create %s in zip: %w", name, err)
	}
	if _, err := w.Write(data); err != nil {
		return fmt.Errorf("write %s: %w", name, err)
	}
	return nil
}

// ----- envelope encryption -----

// EncryptArchive performs envelope encryption (init.md §4.5: "encryption
// keys managed separately from the archived data"): a fresh random
// 32-byte data key is generated per archive and used to AES-256-GCM
// encrypt the archive bytes directly, then the data key itself (never the
// archive) is wrapped — encrypted — with the platform's existing master
// cipher before being persisted. The master key never touches the archive
// bytes directly, and the archive is useless without both the stored
// wrapped key AND the master cipher to unwrap it.
//
// The data key reuses secrets.Cipher's own AES-256-GCM implementation
// (construct an ad-hoc Cipher from the random key, base64-encoded) rather
// than hand-rolling a second AES-GCM implementation for the exact same
// primitive.
func EncryptArchive(raw []byte, masterCipher *secrets.Cipher) (encrypted []byte, wrappedKeyB64 string, err error) {
	if masterCipher == nil {
		return nil, "", fmt.Errorf("master cipher is required")
	}

	dataKey := make([]byte, secrets.RequiredKeyBytes)
	if _, err := rand.Read(dataKey); err != nil {
		return nil, "", fmt.Errorf("generate data key: %w", err)
	}
	dataKeyB64 := base64.StdEncoding.EncodeToString(dataKey)

	dataCipher, err := secrets.New(dataKeyB64)
	if err != nil {
		return nil, "", fmt.Errorf("build data cipher: %w", err)
	}

	encryptedB64, err := dataCipher.Encrypt(string(raw))
	if err != nil {
		return nil, "", fmt.Errorf("encrypt archive: %w", err)
	}

	wrappedKeyB64, err = masterCipher.Encrypt(dataKeyB64)
	if err != nil {
		return nil, "", fmt.Errorf("wrap data key: %w", err)
	}

	return []byte(encryptedB64), wrappedKeyB64, nil
}

// DecryptArchive reverses EncryptArchive: unwrap the data key with the
// master cipher, then decrypt the archive bytes with the recovered data
// key.
func DecryptArchive(encrypted []byte, wrappedKeyB64 string, masterCipher *secrets.Cipher) ([]byte, error) {
	if masterCipher == nil {
		return nil, fmt.Errorf("master cipher is required")
	}

	dataKeyB64, err := masterCipher.Decrypt(wrappedKeyB64)
	if err != nil {
		return nil, fmt.Errorf("unwrap data key: %w", err)
	}
	dataCipher, err := secrets.New(dataKeyB64)
	if err != nil {
		return nil, fmt.Errorf("build data cipher: %w", err)
	}
	raw, err := dataCipher.Decrypt(string(encrypted))
	if err != nil {
		return nil, fmt.Errorf("decrypt archive: %w", err)
	}
	return []byte(raw), nil
}
