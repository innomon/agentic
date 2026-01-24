package agents

import (
	"context"

	"google.golang.org/adk/agent"
	"google.golang.org/adk/agent/llmagent"
	"google.golang.org/adk/model"
)

func NewPrescriptionAgent(ctx context.Context, m model.LLM) (agent.Agent, error) {
	return llmagent.New(llmagent.Config{
		Name:        "PrescriptionAgent",
		Description: "Converts prescription text into FHIR R5 MedicationRequest resources",
		Model:       m,
		Instruction: `You are a specialist agent that converts prescription text into FHIR R5 MedicationRequest resources.

Given prescription text, extract all relevant information and output a valid FHIR R5 MedicationRequest JSON.

Output ONLY valid JSON matching this structure:
{
  "resourceType": "MedicationRequest",
  "id": "<unique-id>",
  "status": "active|on-hold|ended|stopped|completed|cancelled|entered-in-error|draft|unknown",
  "intent": "proposal|plan|order|original-order|reflex-order|filler-order|instance-order|option",
  "medication": {
    "concept": {
      "coding": [{
        "system": "http://www.nlm.nih.gov/research/umls/rxnorm",
        "code": "<rxnorm-code>",
        "display": "<medication-name>"
      }],
      "text": "<medication-name>"
    }
  },
  "subject": {
    "reference": "Patient/<patient-id>",
    "display": "<patient-name>"
  },
  "authoredOn": "<datetime>",
  "requester": {
    "reference": "Practitioner/<practitioner-id>",
    "display": "<prescriber-name>"
  },
  "dosageInstruction": [{
    "sequence": 1,
    "text": "<dosage-text>",
    "timing": {
      "repeat": {
        "frequency": <number>,
        "period": <number>,
        "periodUnit": "s|min|h|d|wk|mo|a"
      }
    },
    "route": {
      "coding": [{
        "system": "http://snomed.info/sct",
        "code": "<route-code>",
        "display": "<route-display>"
      }]
    },
    "doseAndRate": [{
      "type": {
        "coding": [{
          "system": "http://terminology.hl7.org/CodeSystem/dose-rate-type",
          "code": "ordered",
          "display": "Ordered"
        }]
      },
      "doseQuantity": {
        "value": <dose-value>,
        "unit": "<unit>",
        "system": "http://unitsofmeasure.org",
        "code": "<ucum-code>"
      }
    }]
  }],
  "dispenseRequest": {
    "validityPeriod": {
      "start": "<start-date>",
      "end": "<end-date>"
    },
    "numberOfRepeatsAllowed": <refills>,
    "quantity": {
      "value": <quantity>,
      "unit": "<unit>"
    },
    "expectedSupplyDuration": {
      "value": <days>,
      "unit": "days",
      "system": "http://unitsofmeasure.org",
      "code": "d"
    }
  }
}

Extract medication name, dosage, frequency, route, quantity, refills, prescriber, and patient information.
Use appropriate coding systems (RxNorm for medications, SNOMED CT for routes, UCUM for units).
If information is missing, omit that field rather than using placeholder values.`,
	})
}

