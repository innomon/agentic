# Product Guidelines

## Code & Architecture Principles
- **Config-Driven Architecture**: Define all OKF tools, models, and agents via YAML configuration where possible.
- **Go Best Practices**: Handcrafted registries for CLI/slash commands, clean package separation under `pkg/`, no spf13/Cobra libraries.
- **Taxonomy Compliance**: Maintain canonical taxonomy references using workspace-root relative paths (`taxonomy.md`).
- **Standardized ADK-Go Integration**: Implement tools matching ADK-Go `tool.Tool` interfaces and generic registry constructors in `pkg/registry/`.
