# Medical FHIR Transcription

Converts medical documents (PDFs and images) into FHIR R5 compliant JSON using a multi-agent pipeline.

## Usage

```bash
./agentic -console examples/med-fhir/config.yaml
```

## Agent Hierarchy

1. **MedAgent** (Router) - Detects input type (PDF vs Image)
2. **PDFExtractorAgent / OCRAgent** - Extracts text from documents
3. **Txt2FhirAgent** (Classifier) - Classifies document type
4. **Specialist Agents** - Converts to FHIR resources:
   - Prescription → MedicationRequest
   - Discharge Summary → Composition
   - Lab Report → DiagnosticReport
   - Diagnostic Imaging → DiagnosticReport
   - Other → DocumentReference

## FHIR Coding Systems

- **RxNorm**: Medications
- **LOINC**: Lab tests, document types
- **SNOMED CT**: Clinical findings, body sites
- **UCUM**: Units of measure

## Recent Improvements

- **Reliable PDF Routing**: `MedAgent` now strictly routes all PDF files (including scanned image-based PDFs) to `PDFExtractorAgent`. This ensures multimodal vision is used for extraction regardless of PDF internal structure.
- **Atomic Agent Transfers**: Extraction agents are now instructed to call `transfer_to_agent` in the same response turn as the text extraction, preventing pipeline stalls on long documents.
- **Schema-Grounded Generation**: Specialist agents use the `fs_read` tool to load `fhir.schema.json` at runtime, ensuring high-fidelity outputs.

## FHIR Schema Grounding

This example uses a JSON schema to ground the specialist agents' generation, ensuring the output is valid FHIR R5 JSON.

- **Schema File**: `examples/med-fhir/fhir.schema.json`
- **Tooling**: Uses the `fs_read` built-in tool to load the schema at runtime.
- **Reference Schema**: These types serve as a reference for the JSON structure the LLM agents are instructed to produce.
- **Custom Extensions**: When importing `agentic` as a library in your own Go application, you can use these types to strongly-type and validate the agents' output.
- **Standalone Execution**: When running via the `agentic` binary with `config.yaml`, these Go files are **not** compiled into the executable. The agents function as standard `llm` agents using system instructions to generate the JSON.

## Disclaimer

For demonstration purposes. Medical document processing should be validated by healthcare professionals.
