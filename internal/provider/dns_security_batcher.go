package provider

import (
	"context"
	"fmt"

	"github.com/akamai/terraform-provider-guardicore-segmentation/internal/client"
)

type dnsSecurityUpdateReq struct {
	id   string
	edit *client.DnsBlocklistEdit
}

func NewDnsSecurityCreateBatcher(c *client.Client, tuning client.BatcherTuning) *Batcher[*client.DnsBlocklistCreate, string] {
	return NewBatcher(BatcherConfig[*client.DnsBlocklistCreate, string]{
		BatchSize:  tuning.BatchSize,
		FlushDelay: tuning.FlushDelay,
		ExecuteBatch: func(ctx context.Context, items []*client.DnsBlocklistCreate) ([]string, error) {
			bulkItems := make([]client.DnsBlocklistCreate, len(items))
			for i, item := range items {
				bulkItems[i] = *item
			}

			bulkResp, err := c.BulkCreateDnsBlocklists(ctx, bulkItems)
			if err != nil {
				return nil, err
			}
			if len(bulkResp.IDs) != len(items) {
				return nil, fmt.Errorf("bulk create dns blocklists response mismatch: expected %d ids, got %d", len(items), len(bulkResp.IDs))
			}
			return bulkResp.IDs, nil
		},
		ShouldFallback: func(err error) bool { return true },
		ExecuteOne: func(ctx context.Context, item *client.DnsBlocklistCreate) (string, error) {
			return c.CreateDnsBlocklist(ctx, item)
		},
		Publish: func(ctx context.Context) error { return nil },
	})
}

func NewDnsSecurityUpdateBatcher(c *client.Client, tuning client.BatcherTuning) *Batcher[dnsSecurityUpdateReq, struct{}] {
	return NewBatcher(BatcherConfig[dnsSecurityUpdateReq, struct{}]{
		BatchSize:  tuning.BatchSize,
		FlushDelay: tuning.FlushDelay,
		ExecuteBatch: func(ctx context.Context, items []dnsSecurityUpdateReq) ([]struct{}, error) {
			bulkItems := make([]client.BulkEditDnsBlocklistItem, len(items))
			for i, item := range items {
				bulkItems[i] = client.BulkEditDnsBlocklistItem{
					ID:      item.id,
					Name:    item.edit.Name,
					Domains: item.edit.Domains,
					Enabled: item.edit.Enabled,
				}
			}

			if err := c.BulkEditDnsBlocklists(ctx, &client.BulkEditDnsBlocklistRequest{Items: bulkItems}); err != nil {
				return nil, err
			}
			return make([]struct{}, len(items)), nil
		},
		ShouldFallback: func(err error) bool { return true },
		ExecuteOne: func(ctx context.Context, item dnsSecurityUpdateReq) (struct{}, error) {
			return struct{}{}, c.UpdateDnsBlocklist(ctx, item.id, item.edit)
		},
		Publish: func(ctx context.Context) error { return nil },
	})
}

func NewDnsSecurityDeleteBatcher(c *client.Client, tuning client.BatcherTuning) *Batcher[string, struct{}] {
	return NewBatcher(BatcherConfig[string, struct{}]{
		BatchSize:  tuning.BatchSize,
		FlushDelay: tuning.FlushDelay,
		ExecuteBatch: func(ctx context.Context, items []string) ([]struct{}, error) {
			if err := c.BulkDeleteDnsBlocklists(ctx, items); err != nil {
				return nil, err
			}
			return make([]struct{}, len(items)), nil
		},
		ShouldFallback: func(err error) bool { return true },
		ExecuteOne: func(ctx context.Context, item string) (struct{}, error) {
			return struct{}{}, c.DeleteDnsBlocklist(ctx, item)
		},
		Publish: func(ctx context.Context) error { return nil },
	})
}
