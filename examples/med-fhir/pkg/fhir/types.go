package fhir

import "time"

// Common types

type Coding struct {
	System       *string `json:"system,omitempty"`
	Version      *string `json:"version,omitempty"`
	Code         *string `json:"code,omitempty"`
	Display      *string `json:"display,omitempty"`
	UserSelected *bool   `json:"userSelected,omitempty"`
}

type CodeableConcept struct {
	Coding []Coding `json:"coding,omitempty"`
	Text   *string  `json:"text,omitempty"`
}

type Identifier struct {
	Use      *string          `json:"use,omitempty"`
	Type     *CodeableConcept `json:"type,omitempty"`
	System   *string          `json:"system,omitempty"`
	Value    *string          `json:"value,omitempty"`
	Period   *Period          `json:"period,omitempty"`
	Assigner *Reference       `json:"assigner,omitempty"`
}

type Reference struct {
	Reference  *string     `json:"reference,omitempty"`
	Type       *string     `json:"type,omitempty"`
	Identifier *Identifier `json:"identifier,omitempty"`
	Display    *string     `json:"display,omitempty"`
}

type Period struct {
	Start *time.Time `json:"start,omitempty"`
	End   *time.Time `json:"end,omitempty"`
}

type Narrative struct {
	Status string `json:"status"`
	Div    string `json:"div"`
}

type Quantity struct {
	Value      *float64 `json:"value,omitempty"`
	Comparator *string  `json:"comparator,omitempty"`
	Unit       *string  `json:"unit,omitempty"`
	System     *string  `json:"system,omitempty"`
	Code       *string  `json:"code,omitempty"`
}

type Range struct {
	Low  *Quantity `json:"low,omitempty"`
	High *Quantity `json:"high,omitempty"`
}

type Ratio struct {
	Numerator   *Quantity `json:"numerator,omitempty"`
	Denominator *Quantity `json:"denominator,omitempty"`
}

type Annotation struct {
	AuthorReference *Reference `json:"authorReference,omitempty"`
	AuthorString    *string    `json:"authorString,omitempty"`
	Time            *time.Time `json:"time,omitempty"`
	Text            string     `json:"text"`
}

type TimingRepeat struct {
	BoundsDuration *Quantity `json:"boundsDuration,omitempty"`
	BoundsRange    *Range    `json:"boundsRange,omitempty"`
	BoundsPeriod   *Period   `json:"boundsPeriod,omitempty"`
	Count          *int      `json:"count,omitempty"`
	CountMax       *int      `json:"countMax,omitempty"`
	Duration       *float64  `json:"duration,omitempty"`
	DurationMax    *float64  `json:"durationMax,omitempty"`
	DurationUnit   *string   `json:"durationUnit,omitempty"`
	Frequency      *int      `json:"frequency,omitempty"`
	FrequencyMax   *int      `json:"frequencyMax,omitempty"`
	Period         *float64  `json:"period,omitempty"`
	PeriodMax      *float64  `json:"periodMax,omitempty"`
	PeriodUnit     *string   `json:"periodUnit,omitempty"`
	DayOfWeek      []string  `json:"dayOfWeek,omitempty"`
	TimeOfDay      []string  `json:"timeOfDay,omitempty"`
	When           []string  `json:"when,omitempty"`
	Offset         *int      `json:"offset,omitempty"`
}

type Timing struct {
	Event  []time.Time      `json:"event,omitempty"`
	Repeat *TimingRepeat    `json:"repeat,omitempty"`
	Code   *CodeableConcept `json:"code,omitempty"`
}

type DoseAndRate struct {
	Type        *CodeableConcept `json:"type,omitempty"`
	DoseRange   *Range           `json:"doseRange,omitempty"`
	DoseQuanty  *Quantity        `json:"doseQuantity,omitempty"`
	RateRatio   *Ratio           `json:"rateRatio,omitempty"`
	RateRange   *Range           `json:"rateRange,omitempty"`
	RateQuantiy *Quantity        `json:"rateQuantity,omitempty"`
}

