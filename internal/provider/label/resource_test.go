package label

import (
	"context"
	"testing"

	"github.com/akamai/terraform-provider-guardicore-segmentation/internal/client"
	"github.com/akamai/terraform-provider-guardicore-segmentation/internal/provider/common"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

func TestLabelResource_MetadataSchemaAndConfigure(t *testing.T) {
	r := &LabelResource{}

	metaResp := &resource.MetadataResponse{}
	r.Metadata(context.Background(), resource.MetadataRequest{ProviderTypeName: "guardicore"}, metaResp)
	if metaResp.TypeName != "guardicore_label" {
		t.Fatalf("unexpected type name: %s", metaResp.TypeName)
	}

	schemaResp := &resource.SchemaResponse{}
	r.Schema(context.Background(), resource.SchemaRequest{}, schemaResp)
	for _, key := range []string{"id", "key", "value", "criteria", "system_managed", "managed_by"} {
		if _, ok := schemaResp.Schema.Attributes[key]; !ok {
			t.Fatalf("missing attribute %s", key)
		}
	}

	t.Run("nil provider data", func(t *testing.T) {
		resp := &resource.ConfigureResponse{}
		r.Configure(context.Background(), resource.ConfigureRequest{ProviderData: nil}, resp)
		if resp.Diagnostics.HasError() {
			t.Fatalf("unexpected diagnostics: %v", resp.Diagnostics)
		}
	})

	t.Run("unexpected provider data type", func(t *testing.T) {
		resp := &resource.ConfigureResponse{}
		r.Configure(context.Background(), resource.ConfigureRequest{ProviderData: "bad"}, resp)
		if !resp.Diagnostics.HasError() {
			t.Fatal("expected diagnostics error")
		}
		if resp.Diagnostics.Errors()[0].Summary() != "Unexpected Configure Type" {
			t.Fatalf("unexpected summary: %s", resp.Diagnostics.Errors()[0].Summary())
		}
	})

	t.Run("sets dependencies", func(t *testing.T) {
		c, err := client.NewClient(client.Config{BaseURL: "https://example.com", AccessToken: "token"})
		if err != nil {
			t.Fatalf("failed to create client: %v", err)
		}
		createBatcher := &common.Batcher[*client.LabelCreate, string]{}
		updateBatcher := &common.Batcher[common.LabelUpdateReq, struct{}]{}
		deleteBatcher := &common.Batcher[string, struct{}]{}

		resp := &resource.ConfigureResponse{}
		r.Configure(context.Background(), resource.ConfigureRequest{ProviderData: &common.ProviderData{
			Client:                c,
			LabelCreateBatcher:    createBatcher,
			LabelUpdateBatcher:    updateBatcher,
			LabelDeleteBatcher:    deleteBatcher,
			ValidateRefsOnDestroy: true,
			StrictRefsOnDestroy:   true,
		}}, resp)
		if resp.Diagnostics.HasError() {
			t.Fatalf("unexpected diagnostics: %v", resp.Diagnostics)
		}
		if r.client != c || r.createBatcher != createBatcher || r.updateBatcher != updateBatcher || r.deleteBatcher != deleteBatcher {
			t.Fatal("expected configure to set client and batchers")
		}
		if !r.validateRefsOnDestroy || !r.strictRefsOnDestroy {
			t.Fatal("expected destroy reference flags to be set")
		}
	})
}

func TestLabelResourceAPIToModel_SkipsReadOnlyWorksiteCriteria(t *testing.T) {
	r := &LabelResource{}
	model := &LabelResourceModel{}

	label := &client.Label{
		ID:    "label-1",
		Key:   "Worksite",
		Value: "Default",
		DynamicCriteria: []client.LabelCriteria{
			{Field: "z", Op: "EQUALS", Argument: "z"},
			{Field: "scoping_details.worksite.id", Op: "", Argument: "ws-1", Source: stringPtr("Worksite"), ReadOnly: boolPtr(true)},
			{Field: "a", Op: "CONTAINS", Argument: "a"},
		},
	}

	r.apiToModel(label, model)

	if len(model.Criteria) != 2 {
		t.Fatalf("expected 2 criteria after filtering, got %d", len(model.Criteria))
	}

	if model.Criteria[0].Field.ValueString() != "a" || model.Criteria[0].Op.ValueString() != "CONTAINS" {
		t.Fatalf("expected first criterion to be canonicalized to a/CONTAINS, got %s/%s", model.Criteria[0].Field.ValueString(), model.Criteria[0].Op.ValueString())
	}

	if model.Criteria[1].Field.ValueString() != "z" || model.Criteria[1].Op.ValueString() != "EQUALS" {
		t.Fatalf("expected second criterion to be canonicalized to z/EQUALS, got %s/%s", model.Criteria[1].Field.ValueString(), model.Criteria[1].Op.ValueString())
	}
}

func TestLabelResourceAPIToModel_CanonicalizesCriteriaOrderPermutations(t *testing.T) {
	tests := []struct {
		name  string
		input []client.LabelCriteria
		want  []string
	}{
		{
			name: "flat-compound-compound",
			input: []client.LabelCriteria{
				{Field: "name", Op: "STARTSWITH", Argument: "Test"},
				{CompoundCriteria: []client.LabelCriteria{{Field: "container_command", Op: "CONTAINS", Argument: "Test"}}},
				{CompoundCriteria: []client.LabelCriteria{{Field: "container_labels", Op: "STARTSWITH", Argument: "Test"}}},
			},
			want: []string{"flat:name|STARTSWITH|Test", "compound:container_command|CONTAINS|Test", "compound:container_labels|STARTSWITH|Test"},
		},
		{
			name: "compound-flat-compound",
			input: []client.LabelCriteria{
				{CompoundCriteria: []client.LabelCriteria{{Field: "container_command", Op: "CONTAINS", Argument: "Test"}}},
				{Field: "name", Op: "STARTSWITH", Argument: "Test"},
				{CompoundCriteria: []client.LabelCriteria{{Field: "container_labels", Op: "STARTSWITH", Argument: "Test"}}},
			},
			want: []string{"flat:name|STARTSWITH|Test", "compound:container_command|CONTAINS|Test", "compound:container_labels|STARTSWITH|Test"},
		},
		{
			name: "compound-compound-flat",
			input: []client.LabelCriteria{
				{CompoundCriteria: []client.LabelCriteria{{Field: "container_command", Op: "CONTAINS", Argument: "Test"}}},
				{CompoundCriteria: []client.LabelCriteria{{Field: "container_labels", Op: "STARTSWITH", Argument: "Test"}}},
				{Field: "name", Op: "STARTSWITH", Argument: "Test"},
			},
			want: []string{"flat:name|STARTSWITH|Test", "compound:container_command|CONTAINS|Test", "compound:container_labels|STARTSWITH|Test"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := &LabelResource{}
			model := &LabelResourceModel{}
			label := &client.Label{
				ID:              "label-1",
				Key:             "k",
				Value:           "v",
				DynamicCriteria: tt.input,
			}

			r.apiToModel(label, model)
			if len(model.Criteria) != len(tt.want) {
				t.Fatalf("expected %d criteria, got %d", len(tt.want), len(model.Criteria))
			}

			for i, want := range tt.want {
				got := criteriaFingerprint(model.Criteria[i])
				if got != want {
					t.Fatalf("criteria[%d] mismatch: got %q want %q", i, got, want)
				}

				if model.Criteria[i].Field.ValueString() == "" && model.Criteria[i].Op.ValueString() == "" && model.Criteria[i].Argument.ValueString() == "" && len(model.Criteria[i].CompoundCriteria) == 0 {
					t.Fatalf("criteria[%d] is an empty placeholder", i)
				}
			}
		})
	}
}

func TestLabelResourceAPIToModel_CanonicalizesNestedCompoundCriteriaOrder(t *testing.T) {
	r := &LabelResource{}
	model := &LabelResourceModel{}

	label := &client.Label{
		ID:    "label-1",
		Key:   "k",
		Value: "v",
		DynamicCriteria: []client.LabelCriteria{
			{
				CompoundCriteria: []client.LabelCriteria{
					{Field: "z_field", Op: "ENDSWITH", Argument: "z"},
					{Field: "a_field", Op: "CONTAINS", Argument: "a"},
					{Field: "m_field", Op: "EQUALS", Argument: "m"},
				},
			},
		},
	}

	r.apiToModel(label, model)

	if len(model.Criteria) != 1 {
		t.Fatalf("expected 1 criteria, got %d", len(model.Criteria))
	}

	compound := model.Criteria[0].CompoundCriteria
	if len(compound) != 3 {
		t.Fatalf("expected 3 compound criteria, got %d", len(compound))
	}

	got := []string{
		compound[0].Field.ValueString(),
		compound[1].Field.ValueString(),
		compound[2].Field.ValueString(),
	}
	want := []string{"a_field", "m_field", "z_field"}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("compound canonical order mismatch at %d: got %q want %q", i, got[i], want[i])
		}
	}
}

