package provider

import (
	"context"

	"github.com/akamai/terraform-provider-guardicore-segmentation/internal/client"
	"github.com/hashicorp/terraform-plugin-log/tflog"
)

type labelGroupUpdateReq struct {
	id         string
	labelGroup *client.LabelGroupCreate
}

func NewLabelGroupCreateBatcher(c *client.Client, tuning client.BatcherTuning) *Batcher[*client.LabelGroupCreate, *client.LabelGroupCreate] {
	return NewBatcher(BatcherConfig[*client.LabelGroupCreate, *client.LabelGroupCreate]{
		BatchSize:  tuning.BatchSize,
		FlushDelay: tuning.FlushDelay,
		ExecuteBatch: func(ctx context.Context, items []*client.LabelGroupCreate) ([]*client.LabelGroupCreate, error) {
			results := make([]*client.LabelGroupCreate, len(items))
			for i, item := range items {
				created, err := c.CreateLabelGroup(ctx, item)
				if err != nil {
					return nil, err
				}
				results[i] = created
			}
			return results, nil
		},
		Publish: func(ctx context.Context) error {
			return c.PublishLabelGroups(ctx)
		},
		WarnLog: labelGroupWarnLog,
	})
}

func NewLabelGroupUpdateBatcher(c *client.Client, tuning client.BatcherTuning) *Batcher[labelGroupUpdateReq, *client.LabelGroupCreate] {
	return NewBatcher(BatcherConfig[labelGroupUpdateReq, *client.LabelGroupCreate]{
		BatchSize:  tuning.BatchSize,
		FlushDelay: tuning.FlushDelay,
		ExecuteBatch: func(ctx context.Context, items []labelGroupUpdateReq) ([]*client.LabelGroupCreate, error) {
			results := make([]*client.LabelGroupCreate, len(items))
			for i, item := range items {
				updated, err := c.UpdateLabelGroup(ctx, item.id, item.labelGroup)
				if err != nil {
					return nil, err
				}
				results[i] = updated
			}
			return results, nil
		},
		Publish: func(ctx context.Context) error {
			return c.PublishLabelGroups(ctx)
		},
		WarnLog: labelGroupWarnLog,
	})
}

func NewLabelGroupDeleteBatcher(c *client.Client, tuning client.BatcherTuning) *Batcher[string, struct{}] {
	return NewBatcher(BatcherConfig[string, struct{}]{
		BatchSize:  tuning.BatchSize,
		FlushDelay: tuning.FlushDelay,
		ExecuteBatch: func(ctx context.Context, items []string) ([]struct{}, error) {
			results := make([]struct{}, len(items))
			for _, id := range items {
				if err := c.DeleteLabelGroup(ctx, id); err != nil {
					return nil, err
				}
			}
			return results, nil
		},
		Publish: func(ctx context.Context) error {
			return c.PublishLabelGroups(ctx)
		},
		WarnLog: labelGroupWarnLog,
	})
}

func labelGroupWarnLog(msg string) {
	tflog.Warn(context.Background(), "label group publish: "+msg)
}