type DosageInstruction struct {
	Sequence                 *int             `json:"sequence,omitempty"`
	Text                     *string          `json:"text,omitempty"`
	AdditionalInstruction    []CodeableConcept `json:"additionalInstruction,omitempty"`
	PatientInstruction       *string          `json:"patientInstruction,omitempty"`
	Timing                   *Timing          `json:"timing,omitempty"`
	AsNeeded                 *bool            `json:"asNeeded,omitempty"`
	AsNeededFor              []CodeableConcept `json:"asNeededFor,omitempty"`
	Site                     *CodeableConcept `json:"site,omitempty"`
	Route                    *CodeableConcept `json:"route,omitempty"`
	Method                   *CodeableConcept `json:"method,omitempty"`
	DoseAndRate              []DoseAndRate    `json:"doseAndRate,omitempty"`
	MaxDosePerPeriod         []Ratio          `json:"maxDosePerPeriod,omitempty"`
	MaxDosePerAdministration *Quantity        `json:"maxDosePerAdministration,omitempty"`
	MaxDosePerLifetime       *Quantity        `json:"maxDosePerLifetime,omitempty"`
}

type Attachment struct {
	ContentType *string    `json:"contentType,omitempty"`
	Language    *string    `json:"language,omitempty"`
	Data        *string    `json:"data,omitempty"`
	URL         *string    `json:"url,omitempty"`
	Size        *int64     `json:"size,omitempty"`
	Hash        *string    `json:"hash,omitempty"`
	Title       *string    `json:"title,omitempty"`
	Creation    *time.Time `json:"creation,omitempty"`
	Height      *int       `json:"height,omitempty"`
	Width       *int       `json:"width,omitempty"`
	Frames      *int       `json:"frames,omitempty"`
	Duration    *float64   `json:"duration,omitempty"`
	Pages       *int       `json:"pages,omitempty"`
}

// MedicationRequest - For prescriptions

type MedicationRequest struct {
	ResourceType          string              `json:"resourceType"`
	ID                    *string             `json:"id,omitempty"`
	Identifier            []Identifier        `json:"identifier,omitempty"`
	Status                string              `json:"status"`
	StatusReason          *CodeableConcept    `json:"statusReason,omitempty"`
	StatusChanged         *time.Time          `json:"statusChanged,omitempty"`
	Intent                string              `json:"intent"`
	Category              []CodeableConcept   `json:"category,omitempty"`
	Priority              *string             `json:"priority,omitempty"`
	DoNotPerform          *bool               `json:"doNotPerform,omitempty"`
	Medication            CodeableConcept     `json:"medication"`
	Subject               Reference           `json:"subject"`
	Encounter             *Reference          `json:"encounter,omitempty"`
	SupportingInformation []Reference         `json:"supportingInformation,omitempty"`
	AuthoredOn            *time.Time          `json:"authoredOn,omitempty"`
	Requester             *Reference          `json:"requester,omitempty"`
	Performer             []Reference         `json:"performer,omitempty"`
	Reason                []CodeableConcept   `json:"reason,omitempty"`
	CourseOfTherapyType   *CodeableConcept    `json:"courseOfTherapyType,omitempty"`
	DosageInstruction     []DosageInstruction `json:"dosageInstruction,omitempty"`
	Note                  []Annotation        `json:"note,omitempty"`
}

// DiagnosticReport - For lab tests and diagnostic reports

