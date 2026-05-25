# Google Sheets CRM & Pipeline Sync User Manual

This manual provides instructions for configuring, using, and maintaining the Google Sheets CRM Synchronization and Lead Lifecycle Automation pipeline in Project-X.

---

## 1. Google Sheets Integration Setup

To sync lead data from your CRM to a Google Spreadsheet, follow these steps:

### A. Share Your Spreadsheet
The backend writes data using a Google Service Account. You must share your target Google Spreadsheet with the service account email:

*   **Service Account Email**: `studiox-sheets-writer@heroic-artifact-434914-d4.iam.gserviceaccount.com`
*   **Permission Level**: **Editor**

### B. Upload Credentials (SuperAdmin Settings)
1. Log in to Project-X as a **SuperAdmin**.
2. Click on the **Settings** button in the lower left/sidebar of the dashboard.
3. In the Credentials Manager section, upload your Google Service Account credentials JSON file.
4. The system will automatically save the credentials and reload the sheets client without restarting the server.

### C. Configure Studio Settings
For each Studio that needs synchronization:
1. Navigate to **Studio Settings**.
2. Enter the **Spreadsheet ID** (the long string in the sheet's browser URL between `/d/` and `/edit`).
3. Enter the **Tab Name** (default is `Leads`).
4. Toggle the integration status to **Active**.

---

## 2. Lead Lifecycle Automation (Pipeline Sync)

The system automatically manages status changes and updates synchronization flags in both the database and the Google Sheet:

### Automatic Toggles
*   **Trial Purchased**: 
    *   Toggles to **`Yes`** in Google Sheets (and `true` in the DB) automatically when a lead successfully books a class slot via the public scheduling form or is manually updated to **`Trial Booked`** status.
*   **Member Sold**: 
    *   Toggles to **`Yes`** in Google Sheets (and `true` in the DB) automatically when an administrator updates the lead's status to **`Member`** in the CRM pipeline or lead editor.

### Live Pipeline Status Column
A **`Status`** column has been added as the 21st column (Column `U`) in the spreadsheet. It reflects real-time lead stages:
*   `new`
*   `contacted`
*   `trial_booked`
*   `member`
*   `dropped`

---

## 3. Spreadsheet Column Layout

The synchronization engine writes **21 columns** in the following order. If your sheet contains fewer columns, the header row will automatically expand to include the new columns:

| Column | Header | Description |
| :--- | :--- | :--- |
| **A** | Lead ID | Unique identifier of the lead |
| **B** | First Name | Lead's first name |
| **C** | Last Name | Lead's last name |
| **D** | Email Address | Lead's email address |
| **E** | Phone Number | Contact number |
| **F** | Date Of Lead | Creation timestamp |
| **G** | Lead Source | Source (e.g., `public_form`, `manual`) |
| **H** | Offer? | Applied campaign offer |
| **I** | Assigned to | Team member assigned to the lead |
| **J** | # of Attempts | Follow-up call/message attempts |
| **K** | Last Followed Up? | Date/time of last contact attempt |
| **L** | Contact Made? | `Yes` / `No` status |
| **M** | HOT LEAD? | `Yes` / `No` status |
| **N** | Trial Purchased? | `Yes` / `No` (Auto-updated on slot booking) |
| **O** | Notes on Lead | Internal staff notes |
| **P** | Trial Attended? | `Yes` / `No` status |
| **Q** | Member Sold? | `Yes` / `No` (Auto-updated on Member status) |
| **R** | Monthly Fee | Monthly membership pricing |
| **S** | Predicted Revenue Won | Based on `Monthly Fee` x 9 Month average lifetime |
| **T** | Further Notes on Contact | Additional follow-up notes |
| **U** | Status | Current pipeline status (e.g. `trial_booked`, `member`) |

---

## 4. Reliable Outbox Event System

Sync tasks are managed via a transactional outbox pattern to guarantee data delivery:
1. Any lead modification writes an event to the `outbox` database table under the same transaction.
2. The background `sheets_worker` polls the outbox every **5 seconds**.
3. If the Google API rate limit is reached or a network error occurs, the worker backs off exponentially (doubling retry delay up to 30 minutes) and retries.
4. Successful writes are marked as `sent` in the database for auditing.

---

## 5. Production Maintenance & Deployment (EC2)

### Memory-Safe Build Commands
If you are deploying on a resource-constrained host (e.g., AWS EC2 `t2.micro` or `t3.micro` with 1GB RAM), running parallel builds can crash the server. Always build sequentially:

```bash
cd ~/studiox/deploy

# 1. Build the API container (low memory footprint)
sudo docker compose build api

# 2. Build the Web container (high memory footprint Next.js compilation)
sudo docker compose build web

# 3. Apply goose database migrations
sudo docker compose --profile tools run --rm migrate

# 4. Start the updated services
sudo docker compose up -d
```

### Verification & Troubleshooting
To check the status of all services and investigate issues on the EC2 instance, run:

```bash
# Check if all containers are running
sudo docker compose ps

# Tail all logs or specific service logs
sudo docker compose logs -f
sudo docker compose logs -f nginx
sudo docker compose logs -f api
```