func TestLabelResourceAPIToModel_CanonicalizationProducesNoDiffAcrossAPIOrderings(t *testing.T) {
	r := &LabelResource{}

	first := &LabelResourceModel{}
	second := &LabelResourceModel{}

	r.apiToModel(&client.Label{
		ID:    "label-1",
		Key:   "k",
		Value: "v",
		DynamicCriteria: []client.LabelCriteria{
			{Field: "name", Op: "CONTAINS", Argument: "web"},
			{CompoundCriteria: []client.LabelCriteria{{Field: "container_labels", Op: "STARTSWITH", Argument: "prod"}, {Field: "container_command", Op: "CONTAINS", Argument: "run"}}},
		},
	}, first)

	r.apiToModel(&client.Label{
		ID:    "label-1",
		Key:   "k",
		Value: "v",
		DynamicCriteria: []client.LabelCriteria{
			{CompoundCriteria: []client.LabelCriteria{{Field: "container_command", Op: "CONTAINS", Argument: "run"}, {Field: "container_labels", Op: "STARTSWITH", Argument: "prod"}}},
			{Field: "name", Op: "CONTAINS", Argument: "web"},
		},
	}, second)

	if !labelCriteriaEqual(first.Criteria, second.Criteria) {
		t.Fatalf("expected canonicalized criteria to be equal across API order permutations; first=%v second=%v", first.Criteria, second.Criteria)
	}
}

