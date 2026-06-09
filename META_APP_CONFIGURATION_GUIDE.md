# Meta App Configuration Guide
## 1herosocial.ai - Complete Setup Documentation

**Last Updated:** June 2026  
**Status:** Ready for Meta App Configuration

---

## 1. Document Review Summary

### Privacy Policy URL
**Link:** https://1herosocial.ai/privacy

**Content Included:**
- Data collection methods and types
- How we use personal data
- Data storage and security measures
- User rights (GDPR, CCPA, Data Protection Act)
- Third-party services (Stripe, Meta, AWS, Google Cloud)
- Data retention periods
- Account deletion procedures
- Breach notification procedures
- Data Protection Officer contact
- Contact information for data subject requests

**Status:** ✓ COMPLETE

---

### Terms of Service URL
**Link:** https://1herosocial.ai/terms

**Content Included:**
- Agreement to terms
- Use license and prohibited conduct
- Warranty disclaimers
- Limitation of liability
- User account responsibilities
- Payment and billing terms
- Refund policy
- Account termination conditions
- Data and privacy governance
- Account deletion and data removal
- Third-party services disclosure
- Intellectual property rights
- Modifications and governing law
- Important reminders section

**Status:** ✓ COMPLETE

---

### Data Deletion Callback URL
**Endpoint:** https://1herosocial.ai/api/v1/webhooks/meta/data-deletion

**Functionality:**
- Receives signed deletion requests from Meta
- Verifies request signatures
- Deletes user data from 1herosocial.ai systems
- Returns confirmation to Meta
- Audit logs deletion requests
- Complies with GDPR Article 17 (Right to Erasure)
- Complies with CCPA deletion requirements

**Features:**
- Signature verification using Meta app secret
- Immediate acknowledgment of deletion requests
- Automated data purge within 24 hours
- Retention of legal records (7 years for tax, 90 days for audit)
- Logging and audit trail
- Confirmation response to Meta

**Status:** ✓ COMPLETE & DEPLOYED

---

## 2. Contact Email Configuration

**Current:** support@1herosocial.ai

**Uses:**
- Privacy inquiries
- Data subject rights requests
- Account deletion support
- GDPR/CCPA requests
- General support contact

**Response Time:** 5 business days

**Status:** ✓ UPDATED IN ALL DOCUMENTS

---

## 3. Data Protection Officer (DPO) Information

### Required Details for Meta:

```
Name:
  Full Name: [Your Name]
  Optional: Yes - Can be left blank if using title only

Email:
  support@1herosocial.ai
  (DPO contact email for data protection inquiries)

Address (Optional but recommended):
  Street Address: [Your Business Address]
  Apartment/Suite: [Leave blank if not applicable]
  City/District: [City Name]
  State/Province/Region: [State/Region]
  ZIP/Postal Code: [Postal Code]
  Country: United States
```

---

## 4. Recommended DPO Contact Details

### Option 1: Individual DPO
If you have designated a specific Data Protection Officer:

```
Name: [DPO Full Name]
Email: support@1herosocial.ai
Address: 
  Street: [Business Address]
  City: [City]
  State: [State]
  ZIP: [Postal Code]
  Country: United States
```

### Option 2: Organizational DPO (Recommended)
If DPO role is handled by organization:

```
Name: Data Protection Officer
Title: Chief Privacy Officer
Email: support@1herosocial.ai
Address:
  Street: [Your Business Address]
  City: [Your City]
  State: [Your State]
  ZIP: [Your Postal Code]
  Country: United States
```

---

## 5. GDPR Compliance Checklist

All items below are implemented in the documents:

### Data Subject Rights
- ✓ Right to Access (Article 15)
  - URL: https://1herosocial.ai/privacy
  - Contact: support@1herosocial.ai

- ✓ Right to Rectification (Article 16)
  - URL: https://1herosocial.ai/privacy
  - Contact: support@1herosocial.ai

- ✓ Right to Erasure (Article 17)
  - URL: https://1herosocial.ai/delete-account
  - Contact: support@1herosocial.ai

- ✓ Right to Restrict Processing (Article 18)
  - URL: https://1herosocial.ai/privacy
  - Contact: support@1herosocial.ai

- ✓ Right to Data Portability (Article 20)
  - URL: https://1herosocial.ai/privacy
  - Contact: support@1herosocial.ai

- ✓ Right to Object (Article 21)
  - URL: https://1herosocial.ai/privacy
  - Contact: support@1herosocial.ai

### Data Protection Requirements
- ✓ Privacy Policy Published
  - Location: https://1herosocial.ai/privacy

- ✓ Terms of Service Published
  - Location: https://1herosocial.ai/terms

- ✓ Data Deletion Callback URL
  - Endpoint: https://1herosocial.ai/api/v1/webhooks/meta/data-deletion

- ✓ Data Retention Policy
  - Document: DATA_DELETION_PROCEDURES.md
  - Payment records: 7 years
  - Audit logs: 90 days
  - Backups: 24 hours

- ✓ Breach Notification
  - Timeline: 72 hours
  - Contact: support@1herosocial.ai

- ✓ Data Protection Officer Designated
  - Contact: support@1herosocial.ai

---

## 6. Missing or Additional Items to Consider

### Currently Implemented:
- ✓ Privacy Policy
- ✓ Terms of Service
- ✓ Account Deletion Page
- ✓ DPO Contact Information
- ✓ Data Deletion Callback
- ✓ All GDPR Rights documented
- ✓ CCPA Rights documented
- ✓ UK Data Protection Act compliance
- ✓ Data retention policies

