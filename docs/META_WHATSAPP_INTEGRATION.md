# Meta WhatsApp Integration Guide

This guide walks you through setting up a WhatsApp Business Account and connecting it to this platform via Meta's Cloud API.

## Prerequisites

- Facebook/Meta account (business or personal)
- Business email address
- WhatsApp business phone number (for production setup)

---

## Step 1: Login to Meta for Developers

1. Navigate to [Meta for Developers](https://developers.facebook.com/)
2. Click **Log In** (top right)
3. Use your Facebook/Meta account credentials
4. You'll land on **My Apps** dashboard

---

## Step 2: Create a Meta App

1. Click the **Create App** button
2. Choose app type: **Business**
3. Fill in the form:
   - **App Name**: Enter a descriptive name (e.g., *My WhatsApp SaaS*)
   - **Email**: Your business email
   - **Business Account**: Select or create one (if prompted)
4. Click **Create App**

Your Meta app is now created. You'll be redirected to the app dashboard.

---

## Step 3: Add WhatsApp Product

1. From the app dashboard, scroll down to see available products
2. Look for the **WhatsApp** product card
3. Click **WhatsApp → Set Up**

Meta automatically provisions:
- A **test WhatsApp Business Account (WABA)**
- A **test phone number** (for sandbox testing)
- A **temporary access token** (24-hour validity)

---

## Step 4: Open API Setup Page

1. From the left sidebar, navigate to **WhatsApp → API Setup**
2. You'll see three critical values on this page:

### Important Credentials

| Credential | Format | Example |
|-----------|--------|---------|
| **Temporary Access Token** | String starting with `EAAG` | `EAAGxxxxxxxxxxxx` |
| **Phone Number ID** | 15-digit numeric string | `123456789012345` |
| **WhatsApp Business Account ID (WABA ID)** | 15-digit numeric string | `987654321098765` |

**Note:** The temporary token expires in 24 hours. For production, you'll need to create a permanent **System User** token via Meta Business Manager.

---

## Step 5: Enter Credentials into Platform Form

On the **Channels** page (for your studio):

1. Click **Connect WhatsApp**
2. Fill in the four fields:

| Field | Value to Paste |
|-------|-----------------|
| **WhatsApp Business Account (WABA) ID** | WABA ID from API Setup page |
| **Phone Number ID** | Phone Number ID from API Setup page |
| **Display Phone Number** | The phone number shown in API Setup (e.g., `+1 555 645 5341`) |
| **Access Token** | Temporary Access Token (or permanent System User token for production) |

3. Click **Connect**

The channel will appear as **active** once connected successfully.

---

## Step 6: Add Real Phone Number (Production Setup)

Once you're ready to move to production:

1. Return to **WhatsApp → API Setup** on Meta
2. Click **Add Phone Number**
3. Enter the following details:
   - **Business Name**: Your registered business name
   - **Category**: Select your business category
   - **Phone Number**: Your actual WhatsApp business number (must be able to receive SMS/WhatsApp verification)
4. Complete the OTP verification sent to your phone number

After verification, Meta updates your phone number in the system. You can then:
- Generate a **permanent System User token** (valid indefinitely)
- Update the **Access Token** field in the platform with the permanent token
- Disconnect the old test channel and reconnect with production credentials

---

## Local Development with ngrok (Receiving Messages)

To receive messages from clients during local development, you need to expose your local server to the internet using ngrok and configure Meta webhooks.

### Step 1: Start your website/backend
Make sure your Next.js app is running locally (e.g., `localhost:3000`).

### Step 2: Start ngrok
Run ngrok for your app port (3000):
```bash
ngrok http 3000
```
After starting, you’ll get your public URL (e.g., `https://recreate-gout-blizzard.ngrok-free.dev`). **Keep ngrok running.**

### Step 3: Open Meta Developer Portal
Go to [Meta for Developers](https://developers.facebook.com/) and open your app.

### Step 4: Open WhatsApp settings
Inside Meta: **Your App → WhatsApp → Configuration**

### Step 5: Add webhook
In the **Webhook** section, click **Edit** and enter:
- **Callback URL**: `https://your-ngrok-url.ngrok-free.dev/api/v1/webhooks/meta/whatsapp`
- **Verify Token**: Use the same verify token your project/backend uses (e.g., `my_secret_token_123`).

Click **Verify and Save**.

### Step 6: Subscribe to message events
Still in Meta Configuration:
1. Click **Manage** (Webhook fields).
2. Enable: ✅ **messages**
3. Click **Done**.

> [!IMPORTANT]
> This is critical—without this, Meta won’t send client messages to your server.

### Step 7: Check app mode
Make sure your Meta app is set to ✅ **Live mode**.
> [!NOTE]
> If it is still in **Development mode**, only admins and testers can send messages.

### Step 8: Test with client
Ask your client (or use your own phone) to send a WhatsApp message to your connected business number.
Example: *Hi*

### Step 9: Check your inbox
Open your inbox page: `localhost:3000/admin/studios/.../inbox`

If everything is correct, the client’s message should appear there.

**The Communication Flow:**
`Client sends WhatsApp message` → `Meta` → `ngrok URL` → `Your app inbox`

---

## Testing the Connection


Once connected, you can:

1. **Open a conversation** in the Inbox by sending a test message to your WhatsApp number
2. **Reply** to incoming messages via the platform inbox
3. **Check outbound logs** in the API if any send failures occur

---

## Troubleshooting

### "channel not active: disconnected" Error

This occurs when:
- The access token has expired (temporary tokens last 24 hours)
- The token permissions were revoked
- The phone number was deactivated

**Solution**: Generate a new token and reconnect.

### "Invalid credentials" Error

- Verify all three credentials are copied correctly (no extra spaces)
- Ensure the token has **WhatsApp** permissions in Meta Business Manager
- For System User tokens, confirm they have the `whatsapp_business_messaging` permission

### No Messages Received

- Verify the phone number is correctly registered in Meta
- Check that incoming webhooks are configured (Meta will provide webhook settings in API Setup)
- Ensure the channel status is **active** (not `error` or `disconnected`)

---

## Security Notes

- **Never share your access token** publicly or in version control
- Tokens are encrypted at rest in the database
- For production, use **System User tokens** instead of personal access tokens
- Rotate tokens regularly for added security
- Monitor token expiry and set reminders for renewal

---

## Next Steps

1. **Configure Webhook (Advanced)**: For inbound messages, Meta requires a webhook URL. Contact support to enable webhook delivery.
2. **Scale to Multiple Numbers**: Create additional WhatsApp channels for different business lines or regions
3. **Set Up Automation**: Use message templates and automations (covered in separate automation guide)

---

## References

- [Meta WhatsApp Cloud API Docs](https://developers.facebook.com/docs/whatsapp/cloud-api)
- [Getting Access Tokens](https://developers.facebook.com/docs/whatsapp/business-platform/get-started/access-tokens)
- [API Setup Reference](https://developers.facebook.com/docs/whatsapp/business-platform/get-started#api-setup)
