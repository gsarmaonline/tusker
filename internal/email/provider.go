package email

import "context"

// Message holds the fields needed to send an email.
type Message struct {
	To      []string `json:"to"`
	From    string   `json:"from"`
	Subject string   `json:"subject"`
	Body    string   `json:"body"`
	HTML    bool     `json:"html"`
}

// JobPayload is the serialized form of an email send job stored in the jobs table.
type JobPayload struct {
	Provider string `json:"provider"`
	Message
}

// Provider defines the interface each email provider must implement.
type Provider interface {
	Send(ctx context.Context, msg Message) error
}
