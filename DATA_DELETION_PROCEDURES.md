# Data Deletion Procedures
## User Account Deletion & Data Privacy

**Last Updated:** June 2026  
**Document Version:** 1.0

---

## 1. User-Initiated Account Deletion

### 1.1 Process Flow

```
User Goes to Settings
        ↓
Clicks Security Tab
        ↓
Clicks "Delete Account Permanently"
        ↓
Enters Email to Confirm
        ↓
Reviews Warning Dialog
        ↓
Confirms Final Deletion
        ↓
Backend Processes Deletion
        ↓
Account Deleted (24 hours)
        ↓
User Logged Out
```

### 1.2 Step-by-Step Instructions for Users

**Step 1: Navigate to Account Settings**
- Log into your 1herosocial.ai account
- Go to Settings (gear icon)
- Click on "Security" tab

**Step 2: Locate Delete Account Option**
- Scroll down to "Delete Account" section
- Note the red warning border
- Read the warning message carefully

**Step 3: Click Delete Button**
- Click "Delete Account Permanently"
- A form will appear

**Step 4: Confirm Email**
- Enter your account email address
- This must match your registered email
- Click Submit

**Step 5: Final Confirmation**
- A final warning dialog appears
- Warning states: "This action cannot be undone"
- Click "Permanently Delete" to confirm
- Or "Cancel" to abort

**Step 6: Deletion Initiated**
- Account marked for deletion
- You will be logged out immediately
- Deletion completes within 24 hours

### 1.3 What Happens After Deletion Request

**Immediately (0-5 minutes):**
- ✅ User session ended
- ✅ All API tokens revoked
- ✅ Account marked for deletion
- ✅ User redirected to login page

**Within 24 Hours:**
- ✅ All conversations deleted
- ✅ All messages purged
- ✅ Studio information removed
- ✅ Files and documents deleted
- ✅ API credentials deactivated
- ✅ Payment methods removed*

**Permanent Records (Legal Hold):**
- 📋 Payment transactions (7 years)
- 📋 Audit logs (90 days)
- 📋 Deleted data backup (24 hours recovery window)

*Note: Payment history is retained for tax and fraud prevention purposes per legal requirements.

---

## 2. What Gets Deleted

### 2.1 Complete Deletion

| Data Category | Deleted | Timeline |
|---|---|---|
| Account Profile | ✅ Yes | Immediate |
| Studio Information | ✅ Yes | < 24 hours |
| All Messages | ✅ Yes | < 24 hours |
| All Conversations | ✅ Yes | < 24 hours |
| Uploaded Files | ✅ Yes | < 24 hours |
| Photos/Videos | ✅ Yes | < 24 hours |
| API Credentials | ✅ Yes | Immediate |
| Payment Methods | ✅ Yes | < 24 hours |
| Stripe Tokens | ✅ Yes | < 24 hours |
| Meta App Connection | ✅ Yes | < 24 hours |
| User Sessions | ✅ Yes | Immediate |
| Authentication Tokens | ✅ Yes | Immediate |

### 2.2 Permanently Retained (Legal Requirement)

| Data Category | Retained | Duration | Reason |
|---|---|---|---|
| Payment Records | 📋 Yes | 7 years | Tax compliance |
| Transaction History | 📋 Yes | 7 years | Audit trail |
| Fraud Detection Logs | 📋 Yes | 3 years | Fraud prevention |
| System Audit Logs | 📋 Yes | 90 days | Security |
| Backup Data | 📋 Yes | 24 hours | Disaster recovery |

---

## 3. Meta Data Deletion Callback

### 3.1 Overview
When a user deletes their data through Meta's privacy center (WhatsApp, Messenger, or Instagram), Meta sends a deletion request to our server.

### 3.2 Webhook Details

**Endpoint:** `https://1herosocial.ai/api/v1/webhooks/meta/data-deletion`  
**Method:** POST  
**Authentication:** Signed by Meta (signature verification required)

### 3.3 Request Format

```json
{
  "signed_request": "signature.payload"
}
```

- **signature:** HMAC-SHA256 hash
- **payload:** Base64-encoded JSON with user deletion request

### 3.4 Data Verified & Deleted