func criteriaFingerprint(c LabelCriteriaModel) string {
	if len(c.CompoundCriteria) > 0 {
		first := c.CompoundCriteria[0]
		return "compound:" + first.Field.ValueString() + "|" + first.Op.ValueString() + "|" + first.Argument.ValueString()
	}

	return "flat:" + c.Field.ValueString() + "|" + c.Op.ValueString() + "|" + c.Argument.ValueString()
}

func TestWorksiteLabelEditableChange(t *testing.T) {
	base := LabelResourceModel{
		Key:   types.StringValue("Worksite"),
		Value: types.StringValue("ws-a"),
		Criteria: []LabelCriteriaModel{
			{Field: types.StringValue("name"), Op: types.StringValue("CONTAINS"), Argument: types.StringValue("a")},
		},
	}

	t.Run("no changes", func(t *testing.T) {
		if worksiteLabelEditableChange(base, base) {
			t.Fatal("expected no editable change")
		}
	})

	t.Run("value changed", func(t *testing.T) {
		plan := base
		plan.Value = types.StringValue("ws-b")
		if !worksiteLabelEditableChange(base, plan) {
			t.Fatal("expected editable change when value changes")
		}
	})

	t.Run("criteria changed", func(t *testing.T) {
		plan := base
		plan.Criteria = []LabelCriteriaModel{
			{Field: types.StringValue("name"), Op: types.StringValue("CONTAINS"), Argument: types.StringValue("b")},
		}
		if !worksiteLabelEditableChange(base, plan) {
			t.Fatal("expected editable change when criteria changes")
		}
	})
}