type DiagnosticReport struct {
	ResourceType      string            `json:"resourceType"`
	ID                *string           `json:"id,omitempty"`
	Identifier        []Identifier      `json:"identifier,omitempty"`
	BasedOn           []Reference       `json:"basedOn,omitempty"`
	Status            string            `json:"status"`
	Category          []CodeableConcept `json:"category,omitempty"`
	Code              CodeableConcept   `json:"code"`
	Subject           *Reference        `json:"subject,omitempty"`
	Encounter         *Reference        `json:"encounter,omitempty"`
	EffectiveDateTime *time.Time        `json:"effectiveDateTime,omitempty"`
	EffectivePeriod   *Period           `json:"effectivePeriod,omitempty"`
	Issued            *time.Time        `json:"issued,omitempty"`
	Performer         []Reference       `json:"performer,omitempty"`
	ResultsInterpreter []Reference      `json:"resultsInterpreter,omitempty"`
	Specimen          []Reference       `json:"specimen,omitempty"`
	Result            []Reference       `json:"result,omitempty"`
	Note              []Annotation      `json:"note,omitempty"`
	Study             []Reference       `json:"study,omitempty"`
	Conclusion        *string           `json:"conclusion,omitempty"`
	ConclusionCode    []CodeableConcept `json:"conclusionCode,omitempty"`
	PresentedForm     []Attachment      `json:"presentedForm,omitempty"`
}

// Composition - For discharge summaries

type CompositionSection struct {
	Title         *string              `json:"title,omitempty"`
	Code          *CodeableConcept     `json:"code,omitempty"`
	Author        []Reference          `json:"author,omitempty"`
	Focus         *Reference           `json:"focus,omitempty"`
	Text          *Narrative           `json:"text,omitempty"`
	OrderedBy     *CodeableConcept     `json:"orderedBy,omitempty"`
	Entry         []Reference          `json:"entry,omitempty"`
	EmptyReason   *CodeableConcept     `json:"emptyReason,omitempty"`
	Section       []CompositionSection `json:"section,omitempty"`
}

type CompositionAttester struct {
	Mode  CodeableConcept `json:"mode"`
	Time  *time.Time      `json:"time,omitempty"`
	Party *Reference      `json:"party,omitempty"`
}

type CompositionEvent struct {
	Period *Period     `json:"period,omitempty"`
	Detail []Reference `json:"detail,omitempty"`
}

type Composition struct {
	ResourceType string                `json:"resourceType"`
	ID           *string               `json:"id,omitempty"`
	Identifier   []Identifier          `json:"identifier,omitempty"`
	Status       string                `json:"status"`
	Type         CodeableConcept       `json:"type"`
	Category     []CodeableConcept     `json:"category,omitempty"`
	Subject      []Reference           `json:"subject,omitempty"`
	Encounter    *Reference            `json:"encounter,omitempty"`
	Date         time.Time             `json:"date"`
	UseContext   []CodeableConcept     `json:"useContext,omitempty"`
	Author       []Reference           `json:"author"`
	Name         *string               `json:"name,omitempty"`
	Title        string                `json:"title"`
	Note         []Annotation          `json:"note,omitempty"`
	Attester     []CompositionAttester `json:"attester,omitempty"`
	Custodian    *Reference            `json:"custodian,omitempty"`
	RelatesTo    []Reference           `json:"relatesTo,omitempty"`
	Event        []CompositionEvent    `json:"event,omitempty"`
	Section      []CompositionSection  `json:"section,omitempty"`
}

// DocumentReference - For other document types

type DocumentReferenceContent struct {
	Attachment Attachment       `json:"attachment"`
	Profile    []CodeableConcept `json:"profile,omitempty"`
}

type DocumentReferenceAttester struct {
	Mode  CodeableConcept `json:"mode"`
	Time  *time.Time      `json:"time,omitempty"`
	Party *Reference      `json:"party,omitempty"`
}

type DocumentReferenceRelatesTo struct {
	Code   CodeableConcept `json:"code"`
	Target Reference       `json:"target"`
}

