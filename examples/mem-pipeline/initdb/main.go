// Command initdb creates the agent_memories table and pgvector indexes
// required by the mem-pipeline example.
//
// Usage:
//
//	go run examples/mem-pipeline/initdb/main.go [dsn]
//
// If no DSN is provided it defaults to:
//
//	postgres://user:pass@localhost:5432/mem_pipeline?sslmode=disable
package main

import (
	"fmt"
	"log"
	"os"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

const defaultDSN = "postgres://user:pass@localhost:5432/mem_pipeline?sslmode=disable"

const ddl = `
CREATE EXTENSION IF NOT EXISTS vector;

CREATE TABLE IF NOT EXISTS agent_memories (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id TEXT NOT NULL,
    session_id TEXT,
    memory_type VARCHAR(20) CHECK (memory_type IN ('episodic', 'semantic', 'procedural')),
    content TEXT NOT NULL,
    content_embedding vector(768),
    metadata JSONB DEFAULT '{}',
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_mem_lookup
    ON agent_memories (user_id, memory_type);

CREATE INDEX IF NOT EXISTS idx_vector_search
    ON agent_memories USING hnsw (content_embedding vector_cosine_ops);
`

func main() {
	dsn := defaultDSN
	if len(os.Args) > 1 {
		dsn = os.Args[1]
	}

	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
	if err != nil {
		log.Fatalf("failed to connect: %v", err)
	}

	if err := db.Exec(ddl).Error; err != nil {
		log.Fatalf("failed to execute schema: %v", err)
	}

	fmt.Println("✅ agent_memories table and indexes created successfully")
}
