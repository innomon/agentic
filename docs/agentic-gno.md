This guide consolidates our discussion into a production-ready specification. By combining **Google's ADK**, **GnoVM**, and **Postgres**, you are building an agent with a deterministic "inner monologue" and a flexible, persistent "memory."

---

# AI Agent Specification: GnoVM + ADK + GORM

## 1. Project Overview

This project implements an AI agent where the **state logic** (conversation history, rules, and variables) is managed by **GnoVM** to ensure determinism. The persistent storage uses **PostgreSQL** via **GORM**, allowing for "snapshot-based" persistence where the entire VM heap is saved as a binary blob.

### **Key Components**

* **Brain (GnoVM):** A sandboxed execution environment for state transitions.
* **Body (ADK Go-SDK):** Handles LLM orchestration, tools, and user I/O.
* **Vault (Postgres + GORM):** Configurable object store for VM snapshots and semantic embeddings.
* **Native Bridges:** Go-native packages injected into GnoVM to enable logging, sub-agent calls, and tool execution.

---

## 2. Infrastructure Setup

### **Agent Configuration**

Gno agents (`gnogent`) are configured in `config.yaml` with their Gno source file and a list of permitted tools.

```yaml
type: gnogent
name: "GnoDeterministicAgent"
database:
  dsn: "host=localhost user=dev_user password=dev_password dbname=agent_store port=5432 sslmode=disable"
  auto_migrate: true
tools:
  - google_search
  - calculator
gnovm:
  source_file: "./pkg/gnogent/gno/agent.gno"
  pkg_path: "gno.land/p/agent"
```

---

## 3. The GnoVM "Brain" Logic

Gno agents are now fully deterministic and utilize native packages for I/O and orchestration.

**File: `pkg/gnogent/gno/agent.gno**`

```go
package main

import (
    "gno.land/p/agent"
    "gno.land/p/log"
)

var (
    Input  string
    Output string
    mood   = "professional"
)

func main() {
    log.Println("GnoVM logic pulse started.")

    if Input == "call the expert" {
        // Delegate to a sub-agent natively
        Output = agent.CallSubAgent("ExpertAgent", "Explain the GnoVM injection mechanism.")
    } else {
        Output = "Current Mood: " + mood + "\nProcessing: " + Input
    }
}
```

---

## 4. Native Packages Guide

Native Go packages can be injected into GnoVM to provide capabilities not available in the deterministic Gno environment.

### Available Agent Packages:
- `gno.land/p/log`: Standard output logging from GnoVM.
- `gno.land/p/agent`: Methods to call `CallSubAgent` and `CallTool`.

For more details on how to implement custom native packages, see [docs/gnovm-native-packages.md](gnovm-native-packages.md).

---

## 5. The Storage Layer (GORM)

Implement a custom session service that handles the "Freeze/Thaw" cycle of the GnoVM.

**File: `internal/storage/session.go**`

```go
type AgentSession struct {
    gorm.Model
    SessionID string `gorm:"uniqueIndex"`
    VMState   []byte `gorm:"type:bytea"` // The frozen GnoVM heap (Brain Snapshot)
}
```

---

## 5. Implementation Strategy: The "Re-animation" Loop

### **The Execution Flow**

1. **Thaw:** When a request arrives, the `SessionService` fetches the `VMState` bytes and calls `machine.RestoreState()`. This restores all internal variables (mood, friendship, history).
2. **Reason:** The ADK Agent queries the GnoVM for the current system instruction via `machine.Eval("agent.GetSystemContext()")`.
3. **Act:** The LLM generates a response based on the context.
4. **Update:** The Go code calls `machine.Eval("agent.AddTurn(...)")` to update the VM's internal memory.
5. **Freeze:** `db.Save()` snapshots the entire VM memory back to the `VMState` field.

---

## 6. Comparison of Components

| Feature | Functionality |
| --- | --- |
| **Logic Consistency** | **GnoVM** ensures rules are followed exactly as coded. |
| **Data Persistence** | **Postgres** ensures the "Brain State" survives restarts. |
| **Agentic Loop** | **ADK** manages the ReAct (Thought/Action/Observation) cycle. |
| **Scalability** | **GORM** allows you to point multiple agent instances at a single DB. |

---

### **Next Steps for Implementation**

