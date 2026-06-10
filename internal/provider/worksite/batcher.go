package worksite

import (
	"context"

	"github.com/akamai/terraform-provider-guardicore-segmentation/internal/client"
	"github.com/akamai/terraform-provider-guardicore-segmentation/internal/provider/common"
)

func NewWorksiteDeleteBatcher(c *client.Client, tuning client.BatcherTuning) *common.Batcher[string, struct{}] {
	return common.NewBatcher(common.BatcherConfig[string, struct{}]{
		BatchSize:  tuning.BatchSize,
		FlushDelay: tuning.FlushDelay,
		ExecuteBatch: func(ctx context.Context, items []string) ([]struct{}, error) {
			if err := c.BulkDeleteWorksites(ctx, items); err != nil {
				return nil, err
			}
			return make([]struct{}, len(items)), nil
		},
		Publish: func(ctx context.Context) error { return nil },
	})
}
