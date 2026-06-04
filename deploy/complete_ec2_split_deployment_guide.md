# Production Runbook: Split EC2 App & Database Deployment Guide

This guide describes how to configure, secure, and deploy Project-X across two split AWS EC2 instances:
1. **App Server:** Runs Next.js, Go API, and Nginx.
2. **Database Server:** Runs PostgreSQL.

---

## 🏗️ Phase 1: AWS Infrastructure Provisioning

### 1. Launch EC2 Instances

#### App Server (`projectx-app-server`)
* **OS:** Ubuntu Server 24.04 LTS (or 22.04 LTS).
* **Instance Type:** `t3.small` or `t3.medium` (Minimum 2GB RAM is required for Next.js builds).
* **Storage:** `30 GiB` gp3 SSD (Avoid using the default 8 GiB; builds will fail due to lack of disk space).
* **Security Group Inbound Rules:**
  * **SSH (22):** Source: `My IP` (for administrator access).
  * **HTTP (80):** Source: `Anywhere (0.0.0.0/0)` (public web access).
  * **HTTPS (443):** Source: `Anywhere (0.0.0.0/0)` (public secure web access).

#### Database Server (`projectx-db`)
* **OS:** Ubuntu Server 24.04 LTS (or 22.04 LTS).
* **Instance Type:** `t3.micro` or `t3.small`.
* **Storage:** `30 GiB` gp3 SSD.
* **Security Group Inbound Rules:**
  * **SSH (22):** Source: `My IP`.
  * **PostgreSQL (5432):** Source: **Select Custom -> Paste App Server's PRIVATE IP** (e.g. `172.31.12.63/32`).
    > [!IMPORTANT]
    > Because both instances run in the same AWS VPC, traffic is routed through their **Private IPs**. The Database Server must accept inbound traffic from the App Server's **Private IP**, not the public Elastic IP.

---

### 2. Allocate & Associate Elastic IPs (Static IPs)
By default, EC2 public IPs change on instance reboot. You must attach permanent Elastic IPs:

1. Open **EC2 Console** -> **Elastic IPs** -> Click **Allocate Elastic IP address**.
2. Allocate **two separate Elastic IPs**.
3. Associate them to your instances:
   * **Elastic IP #1** -> Associate to **App Server** (e.g. public IP `23.21.75.112`).
   * **Elastic IP #2** -> Associate to **Database Server** (e.g. public IP `3.224.238.210`).

---

## 🗄️ Phase 2: Database Server Configuration

Log into your **Database Server** via SSH using its Elastic IP:
```bash
ssh -i your-key.pem ubuntu@[DB-Server-Public-IP]
```

### 1. Install PostgreSQL 18 & pgvector Extension
```bash
# Install core PostgreSQL
sudo apt update
sudo apt install postgresql postgresql-contrib -y

# Install pgvector extension
sudo apt install postgresql-18-pgvector -y

# NOTE: If the package is not found, compile pgvector from source:
# sudo apt install build-essential postgresql-server-dev-18 git -y
# git clone https://github.com/pgvector/pgvector.git
# cd pgvector && make && sudo make install
```

### 2. Configure Listen Interface
Open the PostgreSQL main configuration file (adjust the version folder `18` if a newer version is installed):
```bash
sudo nano /etc/postgresql/18/main/postgresql.conf
```
* Use `Ctrl + W` to search for `listen_addresses`.
* Uncomment the line (remove the `#` at the beginning) and set it to:
  ```text
  listen_addresses = '*'
  ```
* Save and exit (`Ctrl + O`, Enter, `Ctrl + X`).

### 3. Configure Client Authentication Rules
Open the Client Access configuration file:
```bash
sudo nano /etc/postgresql/18/main/pg_hba.conf
```
Scroll to the very bottom and append the App Server's **Private IP** to allow access:
```text
# Allow remote connections from App Server (using its Private IP)
host    all             all             [App-Server-Private-IP]/32            md5
```
*Save and exit (`Ctrl + O`, Enter, `Ctrl + X`).*