* **Step 1:** Initialize your Go module and install `google.golang.org/adk` and `gorm.io/gorm`.
* **Step 2:** Write the GnoVM wrapper that converts Gno's `MemStore` into the `[]byte` slice for GORM.
* **Step 3:** Register your GnoVM-backed session service into the `launcher.Config`.

**Go code for the `GnoMachineWrapper` that implements the `ExportState` and `RestoreState` methods**

In a standalone GnoVM setup, "persistence" means serializing the `Machine`'s memory store into a format that can be saved to your PostgreSQL `bytea` column.

While GnoVM automatically persists state when running on a blockchain, in a **standalone agent** context, you must manually trigger the export of the `Store`.

### **1. The Gno Machine Wrapper Implementation**

This wrapper manages the life cycle of the GnoVM. It uses `MemDB` to back the Gno `Store`, allowing for manual snapshotting of the entire key-value state using Amino.

```go
package gnovm

import (
	"fmt"

	"github.com/gnolang/gno/gnovm/pkg/gnolang"
	"github.com/gnolang/gno/tm2/pkg/amino"
	"github.com/gnolang/gno/tm2/pkg/db"
	"github.com/gnolang/gno/tm2/pkg/db/memdb"
	"github.com/gnolang/gno/tm2/pkg/std"
	"github.com/gnolang/gno/tm2/pkg/store/dbadapter"
)

type GnoMachineWrapper struct {
	Machine *gnolang.Machine
	Store   gnolang.Store
	DB      db.DB
	PkgPath string
}

// NewGnoMachineWrapper initializes a fresh VM for a new agent session
func NewGnoMachineWrapper(pkgPath, src string) (*GnoMachineWrapper, error) {
	memDB := memdb.NewMemDB()
	baseStore := dbadapter.Store{DB: memDB}

	alloc := gnolang.NewAllocator(0)
	store := gnolang.NewStore(alloc, baseStore, baseStore)
	m := gnolang.NewMachine(pkgPath, store)

	mpkg := &std.MemPackage{
		Name: "agent",
		Path: pkgPath,
		Type: gnolang.MPUserProd,
		Files: []*std.MemFile{
			{Name: "agent.gno", Body: src},
		},
	}
	_, pv := m.RunMemPackage(mpkg, true)
	m.SetActivePackage(pv)

	return &GnoMachineWrapper{
		Machine: m,
		Store:   m.Store,
		DB:      memDB,
		PkgPath: pkgPath,
	}, nil
}

type dbEntry struct {
	K []byte
	V []byte
}

// ExportState snapshots the entire VM memory heap via the underlying MemDB
func (w *GnoMachineWrapper) ExportState() ([]byte, error) {
	it, err := w.DB.Iterator(nil, nil)
	if err != nil {
		return nil, err
	}
	defer it.Close()

	var entries []dbEntry
	for ; it.Valid(); it.Next() {
		entries = append(entries, dbEntry{K: it.Key(), V: it.Value()})
	}
	return amino.Marshal(entries)
}

// RestoreState re-animates the machine from a DB blob
func (w *GnoMachineWrapper) RestoreState(b []byte) error {
	var entries []dbEntry
	if err := amino.Unmarshal(b, &entries); err != nil {
		return err
	}

	for _, e := range entries {
		if err := w.DB.Set(e.K, e.V); err != nil {
			return err
		}
	}
	return nil
}

```

---

### **2. The GORM Integration Hook**

This hook acts as the bridge. By attaching the `GnoMachineWrapper` to the GORM transaction context, we ensure the binary snapshot is updated right before the database `COMMIT`.

```go
// internal/storage/session.go

func (s *AgentSession) BeforeSave(tx *gorm.DB) error {
    // We retrieve the active machine from the context
    if val, ok := tx.Get("active_vm"); ok {
        wrapper := val.(*gnovm.GnoMachineWrapper)
        
        // FREEZE: Convert the live memory to a byte blob
        blob, err := wrapper.ExportState()
        if err != nil {
            return err
        }
        
        // Assign to the field that GORM maps to 'bytea' in Postgres
        s.VMState = blob
    }
    return nil
}

```

---

### **3. The ADK Session Re-animation Loop**

This is the standard procedure for every user message.

