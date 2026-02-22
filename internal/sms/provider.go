package sms

import "context"

// Message holds the result of a sent SMS.
type Message struct {
	SID    string
	Status string
}

// Provider defines the interface each SMS provider must implement.
type Provider interface {
	// Send delivers an SMS from the given number to the recipient.
	Send(ctx context.Context, from, to, body string) (*Message, error)
}
