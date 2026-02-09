---
name: "🚀 Enhancement Request"
about: "Propose a new feature or optimization for Gnogent"
title: "[ENHANCEMENT] <Short Description>"
labels: enhancement, gno-logic
assignees: ''
---

## 🎯 Proposed Enhancement
Provide a clear and concise description of the new capability (e.g., adding a Vector Search tool, implementing AES encryption for snapshots).

## 🧠 GnoVM Impact (The Brain)
- [ ] Does this change the structure of global variables in `agent.gno`?
- [ ] How does this affect the size of the serialized `VMState`?
- [ ] Does it maintain determinism (No `time.Now()` or external I/O inside Gno)?

## 🔒 Security Considerations
- [ ] Does this require new JWT custom claims?
- [ ] Does it respect the `userId` and `channel` isolation?
- [ ] Is sensitive data being stored in the `VMState` blob?

## 🛠 Implementation Plan
1. **Internal:** (e.g., Update `internal/storage/model.go`)
2. **Gno:** (e.g., Update `gno/agent.gno`)
3. **Auth:** (e.g., Update `internal/auth/claims.go`)

## 🧪 Verification Plan
Describe the "Freeze/Thaw" test scenario to ensure memory persistence remains intact.

