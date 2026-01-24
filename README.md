# MedAgent

A medical document transcription agent built with Google's [ADK-Go](https://github.com/google/adk-go) framework. MedAgent converts medical documents (PDFs and images) into FHIR R5 compliant JSON.

## Features

- **Multi-format Input**: Accepts PDF documents and image files (PNG, JPG, JPEG, TIFF)
- **OCR Capabilities**: Extracts text from scanned documents and images using Gemini's multimodal capabilities
- **Document Classification**: Automatically identifies document types (Prescription, Discharge Summary, Lab Report, Diagnostic Report)
- **FHIR R5 Output**: Generates compliant FHIR resources with proper coding systems (LOINC, SNOMED CT, RxNorm, UCUM)

## Architecture

```
┌─────────────────────────────────────────────────────┐
│                MedAgent (Router)                    │
│         Detects input type and routes               │
└─────────────┬───────────────┬───────────────────────┘
              │               │
    ┌─────────▼─────┐   ┌─────▼─────┐
    │ PDF Extractor │   │ OCR Agent │
    │    Agent      │   │           │
    └───────┬───────┘   └─────┬─────┘
            │                 │
            └────────┬────────┘
                     │
           ┌─────────▼─────────┐
           │   Txt2Fhir Agent  │
           │  (Classifier)     │
           └─────────┬─────────┘
                     │
    ┌────────┬───────┼───────┬────────┐
    ▼        ▼       ▼       ▼        ▼
┌────────┐┌────────┐┌────────┐┌────────┐┌────────┐
│Prescrip││Discharge││  Lab   ││Diagnost││ Others │
│  tion  ││ Summary ││ Report ││ic Imag.││        │
└────────┘└────────┘└────────┘└────────┘└────────┘
    │        │       │       │        │
    ▼        ▼       ▼       ▼        ▼
┌────────────────────────────────────────────────┐
│              FHIR R5 JSON Output               │
│ MedicationRequest│Composition│DiagnosticReport │
│               DocumentReference                │
└────────────────────────────────────────────────┘
```

## FHIR R5 Resources Generated

| Document Type | FHIR Resource | Coding Systems |
|---------------|---------------|----------------|
| Prescription | MedicationRequest | RxNorm, SNOMED CT, UCUM |
| Discharge Summary | Composition | LOINC (section codes) |
| Lab Report | DiagnosticReport | LOINC, UCUM |
| Diagnostic/Imaging | DiagnosticReport | LOINC, SNOMED CT |
| Others | DocumentReference | LOINC |

## Prerequisites

- Go 1.24+
- Google API Key for Gemini access

## Setup

1. Set your Google API key:
   ```bash
   export GOOGLE_API_KEY="your-api-key"
   ```

2. Install dependencies:
   ```bash
   go mod tidy
   ```

3. Build the agent:
   ```bash
   go build -o med-agent .
   ```

## Usage

### Console Mode (Interactive)

```bash
./med-agent console
```

### Web UI Mode

```bash
./med-agent web
```

Then open http://localhost:8080 in your browser.

### API Server Mode

```bash
./med-agent api
```

### Example Interaction

```
> I have a prescription image. [attach image]

MedAgent: Routing to OCR Agent for text extraction...
OCR Agent: Extracted prescription text...
Txt2Fhir Agent: Detected Prescription document, routing to Prescription Agent...
Prescription Agent: Generated FHIR MedicationRequest:

{
  "resourceType": "MedicationRequest",
  "status": "active",
  "intent": "order",
  "medication": {
    "concept": {
      "coding": [{
        "system": "http://www.nlm.nih.gov/research/umls/rxnorm",
        "code": "860975",
        "display": "Metformin 500 MG Oral Tablet"
      }]
    }
  },
  ...
}
```

## Project Structure

```
med-agent/
├── main.go                      # Entry point with launcher
├── internal/
│   └── agents/
│       ├── router.go           # Root MedAgent router
│       ├── pdf.go              # PDF extraction agent
│       ├── ocr.go              # OCR agent for images
│       ├── txt2fhir.go         # Document classifier agent
│       └── specialists.go      # FHIR conversion specialists
├── pkg/
│   └── fhir/
│       └── types.go            # FHIR R5 Go type definitions
├── go.mod
├── go.sum
├── AGENTS.md
└── README.md
```

## Agent Hierarchy

1. **MedAgent** (Router)
   - Detects input type (PDF vs Image)
   - Routes to appropriate extraction agent

2. **PDFExtractorAgent**
   - Extracts text from PDF documents
   - Handles multi-page documents
   - Transfers to Txt2FhirAgent

3. **OCRAgent**
   - Extracts text from images
   - Handles handwritten prescriptions
   - Transfers to Txt2FhirAgent

4. **Txt2FhirAgent** (Classifier)
   - Analyzes extracted text
   - Classifies document type
   - Routes to specialist agent

5. **Specialist Agents**
   - **PrescriptionAgent** → MedicationRequest
   - **DischargeSummaryAgent** → Composition
   - **LabReportAgent** → DiagnosticReport (LAB)
   - **DiagnosticReportAgent** → DiagnosticReport (RAD)
   - **OthersAgent** → DocumentReference

## Disclaimer

This agent is for demonstration and development purposes. Medical document processing should be validated by healthcare professionals before clinical use. Always ensure compliance with HIPAA and other healthcare data regulations.
