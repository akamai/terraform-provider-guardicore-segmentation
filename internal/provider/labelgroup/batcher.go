package labelgroup

import (
	"context"

	"github.com/akamai/terraform-provider-guardicore-segmentation/internal/client"
	"github.com/akamai/terraform-provider-guardicore-segmentation/internal/provider/common"
	"github.com/hashicorp/terraform-plugin-log/tflog"
)

func NewLabelGroupCreateBatcher(c *client.Client, tuning client.BatcherTuning) *common.Batcher[*client.LabelGroupCreate, *client.LabelGroupCreate] {
	return common.NewBatcher(common.BatcherConfig[*client.LabelGroupCreate, *client.LabelGroupCreate]{
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
		Publish: labelGroupPublish[*client.LabelGroupCreate](c),
		WarnLog: labelGroupWarnLog,
	})
}

func NewLabelGroupUpdateBatcher(c *client.Client, tuning client.BatcherTuning) *common.Batcher[common.LabelGroupUpdateReq, *client.LabelGroupCreate] {
	return common.NewBatcher(common.BatcherConfig[common.LabelGroupUpdateReq, *client.LabelGroupCreate]{
		BatchSize:  tuning.BatchSize,
		FlushDelay: tuning.FlushDelay,
		ExecuteBatch: func(ctx context.Context, items []common.LabelGroupUpdateReq) ([]*client.LabelGroupCreate, error) {
			results := make([]*client.LabelGroupCreate, len(items))
			for i, item := range items {
				updated, err := c.UpdateLabelGroup(ctx, item.ID, item.LabelGroup)
				if err != nil {
					return nil, err
				}
				results[i] = updated
			}
			return results, nil
		},
		Publish: labelGroupPublish[common.LabelGroupUpdateReq](c),
		WarnLog: labelGroupWarnLog,
	})
}

func NewLabelGroupDeleteBatcher(c *client.Client, tuning client.BatcherTuning) *common.Batcher[string, struct{}] {
	return common.NewBatcher(common.BatcherConfig[string, struct{}]{
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
		Publish: labelGroupPublish[string](c),
		WarnLog: labelGroupWarnLog,
	})
}

// labelGroupPublish publishes label group changes. Label groups have no
// per-object publish scope, so the batch items are not inspected.
func labelGroupPublish[Req any](c *client.Client) func(context.Context, []Req) error {
	return func(ctx context.Context, _ []Req) error {
		return c.PublishLabelGroups(ctx)
	}
}

func labelGroupWarnLog(msg string) {
	tflog.Warn(context.Background(), "label group publish: "+msg)
}
