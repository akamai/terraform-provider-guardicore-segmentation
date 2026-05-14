package provider

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/akamai/terraform-provider-guardicore-segmentation/internal/client"
	"github.com/hashicorp/terraform-plugin-log/tflog"
)

type policyRuleUpdateReq struct {
	id   string
	spec map[string]any
}

func NewPolicyRuleCreateBatcher(c *client.Client, tuning client.BatcherTuning) *Batcher[map[string]any, string] {
	return NewBatcher(BatcherConfig[map[string]any, string]{
		BatchSize:  tuning.BatchSize,
		FlushDelay: tuning.FlushDelay,
		ExecuteBatch: func(ctx context.Context, items []map[string]any) ([]string, error) {
			bulkResp, err := c.BulkCreatePolicyRules(ctx, items)
			if err != nil {
				return nil, err
			}
			if bulkResp.NumberOfFailed > 0 {
				return nil, fmt.Errorf("bulk policy rule create failed: %d of %d failed (%s)",
					bulkResp.NumberOfFailed, bulkResp.TotalNumber, bulkResp.Result)
			}
			if len(bulkResp.Succeeded) != len(items) {
				return nil, fmt.Errorf("bulk policy rule create response mismatch: expected %d succeeded ids, got %d",
					len(items), len(bulkResp.Succeeded))
			}
			return bulkResp.Succeeded, nil
		},
		Publish:        policyRulePublish(c),
		ShouldFallback: policyRuleShouldFallback,
		ExecuteOne: func(ctx context.Context, item map[string]any) (string, error) {
			return c.CreatePolicyRule(ctx, item)
		},
		IsPublishOK: policyRuleIsPublishOK,
		WarnLog:     policyRuleWarnLog,
	})
}

func NewPolicyRuleUpdateBatcher(c *client.Client, tuning client.BatcherTuning) *Batcher[policyRuleUpdateReq, struct{}] {
	return NewBatcher(BatcherConfig[policyRuleUpdateReq, struct{}]{
		BatchSize:  tuning.BatchSize,
		FlushDelay: tuning.FlushDelay,
		ExecuteBatch: func(ctx context.Context, items []policyRuleUpdateReq) ([]struct{}, error) {
			bulkItems := make([]client.PolicyRuleBulkUpdateItem, len(items))
			for i, item := range items {
				bulkItems[i] = client.PolicyRuleBulkUpdateItem{ID: item.id, Rule: item.spec}
			}
			bulkResp, err := c.BulkUpdatePolicyRules(ctx, bulkItems)
			if err != nil {
				return nil, err
			}
			if bulkResp.NumberOfFailed > 0 {
				return nil, fmt.Errorf("bulk policy rule update failed: %d of %d failed (%s)",
					bulkResp.NumberOfFailed, bulkResp.TotalNumber, bulkResp.Result)
			}
			return make([]struct{}, len(items)), nil
		},
		Publish:        policyRulePublish(c),
		ShouldFallback: policyRuleAlwaysFallback,
		ExecuteOne: func(ctx context.Context, item policyRuleUpdateReq) (struct{}, error) {
			return struct{}{}, c.UpdatePolicyRule(ctx, item.id, item.spec)
		},
		IsPublishOK: policyRuleIsPublishOK,
		WarnLog:     policyRuleWarnLog,
	})
}

func NewPolicyRuleDeleteBatcher(c *client.Client, tuning client.BatcherTuning) *Batcher[string, struct{}] {
	return NewBatcher(BatcherConfig[string, struct{}]{
		BatchSize:  tuning.BatchSize,
		FlushDelay: tuning.FlushDelay,
		ExecuteBatch: func(ctx context.Context, items []string) ([]struct{}, error) {
			bulkItems := make([]client.PolicyRuleBulkDeleteItem, len(items))
			for i, id := range items {
				bulkItems[i] = client.PolicyRuleBulkDeleteItem{ID: id}
			}
			bulkResp, err := c.BulkDeletePolicyRules(ctx, bulkItems)
			if err != nil {
				return nil, err
			}
			if bulkResp.NumberOfFailed > 0 {
				return nil, fmt.Errorf("bulk policy rule delete failed: %d of %d failed (%s)",
					bulkResp.NumberOfFailed, bulkResp.TotalNumber, bulkResp.Result)
			}
			return make([]struct{}, len(items)), nil
		},
		Publish:        policyRulePublish(c),
		IsPublishOK:    policyRuleIsPublishOK,
		WarnLog:        policyRuleWarnLog,
		ShouldFallback: policyRuleAlwaysFallback,
		ExecuteOne: func(ctx context.Context, id string) (struct{}, error) {
			return struct{}{}, c.DeletePolicyRule(ctx, id)
		},
	})
}

func policyRulePublish(c *client.Client) func(ctx context.Context) error {
	return func(ctx context.Context) error {
		revisionOrigin := "API_CALL"
		return c.CreatePolicyRevision(ctx, &client.PolicyRevisionRequest{
			Comments: "Published via Terraform",
			Rulesets: []string{},
			Origin:   &revisionOrigin,
		})
	}
}

func policyRuleShouldFallback(err error) bool {
	return strings.Contains(strings.ToLower(err.Error()), "unknown field")
}

// policyRuleAlwaysFallback falls back to individual calls whenever the bulk
// endpoint fails. Used for update/delete where individual calls are proven
// and bulk is an optimization.
func policyRuleAlwaysFallback(_ error) bool {
	return true
}

func policyRuleIsPublishOK(err error) bool {
	return errors.Is(err, client.ErrPolicyRevisionUnchanged)
}

func policyRuleWarnLog(msg string) {
	tflog.Warn(context.Background(), "policy rule publish: "+msg)
}
