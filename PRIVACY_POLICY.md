# Privacy Policy Documentation
## 1herosocial.ai

**Last Updated:** June 2026  
**Effective Date:** June 1, 2026

---

## 1. Introduction

1herosocial.ai ("Company", "we", "us", "our") operates the 1herosocial.ai website and application ("Service"). This Privacy Policy explains how we collect, use, disclose, and safeguard your information when you use our Service.

**Contact:** support@1herosocial.ai

---

## 2. Information We Collect

### 2.1 Information You Provide Directly

- **Account Information:** Name, email, phone number, studio/business name, password
- **Billing Information:** Payment method, transaction history (processed by Stripe)
- **Communication Data:** WhatsApp, Facebook Messenger, Instagram messages and conversations
- **Profile Information:** Logo, brand color, studio settings, availability preferences
- **API Credentials:** Stripe keys, Meta app credentials (encrypted at rest)

### 2.2 Information Collected Automatically

- **Device Information:** Browser type, OS, device type, IP address
- **Usage Data:** Pages visited, features used, time spent, click patterns
- **Cookies:** Session tokens, authentication cookies (HttpOnly, Secure)
- **Log Data:** API request logs, error logs, audit trails

### 2.3 Third-Party Data

- **From Meta:** Contact info from WhatsApp/Messenger/Instagram
- **From Stripe:** Payment confirmation data
- **From AWS:** File metadata from S3 storage

---

## 3. How We Use Your Data

### 3.1 Service Delivery
- Provide messaging and communication features
- Process payments and subscriptions
- Send booking confirmations and receipts
- Deliver WhatsApp/Messenger messages

### 3.2 Service Improvement
- Analyze usage patterns and trends
- Identify bugs and performance issues
- Develop new features based on usage
- A/B test improvements

### 3.3 Communication
- Send technical notices and support messages
- Respond to inquiries and support requests
- Send billing and account notifications
- Marketing communications (with consent)

### 3.4 Security & Compliance
- Detect and prevent fraud
- Monitor for suspicious activity
- Comply with legal obligations
- Enforce terms of service

---

## 4. Data Storage & Security

### 4.1 Storage Location
- **Primary:** PostgreSQL database on AWS
- **Backups:** AWS S3 (encrypted, 30-day retention)
- **Files:** AWS S3 with encryption
- **Logs:** CloudWatch (90-day retention)

### 4.2 Security Measures
- ✅ AES-256 encryption for sensitive data at rest
- ✅ TLS 1.3 encryption in transit
- ✅ HTTPS only (no HTTP)
- ✅ API authentication via JWT tokens
- ✅ Rate limiting and DDoS protection
- ✅ Regular security audits
- ✅ PCI DSS compliance (payments via Stripe)

### 4.3 Access Control
- Role-based access (Super Admin, Studio Admin, User)
- RBAC enforcement at API level
- Audit logs for all data access
- Employee access restricted to production issues

---

## 5. Data Retention & Deletion

### 5.1 Retention Periods

| Data Type | Retention | Purpose |
|-----------|-----------|---------|
| Account Data | Until deletion | Service delivery |
| Messages | Until deletion | User reference |
| Payment Records | 7 years | Tax/audit compliance |
| Logs | 90 days | Security & debugging |
| Backups | 30 days | Disaster recovery |
| Deleted Data | 24 hours | Safe recovery window |

### 5.2 Account Deletion Process

**User-Initiated:**
1. Go to Settings → Security → Delete Account
2. Enter email to confirm identity
3. Click "Permanently Delete Account"
4. Account deleted within 24 hours

**What Gets Deleted:**
- ✅ Account profile and credentials
- ✅ All conversations and messages
- ✅ Studio information
- ✅ Payment methods (but NOT payment history for tax)
- ✅ Files and documents
- ✅ API credentials and tokens

**What's Retained (Legal Requirements):**
- Payment transaction records (7 years - tax law)
- Audit logs (for 90 days - security)
- Deleted data recovery (24 hours - safety)

### 5.3 Meta Data Deletion Callback

**Endpoint:** `https://1herosocial.ai/api/v1/webhooks/meta/data-deletion`

When a user requests deletion via Meta's privacy center:
1. Meta sends signed deletion request to our webhook
2. We verify the signature
3. We delete user data from our database
4. We return confirmation to Meta

---

## 6. User Rights (GDPR/CCPA)

### 6.1 Right to Access
- Request a copy of your personal data
- Export data in portable format
- **How:** Contact support@1herosocial.ai

### 6.2 Right to Rectification
- Correct inaccurate or incomplete data
- Update profile information
- **How:** Edit directly in Settings or contact support

### 6.3 Right to Erasure ("Right to be Forgotten")
- Request permanent deletion of data
- Exception: Legal/tax records retained per law
- **How:** Settings → Security → Delete Account

### 6.4 Right to Restrict Processing
- Prevent certain types of processing
- Pause automated decision-making
- **How:** Contact support@1herosocial.ai

### 6.5 Right to Data Portability
- Get data in machine-readable format
- Transfer data to another service
- **How:** Contact support for data export

