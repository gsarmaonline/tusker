package worker

import (
	"context"
	"encoding/json"
	"errors"
	"log"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/gsarma/tusker/internal/store"
)

// JobExecutor executes a single job by type and payload.
type JobExecutor interface {
	ExecuteJob(ctx context.Context, tenantID uuid.UUID, jobType string, payload json.RawMessage) error
}

// Worker polls the database for pending jobs and executes them concurrently.
type Worker struct {
	store       *store.Queries
	executor    JobExecutor
	concurrency int
}

func New(pool *pgxpool.Pool, executor JobExecutor, concurrency int) *Worker {
	return &Worker{
		store:       store.New(pool),
		executor:    executor,
		concurrency: concurrency,
	}
}

// Start spawns concurrency goroutines that each poll for jobs every 500ms.
// It blocks until ctx is cancelled.
func (w *Worker) Start(ctx context.Context) {
	for i := 0; i < w.concurrency; i++ {
		go w.loop(ctx)
	}
	<-ctx.Done()
}

func (w *Worker) loop(ctx context.Context) {
	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			w.processNext(ctx)
		}
	}
}

func (w *Worker) processNext(ctx context.Context) {
	job, err := w.store.ClaimNextJob(ctx)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return
		}
		log.Printf("worker: claim error: %v", err)
		return
	}

	execErr := w.executor.ExecuteJob(ctx, job.TenantID, job.JobType, json.RawMessage(job.Payload))

	now := time.Now()
	if execErr == nil {
		_, err = w.store.UpdateJobStatus(ctx, store.UpdateJobStatusParams{
			ID:          job.ID,
			Status:      "completed",
			Error:       pgtype.Text{Valid: false},
			CompletedAt: &now,
			RunAt:       job.RunAt,
		})
		if err != nil {
			log.Printf("worker: mark completed error: %v", err)
		}
		return
	}

	// Job failed — retry or mark as permanently failed.
	if job.Attempt < job.MaxAttempts {
		backoff := time.Duration(int64(1)<<uint(job.Attempt)) * 10 * time.Second
		runAt := now.Add(backoff)
		_, err = w.store.UpdateJobStatus(ctx, store.UpdateJobStatusParams{
			ID:          job.ID,
			Status:      "pending",
			Error:       pgtype.Text{String: execErr.Error(), Valid: true},
			CompletedAt: nil,
			RunAt:       runAt,
		})
	} else {
		_, err = w.store.UpdateJobStatus(ctx, store.UpdateJobStatusParams{
			ID:          job.ID,
			Status:      "failed",
			Error:       pgtype.Text{String: execErr.Error(), Valid: true},
			CompletedAt: nil,
			RunAt:       job.RunAt,
		})
	}
	if err != nil {
		log.Printf("worker: update status error: %v", err)
	}
}