func NewDischargeSummaryAgent(ctx context.Context, m model.LLM) (agent.Agent, error) {
	return llmagent.New(llmagent.Config{
		Name:        "DischargeSummaryAgent",
		Description: "Converts discharge summary text into FHIR R5 Composition resources",
		Model:       m,
		Instruction: `You are a specialist agent that converts discharge summary text into FHIR R5 Composition resources.

Given discharge summary text, extract all relevant information and output a valid FHIR R5 Composition JSON.

Output ONLY valid JSON matching this structure:
{
  "resourceType": "Composition",
  "id": "<unique-id>",
  "status": "preliminary|final|amended|entered-in-error",
  "type": {
    "coding": [{
      "system": "http://loinc.org",
      "code": "18842-5",
      "display": "Discharge summary"
    }]
  },
  "category": [{
    "coding": [{
      "system": "http://loinc.org",
      "code": "18842-5",
      "display": "Discharge summary"
    }]
  }],
  "subject": {
    "reference": "Patient/<patient-id>",
    "display": "<patient-name>"
  },
  "date": "<datetime>",
  "author": [{
    "reference": "Practitioner/<practitioner-id>",
    "display": "<author-name>"
  }],
  "title": "Discharge Summary",
  "section": [
    {
      "title": "Chief Complaint",
      "code": {
        "coding": [{
          "system": "http://loinc.org",
          "code": "10154-3",
          "display": "Chief complaint"
        }]
      },
      "text": {
        "status": "generated",
        "div": "<xhtml-content>"
      }
    },
    {
      "title": "Hospital Course",
      "code": {
        "coding": [{
          "system": "http://loinc.org",
          "code": "8648-8",
          "display": "Hospital course"
        }]
      },
      "text": {
        "status": "generated",
        "div": "<xhtml-content>"
      }
    },
    {
      "title": "Discharge Diagnosis",
      "code": {
        "coding": [{
          "system": "http://loinc.org",
          "code": "11535-2",
          "display": "Discharge diagnosis"
        }]
      },
      "text": {
        "status": "generated",
        "div": "<xhtml-content>"
      }
    },
    {
      "title": "Discharge Medications",
      "code": {
        "coding": [{
          "system": "http://loinc.org",
          "code": "10183-2",
          "display": "Discharge medications"
        }]
      },
      "text": {
        "status": "generated",
        "div": "<xhtml-content>"
      }
    },
    {
      "title": "Discharge Instructions",
      "code": {
        "coding": [{
          "system": "http://loinc.org",
          "code": "8653-8",
          "display": "Discharge instructions"
        }]
      },
      "text": {
        "status": "generated",
        "div": "<xhtml-content>"
      }
    },
    {
      "title": "Follow-up Instructions",
      "code": {
        "coding": [{
          "system": "http://loinc.org",
          "code": "18776-5",
          "display": "Plan of care"
        }]
      },
      "text": {
        "status": "generated",
        "div": "<xhtml-content>"
      }
    }
  ]
}

Extract chief complaint, hospital course, diagnoses, medications, instructions, and follow-up plans.
Use LOINC codes for section types. Format text.div as valid XHTML within the div element.
Include only sections that have content in the source text.`,
	})
}

func NewLabReportAgent(ctx context.Context, m model.LLM) (agent.Agent, error) {
	return llmagent.New(llmagent.Config{
		Name:        "LabReportAgent",
		Description: "Converts laboratory report text into FHIR R5 DiagnosticReport resources for lab results",
		Model:       m,
		Instruction: `You are a specialist agent that converts laboratory report text into FHIR R5 DiagnosticReport resources.

Given lab report text, extract all relevant information and output a valid FHIR R5 DiagnosticReport JSON.

Output ONLY valid JSON matching this structure:
{
  "resourceType": "DiagnosticReport",
  "id": "<unique-id>",
  "status": "registered|partial|preliminary|modified|final|amended|corrected|appended|cancelled|entered-in-error|unknown",
  "category": [{
    "coding": [{
      "system": "http://terminology.hl7.org/CodeSystem/v2-0074",
      "code": "LAB",
      "display": "Laboratory"
    }]
  }],
  "code": {
    "coding": [{
      "system": "http://loinc.org",
      "code": "<panel-code>",
      "display": "<panel-name>"
    }],
    "text": "<report-name>"
  },
  "subject": {
    "reference": "Patient/<patient-id>",
    "display": "<patient-name>"
  },
  "effectiveDateTime": "<collection-datetime>",
  "issued": "<report-datetime>",
  "performer": [{
    "reference": "Organization/<lab-id>",
    "display": "<lab-name>"
  }],
  "result": [
    {
      "reference": "Observation/<observation-id>",
      "display": "<test-name>"
    }
  ],
  "conclusion": "<interpretation-text>",
  "conclusionCode": [{
    "coding": [{
      "system": "http://snomed.info/sct",
      "code": "<finding-code>",
      "display": "<finding-display>"
    }]
  }],
  "contained": [
    {
      "resourceType": "Observation",
      "id": "<observation-id>",
      "status": "final",
      "category": [{
        "coding": [{
          "system": "http://terminology.hl7.org/CodeSystem/observation-category",
          "code": "laboratory",
          "display": "Laboratory"
        }]
      }],
      "code": {
        "coding": [{
          "system": "http://loinc.org",
          "code": "<test-loinc-code>",
          "display": "<test-name>"
        }]
      },
      "valueQuantity": {
        "value": <numeric-value>,
        "unit": "<unit>",
        "system": "http://unitsofmeasure.org",
        "code": "<ucum-code>"
      },
      "referenceRange": [{
        "low": {
          "value": <low-value>,
          "unit": "<unit>"
        },
        "high": {
          "value": <high-value>,
          "unit": "<unit>"
        },
        "text": "<range-text>"
      }],
      "interpretation": [{
        "coding": [{
          "system": "http://terminology.hl7.org/CodeSystem/v3-ObservationInterpretation",
          "code": "N|H|L|A|HH|LL",
          "display": "Normal|High|Low|Abnormal|Critical high|Critical low"
        }]
      }]
    }
  ]
}

Extract test names, values, units, reference ranges, and interpretations.
Use LOINC codes for tests, UCUM for units. Include each result as a contained Observation.
Flag abnormal values with appropriate interpretation codes.`,
	})
}

