# Role-Based Routing

Routes users to specialized agents based on database-stored roles with admin override and contextual disambiguation.

## Usage

```bash
./agentic examples/routing/config.yaml console
```

## Features

- User profile database (GORM/PostgreSQL)
- Role-based agent routing (admin, farmer, seller, anonymous)
- Admin user management tools
- LLM-based disambiguation for multi-role users

## Configuration

Update the PostgreSQL DSN in `config.yaml` before running.
