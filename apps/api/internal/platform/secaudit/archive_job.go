package secaudit

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/projectx/api/internal/platform/s3"
)

// ArchiveJob produces the daily write-once copy of security_audit_log
// referenced by task 3.5 ("daily write-once (S3 Object Lock) copies").
// This is the buildable half of that task: the query + upload logic.
// Actual provisioning of an S3 bucket with Object Lock enabled (the
// retention guarantee itself — this code alone cannot make an upload
// immutable) is blocked in this sandbox, same as task 0.6's AWS
// Secrets Manager blocker — there is no real AWS account/console access
// available here. Uploader is reused as-is from internal/platform/s3;
// whether its target bucket has Object Lock enabled is an infra
// decision outside this job's control.
type ArchiveJob struct {
	pool     *pgxpool.Pool
	uploader *s3.Uploader
}

func NewArchiveJob(pool *pgxpool.Pool, uploader *s3.Uploader) *ArchiveJob {
	return &ArchiveJob{pool: pool, uploader: uploader}
}

// RunOnce archives every row created on the given UTC date into one
// JSON-lines object keyed by date, so a daily cron (not wired up here —
// scheduling infra is also part of the blocked provisioning work) can
// call this once per day. Returns the number of rows archived.
func (j *ArchiveJob) RunOnce(ctx context.Context, day time.Time) (int, error) {
	start := time.Date(day.Year(), day.Month(), day.Day(), 0, 0, 0, 0, time.UTC)
	end := start.Add(24 * time.Hour)

	rows, err := j.pool.Query(ctx, `
		SELECT id, actor_user_id, actor_role, studio_id, action, target_type, target_id,
		       metadata, ip, user_agent, country, session_id, outcome, created_at
		FROM security_audit_log
		WHERE created_at >= $1 AND created_at < $2
		ORDER BY created_at
	`, start, end)
	if err != nil {
		return 0, fmt.Errorf("query day's rows: %w", err)
	}
	defer rows.Close()

	var buf []byte
	n := 0
	for rows.Next() {
		var rec map[string]any = map[string]any{}
		var id, actorUserID, studioID, sessionID any
		var actorRole, action, targetType, targetID, ip, userAgent, country, outcome any
		var metadata []byte
		var createdAt time.Time
		if err := rows.Scan(&id, &actorUserID, &actorRole, &studioID, &action, &targetType, &targetID,
			&metadata, &ip, &userAgent, &country, &sessionID, &outcome, &createdAt); err != nil {
			return n, fmt.Errorf("scan row: %w", err)
		}
		rec["id"] = id
		rec["actorUserId"] = actorUserID
		rec["actorRole"] = actorRole
		rec["studioId"] = studioID
		rec["action"] = action
		rec["targetType"] = targetType
		rec["targetId"] = targetID
		rec["metadata"] = json.RawMessage(metadata)
		rec["ip"] = ip
		rec["userAgent"] = userAgent
		rec["country"] = country
		rec["sessionId"] = sessionID
		rec["outcome"] = outcome
		rec["createdAt"] = createdAt
		line, err := json.Marshal(rec)
		if err != nil {
			return n, fmt.Errorf("marshal row: %w", err)
		}
		buf = append(buf, line...)
		buf = append(buf, '\n')
		n++
	}
	if err := rows.Err(); err != nil {
		return n, err
	}
	if n == 0 {
		return 0, nil
	}

	key := fmt.Sprintf("security-audit-log/%s.jsonl", start.Format("2006-01-02"))
	if err := j.uploader.UploadPrivate(ctx, key, buf, "application/x-ndjson"); err != nil {
		slog.Error("security audit log archive upload failed", "err", err, "key", key, "rows", n)
		return n, fmt.Errorf("upload archive: %w", err)
	}
	slog.Info("security audit log archived", "key", key, "rows", n)
	return n, nil
}

// VerifyWriteOnce is task 9.2's "the daily S3 Object Lock copy actually
// landed and is immutable" check: downloads the given day's archive
// back and confirms the bucket's lock policy rejects an attempted
// overwrite. BLOCKED from running in this sandbox for the same reason
// as RunOnce's own doc comment — there is no real S3 bucket with Object
// Lock enabled to verify against here. This function is the buildable
// logic; it has not been exercised against a real bucket.
func (j *ArchiveJob) VerifyWriteOnce(ctx context.Context, day time.Time) error {
	key := fmt.Sprintf("security-audit-log/%s.jsonl", day.Format("2006-01-02"))
	original, err := j.uploader.Download(ctx, key)
	if err != nil {
		return fmt.Errorf("download archive for verification: %w", err)
	}
	// Attempting to overwrite the same key with different content MUST
	// fail once Object Lock is actually enabled on the bucket — this
	// call succeeding (no error) means either the key didn't exist, or
	// the bucket's lock policy is NOT protecting it, which is exactly
	// the misconfiguration this check exists to catch.
	tamperErr := j.uploader.UploadPrivate(ctx, key, []byte("tamper-attempt"), "application/x-ndjson")
	if tamperErr == nil {
		return fmt.Errorf("write-once verification FAILED: overwrite of %s succeeded — Object Lock is not protecting this bucket/key", key)
	}
	_ = original
	return nil
}