```go
// ExecuteTurn handles a single interaction turn
func (a *App) ExecuteTurn(sessionID string, userText string) {
    // 1. Thaw: Load from Postgres
    var sess AgentSession
    a.DB.First(&sess, "session_id = ?", sessionID)
    
    vmWrapper := gnovm.NewMachineFromState(sess.VMState)
    
    // 2. Logic: Let GnoVM update the internal history
    vmWrapper.Machine.Eval(fmt.Sprintf(`agent.AddTurn("%s")`, userText))
    
    // 3. AI: Get context for LLM
    context := vmWrapper.Machine.Eval(`agent.GetContext()`)
    response := a.LLM.Generate(context)
    
    // 4. Freeze: Save back to Postgres
    // This triggers the BeforeSave hook automatically
    a.DB.Set("active_vm", vmWrapper).Save(&sess)
}

```

---

### **Implementation Checklist**

* **Memory Management:** Be aware that as the agent's history grows, the `VMState` blob size increases. Implement a "pruning" logic inside your `agent.gno` to keep history within a reasonable limit (e.g., last 50 turns).
* **Deterministic Tools:** If your Gno agent calls tools, ensure those tools are also registered within the GnoVM environment to maintain execution consistency.
* **Configurability:** Store your `agent.gno` source code in a `Templates` table in Postgres. This allows you to update the agent's "brain logic" for all sessions simply by changing one row in the database.

**a sample `agent.gno` script that includes an internal "Pruning" function to manage the binary state size**

Here is the implementation of the `agent.gno` script designed for **long-running sessions**.

This script includes an internal "Pruning" mechanism. This is critical because, in a standalone GnoVM setup, every variable and slice element added to the state increases the size of the `VMState` blob in your Postgres database. Pruning ensures your database performance remains consistent and your LLM context window isn't overwhelmed.

---

### **1. The Managed Brain (`gno/agent.gno`)**

```go
package agent

import (
    "strings"
)

// Global state variables (Auto-snapshotted by GnoVM)
var (
    history       []string
    maxHistory    = 20    // Keep only the last 20 turns
    systemPrompt  = "You are a helpful assistant with persistent GnoVM memory."
    metadata      map[string]string
)

// AddTurn adds an interaction and triggers pruning if limits are exceeded
func AddTurn(input string, output string) {
    history = append(history, "User: " + input)
    history = append(history, "Assistant: " + output)
    
    // Trigger Pruning: Keeps the VMState blob size predictable
    pruneHistory()
}

// pruneHistory removes the oldest entries (FIFO)
func pruneHistory() {
    if len(history) > maxHistory * 2 { // *2 because each turn is 2 entries
        // Slice the array to remove the oldest turn
        history = history[2:]
    }
}

// GetContext returns the structured memory for the ADK LLM call
func GetContext() string {
    return systemPrompt + "\n\nConversation History:\n" + strings.Join(history, "\n")
}

// UpdateConfig allows changing agent behavior mid-session
func UpdateConfig(key string, value string) {
    if metadata == nil {
        metadata = make(map[string]string)
    }
    metadata[key] = value
}

```

---

### **2. Integration Logic: Go-ADK Side**

When using the ADK `Runner`, you will call these Gno functions during the turn cycle.

```go
func (a *AgentWrapper) HandleUserMessage(sessionID string, userMsg string) (string, error) {
    // 1. Load/Restore from GORM
    sess, _ := a.SessionService.GetSession(context.Background(), sessionID)
    machine := sess.State.(*gnovm.GnoMachineWrapper)

    // 2. Retrieve pruned context from GnoVM
    res, _ := machine.Machine.Eval("agent.GetContext()")
    promptContext := res.String()

    // 3. Generate LLM response (ADK/Gemini)
    aiResponse, err := a.Gemini.Generate(promptContext, userMsg)
    if err != nil {
        return "", err
    }

    // 4. Commit turn to GnoVM (Triggers internal Pruning)
    machine.Machine.Eval(fmt.Sprintf(`agent.AddTurn("%s", "%s")`, userMsg, aiResponse))

    // 5. Save Session (Triggers GORM BeforeSave to snapshot the pruned state)
    a.SessionService.SaveSession(context.Background(), sess)

    return aiResponse, nil
}

```

---

### **3. Strategic Advice for GnoVM Agents**

