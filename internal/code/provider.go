package code

import "context"

// Submission is the result of a single code execution.
type Submission struct {
	Token         string
	Stdout        string
	Stderr        string
	CompileOutput string
	Status        string
	Time          string
	Memory        int
}

// JobPayload is the serialized form of a code.execute job stored in the jobs table.
type JobPayload struct {
	Provider   string `json:"provider"`
	SourceCode string `json:"source_code"`
	LanguageID int    `json:"language_id"`
	Stdin      string `json:"stdin,omitempty"`
}

// Provider defines the interface each code execution provider must implement.
type Provider interface {
	Execute(ctx context.Context, sourceCode string, languageID int, stdin string) (*Submission, error)
}
