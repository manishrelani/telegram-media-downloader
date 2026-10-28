package domain

// OrganizationStatus describes what happened after a media file was downloaded.
type OrganizationStatus string

const (
	OrganizationRenamed          OrganizationStatus = "renamed"
	OrganizationSkipped          OrganizationStatus = "skipped"
	OrganizationFailed           OrganizationStatus = "failed"
	OrganizationAlreadyCompleted OrganizationStatus = "already_completed"
)

// OrganizationOutcome is safe to expose over the backend protocol; SavedPath is a local
// filesystem path and should only be returned to the local desktop UI.
type OrganizationOutcome struct {
	ChatID       int64              `json:"chatId"`
	MessageID    int64              `json:"messageId"`
	OriginalName string             `json:"originalName"`
	SavedPath    string             `json:"savedPath,omitempty"`
	Status       OrganizationStatus `json:"status"`
	Reason       string             `json:"reason,omitempty"`
}

type OrganizationSummary struct {
	Renamed          int                   `json:"renamed"`
	Skipped          int                   `json:"skipped"`
	Failed           int                   `json:"failed"`
	AlreadyCompleted int                   `json:"alreadyCompleted"`
	Outcomes         []OrganizationOutcome `json:"outcomes"`
}
