package incident

import (
	"context"
	"fmt"

	"github.com/akamai/terraform-provider-guardicore-segmentation/internal/client"
	"github.com/akamai/terraform-provider-guardicore-segmentation/internal/provider/common"
)

func NewIncidentCreateBatcher(c *client.Client, tuning client.BatcherTuning) *common.Batcher[*client.IncidentCreate, string] {
	return common.NewBatcher(common.BatcherConfig[*client.IncidentCreate, string]{
		BatchSize:  tuning.BatchSize,
		FlushDelay: tuning.FlushDelay,
		ExecuteBatch: func(ctx context.Context, items []*client.IncidentCreate) ([]string, error) {
			bulkItems := make([]client.IncidentCreate, len(items))
			for i, item := range items {
				bulkItems[i] = *item
			}

			bulkResp, err := c.BulkCreateIncidents(ctx, bulkItems)
			if err != nil {
				return nil, err
			}
			if len(bulkResp.IncidentIDs) != len(items) {
				return nil, fmt.Errorf("bulk create incidents response mismatch: expected %d ids, got %d", len(items), len(bulkResp.IncidentIDs))
			}
			return bulkResp.IncidentIDs, nil
		},
		ShouldFallback: func(err error) bool { return true },
		ExecuteOne: func(ctx context.Context, item *client.IncidentCreate) (string, error) {
			return c.CreateIncident(ctx, item)
		},
		Publish: common.NoPublish[*client.IncidentCreate],
	})
}
