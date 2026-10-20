package policyrule

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"

	"github.com/akamai/terraform-provider-guardicore-segmentation/internal/client"
	"github.com/akamai/terraform-provider-guardicore-segmentation/internal/provider/common"
	"github.com/hashicorp/terraform-plugin-log/tflog"
)

func NewPolicyRuleCreateBatcher(c *client.Client, tuning client.BatcherTuning) *common.Batcher[map[string]any, string] {
	return common.NewBatcher(common.BatcherConfig[map[string]any, string]{
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
		Publish:        policyRulePublish(c, rulesetFromSpec),
		ShouldFallback: policyRuleShouldFallback,
		ExecuteOne: func(ctx context.Context, item map[string]any) (string, error) {
			return c.CreatePolicyRule(ctx, item)
		},
		IsPublishOK: policyRuleIsPublishOK,
		WarnLog:     policyRuleWarnLog,
	})
}

func NewPolicyRuleUpdateBatcher(c *client.Client, tuning client.BatcherTuning) *common.Batcher[common.PolicyRuleUpdateReq, struct{}] {
	return common.NewBatcher(common.BatcherConfig[common.PolicyRuleUpdateReq, struct{}]{
		BatchSize:  tuning.BatchSize,
		FlushDelay: tuning.FlushDelay,
		ExecuteBatch: func(ctx context.Context, items []common.PolicyRuleUpdateReq) ([]struct{}, error) {
			bulkItems := make([]client.PolicyRuleBulkUpdateItem, len(items))
			for i, item := range items {
				bulkItems[i] = client.PolicyRuleBulkUpdateItem{ID: item.ID, Rule: item.Spec}
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
		Publish: policyRulePublish(c, func(item common.PolicyRuleUpdateReq) string {
			return rulesetFromSpec(item.Spec)
		}),
		ShouldFallback: policyRuleAlwaysFallback,
		ExecuteOne: func(ctx context.Context, item common.PolicyRuleUpdateReq) (struct{}, error) {
			return struct{}{}, c.UpdatePolicyRule(ctx, item.ID, item.Spec)
		},
		IsPublishOK: policyRuleIsPublishOK,
		WarnLog:     policyRuleWarnLog,
	})
}

func NewPolicyRuleDeleteBatcher(c *client.Client, tuning client.BatcherTuning) *common.Batcher[common.PolicyRuleDeleteReq, struct{}] {
	return common.NewBatcher(common.BatcherConfig[common.PolicyRuleDeleteReq, struct{}]{
		BatchSize:  tuning.BatchSize,
		FlushDelay: tuning.FlushDelay,
		ExecuteBatch: func(ctx context.Context, items []common.PolicyRuleDeleteReq) ([]struct{}, error) {
			bulkItems := make([]client.PolicyRuleBulkDeleteItem, len(items))
			for i, item := range items {
				bulkItems[i] = client.PolicyRuleBulkDeleteItem{ID: item.ID}
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
		Publish: policyRulePublish(c, func(item common.PolicyRuleDeleteReq) string {
			return item.RulesetName
		}),
		IsPublishOK:    policyRuleIsPublishOK,
		WarnLog:        policyRuleWarnLog,
		ShouldFallback: policyRuleAlwaysFallback,
		ExecuteOne: func(ctx context.Context, item common.PolicyRuleDeleteReq) (struct{}, error) {
			return struct{}{}, c.DeletePolicyRule(ctx, item.ID)
		},
	})
}

// undefinedRuleset is the sentinel the API uses for rules that belong to no
// ruleset (NO_RULESETS_ID server-side). Sending it scopes the revision to
// exactly those rules, rather than leaving the scope empty.
const undefinedRuleset = "undefined"

// policyRulePublish publishes a revision scoped to the rulesets the batch
// touched, derived from the batch items via rulesetOf.
//
// An unscoped revision (no ruleset_name and no rulesets) means "publish every
// pending change" to the API, which would push unrelated drafts — including
// ones made in the UI by other users — live without review. Scoping the
// request keeps the revision limited to the rules Terraform actually manages.
func policyRulePublish[Req any](c *client.Client, rulesetOf func(Req) string) func(context.Context, []Req) error {
	return func(ctx context.Context, items []Req) error {
		rulesets := collectRulesets(items, rulesetOf)
		if len(rulesets) == 0 {
			// Only reachable for an empty batch. Publishing unscoped here
			// would publish everything pending, so do nothing instead.
			policyRuleWarnLog("no rulesets derived from batch; skipping publish")
			return nil
		}

		if includesUnassigned(rulesets) {
			// ruleset_name is optional on the resource, so a managed rule can
			// land in the unassigned bucket. The API expands that sentinel to
			// every pending rule with no ruleset, which in most deployments is
			// where ad-hoc rules created outside Terraform collect.
			tflog.Warn(ctx, "policy revision scope includes rules with no ruleset_name. "+
				"This publishes every pending unassigned rule, including any created outside Terraform. "+
				"Set ruleset_name on all Terraform-managed policy rules to keep the scope narrow.",
				map[string]any{"rulesets": rulesets})
		}

		revisionOrigin := "API_CALL"
		return c.CreatePolicyRevision(ctx, &client.PolicyRevisionRequest{
			Comments: "Published via Terraform",
			Rulesets: rulesets,
			Origin:   &revisionOrigin,
		})
	}
}

// includesUnassigned reports whether the publish scope covers rules with no
// ruleset. That bucket is shared with every other unassigned rule in the
// system, so it is materially wider than a named ruleset.
func includesUnassigned(rulesets []string) bool {
	for _, name := range rulesets {
		if name == undefinedRuleset {
			return true
		}
	}
	return false
}

// collectRulesets returns the deduplicated, sorted set of ruleset names the
// batch touched. Rules with no ruleset map to undefinedRuleset, so a non-empty
// batch always yields a non-empty scope. The result is sorted to keep requests
// deterministic for logging and tests.
func collectRulesets[Req any](items []Req, rulesetOf func(Req) string) []string {
	seen := make(map[string]struct{}, len(items))
	rulesets := make([]string, 0, len(items))

	for _, item := range items {
		name := rulesetOf(item)
		if name == "" {
			name = undefinedRuleset
		}
		if _, duplicate := seen[name]; duplicate {
			continue
		}
		seen[name] = struct{}{}
		rulesets = append(rulesets, name)
	}

	sort.Strings(rulesets)
	return rulesets
}

// rulesetFromSpec reads the ruleset name out of a policy rule request body,
// tolerating both an absent key and a non-string value.
func rulesetFromSpec(spec map[string]any) string {
	name, _ := spec["ruleset_name"].(string)
	return name
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