### 4. Create the Database, User Roles, & Install Vector Extension
Start the PostgreSQL database console:
```bash
sudo -u postgres psql
```
Execute these SQL queries:
```sql
CREATE DATABASE projectx;
CREATE USER projectx WITH PASSWORD 'projectx_dev';
GRANT ALL PRIVILEGES ON DATABASE projectx TO projectx;
ALTER DATABASE projectx OWNER TO projectx;
\q
```

Next, connect to the newly created `projectx` database as the `postgres` superuser to manually install the `vector` extension (this prevents the deployment migration tool from running into permission issues):
```bash
sudo -u postgres psql -d projectx
```
Execute this SQL query:
```sql
CREATE EXTENSION IF NOT EXISTS vector;
\q
```

### 5. Open local UFW Firewall and Restart Service
```bash
# Allow local port 5432 through the Ubuntu software firewall
sudo ufw allow 5432/tcp

# Restart PostgreSQL to apply all configurations
sudo systemctl restart postgresql

# Verify it is listening on 0.0.0.0:5432
sudo ss -lntp | grep 5432
```

---

## 🚀 Phase 3: App Server Configuration & Deployment

Log into your **App Server** via SSH using its Elastic IP:
```bash
ssh -i your-key.pem ubuntu@[App-Server-Public-IP]
```

### 1. Install Prerequisites
```bash
# Install Docker
sudo apt update
sudo apt install docker.io -y
sudo systemctl enable --now docker
sudo usermod -aG docker ubuntu && newgrp docker
```

### 2. Clone the Repository & Configure Env
```bash
# Clone
git clone git@github.com:Puneethgm/studiox.git
cd studiox

# Copy the environment file template
cp .env.example deploy/.env

# Open it to confirm connection variables
nano deploy/.env
```

Ensure your `deploy/.env` matches this database configuration (using the Database Server's **Private IP**):
```ini
POSTGRES_HOST=[DB-Server-Private-IP]   # e.g., 172.31.47.95
POSTGRES_PORT=5432
POSTGRES_USER=projectx
POSTGRES_PASSWORD=projectx_dev
POSTGRES_DB=projectx
POSTGRES_SSLMODE=disable

# Cookie Configuration (Leave blank for IP-based or production domain matching)
COOKIE_DOMAIN=

# Public URLs (Use App Server Public Elastic IP)
API_CORS_ORIGINS=http://[App-Server-Public-IP]
PUBLIC_FORM_BASE_URL=http://[App-Server-Public-IP]
```

### 3. Test Network Connection to Database
Ensure the App Server can reach the database server's port 5432:
```bash
nc -zv [DB-Server-Private-IP] 5432
```
* **Success message:** `Connection to [DB-Server-Private-IP] 5432 port [tcp/*] succeeded!`

### 4. Run Deploy Script
```bash
./deploy/deploy.sh
```

---

## 🔍 Troubleshooting Checklist

* **Hanging on "Applying migrations..."**
  * **Cause:** The database Security Group is blocking the connection.
  * **Resolution:** Double check that you added the App Server's **Private IP** (not the public IP) to the Database Server's AWS Security Group.
* **Error "connection refused"**
  * **Cause:** PostgreSQL is either offline or only listening to local connections.
  * **Resolution:** Ensure `listen_addresses = '*'` is set in `postgresql.conf` on the Database Server, and that you ran `sudo systemctl restart postgresql`.
* **Login Loop (Keeps redirecting back to the login page)**
  * **Cause:** The browser rejected the session cookie because `COOKIE_DOMAIN` does not match the IP address or host domain you are accessing.
  * **Resolution:** Set `COOKIE_DOMAIN=` to be completely blank in `deploy/.env` and run `./deploy.sh` to restart. This forces the browser to inherit the host IP/domain automatically.
* **"not a valid identifier" CRLF Line Ending Error**
  * **Cause:** Files copied or created in Windows carry `\r\n` line endings.
  * **Resolution:** The updated deployment scripts automatically strip `\r` carriage returns. If any other scripts fail, run `sed -i -e 's/\r$//' filename` on the server.