### 6.6 Right to Opt-Out
- Unsubscribe from marketing emails
- Disable non-essential communications
- **How:** Unsubscribe link in emails or Settings

### 6.7 Right to Object
- Object to profiling or targeting
- Opt-out of analytics
- **How:** Settings or contact support

---

## 7. Third-Party Services

### 7.1 Stripe (Payments)
- **Data:** Name, email, payment method, transaction history
- **Processing:** Payment processing and verification
- **Privacy:** https://stripe.com/privacy
- **Compliance:** PCI DSS Level 1

### 7.2 Meta (WhatsApp, Messenger, Instagram)
- **Data:** Phone numbers, messages, profile info
- **Processing:** Message delivery and storage
- **Privacy:** https://www.meta.com/privacy
- **Note:** Meta is also a processor; they have their own privacy policy

### 7.3 AWS (Infrastructure)
- **Data:** All data stored on AWS infrastructure
- **Processing:** Hosting, storage, backups
- **Privacy:** https://aws.amazon.com/privacy
- **Region:** us-east-1 (Virginia, USA)

### 7.4 Google Cloud (Optional - Google Sheets integration)
- **Data:** Only spreadsheet ID and tab name (if enabled)
- **Processing:** Contact sync to Google Sheets
- **Privacy:** https://policies.google.com/privacy

---

## 8. International Data Transfers

### 8.1 Data Location
- Primary servers: AWS us-east-1 (Virginia, USA)
- Backups: AWS us-east-1
- CDN: CloudFront (global)

### 8.2 GDPR Compliance
- Standard Contractual Clauses (SCCs) with AWS
- Data Processing Agreement (DPA) available
- Users in EU have full GDPR rights

### 8.3 Cross-Border Transfers
- Data may be processed in the USA
- EU users consent to Standard Contractual Clauses
- Data adequacy mechanisms in place

---

## 9. Children's Privacy

- **Minimum Age:** 18 years old
- **Policy:** We do not knowingly collect data from minors
- **Action:** If discovered, we will delete immediately
- **Contact:** support@1herosocial.ai if concerned

---

## 10. Policy Changes

- **Notice:** Updates will be posted with "Last Updated" date changed
- **Significant Changes:** Email notification to users
- **Your Consent:** Continued use = acceptance of new policy
- **Archive:** Previous versions available upon request

---

## 11. Contact & Rights Requests

### 11.1 Data Subject Requests
To exercise any rights, contact:
- **Email:** support@1herosocial.ai
- **Response Time:** 30 days (GDPR requirement)
- **Verification:** We may request ID verification

### 11.2 Complaints
EU users can file complaints with local data protection authorities:
- **Ireland:** Data Protection Commission (ico.org.uk)
- **Your Country:** National Data Protection Authority

### 11.3 Support
- **Email:** support@1herosocial.ai
- **Hours:** Business hours, response within 24 hours
- **Privacy Concerns:** Flag with "PRIVACY" in subject line

---

## 12. Special Provisions

### 12.1 B2B Messaging
- Messages are business communications
- May be retained per business requirements
- Deletion does not affect other users' copies

### 12.2 Compliance & Legal
- We may retain data for legal disputes
- Law enforcement requests: We notify users when possible
- Subpoenas and warrants must be legally valid

### 12.3 Marketing & Analytics
- Google Analytics with IP anonymization
- No cross-site tracking
- Users can opt-out in Settings

---

## 13. Security Incident Response

### 13.1 Breach Notification
- **Timeline:** Notify users within 72 hours of discovery
- **Information:** What happened, what data, what actions to take
- **Authorities:** Notify relevant data protection authorities

### 13.2 User Actions
- Change your password immediately
- Monitor your accounts
- Contact support with questions

---

## 14. Data Protection Officer (DPO)

- **Role:** Oversees privacy compliance
- **Contact:** support@1herosocial.ai
- **Available:** For data subject rights requests

---

## 15. Definitions

- **Personal Data:** Any info relating to identified or identifiable person
- **Processing:** Any operation performed on data (collection, use, storage)
- **Controller:** Organization determining purposes of processing (us)
- **Processor:** Organization processing data on behalf of controller (Stripe, AWS, Meta)
- **Data Subject:** The person whose data is being processed

---

## Summary of Changes from Previous Version

**Version 2.0 (Current - June 2026):**
- ✨ Added account deletion feature
- ✨ Added Meta data deletion callback
- ✨ Added data retention periods table
- ✨ Enhanced GDPR compliance section
- ✨ Added security measures detail
- ✨ Clarified international transfers
- ✨ Added DPO information

---

## Approval & Signature

**Document Version:** 2.0  
**Created:** June 2026  
**Review Status:** Draft (Pending Legal Review)  
**Compliance:** GDPR, CCPA, Data Protection Act 2018

---

**Next Steps:**
1. ✅ Legal review by compliance team
2. ✅ Update with any legal feedback
3. ✅ Publish on website
4. ✅ Add to footer as "Privacy Policy" link
5. ✅ Notify users of policy update