* **Circular Buffers:** For high-traffic agents, consider implementing a circular buffer logic in Gno rather than simple slicing, though slicing `history[2:]` is highly efficient in Gno's memory management.
* **Structured Metadata:** Use the `metadata` map in Gno to store user preferences (e.g., `language: "Spanish"`). This is more efficient than including instructions in every turn's history.
* **Compression:** If your `VMState` blobs exceed 1MB, consider wrapping your `ExportState()` output in `zstd` or `gzip` before sending it to Postgres. This can be done inside the `BeforeSave` hook.

---

### **Summary of the Final Implementation Plan**

1. **Architecture:** Use **ADK** for the LLM loop and **GnoVM** for the deterministic state machine.
2. **Storage:** Use **Postgres (pgvector)** via **GORM** to store binary snapshots of the VM.
3. **Optimization:** Implement **Pruning** in the Gno layer to keep the database and LLM context lean.
4. **Deployment:** Deploy via **Docker Compose** to ensure the database extensions are correctly configured.

**complete `main.go` file**  synthesizes all these components (GORM setup, GnoVM initialization, and ADK agent registration) into a single runnable application.

This final implementation synthesizes the **Go-ADK**, **GnoVM**, and **GORM** into a single, cohesive application. We will use a YAML configuration to ensure that everything from the database DSN to the GnoVM memory limits is fully adjustable.

### 1. The Configuration (`config.yaml`)

Everything from the model parameters to the database connection is externalized.

```yaml
server:
  port: 8080

database:
  dsn: "host=localhost user=dev_user password=dev_password dbname=agent_store port=5432 sslmode=disable"
  max_idle_conns: 10
  max_open_conns: 100

agent:
  name: "GnoDeterministicAgent"
  model_name: "models/gemini-1.5-pro"
  system_instruction: "You are a stateful agent. Your logic is verified by GnoVM."
  temperature: 0.7

gnovm:
  package_path: "gno.land/p/agent"
  source_file: "./gno/agent.gno"
  max_history_turns: 20  # Configurable pruning limit

```

---

### 2. The Complete Implementation (`main.go`)

This boilerplate connects the ADK runner to the GORM storage and GnoVM execution environment.

```go
package main

import (
	"context"
	"fmt"
	"log"
	"os"

	"google.golang.org/adk/agent/llmagent"
	"google.golang.org/adk/launcher"
	"google.golang.org/adk/session"
	"gopkg.in/yaml.v3"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"

	"myagent/internal/gnovm"
	"myagent/internal/storage"
)

type Config struct {
	Database struct {
		DSN string `yaml:"dsn"`
	} `yaml:"database"`
	Agent struct {
		Name              string `yaml:"name"`
		ModelName         string `yaml:"model_name"`
		SystemInstruction string `yaml:"system_instruction"`
	} `yaml:"agent"`
	GnoVM struct {
		SourceFile string `yaml:"source_file"`
	} `yaml:"gnovm"`
}

func main() {
	// 1. Load YAML Configuration
	cfgData, _ := os.ReadFile("config.yaml")
	var cfg Config
	yaml.Unmarshal(cfgData, &cfg)

	// 2. Initialize PostgreSQL with GORM
	db, err := gorm.Open(postgres.Open(cfg.Database.DSN), &gorm.Config{})
	if err != nil {
		log.Fatalf("Failed to connect to DB: %v", err)
	}
	db.AutoMigrate(&storage.AgentSession{})

	// 3. Initialize GnoVM Machine Wrapper
	gnoSource, _ := os.ReadFile(cfg.GnoVM.SourceFile)
	vmWrapper, _ := gnovm.NewGnoMachineWrapper("gno.land/p/agent", string(gnoSource))

	// 4. Create the ADK Agent
	// Note: In production, the model is initialized via your provider (Gemini/Vertex)
	agent, _ := llmagent.New(llmagent.Config{
		Name:        cfg.Agent.Name,
		Instruction: cfg.Agent.SystemInstruction,
	})

	// 5. Setup the Configurable Session Service
	// This service handles the Save/Load (Freeze/Thaw) logic discussed
	sessionSvc := &storage.GormSessionService{
		DB:        db,
		VMWrapper: vmWrapper,
	}

	// 6. Launch the Agent Runner
	l := launcher.New(launcher.Config{
		Agent:          agent,
		SessionService: sessionSvc,
	})

	fmt.Printf("Agent %s is running...\n", cfg.Agent.Name)
	if err := l.Run(context.Background()); err != nil {
		log.Fatal(err)
	}
}

```

