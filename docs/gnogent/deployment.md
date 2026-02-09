This **`README_DEPLOY.md`** provides the operational manual for moving Gnogent from a local TUI experiment to a scalable, multi-channel production environment.

---

# 🚀 Gnogent Deployment & Scaling Guide

This guide explains how to scale Gnogent for high availability, multi-user isolation, and production-grade reliability.

## 🏗️ Multi-User Scaling Architecture

Because Gnogent uses a **"Freeze/Thaw"** cycle, the Go backend is effectively **stateless**. This allows you to run multiple instances of the Gnogent agent behind a Load Balancer.

### 1. Horizontal Scaling

* **Stateless Go Containers:** You can spin up `N` instances of the `agent` service. Since the "brain" (GnoVM state) lives in the database, any container can handle any user's request.
* **Sticky Sessions:** While not strictly required, using sticky sessions can reduce latency by keeping the GnoVM heap "warm" in a specific container's memory for a few minutes.

### 2. Multi-Channel Deployment

Gnogent’s use of **Asymmetric JWTs** allows a single backend to serve multiple frontends simultaneously:

* **Web UI:** Signs JWTs via a secure backend API.
* **WhatsApp/Telegram:** A gateway service signs JWTs on behalf of the user's phone number.
* **TUI:** Developers sign JWTs locally using `private.pem`.

---

## 🔒 Production Security Hardening

### 1. Secret Management

* **Do not** bake `private.pem` into the Docker image.
* **Use** a secret manager (AWS Secrets Manager, HashiCorp Vault) to inject the keys into the container at runtime.
* **File Permissions:** Ensure certificates are mounted as read-only (`ro`) in `docker-compose.yml`.

### 2. Database Protection

* **SSL/TLS:** Enable `sslmode=verify-full` in your DSN string to encrypt the traffic between the Go agent and Postgres.
* **Connection Pooling:** In GORM, tune the connection pool to prevent exhausting Postgres during high traffic spikes:
```go
sqlDB.SetMaxIdleConns(10)
sqlDB.SetMaxOpenConns(100)

```



---

## 📈 Monitoring & Observability

### 1. State Size Monitoring

Monitor the growth of the `vm_state` column. If snapshots consistently exceed 5MB, it's time to trigger an aggressive pruning cycle in `agent.gno`.

### 2. Latency Metrics

Track the time taken for:

* **Thaw Time:** Database `SELECT` + `VM.Restore()`.
* **Inference Time:** LLM response generation.
* **Freeze Time:** `VM.Export()` + Database `UPDATE`.

---

## 🛠️ Deployment Checklist

* [ ] **Database:** Postgres `pgvector` extension is active.
* [ ] **Entropy:** Go "Body" is correctly passing `time.Now().Unix()` to Gno "Brain."
* [ ] **Auth:** `public.pem` is correctly rotated and matches the client's `private.pem`.
* [ ] **Migrations:** `db.AutoMigrate` has created the `agent_sessions` table.
* [ ] **Gno Logic:** All stateful variables in `agent.gno` are package-level globals.

---