type DocumentReference struct {
	ResourceType   string                       `json:"resourceType"`
	ID             *string                      `json:"id,omitempty"`
	Identifier     []Identifier                 `json:"identifier,omitempty"`
	Version        *string                      `json:"version,omitempty"`
	BasedOn        []Reference                  `json:"basedOn,omitempty"`
	Status         string                       `json:"status"`
	DocStatus      *string                      `json:"docStatus,omitempty"`
	Modality       []CodeableConcept            `json:"modality,omitempty"`
	Type           *CodeableConcept             `json:"type,omitempty"`
	Category       []CodeableConcept            `json:"category,omitempty"`
	Subject        *Reference                   `json:"subject,omitempty"`
	Context        []Reference                  `json:"context,omitempty"`
	Event          []CodeableConcept            `json:"event,omitempty"`
	BodySite       []CodeableConcept            `json:"bodySite,omitempty"`
	FacilityType   *CodeableConcept             `json:"facilityType,omitempty"`
	PracticeSetting *CodeableConcept            `json:"practiceSetting,omitempty"`
	Period         *Period                      `json:"period,omitempty"`
	Date           *time.Time                   `json:"date,omitempty"`
	Author         []Reference                  `json:"author,omitempty"`
	Attester       []DocumentReferenceAttester  `json:"attester,omitempty"`
	Custodian      *Reference                   `json:"custodian,omitempty"`
	RelatesTo      []DocumentReferenceRelatesTo `json:"relatesTo,omitempty"`
	Description    *string                      `json:"description,omitempty"`
	SecurityLabel  []CodeableConcept            `json:"securityLabel,omitempty"`
	Content        []DocumentReferenceContent   `json:"content"`
}

// Bundle - For wrapping multiple resources

type BundleLink struct {
	Relation string `json:"relation"`
	URL      string `json:"url"`
}

type BundleEntrySearch struct {
	Mode  *string  `json:"mode,omitempty"`
	Score *float64 `json:"score,omitempty"`
}

type BundleEntryRequest struct {
	Method          string     `json:"method"`
	URL             string     `json:"url"`
	IfNoneMatch     *string    `json:"ifNoneMatch,omitempty"`
	IfModifiedSince *time.Time `json:"ifModifiedSince,omitempty"`
	IfMatch         *string    `json:"ifMatch,omitempty"`
	IfNoneExist     *string    `json:"ifNoneExist,omitempty"`
}

type BundleEntryResponse struct {
	Status       string     `json:"status"`
	Location     *string    `json:"location,omitempty"`
	Etag         *string    `json:"etag,omitempty"`
	LastModified *time.Time `json:"lastModified,omitempty"`
	Outcome      any        `json:"outcome,omitempty"`
}

type BundleEntry struct {
	Link     []BundleLink         `json:"link,omitempty"`
	FullURL  *string              `json:"fullUrl,omitempty"`
	Resource any                  `json:"resource,omitempty"`
	Search   *BundleEntrySearch   `json:"search,omitempty"`
	Request  *BundleEntryRequest  `json:"request,omitempty"`
	Response *BundleEntryResponse `json:"response,omitempty"`
}

type Bundle struct {
	ResourceType string        `json:"resourceType"`
	ID           *string       `json:"id,omitempty"`
	Identifier   *Identifier   `json:"identifier,omitempty"`
	Type         string        `json:"type"`
	Timestamp    *time.Time    `json:"timestamp,omitempty"`
	Total        *int          `json:"total,omitempty"`
	Link         []BundleLink  `json:"link,omitempty"`
	Entry        []BundleEntry `json:"entry,omitempty"`
	Issues       any           `json:"issues,omitempty"`
}

// Helper functions

func NewMedicationRequest() *MedicationRequest {
	return &MedicationRequest{
		ResourceType: "MedicationRequest",
	}
}

func NewDiagnosticReport() *DiagnosticReport {
	return &DiagnosticReport{
		ResourceType: "DiagnosticReport",
	}
}

func NewComposition() *Composition {
	return &Composition{
		ResourceType: "Composition",
	}
}

func NewDocumentReference() *DocumentReference {
	return &DocumentReference{
		ResourceType: "DocumentReference",
	}
}

func NewBundle(bundleType string) *Bundle {
	return &Bundle{
		ResourceType: "Bundle",
		Type:         bundleType,
	}
}

func Ptr[T any](v T) *T {
	return &v
}
