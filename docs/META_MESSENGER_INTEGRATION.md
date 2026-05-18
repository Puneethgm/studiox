# Meta Facebook Messenger Integration Guide

This guide walks you through the step-by-step process of connecting a Facebook Page to your platform using the Meta Messenger API.

---

## Part 1 — Setup in Facebook

Messenger integration works with a **Facebook Page**, not a personal account.

### Step 1: Create a Facebook Page
If you don’t already have a business page:
1. Open [Facebook Pages](https://www.facebook.com/pages/create).
2. Create your business page.
   - **Page Name**: Your Business Name
   - **Category**: Business / Software / Services

### Step 2: Enable Messaging
1. Open your Facebook Page.
2. Go to **Page Settings → Messaging**.
3. Turn **ON**: ✅ **Allow people to message your page**.
4. Save your changes.

### Step 3: Get Page ID
1. Open your page.
2. Go to the **About** or **Page Info** section.
3. Scroll down to find and copy your **Page ID** (a long numeric string).
   > [!TIP]
   > You will need this ID later when connecting the channel in the platform.

---

## Part 2 — Setup in Meta Developer Portal

1. Open the [Meta for Developers](https://developers.facebook.com/) portal.
2. Log in with the same Facebook account used to create the Page.

### Step 4: Open your App
1. Open your existing Meta App.
2. If you don't have an app yet:
   - Click **Create App** → Choose **Business**.

### Step 5: Enable Messenger
1. In your Meta App dashboard, look for the use cases list.
2. Click **Engage with customers on Messenger from Meta**.
3. Complete the initial setup steps.

### Step 6: Connect your Facebook Page
1. Meta will ask you to choose a Facebook Page to associate with the app.
2. Select your newly created Page.
3. Click **Continue**.

### Step 7: Generate Page Access Token
1. Inside the Messenger setup section, find **Token Generation**.
2. Select your Facebook Page from the dropdown.
3. Click **Generate Token**.
4. **Copy the token** and store it securely.
   > [!IMPORTANT]
   > This token is required for the backend to send messages on behalf of your Page.

### Step 8: Start ngrok
To receive messages locally, you must expose your local server.
1. Run ngrok for your app port (e.g., 3000):
   ```bash
   ngrok http 3000
   ```
2. You will get a public URL like: `https://recreate-gout-blizzard.ngrok-free.dev`. **Keep this running.**

### Step 9: Configure Webhook
1. In the Meta Developer Portal, go to **Messenger → Settings → Webhooks**.
2. Click **Add Callback URL** and enter:
   - **Callback URL**: `https://recreate-gout-blizzard.ngrok-free.dev/api/v1/webhooks/meta/messenger`
   - **Verify Token**: Use the same verify token your project/backend uses.
3. Click **Verify and Save**.

> [!NOTE]
> The default project path is `/api/v1/webhooks/meta/messenger`. Ensure your backend is running before clicking Verify.

### Step 10: Subscribe to Events
1. After successful verification, click **Manage** in the Webhooks section.
2. Enable: ✅ **messages**
3. Optional: ✅ **messaging_postbacks**
4. Click **Save**.

---

## Testing the Integration

### 1. Send a Test Message
Ask a friend or use a separate Facebook account to visit your Page and send a message (e.g., "Hi!").

### 2. Check your Inbox
Open your platform inbox:
`localhost:3000/admin/studios/.../inbox`

If everything is configured correctly, the message should appear instantly.

### 3. Verify the Flow
`Client sends Messenger message` → `Meta` → `ngrok URL` → `Your App Inbox`
