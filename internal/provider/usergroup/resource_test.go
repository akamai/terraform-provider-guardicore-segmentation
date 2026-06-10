package usergroup

import (
	"context"
	"errors"
	"testing"

	"github.com/akamai/terraform-provider-guardicore-segmentation/internal/client"
	"github.com/akamai/terraform-provider-guardicore-segmentation/internal/provider/common"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

func TestUserGroupResource_Metadata(t *testing.T) {
	r := &UserGroupResource{}
	resp := &resource.MetadataResponse{}
	r.Metadata(context.Background(), resource.MetadataRequest{ProviderTypeName: "guardicore"}, resp)

	if resp.TypeName != "guardicore_user_group" {
		t.Fatalf("unexpected type name: %s", resp.TypeName)
	}
}

func TestUserGroupResource_Schema(t *testing.T) {
	r := &UserGroupResource{}
	resp := &resource.SchemaResponse{}
	r.Schema(context.Background(), resource.SchemaRequest{}, resp)

	if len(resp.Schema.Attributes) != 5 {
		t.Fatalf("expected 5 attributes, got %d", len(resp.Schema.Attributes))
	}
	if _, ok := resp.Schema.Attributes["id"]; !ok {
		t.Fatal("expected id attribute")
	}
	if _, ok := resp.Schema.Attributes["title"]; !ok {
		t.Fatal("expected title attribute")
	}
	if _, ok := resp.Schema.Attributes["orchestrations_groups"]; !ok {
		t.Fatal("expected orchestrations_groups attribute")
	}
	if _, ok := resp.Schema.Attributes["system_managed"]; !ok {
		t.Fatal("expected system_managed attribute")
	}
	if _, ok := resp.Schema.Attributes["managed_by"]; !ok {
		t.Fatal("expected managed_by attribute")
	}
}

func TestResolveOrchestrationGroups_PrefersOrchestrationsGroups(t *testing.T) {
	var diags diag.Diagnostics
	ug := &client.UserGroup{
		OrchestrationsGroups: []client.OrchestrationGroup{{OrchestrationID: "orch-1", Groups: []string{"g1", "g2"}}},
		GroupsByDomainName: map[string]client.DomainGroupInfo{
			"domain.local": {OrchestrationID: "local", Groups: []client.DomainGroup{{ID: "ignored"}}},
		},
	}

	got := resolveOrchestrationGroups(context.Background(), ug, &diags)
	if len(got) != 1 {
		t.Fatalf("expected 1 orchestration group, got %d", len(got))
	}
	if got[0].OrchestrationID.ValueString() != "orch-1" {
		t.Fatalf("unexpected orchestration id: %s", got[0].OrchestrationID.ValueString())
	}
	if diags.HasError() {
		t.Fatalf("unexpected diagnostics: %+v", diags)
	}
}

func TestResolveOrchestrationGroups_FallbackGroupsByDomainName(t *testing.T) {
	var diags diag.Diagnostics
	ug := &client.UserGroup{
		GroupsByDomainName: map[string]client.DomainGroupInfo{
			"d1": {OrchestrationID: "orch-a", Groups: []client.DomainGroup{{ID: "g1"}, {ID: "g2"}}},
		},
	}

	got := resolveOrchestrationGroups(context.Background(), ug, &diags)
	if len(got) != 1 {
		t.Fatalf("expected 1 orchestration group, got %d", len(got))
	}
	if got[0].OrchestrationID.ValueString() != "orch-a" {
		t.Fatalf("unexpected orchestration id: %s", got[0].OrchestrationID.ValueString())
	}
	if got[0].Groups.IsNull() || got[0].Groups.IsUnknown() {
		t.Fatal("expected non-null group list")
	}
}

func TestResolveOrchestrationGroups_EmptyReturnsNil(t *testing.T) {
	var diags diag.Diagnostics
	got := resolveOrchestrationGroups(context.Background(), &client.UserGroup{}, &diags)
	if got != nil {
		t.Fatalf("expected nil, got %#v", got)
	}
}

func TestPolicyRuleReferencesUserGroup(t *testing.T) {
	tests := []struct {
		name string
		rule map[string]any
		id   string
		want bool
	}{
		{
			name: "source match",
			rule: map[string]any{"source": map[string]any{"user_group_ids": []any{"ug-1", "ug-2"}}},
			id:   "ug-2",
			want: true,
		},
		{
			name: "destination match",
			rule: map[string]any{"destination": map[string]any{"user_group_ids": []any{"ug-9"}}},
			id:   "ug-9",
			want: true,
		},
		{
			name: "no match",
			rule: map[string]any{"source": map[string]any{"user_group_ids": []any{"ug-1"}}},
			id:   "ug-x",
			want: false,
		},
		{
			name: "wrong types ignored",
			rule: map[string]any{"source": "bad", "destination": map[string]any{"user_group_ids": "bad"}},
			id:   "ug-1",
			want: false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := policyRuleReferencesUserGroup(tc.rule, tc.id); got != tc.want {
				t.Fatalf("got %v want %v", got, tc.want)
			}
		})
	}
}

