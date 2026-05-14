package provider

import (
	"context"
	"errors"

	"github.com/akamai/terraform-provider-guardicore-segmentation/internal/client"
	"github.com/hashicorp/terraform-plugin-log/tflog"
)

type userGroupUpdateReq struct {
	id        string
	userGroup *client.UserGroupCreate
}

func NewUserGroupCreateBatcher(c *client.Client, tuning client.BatcherTuning) *Batcher[*client.UserGroupCreate, string] {
	return NewBatcher(BatcherConfig[*client.UserGroupCreate, string]{
		BatchSize:  tuning.BatchSize,
		FlushDelay: tuning.FlushDelay,
		ExecuteBatch: func(ctx context.Context, items []*client.UserGroupCreate) ([]string, error) {
			results := make([]string, len(items))
			for i, item := range items {
				id, err := c.CreateUserGroup(ctx, item)
				if err != nil {
					return nil, err
				}
				results[i] = id
			}
			return results, nil
		},
		Publish:     userGroupPublish(c),
		IsPublishOK: userGroupIsPublishOK,
		WarnLog:     userGroupWarnLog,
	})
}

func NewUserGroupUpdateBatcher(c *client.Client, tuning client.BatcherTuning) *Batcher[userGroupUpdateReq, struct{}] {
	return NewBatcher(BatcherConfig[userGroupUpdateReq, struct{}]{
		BatchSize:  tuning.BatchSize,
		FlushDelay: tuning.FlushDelay,
		ExecuteBatch: func(ctx context.Context, items []userGroupUpdateReq) ([]struct{}, error) {
			results := make([]struct{}, len(items))
			for _, item := range items {
				if err := c.UpdateUserGroup(ctx, item.id, item.userGroup); err != nil {
					return nil, err
				}
			}
			return results, nil
		},
		Publish:     userGroupPublish(c),
		IsPublishOK: userGroupIsPublishOK,
		WarnLog:     userGroupWarnLog,
	})
}

func NewUserGroupDeleteBatcher(c *client.Client, tuning client.BatcherTuning) *Batcher[string, struct{}] {
	return NewBatcher(BatcherConfig[string, struct{}]{
		BatchSize:  tuning.BatchSize,
		FlushDelay: tuning.FlushDelay,
		ExecuteBatch: func(ctx context.Context, items []string) ([]struct{}, error) {
			results := make([]struct{}, len(items))
			for _, id := range items {
				if err := c.DeleteUserGroup(ctx, id); err != nil {
					return nil, err
				}
			}
			return results, nil
		},
		Publish:     userGroupPublish(c),
		IsPublishOK: userGroupIsPublishOK,
		WarnLog:     userGroupWarnLog,
	})
}

func userGroupPublish(c *client.Client) func(ctx context.Context) error {
	return func(ctx context.Context) error {
		return c.CreateUserGroupRevision(ctx, &client.UserGroupRevisionRequest{
			Comments: "Published via Terraform",
		})
	}
}

func userGroupIsPublishOK(err error) bool {
	return errors.Is(err, client.ErrUserGroupRevisionUnchanged)
}

func userGroupWarnLog(msg string) {
	tflog.Warn(context.Background(), "user group publish: "+msg)
}
