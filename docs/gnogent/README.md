Here is the comprehensive, final **README.md** for **Gnogent**. It is structured to serve both as a high-level overview for stakeholders and a technical manual for developers, incorporating the advanced security, state management, and diagnostic features we've built.

---

# 🚀 Gnogent

**Gnogent** (Gno + Agent) is a secure, industrial-grade framework for building stateful AI agents. It uniquely combines the **deterministic logic** of GnoVM with **Google's Agent Development Kit (ADK)** and **PostgreSQL (GORM)** to solve the two biggest challenges in agentic AI: **persistent, verifiable memory** and **asymmetric cryptographic security.**

---

## 🧠 Why Gnogent?

Most AI agents lose their "train of thought" if the server restarts or sessions expire. Gnogent uses a **"Freeze/Thaw" architecture**:

* **Thaw:** When a user sends a message, Gnogent retrieves their "frozen" brain (a GnoVM memory snapshot) from Postgres.
* **Think:** The agent processes the message using deterministic Gno logic.
* **Freeze:** The updated brain is snapshotted back into a binary blob and stored securely.

---

## 🛠 Project Structure

```text
.
├── cmd/agent/main.go        # Entry point & ADK Orchestrator
├── gno/
│   └── agent.gno            # Deterministic brain logic (Short-term memory)
├── internal/
│   ├── auth/                # Asymmetric JWT (RS256) verification
│   ├── gnovm/               # GnoVM machine & store wrappers
│   ├── health/              # Pre-flight diagnostic services
│   └── storage/             # GORM models & Session Service (Freeze/Thaw)
├── certs/                   # RSA Public/Private keys
├── scripts/                 # Test token generation tools
├── config.yaml              # Global configuration
└── docker-compose.yml       # Postgres + pgvector setup

```

---

## 🔒 Security & Authentication

Gnogent is built for multi-channel deployment (WhatsApp, Web, TUI). It enforces **Asymmetric JWT Authentication**:

1. **Clients** sign requests with a **Private Key**.
2. **Gnogent** verifies requests with a **Public Key**.
3. **Claims:** Every request must include `userId`, `channel`, `domain`, and `ip`. These are injected into the GnoVM context to prevent session hijacking.

---

## 🚀 Getting Started

### 1. Environment Setup

Clone the repository and install dependencies:

```bash
go mod init gnogent
go mod tidy

```

### 2. Launch Infrastructure

Start the PostgreSQL instance with `pgvector` support:

```bash
docker-compose up -d

```

### 3. Pre-flight Diagnostics

Gnogent won't start if your database is down or keys are missing. Run the health check:

```bash
# This is integrated into the main startup, but can be run via Makefile
make check

```

### 4. Run the Agent

```bash
make run

```

---

## 🔧 Configuration (`config.yaml`)

```yaml
project: "Gnogent"
auth:
  public_key_path: "./certs/public.pem"
database:
  dsn: "host=localhost user=dev_user password=dev_password dbname=gnogent_db port=5432"
agent:
  model_name: "models/gemini-1.5-pro"
  instruction: "You are a stateful Gnogent. Use your GnoVM memory to track user goals."

```

---

## 🧪 Troubleshooting

| Issue | Potential Cause | Resolution |
| --- | --- | --- |
| **GnoVM Data Loss** | Variable scope | Ensure history variables are **global** in `agent.gno`. |
| **Auth Failure** | Key mismatch | Verify `certs/public.pem` matches the client's private key. |
| **Slow Response** | Heap Bloat | Implement pruning in GnoVM; offload to `pgvector` long-term storage. |
| **DB Errors** | `vector` extension | Run `CREATE EXTENSION IF NOT EXISTS vector;` in Postgres. |

---

## 📈 Monitoring

Gnogent exposes a health endpoint and logs the "Freeze/Thaw" latency. To inspect the current size of an agent's brain:

```sql
SELECT session_id, length(vm_state) FROM agent_sessions;

```


---

How to Deploy
Initialize Folders: Ensure your ./gno, ./certs, and config.yaml are populated.

Generate Keys: If you haven't yet, ensure certs/public.pem and certs/private.pem exist.

Launch:

 docker-compose up --build

**Final Project Checklist**

Security: RSA-2048 JWT authentication verified in Go.

Logic: Deterministic Personality and Mood engines in GnoVM.

Storage: Binary "Freeze/Thaw" snapshots in Postgres via GORM.

Health: Automated pre-flight checks and Docker health probes.

Interface: A TUI for development and a containerized API for production.

**cryptographically-secured, stateful AI framework** :

- The Gno Brain (Deterministic, persistent logic).

- The Go Body (Secure orchestration).

- The Postgres Vault (Binary state snapshots).

- The TUI Console (Visual state monitoring).

- The CI/CD Pipeline (Automated integrity).

---

**A Witticism to End On:**

"An AI with a memory is an assistant; an AI with a deterministic, cryptographically-secured memory is a Gnogent."


