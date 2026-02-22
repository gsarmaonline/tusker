package email_test

import (
	"encoding/json"
	"reflect"
	"testing"

	"github.com/gsarma/tusker/internal/email"
)

func TestJobPayload_JSONRoundTrip(t *testing.T) {
	original := email.JobPayload{
		Provider: "smtp",
		Message: email.Message{
			To:      []string{"alice@example.com", "bob@example.com"},
			From:    "noreply@myapp.com",
			Subject: "Hello",
			Body:    "<h1>Hi</h1>",
			HTML:    true,
		},
	}

	data, err := json.Marshal(original)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	var got email.JobPayload
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if !reflect.DeepEqual(original, got) {
		t.Errorf("round-trip mismatch:\n  want %+v\n  got  %+v", original, got)
	}
}

func TestJobPayload_JSONKeys(t *testing.T) {
	// Verify the JSON key names match what the worker expects when deserialising.
	p := email.JobPayload{
		Provider: "sendgrid",
		Message: email.Message{
			To:      []string{"x@y.com"},
			From:    "a@b.com",
			Subject: "Sub",
			Body:    "Body text",
			HTML:    false,
		},
	}
	data, _ := json.Marshal(p)

	var raw map[string]interface{}
	json.Unmarshal(data, &raw)

	for _, key := range []string{"provider", "to", "from", "subject", "body", "html"} {
		if _, ok := raw[key]; !ok {
			t.Errorf("expected JSON key %q to be present", key)
		}
	}
}

func TestMessage_HTMLDefault(t *testing.T) {
	data := []byte(`{"to":["a@b.com"],"from":"x@y.com","subject":"s","body":"b"}`)
	var msg email.Message
	if err := json.Unmarshal(data, &msg); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if msg.HTML {
		t.Error("HTML field should default to false when absent from JSON")
	}
}
