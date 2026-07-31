# ARD Implementation Design — `innomon/agentic`

> **Spec:** [ARD v0.9 Draft](https://github.com/ards-project/ard-spec/blob/main/spec/ard.md)  
> **Target repo:** `github.com/innomon/agentic`  
> **New package:** `github.com/innomon/agentic/pkg/ard`

---

## 1. Proposed Repository Layout

```
innomon/agentic/
├── go.mod
├── pkg/
│   ├── routing/              # existing — agent router/dispatcher
│   │   └── ...
│   └── ard/                  # NEW — ARD publisher + optional registry
│       ├── mediatype.go      # Media-type string constants
│       ├── catalog.go        # Catalog / Host / Entry / TrustManifest types
│       ├── builder.go        # CatalogBuilder: introspects routing.Router → Catalog
│       ├── handler.go        # HTTP handler: serves /.well-known/ai-catalog.json
│       ├── registry.go       # Optional: lightweight in-process registry (POST /search)
│       └── ard_test.go       # Unit tests
└── example/
    └── ard/
        └── main.go           # Runnable demo wiring everything together
```

---

## 2. `mediatype.go` — Media-Type Constants

```go
// Package ard implements the Agentic Resource Discovery (ARD) specification
// (v0.9 Draft) for the agentic framework. It provides catalog publishing
// (/.well-known/ai-catalog.json) and an optional lightweight registry.
//
// Spec: https://github.com/ards-project/ard-spec/blob/main/spec/ard.md
package ard

// Standard ARD media-type strings for use in Entry.MediaType.
const (
    MediaTypeMCPServer     = "application/mcp-server+json"
    MediaTypeMCPServerCard = "application/mcp-server-card+json"
    MediaTypeA2AAgentCard  = "application/a2a-agent-card+json"
    MediaTypeRegistry      = "application/ai-registry+json"
    MediaTypeOpenAPIJSON   = "application/openapi+json"
    MediaTypeOpenAPIYAML   = "application/openapi+yaml"
    MediaTypeSkill         = "application/skill+json"
)
```

---

## 3. `catalog.go` — Core Types

```go
package ard

import (
    "encoding/json"
    "time"
)

// Catalog is the top-level structure serialized as ai-catalog.json.
// It is served at /.well-known/ai-catalog.json per the ARD spec.
type Catalog struct {
    SpecVersion string       `json:"specVersion"`
    Host        Host         `json:"host"`
    Entries     []Entry      `json:"entries"`
    Collections []Collection `json:"collections,omitempty"`
}

// Host identifies the entity publishing the catalog.
type Host struct {
    DisplayName string `json:"displayName"`
    Identifier  string `json:"identifier"`
}

// Entry represents a single discoverable agentic resource.
type Entry struct {
    Identifier    string         `json:"identifier"`
    MediaType     string         `json:"mediaType"`
    URL           string         `json:"url"`
    Description   string         `json:"description"`
    TrustManifest *TrustManifest `json:"trustManifest,omitempty"`
}

// TrustManifest holds optional progressive trust signals for an Entry.
type TrustManifest struct {
    Attestations     []string   `json:"attestations,omitempty"`
    Signature        string     `json:"signature,omitempty"`
    WorkloadIdentity string     `json:"workloadIdentity,omitempty"`
    ValidUntil       *time.Time `json:"validUntil,omitempty"`
}

// Collection links to a sub-catalog or nested catalog feed.
type Collection struct {
    Name string `json:"name"`
    URL  string `json:"url"`
}

// JSON serializes the catalog to indented JSON bytes.
func (c *Catalog) JSON() ([]byte, error) {
    return json.MarshalIndent(c, "", "  ")
}
```

---

## 4. `builder.go` — Catalog Builder

The builder is the integration point with `pkg/routing`. It lets the consuming
app register agents and generate the catalog automatically.

```go
package ard

import "fmt"

// AgentRegistration holds the metadata needed to create an ARD catalog Entry
// for a single agent managed by the agentic routing framework.
type AgentRegistration struct {
    Name          string
    Namespace     string
    MediaType     string
    URL           string
    Description   string
    TrustManifest *TrustManifest
}

// CatalogBuilder constructs an ARD Catalog from a set of AgentRegistrations.
type CatalogBuilder struct {
    publisher   string
    host        Host
    agents      []AgentRegistration
    collections []Collection
}

// NewCatalogBuilder creates a CatalogBuilder for the given publisher and host.
func NewCatalogBuilder(publisher string, host Host) *CatalogBuilder {
    return &CatalogBuilder{publisher: publisher, host: host}
}

// Register adds an agent to the catalog being built.
func (b *CatalogBuilder) Register(a AgentRegistration) *CatalogBuilder {
    b.agents = append(b.agents, a)
    return b
}

// AddCollection appends a sub-catalog link to the Catalog.
func (b *CatalogBuilder) AddCollection(c Collection) *CatalogBuilder {
    b.collections = append(b.collections, c)
    return b
}

// Build assembles and returns the final Catalog.
// Each registered AgentRegistration becomes one Entry with URN:
//   urn:ai:<publisher>:<namespace>:<name>
func (b *CatalogBuilder) Build() Catalog {
    entries := make([]Entry, 0, len(b.agents))
    for _, a := range b.agents {
        entries = append(entries, Entry{
            Identifier:    fmt.Sprintf("urn:ai:%s:%s:%s", b.publisher, a.Namespace, a.Name),
            MediaType:     a.MediaType,
            URL:           a.URL,
            Description:   a.Description,
            TrustManifest: a.TrustManifest,
        })
    }
    return Catalog{
        SpecVersion: "1.0",
        Host:        b.host,
        Entries:     entries,
        Collections: b.collections,
    }
}
```

---

## 5. `handler.go` — HTTP Handler

```go
package ard

import (
    "log/slog"
    "net/http"
)

const wellKnownPath = "/.well-known/ai-catalog.json"

// Handler returns an http.Handler that serves the ARD catalog at
// /.well-known/ai-catalog.json with the correct Content-Type.
//
// Usage:
//
//   mux.Handle(ard.WellKnownPath(), ard.Handler(catalog))
func Handler(catalog Catalog) http.Handler {
    payload, err := catalog.JSON()
    if err != nil {
        panic("ard: failed to serialize catalog: " + err.Error())
    }
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        if r.Method != http.MethodGet && r.Method != http.MethodHead {
            http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
            return
        }
        w.Header().Set("Content-Type", "application/json; charset=utf-8")
        w.Header().Set("Cache-Control", "public, max-age=3600")
        w.WriteHeader(http.StatusOK)
        if r.Method == http.MethodHead {
            return
        }
        if _, err := w.Write(payload); err != nil {
            slog.Error("ard: failed to write catalog response", "err", err)
        }
    })
}

// WellKnownPath returns the canonical ARD catalog path.
func WellKnownPath() string { return wellKnownPath }
```

---

## 6. `registry.go` — Lightweight In-Process Registry (optional)

Makes `agentic` itself discoverable as an ARD registry, enabling federation.

```go
package ard

import (
    "encoding/json"
    "net/http"
    "strings"
)

// SearchRequest is the body of POST /search per the ARD Registry API.
type SearchRequest struct {
    Query   string        `json:"query"`
    Filters SearchFilters `json:"filters,omitempty"`
}

// SearchFilters narrows a registry query.
type SearchFilters struct {
    Type       string   `json:"type,omitempty"`
    Compliance []string `json:"compliance,omitempty"`
}

// SearchResponse is the body returned by POST /search.
type SearchResponse struct {
    Results []Entry `json:"results"`
}

// RegistryHandler returns an http.Handler for the ARD registry search endpoint.
// It performs simple in-memory keyword + mediaType matching against the catalog.
//
// Mount at /ard/search (or any path) in the consuming application.
func RegistryHandler(catalog Catalog) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        if r.Method != http.MethodPost {
            http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
            return
        }
        var req SearchRequest
        if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
            http.Error(w, "bad request: "+err.Error(), http.StatusBadRequest)
            return
        }
        results := search(catalog.Entries, req)
        w.Header().Set("Content-Type", "application/json; charset=utf-8")
        _ = json.NewEncoder(w).Encode(SearchResponse{Results: results})
    })
}

func search(entries []Entry, req SearchRequest) []Entry {
    query := strings.ToLower(req.Query)
    var out []Entry
    for _, e := range entries {
        if req.Filters.Type != "" && e.MediaType != req.Filters.Type {
            continue
        }
        if len(req.Filters.Compliance) > 0 && !hasAttestations(e, req.Filters.Compliance) {
            continue
        }
        if query == "" ||
            strings.Contains(strings.ToLower(e.Description), query) ||
            strings.Contains(strings.ToLower(e.Identifier), query) {
            out = append(out, e)
        }
    }
    return out
}

func hasAttestations(e Entry, required []string) bool {
    if e.TrustManifest == nil {
        return false
    }
    have := make(map[string]bool, len(e.TrustManifest.Attestations))
    for _, a := range e.TrustManifest.Attestations {
        have[a] = true
    }
    for _, r := range required {
        if !have[r] {
            return false
        }
    }
    return true
}
```

---

## 7. `example/ard/main.go` — Wiring It Together

```go
package main

import (
    "log"
    "net/http"

    "github.com/innomon/agentic/pkg/ard"
)

func main() {
    builder := ard.NewCatalogBuilder("innomon", ard.Host{
        DisplayName: "Innomon Agentic Platform",
        Identifier:  "https://innomon.in",
    })

    builder.
        Register(ard.AgentRegistration{
            Name:        "support",
            Namespace:   "agent",
            MediaType:   ard.MediaTypeA2AAgentCard,
            URL:         "https://innomon.in/agents/support",
            Description: "Tier-1 customer support agent: handles FAQs, ticket triage, and escalation routing.",
            TrustManifest: &ard.TrustManifest{
                Attestations: []string{"SOC2-Type2"},
            },
        }).
        Register(ard.AgentRegistration{
            Name:        "weather",
            Namespace:   "mcp",
            MediaType:   ard.MediaTypeMCPServer,
            URL:         "https://innomon.in/mcp/weather",
            Description: "Real-time weather data MCP server supporting 200+ cities.",
        }).
        Register(ard.AgentRegistration{
            Name:        "registry",
            Namespace:   "ard",
            MediaType:   ard.MediaTypeRegistry,
            URL:         "https://innomon.in/ard/search",
            Description: "ARD-compliant registry exposing all Innomon agentic capabilities.",
        })

    catalog := builder.Build()

    mux := http.NewServeMux()
    mux.Handle(ard.WellKnownPath(), ard.Handler(catalog))
    mux.Handle("/ard/search", ard.RegistryHandler(catalog))
    // ... mount existing routing.Router handlers here ...

    log.Println("ARD catalog  → http://localhost:8080/.well-known/ai-catalog.json")
    log.Println("ARD registry → http://localhost:8080/ard/search")
    log.Fatal(http.ListenAndServe(":8080", mux))
}
```

---

## 8. Generated `ai-catalog.json` (sample output)

```json
{
  "specVersion": "1.0",
  "host": {
    "displayName": "Innomon Agentic Platform",
    "identifier": "https://innomon.in"
  },
  "entries": [
    {
      "identifier": "urn:ai:innomon:agent:support",
      "mediaType": "application/a2a-agent-card+json",
      "url": "https://innomon.in/agents/support",
      "description": "Tier-1 customer support agent: handles FAQs, ticket triage, and escalation routing.",
      "trustManifest": {
        "attestations": ["SOC2-Type2"]
      }
    },
    {
      "identifier": "urn:ai:innomon:mcp:weather",
      "mediaType": "application/mcp-server+json",
      "url": "https://innomon.in/mcp/weather",
      "description": "Real-time weather data MCP server supporting 200+ cities."
    },
    {
      "identifier": "urn:ai:innomon:ard:registry",
      "mediaType": "application/ai-registry+json",
      "url": "https://innomon.in/ard/search",
      "description": "ARD-compliant registry exposing all Innomon agentic capabilities."
    }
  ]
}
```

---

## 9. `robots.txt` Advertisement Signal

```
User-agent: *
Agentmap: https://innomon.in/.well-known/ai-catalog.json
```

---

## 10. Integration Checklist

- [ ] Create `pkg/ard/` directory in `innomon/agentic`
- [ ] Add `mediatype.go`, `catalog.go`, `builder.go`, `handler.go`
- [ ] Optionally add `registry.go` for federated search
- [ ] In consuming apps (e.g. `whatsadk`), wire `ard.Handler(catalog)` into HTTP mux
- [ ] Register each `routing.Router` agent as an `ard.AgentRegistration`
- [ ] Deploy with TLS (HTTPS required — domain ownership anchors ARD trust)
- [ ] Add `Agentmap:` directive to `robots.txt`
- [ ] Optionally add `<link rel="ai-catalog" href="...">` to any web UI `<head>`

---

## 11. Future Enhancements

| Enhancement | Notes |
|---|---|
| Auto-introspection | Hook into `pkg/routing.Router` to auto-generate registrations without manual `Register()` calls |
| Vector search | Replace keyword search in `registry.go` with embedding-based similarity |
| Trust manifest signing | Add ECDSA/Ed25519 signing of catalog entries using the publisher's private key |
| Catalog versioning | Add `ETag` / `Last-Modified` headers for efficient registry polling |
| robots.txt handler | Auto-inject `Agentmap:` directive into robots.txt responses |
| DNS SVCB | Publish `_catalog._agents.innomon.in` DNS record for DNS-based discovery |