When Meta deletion request arrives:
1. ✅ Verify signature using Meta app secret
2. ✅ Extract user identifier from payload
3. ✅ Find associated users in database
4. ✅ Delete all user data
5. ✅ Return confirmation to Meta
6. ✅ Log deletion action for audit trail

### 3.5 Response to Meta

```json
{
  "url": "https://1herosocial.ai/privacy"
}
```

**Success:** 200 OK with privacy policy URL  
**Failure:** 400 Bad Request (Meta will retry)

---

## 4. GDPR Data Subject Rights

### 4.1 Right to Access (GDPR Article 15)

**Request:**
- Email: govind.infaira@gmail.com
- Subject: "GDPR Data Subject Access Request"
- Include: Your email/user ID

**Response Timeline:** 30 days maximum

**What You Get:**
- Copy of all personal data we hold
- Purposes of processing
- Recipients of data
- Retention periods
- Your rights

### 4.2 Right to Rectification (GDPR Article 16)

**How to Exercise:**
- Go to Settings and update your information
- OR contact: govind.infaira@gmail.com
- OR submit form at: https://1herosocial.ai/privacy

**Response Timeline:** 30 days maximum

**We Will:**
- Update incorrect information
- Supplement incomplete data
- Notify recipients of the change (where feasible)

### 4.3 Right to Erasure (GDPR Article 17)

**How to Request:**
- Settings → Security → Delete Account
- OR email: govind.infaira@gmail.com
- Subject: "GDPR Right to Erasure Request"

**Timeline:** 24 hours for most data

**Exceptions:**
- Legal obligations (tax records: 7 years)
- Fraud prevention (3 years)
- Legal claims (as long as needed)

### 4.4 Right to Restrict Processing (GDPR Article 18)

**Request to:**
- Email: govind.infaira@gmail.com
- Subject: "GDPR Restrict Processing Request"

**We Will:**
- Stop active processing
- Mark data as restricted
- Only process with your consent or legal basis
- Notify you before lifting restriction

### 4.5 Right to Data Portability (GDPR Article 20)

**Request to:**
- Email: govind.infaira@gmail.com
- Subject: "GDPR Data Portability Request"

**Format Provided:**
- CSV or JSON format
- Machine-readable
- Structured and organized
- Includes all personal data

**Timeline:** 30 days maximum

### 4.6 Right to Object (GDPR Article 21)

**Can Object To:**
- Marketing communications (always allowed)
- Profiling/automated decisions
- Data processing for direct marketing

**How:**
- Unsubscribe link in emails
- Settings → Communications preferences
- Email: govind.infaira@gmail.com

---

## 5. Verification Process

### 5.1 Identity Verification Requirements

**For Data Subject Rights Requests:**
1. Email from registered email address
2. Provide user ID or studio name
3. Answer security question (if applicable)
4. Photo ID (for deletion confirmation)

**Timeline:** Verification within 5 business days

### 5.2 Deletion Confirmation Email

After account deletion is confirmed:

```
Subject: Your 1herosocial.ai Account Has Been Deleted

Dear [User Name],

Your request to permanently delete your 1herosocial.ai account has been 
processed on [Date].

What was deleted:
- Account profile and settings
- All conversations and messages
- Studio information
- Files and documents
- API credentials

What was retained (legal requirement):
- Payment records (7 years)
- Audit logs (90 days)

You can no longer log in with these credentials. If this was a mistake,
please contact us within 30 days for recovery assistance.

Questions? Contact: govind.infaira@gmail.com

Best regards,
The 1herosocial.ai Privacy Team
```

---

## 6. Data Retention Calendar

| Event | Retention Start | Duration | Deletion Date | Purpose |
|---|---|---|---|---|
| User Registration | Account creation | Until deletion | On request | Service delivery |
| Last Login | Last activity | 6 months inactivity notice | Sent at 6 months | Engagement |
| Payment Transaction | Payment date | 7 years | Tax year + 7 | Tax compliance |
| Failed Login Attempt | Attempt time | 90 days | Auto-delete | Security |
| API Request Log | Request timestamp | 90 days | Auto-delete | Debugging |
| Deleted User Data | Deletion date | 24 hours | Auto-delete | Safe recovery |
| System Backup | Creation time | 30 days | Auto-delete | Disaster recovery |
| Abuse Report | Report date | 2 years | Manual review | Legal hold |

---

## 7. Breach Notification