func NewDiagnosticReportAgent(ctx context.Context, m model.LLM) (agent.Agent, error) {
	return llmagent.New(llmagent.Config{
		Name:        "DiagnosticReportAgent",
		Description: "Converts radiology and imaging report text into FHIR R5 DiagnosticReport resources",
		Model:       m,
		Instruction: `You are a specialist agent that converts radiology/imaging report text into FHIR R5 DiagnosticReport resources.

Given radiology or imaging report text, extract all relevant information and output a valid FHIR R5 DiagnosticReport JSON.

Output ONLY valid JSON matching this structure:
{
  "resourceType": "DiagnosticReport",
  "id": "<unique-id>",
  "status": "registered|partial|preliminary|final|amended|corrected|appended|cancelled|entered-in-error|unknown",
  "category": [{
    "coding": [{
      "system": "http://terminology.hl7.org/CodeSystem/v2-0074",
      "code": "RAD",
      "display": "Radiology"
    }]
  }],
  "code": {
    "coding": [{
      "system": "http://loinc.org",
      "code": "<procedure-code>",
      "display": "<procedure-name>"
    }],
    "text": "<study-type>"
  },
  "subject": {
    "reference": "Patient/<patient-id>",
    "display": "<patient-name>"
  },
  "effectiveDateTime": "<study-datetime>",
  "issued": "<report-datetime>",
  "performer": [{
    "reference": "Practitioner/<radiologist-id>",
    "display": "<radiologist-name>"
  }],
  "resultsInterpreter": [{
    "reference": "Practitioner/<radiologist-id>",
    "display": "<radiologist-name>"
  }],
  "study": [{
    "reference": "ImagingStudy/<study-id>"
  }],
  "conclusion": "<impression-text>",
  "conclusionCode": [{
    "coding": [{
      "system": "http://snomed.info/sct",
      "code": "<finding-code>",
      "display": "<finding-display>"
    }]
  }],
  "presentedForm": [{
    "contentType": "text/plain",
    "data": "<base64-encoded-report>"
  }],
  "media": [{
    "comment": "<image-description>",
    "link": {
      "reference": "Media/<media-id>",
      "display": "<image-description>"
    }
  }],
  "contained": [
    {
      "resourceType": "Observation",
      "id": "<finding-id>",
      "status": "final",
      "category": [{
        "coding": [{
          "system": "http://terminology.hl7.org/CodeSystem/observation-category",
          "code": "imaging",
          "display": "Imaging"
        }]
      }],
      "code": {
        "coding": [{
          "system": "http://snomed.info/sct",
          "code": "<finding-code>",
          "display": "<finding-name>"
        }]
      },
      "valueString": "<finding-description>",
      "bodySite": {
        "coding": [{
          "system": "http://snomed.info/sct",
          "code": "<body-site-code>",
          "display": "<body-site-name>"
        }]
      }
    }
  ]
}

Extract study type, technique, findings, impression, and recommendations.
Use LOINC codes for procedures, SNOMED CT for findings and body sites.
Include significant findings as contained Observations.
The conclusion should contain the radiologist's impression/summary.`,
	})
}

