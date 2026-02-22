package email

import (
	"context"
	"fmt"
	"net/smtp"
	"strings"
)

// SMTPConfig holds credentials for an SMTP server.
type SMTPConfig struct {
	Host     string `json:"host"`
	Port     int    `json:"port"`
	Username string `json:"username"`
	Password string `json:"password"`
}

// SMTPProvider sends email via SMTP using Go's standard library.
type SMTPProvider struct {
	cfg SMTPConfig
}

func NewSMTPProvider(cfg SMTPConfig) *SMTPProvider {
	return &SMTPProvider{cfg: cfg}
}

func (p *SMTPProvider) Send(_ context.Context, msg Message) error {
	auth := smtp.PlainAuth("", p.cfg.Username, p.cfg.Password, p.cfg.Host)

	contentType := "text/plain"
	if msg.HTML {
		contentType = "text/html"
	}

	header := fmt.Sprintf(
		"From: %s\r\nTo: %s\r\nSubject: %s\r\nContent-Type: %s; charset=UTF-8\r\n\r\n",
		msg.From,
		strings.Join(msg.To, ", "),
		msg.Subject,
		contentType,
	)
	body := []byte(header + msg.Body)

	addr := fmt.Sprintf("%s:%d", p.cfg.Host, p.cfg.Port)
	return smtp.SendMail(addr, auth, msg.From, msg.To, body)
}
