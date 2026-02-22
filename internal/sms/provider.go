package sms

import "context"

// Message holds the result of a sent SMS.
type Message struct {
	SID    string
	Status string
}

// JobPayload is the serialized form of an SMS send job stored in the jobs table.
type JobPayload struct {
	Provider string `json:"provider"`
	From     string `json:"from"`
	To       string `json:"to"`
	Body     string `json:"body"`
}

// Provider defines the interface each SMS provider must implement.
type Provider interface {
	// Send delivers an SMS from the given number to the recipient.
	Send(ctx context.Context, from, to, body string) (*Message, error)
}
