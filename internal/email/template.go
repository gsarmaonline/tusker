package email

import (
	"bytes"
	"fmt"
	htmltemplate "html/template"
	texttemplate "text/template"
)

// TemplateDefinition holds the raw (un-rendered) template strings.
type TemplateDefinition struct {
	Subject string
	Body    string
	HTML    string
}

// RenderedTemplate holds the rendered output ready to send.
type RenderedTemplate struct {
	Subject string
	Body    string
	HTML    string
}

// DefaultTemplates contains built-in templates that serve as fallbacks when a
// tenant has not defined a custom override.
var DefaultTemplates = map[string]TemplateDefinition{
	"welcome": {
		Subject: "Welcome to {{.ServiceName}}",
		Body:    "Hi {{.UserName}},\n\nWelcome to {{.ServiceName}}! We're glad to have you.\n\nBest,\nThe {{.ServiceName}} Team",
		HTML:    `<p>Hi {{.UserName}},</p><p>Welcome to <strong>{{.ServiceName}}</strong>! We're glad to have you.</p><p>Best,<br>The {{.ServiceName}} Team</p>`,
	},
	"login_alert": {
		Subject: "New sign-in to your {{.ServiceName}} account",
		Body:    "Hi {{.UserName}},\n\nWe detected a new sign-in to your {{.ServiceName}} account. If this was you, no action is needed.\n\nIf you did not sign in, please secure your account immediately.",
		HTML:    `<p>Hi {{.UserName}},</p><p>We detected a new sign-in to your <strong>{{.ServiceName}}</strong> account. If this was you, no action is needed.</p><p>If you did not sign in, please secure your account immediately.</p>`,
	},
	"password_reset": {
		Subject: "Reset your {{.ServiceName}} password",
		Body:    "Hi {{.UserName}},\n\nClick the link below to reset your password:\n{{.ResetLink}}\n\nThis link expires in {{.ExpiresIn}}.",
		HTML:    `<p>Hi {{.UserName}},</p><p>Click the link below to reset your password:</p><p><a href="{{.ResetLink}}">Reset Password</a></p><p>This link expires in {{.ExpiresIn}}.</p>`,
	},
	"magic_link": {
		Subject: "Your {{.ServiceName}} sign-in link",
		Body:    "Hi {{.UserName}},\n\nUse the link below to sign in:\n{{.MagicLink}}\n\nThis link expires in {{.ExpiresIn}}.",
		HTML:    `<p>Hi {{.UserName}},</p><p>Use the link below to sign in:</p><p><a href="{{.MagicLink}}">Sign In</a></p><p>This link expires in {{.ExpiresIn}}.</p>`,
	},
}

// ValidateTemplate parses all fields of def to catch template syntax errors
// at upsert time rather than at send time.
func ValidateTemplate(def TemplateDefinition) error {
	if _, err := texttemplate.New("subject").Parse(def.Subject); err != nil {
		return fmt.Errorf("invalid subject template: %w", err)
	}
	if _, err := texttemplate.New("body").Parse(def.Body); err != nil {
		return fmt.Errorf("invalid body template: %w", err)
	}
	if def.HTML != "" {
		if _, err := htmltemplate.New("html").Parse(def.HTML); err != nil {
			return fmt.Errorf("invalid html template: %w", err)
		}
	}
	return nil
}

// RenderTemplate executes a TemplateDefinition against vars and returns the
// rendered subject, plain-text body, and HTML body.
func RenderTemplate(def TemplateDefinition, vars map[string]any) (RenderedTemplate, error) {
	subject, err := renderText(def.Subject, vars)
	if err != nil {
		return RenderedTemplate{}, fmt.Errorf("render subject: %w", err)
	}

	body, err := renderText(def.Body, vars)
	if err != nil {
		return RenderedTemplate{}, fmt.Errorf("render body: %w", err)
	}

	var htmlOut string
	if def.HTML != "" {
		htmlOut, err = renderHTML(def.HTML, vars)
		if err != nil {
			return RenderedTemplate{}, fmt.Errorf("render html: %w", err)
		}
	}

	return RenderedTemplate{Subject: subject, Body: body, HTML: htmlOut}, nil
}

func renderText(tmplStr string, vars map[string]any) (string, error) {
	t, err := texttemplate.New("").Parse(tmplStr)
	if err != nil {
		return "", err
	}
	var buf bytes.Buffer
	if err := t.Execute(&buf, vars); err != nil {
		return "", err
	}
	return buf.String(), nil
}

func renderHTML(tmplStr string, vars map[string]any) (string, error) {
	t, err := htmltemplate.New("").Parse(tmplStr)
	if err != nil {
		return "", err
	}
	var buf bytes.Buffer
	if err := t.Execute(&buf, vars); err != nil {
		return "", err
	}
	return buf.String(), nil
}