### Optional Enhancements:
1. **Cookie Policy** (Optional)
   - Currently: Linked from Privacy Policy
   - Recommendation: Create separate page if using cookies

2. **Accessibility Statement** (Optional)
   - For ADA/WCAG compliance
   - Recommendation: Add if required by law

3. **Subprocessor List** (Optional)
   - Current: Stripe, Meta, AWS, Google Cloud listed
   - Recommendation: Keep updated with any new processors

4. **Legitimate Interest Assessment** (Optional)
   - For EU users
   - Recommendation: Document processing basis

---

## 7. Meta App Configuration Checklist

### Required Fields
- [ ] App ID: 2405726999940224
- [ ] Display Name: 1herosocial.ai
- [ ] Namespace: 1herosocial_ai
- [ ] Contact Email: support@1herosocial.ai
- [ ] Privacy Policy URL: https://1herosocial.ai/privacy
- [ ] Terms of Service URL: https://1herosocial.ai/terms
- [ ] Data Deletion Callback URL: https://1herosocial.ai/api/v1/webhooks/meta/data-deletion

### Optional but Recommended
- [ ] DPO Name: [Your Name or Organization]
- [ ] DPO Email: support@1herosocial.ai
- [ ] DPO Address: [Your Business Address]
- [ ] DPO City: [Your City]
- [ ] DPO State: [Your State]
- [ ] DPO ZIP: [Your Postal Code]
- [ ] DPO Country: United States
- [ ] App Icon: 1024x1024 PNG
- [ ] App Category: Business

---

## 8. Webhook Configuration for Meta

### WhatsApp Business Account
**Webhook URL:** https://1herosocial.ai/api/v1/webhooks/meta/whatsapp  
**Verify Token:** my_secret_token_123  
**Subscribe to:** messages, message_status

### Facebook Messenger
**Webhook URL:** https://1herosocial.ai/api/v1/webhooks/meta/messenger  
**Verify Token:** my_secret_token_123  
**Subscribe to:** messages, messaging_postbacks

### Instagram Direct Messages
**Webhook URL:** https://1herosocial.ai/api/v1/webhooks/meta/instagram  
**Verify Token:** my_secret_token_123  
**Subscribe to:** messages, message_status

---

## 9. Data Deletion Flow

```
User deletes via Meta Privacy Center
        ↓
Meta sends signed request to:
https://1herosocial.ai/api/v1/webhooks/meta/data-deletion
        ↓
We verify signature using app secret
        ↓
We delete user data from database
        ↓
We return confirmation to Meta
        ↓
User data deleted from 1herosocial.ai
(except legally required records)
        ↓
Audit log created for compliance
```

---

## 10. Recommended DPO Information Template

Fill in the following details and provide to Meta:

```
DATA PROTECTION OFFICER CONTACT

Name: 
[Provide name or leave as "Data Protection Officer"]

Email: 
support@1herosocial.ai

Phone: 
[Optional - Leave blank if not available]

Mailing Address:
Street: [Business address line 1]
Suite/Apt: [Leave blank if not applicable]
City: [Your city]
State: [Your state]
ZIP Code: [Your postal code]
Country: United States

Availability:
Business Hours: [Monday-Friday, 9am-5pm EST]
Response Time: 5 business days
```

---

## 11. Key URLs Reference

| Purpose | URL |
|---------|-----|
| Privacy Policy | https://1herosocial.ai/privacy |
| Terms of Service | https://1herosocial.ai/terms |
| Account Deletion | https://1herosocial.ai/delete-account |
| WhatsApp Webhook | https://1herosocial.ai/api/v1/webhooks/meta/whatsapp |
| Messenger Webhook | https://1herosocial.ai/api/v1/webhooks/meta/messenger |
| Instagram Webhook | https://1herosocial.ai/api/v1/webhooks/meta/instagram |
| Data Deletion Webhook | https://1herosocial.ai/api/v1/webhooks/meta/data-deletion |

---

## 12. Next Steps

1. **Fill in DPO Information**
   - Name or Title
   - Email: support@1herosocial.ai
   - Business Address

2. **Configure Meta App Dashboard**
   - Enter all required fields from Section 7
   - Upload app icon (1024x1024)
   - Set app category

3. **Verify URLs**
   - Test Privacy Policy URL
   - Test Terms of Service URL
   - Test Data Deletion Callback

4. **Configure Webhooks**
   - Set up WhatsApp webhook
   - Set up Messenger webhook
   - Set up Instagram webhook
   - Set verify token: my_secret_token_123

5. **Submit for Review**
   - Meta will review compliance
   - Ensure DPO contact is verified
   - Wait for approval

---

## 13. Support Contact

For all privacy, data protection, and compliance inquiries:

**Email:** support@1herosocial.ai  
**Subject Line:** Start with "PRIVACY:" or "GDPR:"  
**Response Time:** 5 business days  
**Available:** Business hours, Monday-Friday

---

## 14. Documentation Status

**All Required Documents:** ✓ COMPLETE
- Privacy Policy: ✓ Published at /privacy
- Terms of Service: ✓ Published at /terms
- Account Deletion: ✓ Published at /delete-account
- Data Deletion Webhook: ✓ Implemented and tested
- DPO Information: ⏳ Pending (awaiting your details)

**Compliance:** ✓ GDPR, CCPA, Data Protection Act 2018

**Ready for:** Meta App Configuration

---

**Document Version:** 1.0  
**Created:** June 2026  
**Status:** Ready for Implementation  
**Approval:** Pending User DPO Details

---

END OF CONFIGURATION GUIDE
