# Security Incident Response Runbook

Status: draft — written as part of `platform-security-hardening` task 9.3.
**Not yet reviewed by a human or staffed with real on-call contacts** — the
on-call rotation and contact details below are placeholders the team must
fill in; this document is not "done" until someone signs off on it and the
placeholders are replaced with real names/numbers.

## 1. On-call rotation

| Role | Name | Contact | Backup |
|---|---|---|---|
| Primary on-call | _TBD_ | _TBD_ | _TBD_ |
| Secondary on-call | _TBD_ | _TBD_ | _TBD_ |
| Incident commander (if different from on-call) | _TBD_ | _TBD_ | _TBD_ |
| Legal/compliance contact (for breach notification) | _TBD_ | _TBD_ | _TBD_ |

Rotation schedule: _TBD_ — e.g. weekly handoff, PagerDuty schedule link, etc.

## 2. Severity levels

| Level | Definition | Example | Response time |
|---|---|---|---|
| SEV1 | Active data breach, credential compromise, or platform-wide outage caused by a security issue | Attacker has working admin credentials; customer PII confirmed exfiltrated | Immediate — page on-call now |
| SEV2 | Contained security issue with limited blast radius, not actively being exploited | A leaked API key found in git history, not yet confirmed used | Within 1 hour |
| SEV3 | Security-relevant finding with no immediate risk | A dependency vulnerability scan finding, not exploitable in this app's usage | Within 1 business day |

## 3. Immediate response steps (SEV1/SEV2)

1. **Declare the incident.** Page the primary on-call (see §1). If they
   don't acknowledge within 15 minutes, escalate to secondary.
2. **Contain.** Depending on the incident type:
   - Suspected compromised account/session → `POST /api/v1/admin/users/{userId}/sessions/end`
     (force-logout) or, for a platform-wide concern,
     `POST /api/v1/admin/sessions/end-all` (platform-security-hardening
     task 1.6 — this also rotates the JWT signing key via task 1.9,
     invalidating every outstanding token, not just revoking sessions).
   - Suspected leaked platform secret (AI provider key, OCR key, etc) →
     rotate it immediately at the provider, then update it via
     `PUT /api/v1/admin/ai-providers/{provider}/key` or the equivalent
     studio-level endpoint (task 0.2's write-only key handling means the
     old value can't be read back, only replaced).
   - Suspected 2FA bypass / stolen recovery codes → force a 2FA reset
     via the admin-reset chain (task 2.8:
     `POST /api/v1/admin/users/{userId}/2fa/reset`), which also revokes
     all of that user's sessions and emails them.
   - Suspected SQL injection / RCE / other code-level exploit → take the
     affected endpoint down (feature flag or route removal) before
     investigating root cause; do not leave an active exploit path live
     while debugging.
3. **Preserve evidence.** Do NOT delete or truncate `security_audit_log`
   rows (they are already append-only — task 3.5 — but this is a
   reminder not to work around that via direct DB access either).
   Snapshot relevant logs (application logs, `security_audit_log`
   entries for the affected account/time window, infrastructure logs)
   before any remediation step that might overwrite them.
4. **Assess scope.** Query `security_audit_log` for the affected
   account/studio and time window (task 3.6's read-access endpoints, or
   direct DB access for an incident commander). Identify: what was
   accessed, by whom (or what), when it started, whether it's ongoing.
5. **Notify.** See §4 for who needs to be told and when.
6. **Remediate.** Fix the root cause — rotate every potentially-exposed
   credential (not just the one known to be compromised), patch the
   vulnerability, etc.
7. **Post-incident review.** Within 5 business days of resolution, write
   up: timeline, root cause, what contained it, what the permanent fix
   was, and at least one concrete follow-up action with an owner.

## 4. Breach notification

Singapore (PDPA) and Malaysia (PDPA) both require notifying affected
individuals and the relevant regulator when a data breach meets their
respective notification thresholds. This section gives draft templates
only — **legal/compliance sign-off is required before sending any real
notification**; these templates are a starting point, not pre-approved
copy.

### 4.1 Singapore PDPA — notifiable data breach

Singapore's PDPA requires notifying the Personal Data Protection
Commission (PDPC) and affected individuals when a breach is likely to
result in significant harm, or affects 500+ individuals. Notify PDPC
within 3 calendar days of determining the breach is notifiable.

**Template — notification to PDPC:**

> Subject: Data Breach Notification — [Company name]
>
> We are writing to notify the Personal Data Protection Commission of a
> data breach affecting [Company name]'s systems, discovered on
> [date discovered].
>
> **Nature of the breach:** [brief description — e.g. unauthorized
> access to a customer database via a compromised admin account]
>
> **Categories and approximate number of individuals affected:**
> [e.g. ~X studio_staff accounts across Y studios; customer lead
> records including name/phone/email]
>
> **Categories of personal data involved:** [e.g. names, phone numbers,
> email addresses, fitness goals — NOT payment card data, which is
> never stored by this system / is tokenized via Stripe]
>
> **Actions taken:** [e.g. affected credentials rotated, sessions
> revoked, vulnerability patched on [date]]
>
> **Contact for further information:** [legal/compliance contact]

**Template — notification to affected individuals:**

> Subject: Important security notice about your account
>
> We're writing to let you know about a security incident that may have
> affected your personal data.
>
> **What happened:** [plain-language description]
>
> **What information was involved:** [specific data types — be precise,
> not vague]
>
> **What we've done:** [remediation steps taken]
>
> **What you can do:** [e.g. "we've already reset your password — no
> action needed" or "please change your password as a precaution"]
>
> **Questions:** Contact us at [support contact].

### 4.2 Malaysia PDPA — notifiable data breach

Malaysia's PDPA (as amended, with mandatory breach notification
provisions) similarly requires notifying the Department of Personal Data
Protection and affected individuals for qualifying breaches. Use the same
two templates above, adjusted for the Malaysian regulator's specific
submission process and timeline (**confirm the current notification
deadline with legal/compliance before relying on this document** — PDPA
breach-notification rules are newer and may differ in deadline from
Singapore's).

## 5. What this runbook does NOT cover (remaining gaps)

- Real on-call tooling (PagerDuty/Opsgenie or similar) is not set up —
  the "page the primary on-call" step in §3 has no actual paging
  mechanism behind it yet.
- This runbook has not been reviewed by legal/compliance or exercised
  in a tabletop drill.
- Specific internal escalation paths (e.g. "who approves a public
  statement") are not defined.
