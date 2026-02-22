package email

import "context"

// Message holds the fields needed to send an email.
type Message struct {
	To      []string
	From    string
	Subject string
	Body    string
	HTML    bool
}

// Provider defines the interface each email provider must implement.
type Provider interface {
	Send(ctx context.Context, msg Message) error
}
