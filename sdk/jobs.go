package tusker

import (
	"context"
	"fmt"
	"net/http"
)

// JobsService provides job status lookup operations.
type JobsService struct {
	c *Client
}

// Get retrieves the current status and metadata for a background job.
// jobID is the UUID returned when a send operation is queued asynchronously.
func (s *JobsService) Get(ctx context.Context, jobID string) (*Job, error) {
	path := fmt.Sprintf("/jobs/%s", jobID)
	return doRequest[Job](ctx, s.c, http.MethodGet, path, nil, http.StatusOK)
}