func TestValidateLabelCriteria(t *testing.T) {
	t.Run("valid flat and compound criteria", func(t *testing.T) {
		criteria := []LabelCriteriaModel{
			{
				Field:    types.StringValue("name"),
				Op:       types.StringValue("CONTAINS"),
				Argument: types.StringValue("web"),
			},
			{
				CompoundCriteria: []LabelCompoundCriteriaModel{
					{Field: types.StringValue("container_name"), Op: types.StringValue("EQUALS"), Argument: types.StringValue("nginx")},
					{Field: types.StringValue("container_image"), Op: types.StringValue("CONTAINS"), Argument: types.StringValue("nginx")},
				},
			},
		}

		var diags diag.Diagnostics
		validateLabelCriteria(criteria, &diags)
		if diags.HasError() {
			t.Fatalf("expected no diagnostics, got %v", diags)
		}
	})

	t.Run("flat and compound set together is invalid", func(t *testing.T) {
		criteria := []LabelCriteriaModel{{
			Field:            types.StringValue("name"),
			Op:               types.StringValue("CONTAINS"),
			Argument:         types.StringValue("web"),
			CompoundCriteria: []LabelCompoundCriteriaModel{{Field: types.StringValue("x"), Op: types.StringValue("EQUALS"), Argument: types.StringValue("y")}},
		}}

		var diags diag.Diagnostics
		validateLabelCriteria(criteria, &diags)
		if !diags.HasError() {
			t.Fatal("expected diagnostics for mixed flat and compound criterion")
		}
	})

	t.Run("partial flat criterion is invalid", func(t *testing.T) {
		criteria := []LabelCriteriaModel{{
			Field:    types.StringValue("name"),
			Op:       types.StringValue("CONTAINS"),
			Argument: types.StringNull(),
		}}

		var diags diag.Diagnostics
		validateLabelCriteria(criteria, &diags)
		if !diags.HasError() {
			t.Fatal("expected diagnostics for partial flat criterion")
		}
	})

	t.Run("empty criterion is invalid", func(t *testing.T) {
		var diags diag.Diagnostics
		validateLabelCriteria([]LabelCriteriaModel{{}}, &diags)
		if !diags.HasError() {
			t.Fatal("expected diagnostics for empty criterion")
		}
	})

	t.Run("duplicate flat criterion is invalid", func(t *testing.T) {
		dup := LabelCriteriaModel{Field: types.StringValue("name"), Op: types.StringValue("CONTAINS"), Argument: types.StringValue("web")}
		var diags diag.Diagnostics
		validateLabelCriteria([]LabelCriteriaModel{dup, dup}, &diags)
		if !diags.HasError() {
			t.Fatal("expected diagnostics for duplicate flat criterion")
		}
	})

	t.Run("duplicate compound criterion in group is invalid", func(t *testing.T) {
		cc := LabelCompoundCriteriaModel{Field: types.StringValue("container_name"), Op: types.StringValue("EQUALS"), Argument: types.StringValue("nginx")}
		criteria := []LabelCriteriaModel{{CompoundCriteria: []LabelCompoundCriteriaModel{cc, cc}}}
		var diags diag.Diagnostics
		validateLabelCriteria(criteria, &diags)
		if !diags.HasError() {
			t.Fatal("expected diagnostics for duplicate compound criterion")
		}
	})

	t.Run("duplicate compound group is invalid", func(t *testing.T) {
		criteria := []LabelCriteriaModel{
			{CompoundCriteria: []LabelCompoundCriteriaModel{{Field: types.StringValue("container_name"), Op: types.StringValue("EQUALS"), Argument: types.StringValue("nginx")}}},
			{CompoundCriteria: []LabelCompoundCriteriaModel{{Field: types.StringValue("container_name"), Op: types.StringValue("EQUALS"), Argument: types.StringValue("nginx")}}},
		}
		var diags diag.Diagnostics
		validateLabelCriteria(criteria, &diags)
		if !diags.HasError() {
			t.Fatal("expected diagnostics for duplicate compound groups")
		}
	})
}

func TestLabelCriteriaEqual(t *testing.T) {
	base := []LabelCriteriaModel{{
		Field:    types.StringValue("name"),
		Op:       types.StringValue("CONTAINS"),
		Argument: types.StringValue("web"),
	}}

	if !labelCriteriaEqual(base, base) {
		t.Fatal("expected criteria slices to be equal")
	}

	changed := []LabelCriteriaModel{{
		Field:    types.StringValue("name"),
		Op:       types.StringValue("CONTAINS"),
		Argument: types.StringValue("api"),
	}}
	if labelCriteriaEqual(base, changed) {
		t.Fatal("expected criteria slices to differ")
	}

	compoundA := []LabelCriteriaModel{{
		CompoundCriteria: []LabelCompoundCriteriaModel{{
			Field:    types.StringValue("container_name"),
			Op:       types.StringValue("EQUALS"),
			Argument: types.StringValue("nginx"),
		}},
	}}
	compoundB := []LabelCriteriaModel{{
		CompoundCriteria: []LabelCompoundCriteriaModel{{
			Field:    types.StringValue("container_name"),
			Op:       types.StringValue("EQUALS"),
			Argument: types.StringValue("redis"),
		}},
	}}
	if labelCriteriaEqual(compoundA, compoundB) {
		t.Fatal("expected compound criteria slices to differ")
	}
}

