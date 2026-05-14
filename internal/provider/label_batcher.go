package provider

import (
	"context"
	"fmt"
	"time"

	"github.com/akamai/terraform-provider-guardicore-segmentation/internal/client"
)

type labelUpdateReq struct {
	id    string
	label *client.LabelUpdate
}

func NewLabelCreateBatcher(c *client.Client, tuning client.BatcherTuning) *Batcher[*client.LabelCreate, string] {
	return NewBatcher(BatcherConfig[*client.LabelCreate, string]{
		BatchSize:  tuning.BatchSize,
		FlushDelay: tuning.FlushDelay,
		ExecuteBatch: func(ctx context.Context, items []*client.LabelCreate) ([]string, error) {
			bulkItems := make([]client.LabelCreate, len(items))
			for i, item := range items {
				bulkItems[i] = *item
			}

			bulkResp, err := c.BulkCreateLabels(ctx, bulkItems)
			if err != nil {
				return nil, err
			}
			if len(bulkResp.Failed) > 0 {
				return nil, fmt.Errorf("bulk create labels failed: %d failed", len(bulkResp.Failed))
			}
			if len(bulkResp.Succeeded) != len(items) {
				return nil, fmt.Errorf("bulk create labels response mismatch: expected %d succeeded ids, got %d", len(items), len(bulkResp.Succeeded))
			}

			return bulkResp.Succeeded, nil
		},
		Publish: func(ctx context.Context) error { return nil },
	})
}

func NewLabelUpdateBatcher(c *client.Client, tuning client.BatcherTuning) *Batcher[labelUpdateReq, struct{}] {
	return NewBatcher(BatcherConfig[labelUpdateReq, struct{}]{
		BatchSize:  tuning.BatchSize,
		FlushDelay: tuning.FlushDelay,
		ExecuteBatch: func(ctx context.Context, items []labelUpdateReq) ([]struct{}, error) {
			for _, item := range items {
				if _, err := c.UpdateLabel(ctx, item.id, item.label); err != nil {
					return nil, fmt.Errorf("update label %s: %w", item.id, err)
				}
			}
			return make([]struct{}, len(items)), nil
		},
		Publish: func(ctx context.Context) error { return nil },
	})
}

func NewLabelDeleteBatcher(c *client.Client, tuning client.BatcherTuning) *Batcher[string, struct{}] {
	return NewBatcher(BatcherConfig[string, struct{}]{
		BatchSize:  tuning.BatchSize,
		FlushDelay: tuning.FlushDelay,
		ExecuteBatch: func(ctx context.Context, items []string) ([]struct{}, error) {
			bulkResp, err := c.BulkDeleteLabels(ctx, items)
			if err != nil {
				return nil, err
			}
			if len(bulkResp.Failed) > 0 {
				return nil, fmt.Errorf("bulk delete labels: %d failed", len(bulkResp.Failed))
			}

			for _, id := range items {
				label, err := c.GetLabel(ctx, id)
				if err != nil {
					return nil, fmt.Errorf("bulk delete labels verification failed for %q: %w", id, err)
				}
				if label != nil {
					return nil, fmt.Errorf("bulk delete labels verification failed: label %q still exists", id)
				}
			}

			return make([]struct{}, len(items)), nil
		},
		ShouldFallback: func(err error) bool { return true },
		ExecuteOne: func(ctx context.Context, item string) (struct{}, error) {
			if err := c.DeleteLabel(ctx, item); err != nil {
				return struct{}{}, err
			}
			time.Sleep(time.Second)
			label, err := c.GetLabel(ctx, item)
			if err != nil {
				return struct{}{}, fmt.Errorf("delete label verification for %q: %w", item, err)
			}
			if label == nil {
				return struct{}{}, nil
			}
			// Bulk delete may not remove labels with dynamic criteria;
			// fall back to the individual DELETE endpoint.
			if err := c.DeleteLabelByID(ctx, item); err != nil {
				return struct{}{}, err
			}
			return struct{}{}, nil
		},
		Publish: func(ctx context.Context) error { return nil },
	})
}