func TestUserGroupResource_modelToAPI(t *testing.T) {
	r := &UserGroupResource{}
	groups, diags := types.ListValueFrom(context.Background(), types.StringType, []string{"g1", "g2"})
	if diags.HasError() {
		t.Fatalf("list value diagnostics: %+v", diags)
	}

	model := &UserGroupResourceModel{
		Title: types.StringValue("title-1"),
		OrchestrationsGroups: []OrchestrationGroupModel{{
			OrchestrationID: types.StringValue("orch-1"),
			Groups:          groups,
		}},
	}

	var outDiags diag.Diagnostics
	api := r.modelToAPI(context.Background(), model, &outDiags)
	if outDiags.HasError() {
		t.Fatalf("unexpected diagnostics: %+v", outDiags)
	}
	if api.Title != "title-1" {
		t.Fatalf("unexpected title: %s", api.Title)
	}
	if len(api.OrchestrationsGroups) != 1 || len(api.OrchestrationsGroups[0].Groups) != 2 {
		t.Fatalf("unexpected orchestrations_groups: %+v", api.OrchestrationsGroups)
	}
}

func TestAddUserGroupClientError(t *testing.T) {
	var diags diag.Diagnostics
	addUserGroupClientError(&diags, "update", errors.New("boom"))
	if !diags.HasError() {
		t.Fatal("expected diagnostics error")
	}
	if diags.Errors()[0].Summary() != "Client Error" {
		t.Fatalf("unexpected summary: %s", diags.Errors()[0].Summary())
	}
}

func TestUserGroupResource_ModelHelpers(t *testing.T) {
	groups, d := types.ListValueFrom(context.Background(), types.StringType, []string{"g1"})
	if d.HasError() {
		t.Fatalf("unexpected list diagnostics: %+v", d)
	}

	r := &UserGroupResource{
		client:        &fakeUserGroupResourceClient{getResult: &client.UserGroup{ID: "u-1", Title: "admins", OrchestrationsGroups: []client.OrchestrationGroup{{OrchestrationID: "orch-1", Groups: []string{"g1"}}}}},
		createBatcher: &fakeUserGroupCreateBatcher{id: "u-1"},
		updateBatcher: &fakeUserGroupUpdateBatcher{},
		deleteBatcher: &fakeUserGroupDeleteBatcher{},
	}

	model := UserGroupResourceModel{
		Title: types.StringValue("admins"),
		OrchestrationsGroups: []OrchestrationGroupModel{{
			OrchestrationID: types.StringValue("orch-1"),
			Groups:          groups,
		}},
	}

	created, err := r.createModel(context.Background(), model)
	if err != nil {
		t.Fatalf("createModel error: %v", err)
	}
	if created.ID.ValueString() != "u-1" {
		t.Fatalf("unexpected created model: %+v", created)
	}

	read, remove, err := r.readModel(context.Background(), UserGroupResourceModel{ID: types.StringValue("u-1")}, &diag.Diagnostics{})
	if err != nil || remove {
		t.Fatalf("unexpected read result err=%v remove=%v", err, remove)
	}
	if read.ID.ValueString() != "u-1" {
		t.Fatalf("unexpected read model: %+v", read)
	}

	upd := model
	upd.ID = types.StringValue("u-1")
	updated, gone, err := r.updateModel(context.Background(), upd, &diag.Diagnostics{})
	if err != nil {
		t.Fatalf("updateModel error: %v", err)
	}
	if gone {
		t.Fatalf("unexpected gone=true from updateModel: %+v", updated)
	}
	if updated.ID.ValueString() != "u-1" {
		t.Fatalf("unexpected updated model: %+v", updated)
	}

	if err := r.deleteModel(context.Background(), UserGroupResourceModel{ID: types.StringValue("u-1")}); err != nil {
		t.Fatalf("deleteModel error: %v", err)
	}
}

