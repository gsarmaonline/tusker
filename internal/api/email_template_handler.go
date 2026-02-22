package api

import (
	"context"
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5"

	"github.com/gsarma/tusker/internal/email"
	"github.com/gsarma/tusker/internal/store"
	"github.com/gsarma/tusker/internal/tenant"
)

// UpsertEmailTemplate creates or replaces a named email template for the tenant.
func (h *Handler) UpsertEmailTemplate(c *gin.Context) {
	t := tenant.FromContext(c)

	var body struct {
		Name    string `json:"name" binding:"required"`
		Subject string `json:"subject" binding:"required"`
		Body    string `json:"body" binding:"required"`
		HTML    string `json:"html"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	def := email.TemplateDefinition{
		Subject: body.Subject,
		Body:    body.Body,
		HTML:    body.HTML,
	}
	if err := email.ValidateTemplate(def); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	row, err := h.queries.UpsertEmailTemplate(c.Request.Context(), store.UpsertEmailTemplateParams{
		TenantID: t.ID,
		Name:     body.Name,
		Subject:  body.Subject,
		Body:     body.Body,
		Html:     body.HTML,
	})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to save template"})
		return
	}

	c.JSON(http.StatusOK, row)
}

// ListEmailTemplates returns all templates for the tenant: custom DB rows merged
// with built-in defaults. Each entry includes a "custom" boolean flag.
func (h *Handler) ListEmailTemplates(c *gin.Context) {
	t := tenant.FromContext(c)

	rows, err := h.queries.ListEmailTemplates(c.Request.Context(), t.ID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to list templates"})
		return
	}

	// Index custom templates by name for quick lookup.
	customByName := make(map[string]store.EmailTemplate, len(rows))
	for _, r := range rows {
		customByName[r.Name] = r
	}

	type templateEntry struct {
		Name    string `json:"name"`
		Subject string `json:"subject"`
		Body    string `json:"body"`
		HTML    string `json:"html"`
		Custom  bool   `json:"custom"`
	}

	// Start with defaults, then override/add with custom entries.
	nameOrder := make([]string, 0)
	seen := make(map[string]bool)

	for name := range email.DefaultTemplates {
		nameOrder = append(nameOrder, name)
		seen[name] = true
	}
	for _, r := range rows {
		if !seen[r.Name] {
			nameOrder = append(nameOrder, r.Name)
		}
	}

	result := make([]templateEntry, 0, len(nameOrder))
	for _, name := range nameOrder {
		if custom, ok := customByName[name]; ok {
			result = append(result, templateEntry{
				Name:    custom.Name,
				Subject: custom.Subject,
				Body:    custom.Body,
				HTML:    custom.Html,
				Custom:  true,
			})
		} else if def, ok := email.DefaultTemplates[name]; ok {
			result = append(result, templateEntry{
				Name:    name,
				Subject: def.Subject,
				Body:    def.Body,
				HTML:    def.HTML,
				Custom:  false,
			})
		}
	}

	c.JSON(http.StatusOK, result)
}

// DeleteEmailTemplate removes a custom template, reverting to the built-in default
// if one exists.
func (h *Handler) DeleteEmailTemplate(c *gin.Context) {
	t := tenant.FromContext(c)
	name := c.Param("name")

	err := h.queries.DeleteEmailTemplate(c.Request.Context(), store.DeleteEmailTemplateParams{
		TenantID: t.ID,
		Name:     name,
	})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to delete template"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"status": "deleted"})
}

// SendEmailWithTemplate resolves a named template, renders it with the provided
// variables, then sends it via the specified email provider.
func (h *Handler) SendEmailWithTemplate(c *gin.Context) {
	t := tenant.FromContext(c)
	providerName := c.Param("provider")

	var body struct {
		Template  string         `json:"template" binding:"required"`
		To        []string       `json:"to" binding:"required"`
		From      string         `json:"from" binding:"required"`
		Variables map[string]any `json:"variables"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	def, err := h.resolveTemplate(c.Request.Context(), t, body.Template)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "template not found: " + body.Template})
		return
	}

	rendered, err := email.RenderTemplate(def, body.Variables)
	if err != nil {
		c.JSON(http.StatusUnprocessableEntity, gin.H{"error": "template render error: " + err.Error()})
		return
	}

	p, err := h.buildEmailProvider(c.Request.Context(), t, providerName)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	msg := email.Message{
		To:      body.To,
		From:    body.From,
		Subject: rendered.Subject,
		Body:    rendered.Body,
		HTML:    rendered.HTML != "",
	}
	if rendered.HTML != "" {
		msg.Body = rendered.HTML
	}

	if err := p.Send(c.Request.Context(), msg); err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": "failed to send email"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"status": "sent"})
}

// resolveTemplate looks up a template by name: DB first, then built-in defaults.
func (h *Handler) resolveTemplate(ctx context.Context, t *store.Tenant, name string) (email.TemplateDefinition, error) {
	row, err := h.queries.GetEmailTemplate(ctx, store.GetEmailTemplateParams{
		TenantID: t.ID,
		Name:     name,
	})
	if err == nil {
		return email.TemplateDefinition{
			Subject: row.Subject,
			Body:    row.Body,
			HTML:    row.Html,
		}, nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return email.TemplateDefinition{}, err
	}

	if def, ok := email.DefaultTemplates[name]; ok {
		return def, nil
	}

	return email.TemplateDefinition{}, pgx.ErrNoRows
}
