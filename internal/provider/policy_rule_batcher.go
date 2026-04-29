package provider

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/akamai/terraform-provider-guardicore-segmentation/internal/client"
)

const policyRuleBatchFlushDelay = 100 * time.Millisecond

type policyRuleCreateRequest struct {
	spec   map[string]any
	result chan policyRuleCreateResult
}

type policyRuleCreateResult struct {
	id  string
	err error
}

// PolicyRuleCreateBatcher batches policy rule create requests and publishes
// one policy revision per flushed batch.
type PolicyRuleCreateBatcher struct {
	client *client.Client

	mu     sync.Mutex
	queue  []policyRuleCreateRequest
	timer  *time.Timer
	timerC <-chan time.Time
}

func NewPolicyRuleCreateBatcher(c *client.Client) *PolicyRuleCreateBatcher {
	return &PolicyRuleCreateBatcher{client: c}
}

func (b *PolicyRuleCreateBatcher) EnqueueCreate(ctx context.Context, spec map[string]any) (string, error) {
	resCh := make(chan policyRuleCreateResult, 1)

	b.mu.Lock()
	b.queue = append(b.queue, policyRuleCreateRequest{spec: spec, result: resCh})
	if b.timer == nil {
		b.timer = time.NewTimer(policyRuleBatchFlushDelay)
		b.timerC = b.timer.C
		go b.waitAndFlush()
	}
	b.mu.Unlock()

	select {
	case <-ctx.Done():
		return "", ctx.Err()
	case result := <-resCh:
		return result.id, result.err
	}
}

func (b *PolicyRuleCreateBatcher) waitAndFlush() {
	<-b.timerC
	b.flush()
}

func (b *PolicyRuleCreateBatcher) flush() {
	b.mu.Lock()
	batch := b.queue
	b.queue = nil
	b.timer = nil
	b.timerC = nil
	b.mu.Unlock()

	if len(batch) == 0 {
		return
	}

	specs := make([]map[string]any, 0, len(batch))
	for _, req := range batch {
		specs = append(specs, req.spec)
	}

	bulkResp, err := b.client.BulkCreatePolicyRules(context.Background(), specs)
	if err != nil {
		if b.shouldFallbackToSingleCreate(err) {
			ids, singleErr := b.createPolicyRulesIndividually(batch)
			if singleErr != nil {
				b.failBatch(batch, singleErr)
				return
			}
			if err := b.publishBatch(); err != nil {
				b.failBatch(batch, fmt.Errorf("failed to publish policy rule batch: %w", err))
				return
			}
			for i, req := range batch {
				req.result <- policyRuleCreateResult{id: ids[i]}
			}
			return
		}
		b.failBatch(batch, err)
		return
	}

	if bulkResp.NumberOfFailed > 0 {
		b.failBatch(batch, fmt.Errorf("bulk policy rule create failed: %d of %d failed (%s)", bulkResp.NumberOfFailed, bulkResp.TotalNumber, bulkResp.Result))
		return
	}

	if len(bulkResp.Succeeded) != len(batch) {
		b.failBatch(batch, fmt.Errorf("bulk policy rule create response mismatch: expected %d succeeded ids, got %d", len(batch), len(bulkResp.Succeeded)))
		return
	}

	if err := b.publishBatch(); err != nil {
		b.failBatch(batch, fmt.Errorf("failed to publish policy rule batch: %w", err))
		return
	}

	for i, req := range batch {
		req.result <- policyRuleCreateResult{id: bulkResp.Succeeded[i]}
	}
}

func (b *PolicyRuleCreateBatcher) failBatch(batch []policyRuleCreateRequest, err error) {
	for _, req := range batch {
		req.result <- policyRuleCreateResult{err: err}
	}
}

func (b *PolicyRuleCreateBatcher) shouldFallbackToSingleCreate(err error) bool {
	errMsg := strings.ToLower(err.Error())
	return strings.Contains(errMsg, "unknown field")
}

func (b *PolicyRuleCreateBatcher) createPolicyRulesIndividually(batch []policyRuleCreateRequest) ([]string, error) {
	ids := make([]string, 0, len(batch))
	for _, req := range batch {
		id, err := b.client.CreatePolicyRule(context.Background(), req.spec)
		if err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	return ids, nil
}

func (b *PolicyRuleCreateBatcher) publishBatch() error {
	revisionOrigin := "API_CALL"
	revisionRequest := &client.PolicyRevisionRequest{
		Comments: "Published via Terraform",
		Rulesets: []string{},
		Origin:   &revisionOrigin,
	}
	return b.client.CreatePolicyRevision(context.Background(), revisionRequest)
}