### 7.1 If a Breach Occurs

**User Notification Timeline:**
- ✅ Inform within 72 hours of discovery
- ✅ Email to registered address
- ✅ Detailed breach report
- ✅ Recommended actions

**Content of Notification:**
- What happened
- What data was affected
- When we discovered it
- What we've done
- What users should do
- Contact for questions

### 7.2 Regulatory Notification

- ✅ Notify data protection authorities
- ✅ Provide incident details
- ✅ Share remediation steps
- ✅ Assist with investigation

---

## 8. Third-Party Data Handling

### 8.1 Stripe (Payments)

**Data They Receive:**
- Name, email, payment method
- Transaction amount and date
- Billing address (if provided)

**Their Deletion:**
- Stripe retains per PCI DSS (7 years)
- Request deletion via Stripe support
- We cannot force their deletion

### 8.2 Meta (WhatsApp, Messenger, Instagram)

**Data They Receive:**
- Phone numbers
- Message content
- Timestamps
- Read receipts

**Their Deletion:**
- Meta deletes via their privacy center
- Meta sends deletion request to us
- We delete from our database
- Sync with their deletion schedule

### 8.3 AWS (Infrastructure)

**Data They Store:**
- Database backups
- File storage
- Logs

**Their Deletion:**
- Automatic per our retention policy
- Encrypted deletion (not just removed)
- Verified deletion certificate available

---

## 9. Compliance Checklist

### 9.1 GDPR Compliance
- ✅ Legal basis for processing documented
- ✅ Data Protection Impact Assessment (DPIA) conducted
- ✅ Standard Contractual Clauses (SCC) with processors
- ✅ Data Processing Agreement (DPA) executed
- ✅ Privacy Policy published
- ✅ User consent mechanism implemented
- ✅ Data Subject Rights procedures established
- ✅ Breach notification procedures documented
- ✅ Data Retention policy defined
- ✅ DPO appointed and contactable

### 9.2 CCPA Compliance (California)
- ✅ Privacy notice published
- ✅ Right to know disclosure
- ✅ Right to delete mechanism
- ✅ Right to opt-out of sale
- ✅ Non-discrimination clause
- ✅ Verification procedures

### 9.3 Data Protection Act 2018 (UK)
- ✅ Data Protection Officer information
- ✅ Lawful basis assessment
- ✅ Rights information
- ✅ Legitimate interests documentation

---

## 10. Auditing & Monitoring

### 10.1 Deletion Audit Log

Every deletion is logged with:
- Date & time of deletion request
- User ID and email
- Initiated by (user/admin/legal)
- Data deleted (list)
- Approved by (staff member)
- Completion timestamp
- Verification hash

### 10.2 Quarterly Review

- ✅ Review deletion requests
- ✅ Verify data was deleted
- ✅ Check compliance
- ✅ Update procedures if needed
- ✅ Document findings

### 10.3 Annual Audit

- ✅ Third-party privacy audit
- ✅ Penetration testing
- ✅ Compliance certification
- ✅ Policy review
- ✅ Staff training verification

---

## 11. Contact & Support

### Deletion Requests & Privacy Inquiries
- **Email:** govind.infaira@gmail.com
- **Subject:** Start with "PRIVACY:" or "GDPR:"
- **Response Time:** 5 business days

### Types of Requests Supported
- ✅ Account deletion
- ✅ Data subject rights
- ✅ Privacy inquiries
- ✅ Breach notifications
- ✅ GDPR requests
- ✅ CCPA requests

---

**Document Status:** Ready for Review  
**Approval Required:** Legal Team  
**Implementation Date:** Upon Approval

---

## Appendix A: User FAQ

**Q: How long does deletion take?**  
A: Immediate logout, full deletion within 24 hours.

**Q: Can I recover my account after deletion?**  
A: Yes, within 30 days contact support. After 30 days, permanent deletion.

**Q: What about Meta/Stripe?**  
A: We delete our data. They have their own retention policies.

**Q: Who can see my data after deletion?**  
A: No one. Data is permanently deleted except legal records.

**Q: Do you sell my data?**  
A: No. We do not sell, trade, or share personal data.

**Q: How do I get a copy of my data?**  
A: Email govind.infaira@gmail.com with subject "Data Export Request".

---

**END OF DOCUMENT**
