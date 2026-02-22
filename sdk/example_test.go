package tusker_test

import (
	"context"
	"fmt"
	"log"

	tusker "github.com/gsarma/tusker/sdk"
)

func Example_basicUsage() {
	ctx := context.Background()

	// --- Provision a new tenant (one-time setup, no API key needed) ---
	provisioner := tusker.NewProvisioner("https://api.tusker.io")
	tenant, err := provisioner.CreateTenant(ctx)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println("Tenant ID:", tenant.TenantID)
	// Save tenant.APIKey securely — it is shown only once.

	// --- Create an authenticated client ---
	client := tusker.New("https://api.tusker.io", tenant.APIKey)

	// --- Configure SMTP email ---
	err = client.Email.SetConfig(ctx, "smtp", tusker.SMTPConfig{
		Host:     "smtp.example.com",
		Port:     587,
		Username: "user@example.com",
		Password: "secret",
	})
	if err != nil {
		log.Fatal(err)
	}

	// --- Send an email (async, returns a job ID) ---
	sendResp, err := client.Email.Send(ctx, "smtp", tusker.SendEmailRequest{
		To:      []string{"recipient@example.com"},
		From:    "no-reply@example.com",
		Subject: "Hello from Tusker",
		Body:    "This email was sent via the Tusker Go SDK.",
	}, nil)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println("Job ID:", sendResp.JobID)

	// --- Check job status ---
	job, err := client.Jobs.Get(ctx, sendResp.JobID)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println("Job status:", job.Status)
}

func Example_oauth() {
	ctx := context.Background()
	client := tusker.New("https://api.tusker.io", "your-api-key")

	// Configure OAuth credentials for Google
	err := client.OAuth.SetConfig(ctx, "google", tusker.SetOAuthConfigRequest{
		ClientID:     "your-google-client-id",
		ClientSecret: "your-google-client-secret",
	})
	if err != nil {
		log.Fatal(err)
	}

	// Build the authorization URL to redirect your user to
	authURL := client.OAuth.GetAuthorizeURL("google", "https://myapp.com/oauth/callback")
	fmt.Println("Redirect user to:", authURL)

	// After the user grants access, Tusker stores the token.
	// Retrieve it later by user ID:
	token, err := client.OAuth.GetToken(ctx, "google", "user-123")
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println("Access token:", token.AccessToken)
}

func Example_sms() {
	ctx := context.Background()
	client := tusker.New("https://api.tusker.io", "your-api-key")

	// Configure Twilio credentials
	err := client.SMS.SetConfig(ctx, "twilio", tusker.TwilioConfig{
		AccountSID: "ACxxxx",
		AuthToken:  "your-auth-token",
	})
	if err != nil {
		log.Fatal(err)
	}

	// Send an SMS synchronously
	resp, err := client.SMS.Send(ctx, "twilio", tusker.SendSMSRequest{
		From: "+15550001234",
		To:   "+15559876543",
		Body: "Your verification code is 123456",
	}, &tusker.SendOptions{Sync: true})
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println("Message SID:", resp.MessageSID)
}

func Example_emailTemplates() {
	ctx := context.Background()
	client := tusker.New("https://api.tusker.io", "your-api-key")

	// Create a custom template
	_, err := client.Email.CreateTemplate(ctx, tusker.CreateEmailTemplateRequest{
		Name:    "invite",
		Subject: "You're invited to {{.ServiceName}}",
		Body:    "Hi {{.UserName}}, you have been invited. Click here: {{.InviteLink}}",
	})
	if err != nil {
		log.Fatal(err)
	}

	// Send using the template
	_, err = client.Email.SendTemplate(ctx, "smtp", tusker.SendTemplateRequest{
		Template: "invite",
		To:       []string{"newuser@example.com"},
		From:     "noreply@example.com",
		Variables: map[string]string{
			"ServiceName": "Acme",
			"UserName":    "Alice",
			"InviteLink":  "https://acme.io/join/abc123",
		},
	})
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println("Template email sent")
}
