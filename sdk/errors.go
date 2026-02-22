package tusker

import "fmt"

// APIError is returned when the Tusker API responds with a non-success status.
type APIError struct {
	StatusCode int
	Message    string
}

func (e *APIError) Error() string {
	return fmt.Sprintf("tusker: HTTP %d: %s", e.StatusCode, e.Message)
}