func TestUserGroupResource_readModelNotFound(t *testing.T) {
	r := &UserGroupResource{client: &fakeUserGroupResourceClient{getResult: nil}}
	_, remove, err := r.readModel(context.Background(), UserGroupResourceModel{ID: types.StringValue("u-1")}, &diag.Diagnostics{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !remove {
		t.Fatal("expected remove=true")
	}
}

// TestUserGroupResource_updateModelGoneAfterWrite covers F-01: if the user
// group is deleted server-side between Update and the follow-up GetUserGroup,
// updateModel must report gone=true so the caller can RemoveResource instead
// of writing stale state.
func TestUserGroupResource_updateModelGoneAfterWrite(t *testing.T) {
	fake := &fakeUserGroupResourceClient{getResult: nil}
	r := &UserGroupResource{client: fake, updateBatcher: &fakeUserGroupUpdateBatcher{}}
	plan := UserGroupResourceModel{
		ID:                   types.StringValue("u-1"),
		Title:                types.StringValue("group"),
		OrchestrationsGroups: []OrchestrationGroupModel{},
	}

	_, gone, err := r.updateModel(context.Background(), plan, &diag.Diagnostics{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !gone {
		t.Fatal("expected gone=true when GetUserGroup returns nil")
	}
}

func TestUserGroupResource_checkReferencesOnDestroy(t *testing.T) {
	r := &UserGroupResource{client: &fakeUserGroupResourceClient{rules: []map[string]any{{"id": "r-1", "source": map[string]any{"user_group_ids": []any{"u-1"}}}}}}

	var warnDiags diag.Diagnostics
	r.checkUserGroupReferencesOnDestroy(context.Background(), "u-1", false, &warnDiags)
	if warnDiags.HasError() || len(warnDiags.Warnings()) == 0 {
		t.Fatalf("expected warning diagnostic, got errors=%v warnings=%d", warnDiags.HasError(), len(warnDiags.Warnings()))
	}

	var errDiags diag.Diagnostics
	r.checkUserGroupReferencesOnDestroy(context.Background(), "u-1", true, &errDiags)
	if !errDiags.HasError() {
		t.Fatal("expected strict mode error")
	}
}

type fakeUserGroupResourceClient struct {
	getResult *client.UserGroup
	getErr    error
	rules     []map[string]any
	rulesErr  error
}

func (f *fakeUserGroupResourceClient) GetUserGroup(context.Context, string) (*client.UserGroup, error) {
	return f.getResult, f.getErr
}

func (f *fakeUserGroupResourceClient) ListPolicyRules(context.Context) ([]map[string]any, error) {
	return f.rules, f.rulesErr
}

type fakeUserGroupCreateBatcher struct {
	id  string
	err error
}

func (f *fakeUserGroupCreateBatcher) Enqueue(context.Context, *client.UserGroupCreate) (string, error) {
	return f.id, f.err
}

type fakeUserGroupUpdateBatcher struct{ err error }

func (f *fakeUserGroupUpdateBatcher) Enqueue(context.Context, common.UserGroupUpdateReq) (struct{}, error) {
	return struct{}{}, f.err
}

type fakeUserGroupDeleteBatcher struct{ err error }

func (f *fakeUserGroupDeleteBatcher) Enqueue(context.Context, string) (struct{}, error) {
	return struct{}{}, f.err
}
