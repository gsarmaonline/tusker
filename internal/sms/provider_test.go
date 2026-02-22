package sms_test

import (
	"encoding/json"
	"reflect"
	"testing"

	"github.com/gsarma/tusker/internal/sms"
)

func TestJobPayload_JSONRoundTrip(t *testing.T) {
	original := sms.JobPayload{
		Provider: "twilio",
		From:     "+15550001111",
		To:       "+15559998888",
		Body:     "Your code is 123456",
	}

	data, err := json.Marshal(original)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	var got sms.JobPayload
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if !reflect.DeepEqual(original, got) {
		t.Errorf("round-trip mismatch:\n  want %+v\n  got  %+v", original, got)
	}
}

func TestJobPayload_JSONKeys(t *testing.T) {
	p := sms.JobPayload{Provider: "twilio", From: "+1", To: "+2", Body: "hi"}
	data, _ := json.Marshal(p)

	var raw map[string]interface{}
	json.Unmarshal(data, &raw)

	for _, key := range []string{"provider", "from", "to", "body"} {
		if _, ok := raw[key]; !ok {
			t.Errorf("expected JSON key %q to be present", key)
		}
	}
}
