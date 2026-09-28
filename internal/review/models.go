package review

import "time"

type ReviewRequest struct {
	ReviewID            string    `json:"review_id"`
	InstallationID      int64     `json:"installation_id"`
	Organization        string    `json:"organization"`
	Repository          string    `json:"repository"`
	PullRequestNumber   int       `json:"pull_request_number"`
	BaseSHA             string    `json:"base_sha"`
	HeadSHA             string    `json:"head_sha"`
	EventType           string    `json:"event_type"`
	RequestedBy         string    `json:"requested_by"`
	RequestedAt         time.Time `json:"requested_at"`
	ReviewMode          string    `json:"review_mode"`
	ReviewPolicyVersion string    `json:"review_policy_version"`
}

type ReviewFinding struct {
	FindingID   string  `json:"finding_id"`
	Source      string  `json:"source"`
	Category    string  `json:"category"`
	Severity    string  `json:"severity"`
	Confidence  float64 `json:"confidence"`
	Title       string  `json:"title"`
	Explanation string  `json:"explanation"`
	Suggestion  string  `json:"suggestion"`
	FilePath    string  `json:"file_path"`
	StartLine   int     `json:"start_line"`
	EndLine     int     `json:"end_line"`
	Evidence    string  `json:"evidence"`
}

type ReviewContext struct {
	Organization      string
	Repository        string
	PullRequestNumber int
	PullRequestTitle  string
	PullRequestBody   string
	HeadSHA           string
	BaseSHA           string
	ChangedFiles      []ChangedFile
	SourceFiles       []SourceFile
	AnalyzerFindings  []ReviewFinding
	ContextBudget     ContextBudget
}

type ContextBudget struct {
	MaxFiles        int
	MaxBytes        int
	MaxTokens       int
	UsedBytes       int
	EstimatedTokens int
}

type ChangedFile struct {
	Path      string
	Status    string
	Additions int
	Deletions int
	Patch     string
}

type SourceFile struct {
	Path    string
	Content string
}

type AnalyzerResult struct {
	AnalyzerName    string
	AnalyzerVersion string
	Status          string
	DurationMillis  int64

	Findings    []ReviewFinding
	Diagnostics []string
}

type ReviewResult struct {
	ReviewID          string
	Organization      string
	Repository        string
	PullRequestNumber int

	BaseSHA string
	HeadSHA string

	Status string

	Findings        []ReviewFinding
	AnalyzerSummary []AnalyzerResult

	ModelSummary string

	DurationMillis int64

	Errors []string
}
