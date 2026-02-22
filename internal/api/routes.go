package api

import (
	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/gsarma/tusker/internal/crypto"
	"github.com/gsarma/tusker/internal/store"
	"github.com/gsarma/tusker/internal/tenant"
)

func RegisterRoutes(r *gin.Engine, db *pgxpool.Pool, enc *crypto.Encryptor) {
	tenantSvc := tenant.NewService(db, enc)
	queries := store.New(db)
	h := &Handler{
		queries:   queries,
		tenantSvc: tenantSvc,
		enc:       enc,
	}

	// Tenant provisioning (would be admin-gated in production)
	r.POST("/tenants", h.CreateTenant)

	// Authenticated routes
	authed := r.Group("/", tenantSvc.AuthMiddleware())
	{
		authed.POST("/oauth/:provider/config", h.SetProviderConfig)
		authed.GET("/oauth/:provider/authorize", h.Authorize)
		authed.GET("/oauth/:provider/token", h.GetToken)
		authed.DELETE("/oauth/:provider/token", h.DeleteToken)

		authed.POST("/email/:provider/config", h.SetEmailProviderConfig)
		authed.POST("/email/:provider/send", h.SendEmail)
		authed.POST("/sms/:provider/config", h.SetSMSProviderConfig)
		authed.POST("/sms/:provider/send", h.SendSMS)

		authed.POST("/email/templates", h.UpsertEmailTemplate)
		authed.GET("/email/templates", h.ListEmailTemplates)
		authed.DELETE("/email/templates/:name", h.DeleteEmailTemplate)
		authed.POST("/email/:provider/send-template", h.SendEmailWithTemplate)
	}

	// Callback is called by the provider — no tenant auth header, tenant from state param
	r.GET("/oauth/:provider/callback", h.Callback)
}
