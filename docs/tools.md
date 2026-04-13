# ADK Tool calling

[google search tool](https://google.github.io/adk-docs/tools/gemini-api/google-search/)

## Built-in Tools

### `fs_read`
Reads a file from the local filesystem.
- **Parameters**:
  - `path` (string, required): Absolute or relative path to the file.
- **Output**: File content as string.

### `fhir_get_schema`
Extracts a specialized subset of the FHIR R5 schema for a specific resource type.
- **Implementation**: Uses an embedded 4.2MB FHIR schema within the `fsread` package for sub-millisecond extraction.
- **Parameters**:
  - `resource_type` (string, required): The FHIR resource type to extract (e.g., `MedicationRequest`, `Composition`).
- **Output**: A JSON schema string containing only the requested resource type and all recursively referenced definitions.