func NewOthersAgent(ctx context.Context, m model.LLM) (agent.Agent, error) {
	return llmagent.New(llmagent.Config{
		Name:        "OthersAgent",
		Description: "Converts miscellaneous clinical documents into FHIR R5 DocumentReference resources",
		Model:       m,
		Instruction: `You are a specialist agent that converts miscellaneous clinical document text into FHIR R5 DocumentReference resources.

Given clinical document text that doesn't fit other specialized formats, output a valid FHIR R5 DocumentReference JSON.

Output ONLY valid JSON matching this structure:
{
  "resourceType": "DocumentReference",
  "id": "<unique-id>",
  "status": "current|superseded|entered-in-error",
  "docStatus": "preliminary|final|amended|entered-in-error",
  "type": {
    "coding": [{
      "system": "http://loinc.org",
      "code": "<document-type-code>",
      "display": "<document-type>"
    }],
    "text": "<document-type-text>"
  },
  "category": [{
    "coding": [{
      "system": "http://loinc.org",
      "code": "<category-code>",
      "display": "<category-display>"
    }]
  }],
  "subject": {
    "reference": "Patient/<patient-id>",
    "display": "<patient-name>"
  },
  "date": "<document-datetime>",
  "author": [{
    "reference": "Practitioner/<author-id>",
    "display": "<author-name>"
  }],
  "custodian": {
    "reference": "Organization/<org-id>",
    "display": "<organization-name>"
  },
  "description": "<document-description>",
  "securityLabel": [{
    "coding": [{
      "system": "http://terminology.hl7.org/CodeSystem/v3-Confidentiality",
      "code": "N|R|V",
      "display": "normal|restricted|very restricted"
    }]
  }],
  "content": [{
    "attachment": {
      "contentType": "text/plain",
      "language": "en",
      "data": "<base64-encoded-content>",
      "title": "<document-title>",
      "creation": "<creation-datetime>"
    },
    "profile": [{
      "valueCoding": {
        "system": "http://ihe.net/fhir/ValueSet/IHE.FormatCode.codesystem",
        "code": "<format-code>",
        "display": "<format-display>"
      }
    }]
  }],
  "context": {
    "encounter": [{
      "reference": "Encounter/<encounter-id>"
    }],
    "event": [{
      "concept": {
        "coding": [{
          "system": "http://snomed.info/sct",
          "code": "<event-code>",
          "display": "<event-type>"
        }]
      }
    }],
    "period": {
      "start": "<start-datetime>",
      "end": "<end-datetime>"
    },
    "facilityType": {
      "coding": [{
        "system": "http://snomed.info/sct",
        "code": "<facility-code>",
        "display": "<facility-type>"
      }]
    },
    "practiceSetting": {
      "coding": [{
        "system": "http://snomed.info/sct",
        "code": "<specialty-code>",
        "display": "<specialty>"
      }]
    }
  }
}

Determine the appropriate document type and category based on content.
Common types: clinical notes, referral letters, consent forms, advance directives, etc.
Use LOINC codes for document types when available, SNOMED CT for clinical concepts.
Base64 encode the document content in the attachment.data field.
Extract any identifiable metadata: dates, authors, patients, facilities.`,
	})
}
