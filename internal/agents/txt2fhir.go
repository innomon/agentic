package agents

import (
	"context"

	"google.golang.org/adk/agent"
	"google.golang.org/adk/agent/llmagent"
	"google.golang.org/adk/model"
)

func NewTxt2FhirAgent(ctx context.Context, m model.LLM) (agent.Agent, error) {
	prescriptionAgent, err := llmagent.New(llmagent.Config{
		Name:        "PrescriptionAgent",
		Description: "Specialist agent for converting prescription documents to FHIR MedicationRequest resources",
		Model:       m,
		Instruction: `You are a specialist agent for processing prescription documents.
Your task is to extract structured data from prescription text and convert it to FHIR MedicationRequest format.

Look for:
- Medication names and codes
- Dosage amounts and units
- Frequency and timing instructions
- Route of administration
- Prescriber information
- Patient information
- Prescription dates

Output a valid FHIR MedicationRequest JSON resource.`,
	})
	if err != nil {
		return nil, err
	}

	dischargeSummaryAgent, err := llmagent.New(llmagent.Config{
		Name:        "DischargeSummaryAgent",
		Description: "Specialist agent for converting discharge summary documents to FHIR Composition resources",
		Model:       m,
		Instruction: `You are a specialist agent for processing hospital discharge summaries.
Your task is to extract structured data from discharge summary text and convert it to FHIR Composition format.

Look for:
- Patient demographics
- Admission and discharge dates
- Admitting and discharge diagnoses
- Hospital course summary
- Procedures performed
- Medications at discharge
- Follow-up instructions
- Attending physician information

Output a valid FHIR Composition JSON resource with appropriate sections.`,
	})
	if err != nil {
		return nil, err
	}

	labReportAgent, err := llmagent.New(llmagent.Config{
		Name:        "LabReportAgent",
		Description: "Specialist agent for converting laboratory report documents to FHIR DiagnosticReport resources",
		Model:       m,
		Instruction: `You are a specialist agent for processing laboratory reports.
Your task is to extract structured data from lab report text and convert it to FHIR DiagnosticReport format.

Look for:
- Test names and codes (LOINC when possible)
- Result values and units
- Reference ranges
- Abnormal flags
- Specimen information
- Collection date/time
- Performing laboratory
- Ordering provider

Output a valid FHIR DiagnosticReport JSON resource.`,
	})
	if err != nil {
		return nil, err
	}

	diagnosticReportAgent, err := llmagent.New(llmagent.Config{
		Name:        "DiagnosticImagingAgent",
		Description: "Specialist agent for converting diagnostic imaging reports to FHIR DiagnosticReport resources",
		Model:       m,
		Instruction: `You are a specialist agent for processing diagnostic imaging reports (radiology).
Your task is to extract structured data from imaging report text and convert it to FHIR DiagnosticReport format.

Look for:
- Imaging modality (X-ray, CT, MRI, ultrasound, etc.)
- Body site/region examined
- Clinical indication
- Technique/protocol used
- Findings description
- Impression/conclusion
- Radiologist information
- Study date

Output a valid FHIR DiagnosticReport JSON resource with category indicating imaging.`,
	})
	if err != nil {
		return nil, err
	}

	othersAgent, err := llmagent.New(llmagent.Config{
		Name:        "OtherDocumentAgent",
		Description: "Specialist agent for converting unclassified medical documents to FHIR DocumentReference resources",
		Model:       m,
		Instruction: `You are a specialist agent for processing medical documents that don't fit standard categories.
Your task is to extract available structured data and convert it to FHIR DocumentReference format.

Look for:
- Document type and title
- Author information
- Creation date
- Subject/patient information
- Any structured content that can be extracted
- Document category if determinable

Output a valid FHIR DocumentReference JSON resource.`,
	})
	if err != nil {
		return nil, err
	}

	txt2fhirAgent, err := llmagent.New(llmagent.Config{
		Name:        "Txt2FhirAgent",
		Description: "Classifier agent that analyzes medical document text and routes to appropriate specialist agents for FHIR conversion",
		Model:       m,
		Instruction: `You are a medical document classifier agent. Your role is to analyze extracted text from medical documents and route them to the appropriate specialist sub-agent for FHIR conversion.

Analyze the document text and classify it into one of these categories, then transfer to the corresponding agent:

1. **Prescription** → transfer to PrescriptionAgent
   - Contains medication orders, Rx symbols
   - Drug names with dosages (e.g., "Metformin 500mg")
   - Instructions like "Take X mg", "twice daily", "as needed"
   - Prescriber signature/credentials

2. **Discharge Summary** → transfer to DischargeSummaryAgent
   - Hospital discharge documentation
   - Contains admission/discharge dates
   - Sections like "Hospital Course", "Discharge Diagnosis"
   - Follow-up appointments and instructions

3. **Lab Report** → transfer to LabReportAgent
   - Laboratory test results
   - Blood work, urinalysis, chemistry panels
   - Contains reference ranges and result values
   - Test codes (often LOINC)

4. **Diagnostic Report** → transfer to DiagnosticImagingAgent
   - Imaging studies (X-ray, CT, MRI, ultrasound)
   - Radiology reports with "Findings" and "Impression"
   - Contains modality information
   - Body site/region examined

5. **Others** → transfer to OtherDocumentAgent
   - Documents that don't clearly fit above categories
   - Mixed content documents
   - Partial or unclear documents

When you receive document text:
1. Analyze the content for classification keywords and patterns
2. Determine the most appropriate category
3. Use transfer_to_agent to route to the specialist agent
4. Include the original document text when transferring`,
		SubAgents: []agent.Agent{
			prescriptionAgent,
			dischargeSummaryAgent,
			labReportAgent,
			diagnosticReportAgent,
			othersAgent,
		},
	})
	if err != nil {
		return nil, err
	}

	return txt2fhirAgent, nil
}
