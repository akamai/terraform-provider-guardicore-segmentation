package asset

import (
	"context"
	"fmt"

	"github.com/akamai/terraform-provider-guardicore-segmentation/internal/client"
	"github.com/akamai/terraform-provider-guardicore-segmentation/internal/provider/common"
)

func NewAssetCreateBatcher(c *client.Client, tuning client.BatcherTuning) *common.Batcher[*client.AssetCreate, string] {
	return common.NewBatcher(common.BatcherConfig[*client.AssetCreate, string]{
		BatchSize:  tuning.BatchSize,
		FlushDelay: tuning.FlushDelay,
		ExecuteBatch: func(ctx context.Context, items []*client.AssetCreate) ([]string, error) {
			bulkItems := make([]client.AssetCreate, len(items))
			for i, item := range items {
				bulkItems[i] = *item
			}

			bulkResp, err := c.BulkCreateAssets(ctx, bulkItems)
			if err != nil {
				return nil, err
			}
			if bulkResp.NumberOfFailed > 0 {
				errMsg := "unknown error"
				if len(bulkResp.Errors) > 0 {
					errMsg = bulkResp.Errors[0].Error
				}
				return nil, fmt.Errorf("bulk create assets failed: %s", errMsg)
			}

			ids := make([]string, len(items))
			for i, item := range items {
				id, ok := bulkResp.CreatedAssetIDs[item.OrchestrationObjID]
				if !ok {
					return nil, fmt.Errorf("bulk create assets response missing asset id for orchestration_obj_id %q", item.OrchestrationObjID)
				}
				ids[i] = id
			}
			return ids, nil
		},
		Publish: common.NoPublish[*client.AssetCreate],
	})
}

func NewAssetUpdateBatcher(c *client.Client, tuning client.BatcherTuning) *common.Batcher[*client.AssetBulkUpdateItem, struct{}] {
	return common.NewBatcher(common.BatcherConfig[*client.AssetBulkUpdateItem, struct{}]{
		BatchSize:  tuning.BatchSize,
		FlushDelay: tuning.FlushDelay,
		ExecuteBatch: func(ctx context.Context, items []*client.AssetBulkUpdateItem) ([]struct{}, error) {
			bulkItems := make([]client.AssetBulkUpdateItem, len(items))
			for i, item := range items {
				bulkItems[i] = *item
			}

			bulkResp, err := c.BulkUpdateAssets(ctx, bulkItems)
			if err != nil {
				return nil, err
			}
			if bulkResp.NumberOfFailed > 0 {
				errMsg := "unknown error"
				if len(bulkResp.Errors) > 0 {
					errMsg = bulkResp.Errors[0].Error
				}
				return nil, fmt.Errorf("bulk update assets failed: %s", errMsg)
			}

			return make([]struct{}, len(items)), nil
		},
		Publish: common.NoPublish[*client.AssetBulkUpdateItem],
	})
}

func NewAssetDeleteBatcher(c *client.Client, tuning client.BatcherTuning) *common.Batcher[string, struct{}] {
	return common.NewBatcher(common.BatcherConfig[string, struct{}]{
		BatchSize:  tuning.BatchSize,
		FlushDelay: tuning.FlushDelay,
		ExecuteBatch: func(ctx context.Context, items []string) ([]struct{}, error) {
			if err := c.BulkDeactivateAssets(ctx, items); err != nil {
				return nil, err
			}
			return make([]struct{}, len(items)), nil
		},
		Publish: common.NoPublish[string],
	})
}