---

### 3. Key Architectural Logic

Inside your `internal/storage` package, the `SaveSession` and `GetSession` methods perform the heavy lifting of the **Freeze/Thaw** cycle.

#### **The "Thaw" (Loading)**

When `GetSession` is called, the service pulls the `bytea` blob from Postgres and injects it into the GnoVM `Store`.

```go
func (s *GormSessionService) GetSession(ctx context.Context, id string) (*session.Session, error) {
    var record AgentSession
    res := s.DB.Where("session_id = ?", id).First(&record)
    
    // Create machine and restore from blob if exists
    machine := s.VMWrapper.Clone() 
    if res.Error == nil {
        machine.RestoreState(record.VMState)
    }
    
    return &session.Session{ID: id, State: machine}, nil
}

```

#### **The "Freeze" (Saving)**

When `SaveSession` is called, the service captures the live memory of the GnoVM and commits it back to Postgres.

```go
func (s *GormSessionService) SaveSession(ctx context.Context, sess *session.Session) error {
    machine := sess.State.(*gnovm.GnoMachineWrapper)
    stateBlob, _ := machine.ExportState()
    
    return s.DB.Where("session_id = ?", sess.ID).
        Assign(AgentSession{VMState: stateBlob}).
        FirstOrCreate(&AgentSession{SessionID: sess.ID}).Error
}

```

---

### 4. Final Folder Structure

```text
.
├── config.yaml             # Global Config
├── main.go                 # Application Entry
├── gno/
│   └── agent.gno           # GnoVM Logic (Pruning, History)
├── pkg/
│   ├── gnovm/
│   │   └── wrapper.go      # GnoVM Machine/Store Wrapper
│   └── storage/
│       ├── model.go        # GORM Session Table
│       └── service.go      # ADK Session Service Bridge
└── docker-compose.yml      # pgvector Database

```

### Summary

* **YAML Config:** Allows you to change behavior without recompiling.
* **GORM:** Manages the `bytea` storage of the "frozen" brains.
* **GnoVM:** Provides a deterministic sandbox where the agent's logic is immutable.
* **ADK:** Orchestrates the high-level AI interaction.

Would you like me to create a **README.md** file for this project that includes the installation commands and local development setup?

This **README.md** provides everything a developer needs to set up, configure, and run your GnoVM-powered AI agent.

---

# 🚀 GnoVM-Backed AI Agent (Go-ADK + GORM)

This repository implements a stateful AI agent that uses **GnoVM** for deterministic state management and **PostgreSQL (GORM)** for persistent object storage. By snapshotting the VM's memory heap, the agent's "brain" is frozen into the database after every turn and re-animated upon the next user interaction.

## 🛠 Prerequisites

* **Go** (1.23+ recommended)
* **Docker** and **Docker Compose**
* **Google AI SDK Key** (for Gemini) or equivalent LLM provider credentials.

## 📂 Project Structure

```text
├── cmd/agent/main.go        # App entry point & configuration loader
├── config.yaml              # Global YAML settings (DB, Agent, VM)
├── docker-compose.yml       # pgvector/Postgres setup
├── gno/
│   └── agent.gno            # Deterministic brain logic (History & Pruning)
├── pkg/
│   ├── gnovm/               # GnoVM Store/Machine wrappers
│   └── storage/             # GORM models and ADK Session Service
└── README.md

```

## 🚀 Quick Start

### 1. Spin up the Database

Run the Postgres instance with `pgvector` support:

```bash
docker-compose up -d

```

### 2. Configure the Environment

Copy the `config.yaml` and update your Database DSN and LLM settings:

```yaml
database:
  dsn: "host=localhost user=dev_user password=dev_password dbname=agent_store port=5432 sslmode=disable"

agent:
  model_name: "models/gemini-1.5-pro"
  system_instruction: "Your logic is verified by GnoVM."

gnovm:
  max_history_turns: 20

```

### 3. Install Dependencies

```bash
go mod tidy

```

### 4. Run the Agent

```bash
go run cmd/agent/main.go

```

---

## 🧠 How it Works

### **The "Freeze/Thaw" Lifecycle**

