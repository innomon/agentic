# Medical FHIR Transcription

Converts medical documents (PDFs and images) into FHIR R5 compliant JSON using a multi-agent pipeline.

## Usage

```bash
./agentic examples/med-fhir/config.yaml console
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

## Disclaimer

For demonstration purposes. Medical document processing should be validated by healthcare professionals.
