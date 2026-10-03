# PayTrack AI Backend (Golang)

An automated backend service that synchronizes payment confirmation emails (receipts, bank debits, UPI transactions, credit card alerts, subscriptions), analyzes them with a **Local LLM (Ollama)** with rule-based heuristic fallback, updates a local SQLite database, runs on an automated **4x daily schedule**, and exposes REST APIs for a mobile expense tracker app.

---

## 🌟 Architecture & Features

- **Built-in Web Admin Dashboard**: Beautiful real-time monitoring interface at `http://localhost:8080/admin` for tracking users, connected inboxes, sync logs, money volume, and live AI transaction streams.
- **High-Throughput Parallel Worker Pool**: Multi-goroutine worker pool (`SYNC_WORKERS=5` by default) syncing hundreds of user mailboxes concurrently without blocking.
- **Dual-Engine Database (PostgreSQL & SQLite)**: Instant zero-config SQLite for single-node development, or battle-tested PostgreSQL with connection pooling for enterprise multi-tenant scale.
- **Automated 4x Daily Fetching**: Built-in cron scheduler (`0 0,6,12,18 * * *` at 12:00 AM, 6:00 AM, 12:00 PM, 6:00 PM) plus on-demand manual sync API (`POST /api/v1/sync`).
- **Multi-Tenant User Isolation**: Secure signup & login with bcrypt password hashing and 30-day JWT authentication. All expenses and accounts are strictly scoped per user.
- **Universal Email Support**: Connects securely via TLS over IMAP (Gmail, Outlook, Yahoo, iCloud, Fastmail, Proton, custom IMAP). App passwords encrypted with AES-256-GCM at rest.
- **Local LLM Analysis (Ollama)**: Zero-cloud-cost, privacy-first email understanding using local models (`llama3.2`, `mistral`, `qwen2.5`) with structured JSON schema output.
- **Smart Heuristic Fallback**: Includes instant regex/rule parsing for common bank alerts, UPI, credit cards, and receipts even if the local LLM is offline.
- **Deduplication Engine**: Automatically prevents duplicate records across message IDs, date intervals, amounts, and merchants.
- **Mobile-Ready REST API**: Rich endpoints with CORS enabled for mobile and web clients:
  - Monthly spending breakdowns and categories
  - Daily spending curves & trends
  - Filterable & paginated expense lists
  - Manual expense logging and user overrides
  - Live sync logs and status monitoring

---

## 🚀 Quick Start

### 1. Prerequisites
- **Go 1.23+**
- **Ollama** (optional but recommended for local AI analysis):
  ```bash
  brew install ollama
  ollama run llama3.2
  ```

### 2. Configure Environment (`.env`)
Copy `.env.example` to `.env` and fill in your email credentials:

```bash
cp .env.example .env
```

Edit `.env`:
```ini
PORT=8080
DB_PATH=./paytrack.db

# Email IMAP Credentials
# For Gmail: Use an App Password (Google Account -> Security -> 2-Step Verification -> App Passwords)
IMAP_SERVER=imap.gmail.com:993
IMAP_USERNAME=your_email@gmail.com
IMAP_PASSWORD=xxxx-xxxx-xxxx-xxxx
IMAP_FOLDER=INBOX
FETCH_BATCH_SIZE=50
LOOKBACK_DAYS=7

# Local LLM (Ollama)
OLLAMA_BASE_URL=http://localhost:11434
OLLAMA_MODEL=llama3.2
LLM_ENABLED=true

# Automated Schedule (Cron: 4 times a day)
CRON_SCHEDULE=0 0,6,12,18 * * *
CRON_ENABLED=true
```

### 3. Run with Docker Compose (Bare Metal / VPS Server)

To deploy on any Linux, macOS, or Windows server with Docker installed:

```bash
# 1. Clone repository & enter directory
git clone <repo_url>
cd paytrack-backend

# 2. Add your email credentials to .env
cp .env.example .env
nano .env   # Fill in IMAP_USERNAME and IMAP_PASSWORD

# 3. Start everything with one command
docker compose up -d
```