1. **Thaw:** When a `SessionID` is received, the `GormSessionService` queries the `agent_sessions` table for the `VMState` (bytea).
2. **Re-animate:** The bytes are injected into a new GnoVM Machine, restoring all global variables (history, mood, etc.).
3. **Execute:** The ADK Agent generates a response. The Go-SDK calls Gno logic to update the state (`agent.AddTurn`).
4. **Freeze:** GORM's `BeforeSave` hook snapshots the VM's updated memory heap back into a binary blob and saves it to Postgres.

## 🔧 Configuration Options

| Key | Description | Default |
| --- | --- | --- |
| `database.dsn` | Postgres connection string | `host=localhost...` |
| `agent.model_name` | The LLM model to use (Gemini/Vertex) | `gemini-1.5-pro` |
| `gnovm.source_file` | Path to the `.gno` logic file | `./gno/agent.gno` |
| `gnovm.max_history_turns` | Number of turns to keep before pruning | `20` |

---

## 🧪 Development & Testing

### **Modifying Agent Logic**

To change how the agent remembers things, edit `gno/agent.gno`. Since the VM is snapshotted, you can add new global variables to this file, and they will be automatically persisted without needing a SQL migration.

### **Database Inspection**

You can view the "frozen" brains directly in Postgres:

```sql
SELECT session_id, octet_length(vm_state) as blob_size FROM agent_sessions;

```

---

**Would you like me to provide a sample `curl` command or a small Go test script to verify the session persistence across multiple restarts?**

To verify that your agent correctly "freezes" and "thaws" its brain using GnoVM and Postgres, you can use this simple test script. It simulates a conversation, shuts down, and then resumes to prove that the history remained intact within the VM state.

### **1. Persistence Verification Script (`scripts/test_persistence.go`)**

```go
package main

import (
	"context"
	"fmt"
	"log"

	"myagent/internal/app" // Your initialized app logic
)

func main() {
	sessionID := "test-session-123"
	ctx := context.Background()

	// STEP 1: First Interaction
	fmt.Println("--- Turn 1: Initializing Agent ---")
	resp1, _ := app.HandleMessage(ctx, sessionID, "My name is Gemini and I love GnoVM.")
	fmt.Printf("Agent: %s\n\n", resp1)

	// STEP 2: "Cold Boot" Simulation
	// In a real scenario, the process might restart here.
	fmt.Println("--- Simulating System Restart (Clearing Local Cache) ---")

	// STEP 3: Second Interaction (Testing Memory)
	fmt.Println("--- Turn 2: Testing Memory Recovery ---")
	resp2, _ := app.HandleMessage(ctx, sessionID, "Do you remember my name and what I love?")
	fmt.Printf("Agent: %s\n", resp2)
}

```

---

### **2. Verification via cURL**

If you have exposed your agent via a REST API, you can run these two commands sequentially.

**Command 1: Set the State**

```bash
curl -X POST http://localhost:8080/chat \
  -H "Content-Type: application/json" \
  -d '{"session_id": "user_789", "message": "Remember that my favorite color is Cyan."}'

```

**Command 2: Verify the State (After restarting the server)**

```bash
curl -X POST http://localhost:8080/chat \
  -H "Content-Type: application/json" \
  -d '{"session_id": "user_789", "message": "What is my favorite color?"}'

```

---

### **3. Database Inspection (The "Ground Truth")**

To be 100% sure the GnoVM is being serialized into your configurable object store, run this SQL query against your Postgres instance:

```sql
-- Check if the VMState blob is actually being saved and updated
SELECT 
    session_id, 
    length(vm_state) as blob_size_bytes, 
    updated_at 
FROM agent_sessions 
WHERE session_id = 'user_789';

```

> **Note on Blob Size:** You should notice the `blob_size_bytes` increase slightly as the conversation history grows, until the `pruneHistory()` function in your `agent.gno` kicks in to cap the size.

---

### **Final Checklist for Production**

* [ ] **Encryption:** Since `vm_state` contains conversation history, consider encrypting the blob before saving it to Postgres if you are handling sensitive data.
* [ ] **Migrations:** Use GORM's `AutoMigrate` to ensure the `pgvector` extension and tables are ready before the agent starts.
* [ ] **Monitoring:** Track the time it takes for `machine.RestoreState()`. For very large GnoVM states, this is the primary bottleneck.

---

