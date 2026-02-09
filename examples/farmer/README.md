# Organic Farming Advisor

An organic farming advisory agent for Indian agriculture with product recommendations and persistent memory.

## Usage

```bash
./agentic examples/farmer/config.yaml console
```

## Features

- Expert organic farming advice for Indian agriculture
- Product recommendations (seeds, bio-fertilizers, bio-pesticides)
- Persistent conversation memory via PostgreSQL
- Farmer identification via Google social login email

## Configuration

Update the PostgreSQL DSN in `config.yaml` before running:

```yaml
session:
  dsn: postgres://user:pass@localhost:5432/farmer?sslmode=disable
memory:
  dsn: postgres://user:pass@localhost:5432/farmer?sslmode=disable
```