func TestLabelResourceModelToAPI(t *testing.T) {
	r := &LabelResource{}

	t.Run("empty criteria maps to empty API criteria", func(t *testing.T) {
		model := &LabelResourceModel{
			Key:   types.StringValue("env"),
			Value: types.StringValue("prod"),
		}

		api := r.modelToAPI(model)
		if api.Key != "env" || api.Value != "prod" {
			t.Fatalf("unexpected key/value mapping: %#v", api)
		}
		if len(api.Criteria) != 0 {
			t.Fatalf("expected empty criteria slice, got %#v", api.Criteria)
		}
	})

	t.Run("flat and compound criteria map correctly", func(t *testing.T) {
		model := &LabelResourceModel{
			Key:   types.StringValue("env"),
			Value: types.StringValue("prod"),
			Criteria: []LabelCriteriaModel{
				{Field: types.StringValue("name"), Op: types.StringValue("CONTAINS"), Argument: types.StringValue("web")},
				{CompoundCriteria: []LabelCompoundCriteriaModel{{Field: types.StringValue("container_name"), Op: types.StringValue("EQUALS"), Argument: types.StringValue("nginx")}}},
			},
		}

		api := r.modelToAPI(model)
		if len(api.Criteria) != 2 {
			t.Fatalf("expected 2 criteria, got %d", len(api.Criteria))
		}
		if api.Criteria[0].Field != "name" || api.Criteria[0].Argument != "web" {
			t.Fatalf("unexpected flat criteria mapping: %#v", api.Criteria[0])
		}
		if len(api.Criteria[1].CompoundCriteria) != 1 || api.Criteria[1].CompoundCriteria[0].Field != "container_name" {
			t.Fatalf("unexpected compound criteria mapping: %#v", api.Criteria[1])
		}
	})
}

func TestLabelGroupReferencesLabel(t *testing.T) {
	group := &client.LabelGroup{
		IncludeLabels: &client.OrLabelsRead{OrLabels: []client.AndLabelsRead{{AndLabels: []client.LabelInGroup{{ID: "l-include"}}}}},
		ExcludeLabels: &client.OrLabelsRead{OrLabels: []client.AndLabelsRead{{AndLabels: []client.LabelInGroup{{ID: "l-exclude"}}}}},
	}

	if !labelGroupReferencesLabel(group, "l-include") {
		t.Fatal("expected include reference to be detected")
	}
	if !labelGroupReferencesLabel(group, "l-exclude") {
		t.Fatal("expected exclude reference to be detected")
	}
	if labelGroupReferencesLabel(group, "missing") {
		t.Fatal("expected missing reference to return false")
	}
}

func TestLabelResourceAPIToModel_ClearsCriteriaWhenNoneReturned(t *testing.T) {
	r := &LabelResource{}
	model := &LabelResourceModel{
		Criteria: []LabelCriteriaModel{{
			Field:    types.StringValue("name"),
			Op:       types.StringValue("CONTAINS"),
			Argument: types.StringValue("stale"),
		}},
	}

	r.apiToModel(&client.Label{ID: "l-1", Key: "k", Value: "v", DynamicCriteria: nil}, model)

	if model.ID.ValueString() != "l-1" || model.Key.ValueString() != "k" || model.Value.ValueString() != "v" {
		t.Fatalf("unexpected identity mapping: %#v", model)
	}
	if model.Criteria != nil {
		t.Fatalf("expected criteria to be cleared when API returns none, got %#v", model.Criteria)
	}
}

func TestLabelResourceModelToAPI_HandlesUnknownCriteriaValues(t *testing.T) {
	r := &LabelResource{}
	model := &LabelResourceModel{
		Key:   types.StringValue("env"),
		Value: types.StringValue("prod"),
		Criteria: []LabelCriteriaModel{{
			Field:    types.StringUnknown(),
			Op:       types.StringUnknown(),
			Argument: types.StringUnknown(),
		}},
	}

	api := r.modelToAPI(model)
	if len(api.Criteria) != 1 {
		t.Fatalf("expected one criterion, got %d", len(api.Criteria))
	}
	if api.Criteria[0].Field != "" || api.Criteria[0].Op != "" || api.Criteria[0].Argument != "" {
		t.Fatalf("expected unknown values to map to empty strings, got %#v", api.Criteria[0])
	}
}