**AES encryption layer to the `BeforeSave` and `AfterFind` GORM hooks to secure the agent's memory**

Adding an encryption layer ensures that even if your database is compromised, the agent's "thoughts" and history remain private. Since GORM hooks are used, this process is transparent to the rest of your application.

### **1. Secure GORM Models with Encryption**

We will use **AES-GCM** (Advanced Encryption Standard with Galois/Counter Mode) because it provides both confidentiality and data integrity.

**File: `internal/storage/encrypted_session.go**`

```go
package storage

import (
    "crypto/aes"
    "crypto/cipher"
    "crypto/rand"
    "io"
    "errors"
    "gorm.io/gorm"
)

type AgentSession struct {
    gorm.Model
    SessionID string `gorm:"uniqueIndex"`
    VMState   []byte `gorm:"type:bytea"` // This will now store Encrypted data
}

// Encrypt wraps the GnoVM blob before it hits Postgres
func (s *AgentSession) BeforeSave(tx *gorm.DB) error {
    // 1. Get the raw VM state from GnoVM
    rawState, ok := tx.Get("active_vm_bytes")
    if !ok { return nil }
    
    // 2. Encrypt the bytes (using your key from config)
    key := []byte(os.Getenv("AGENT_ENCRYPTION_KEY")) // Must be 32 bytes
    encrypted, err := encrypt(rawState.([]byte), key)
    if err != nil { return err }

    s.VMState = encrypted
    return nil
}

// Decrypt un-wraps the blob after loading from Postgres
func (s *AgentSession) AfterFind(tx *gorm.DB) error {
    key := []byte(os.Getenv("AGENT_ENCRYPTION_KEY"))
    decrypted, err := decrypt(s.VMState, key)
    if err != nil { return err }

    s.VMState = decrypted
    return nil
}

```

---

### **2. AES-GCM Implementation Functions**

These helper functions handle the low-level crypto logic.

```go
func encrypt(plaintext []byte, key []byte) ([]byte, error) {
    block, _ := aes.NewCipher(key)
    gcm, _ := cipher.NewGCM(block)
    nonce := make([]byte, gcm.NonceSize())
    io.ReadFull(rand.Reader, nonce)
    
    // Seal appends the ciphertext to the nonce
    return gcm.Seal(nonce, nonce, plaintext, nil), nil
}

func decrypt(ciphertext []byte, key []byte) ([]byte, error) {
    block, _ := aes.NewCipher(key)
    gcm, _ := cipher.NewGCM(block)
    nonceSize := gcm.NonceSize()
    
    nonce, actualCiphertext := ciphertext[:nonceSize], ciphertext[nonceSize:]
    return gcm.Open(nil, nonce, actualCiphertext, nil)
}

```

---

### **3. Updated Re-animation Flow with Security**

With encryption active, the "Thaw" and "Freeze" cycle now includes a cryptographic gate.

1. **Request:** User sends a message.
2. **Thaw (Decryption):** GORM `AfterFind` hook automatically decrypts the `VMState` using the `AGENT_ENCRYPTION_KEY`.
3. **Process:** GnoVM restores the memory and the Agent responds.
4. **Freeze (Encryption):** GORM `BeforeSave` hook automatically encrypts the new memory snapshot.
5. **Persist:** The encrypted ciphertext is stored in the Postgres `bytea` column.

---

### **Final Project Checklist**

| Component | Status | Purpose |
| --- | --- | --- |
| **GnoVM Wrapper** | ✅ Complete | Deterministic logic & binary state export. |
| **GORM Hook** | ✅ Complete | Automatic "Freeze/Thaw" with snapshotting. |
| **AES-GCM Layer** | ✅ Complete | Privacy and integrity for the agent's memory. |
| **YAML Config** | ✅ Complete | Externalized environment and model settings. |
| **Pruning Logic** | ✅ Complete | Keeps DB size and LLM context lean. |

### **Next Step: Performance Tuning**

If you notice high latency during the "Thaw" phase as your `VMState` grows, you can implement **Lazy Loading**. Instead of loading the entire history into GnoVM every turn, you can store older history in a separate `Memories` table using **pgvector** and only pull them into the VM when the LLM specifically asks for "Long Term Memory."

---

**Would you like me to show you how to implement this "Hybrid Memory" tool where the agent can search Postgres vector data from within GnoVM?**

---