> **Note**: On first startup, the `ollama-init` service will automatically wait for Ollama and download the local `qwen2.5:3b` model for you. PayTrack backend will then start automatically!

Check live logs:
```bash
docker compose logs -f paytrack-backend
```

### 4. Or Run Locally with Go
```bash
# Run directly
go run ./cmd/server

# Or compile binary
go build -o bin/paytrack-server ./cmd/server
./bin/paytrack-server
```

---

## 📡 REST API Reference

All protected endpoints require the header:
```
Authorization: Bearer <your_jwt_token>
```

---

### 🔐 User Authentication (Public)

#### `POST /api/v1/auth/signup`
Register a new user account and get a JWT token.
```json
{
  "email": "user@example.com",
  "password": "mypassword123",
  "full_name": "Jane Doe"
}
```

#### `POST /api/v1/auth/login`
Authenticate with email and password to receive a JWT token.
```json
{
  "email": "user@example.com",
  "password": "mypassword123"
}
```

#### `GET /api/v1/auth/me` *(Protected)*
Get current user profile and list of connected mailboxes.

---

### 📬 Connected Email Inboxes *(Protected)*

#### `POST /api/v1/accounts`
Connect a new mailbox (Gmail, Outlook, Yahoo, iCloud, or custom IMAP). App passwords are encrypted at rest with **AES-256-GCM**.
```json
{
  "provider": "Gmail",
  "email_address": "jane.doe@gmail.com",
  "app_password": "xxxx-xxxx-xxxx-xxxx"
}
```

#### `GET /api/v1/accounts`
List all connected email inboxes for the authenticated user.

#### `DELETE /api/v1/accounts/:id`
Disconnect and remove a connected mailbox.

---

### 📊 Analytics & Mobile Dashboard *(Protected)*

#### `GET /api/v1/analytics/dashboard`
Returns high-level summary, month-over-month percentage change, top spending categories, and recent transactions for mobile home view.

**Response (200 OK):**
```json
{
  "current_month_expense": 482.50,
  "previous_month_expense": 520.10,
  "month_over_month_change_pct": -7.23,
  "total_transactions": 28,
  "top_categories": [
    { "category": "Food & Dining", "total": 185.00, "count": 12, "percentage": 38.34 },
    { "category": "Shopping", "total": 120.50, "count": 4, "percentage": 24.97 },
    { "category": "Transportation", "total": 92.00, "count": 6, "percentage": 19.07 }
  ],
  "last_synced_at": "2026-10-03T06:00:00Z"
}
```

#### `GET /api/v1/analytics/monthly?year=2026&month=10`
Returns full monthly breakdown with category percentages, top merchants, and day-by-day spending curve for charts.

---

### 💳 Expenses Management

#### `GET /api/v1/expenses`
Paginated, searchable, and filterable transaction list.

**Query Parameters:**
- `page` (default `1`)
- `limit` (default `20`)
- `category` (e.g. `Food & Dining`, `Transportation`, `Shopping`)
- `search` (searches merchant name, subject, or notes)
- `start_date` / `end_date` (e.g. `2026-10-01`)
- `type` (`DEBIT` or `CREDIT`)

#### `POST /api/v1/expenses`
Create a manual cash/offline expense.
```json
{
  "amount": 25.00,
  "currency": "USD",
  "merchant": "Local Farmer Market",
  "category": "Food & Dining",
  "payment_method": "Cash",
  "date": "2026-10-03T07:00:00Z",
  "notes": "Fresh vegetables"
}
```

#### `GET /api/v1/expenses/:id`
Get single expense details.

#### `PUT /api/v1/expenses/:id`
Update expense category, merchant, or notes.

#### `DELETE /api/v1/expenses/:id`
Delete an expense.

---

### 🔄 Email Synchronization

#### `POST /api/v1/sync?async=false`
Triggers an immediate on-demand synchronization.

#### `GET /api/v1/sync/status`
Checks if a sync is currently running and gets the latest sync timestamp.

#### `GET /api/v1/sync/logs`
Returns the recent 20 sync execution logs with number of emails processed, payments found, and saved count.

---

## 🧪 Testing

Run all unit and integration tests:
```bash
go test ./... -v
```
