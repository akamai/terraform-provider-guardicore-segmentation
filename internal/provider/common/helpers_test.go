package common

import (
	"context"
	"reflect"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

func TestContainsUnknownSentinel(t *testing.T) {
	tests := []struct {
		name  string
		value any
		want  bool
	}{
		{
			name:  "known unknown UUID sentinel",
			value: "74D93920-ED26-11E3-AC10-0800200C9A66",
			want:  true,
		},
		{
			name:  "known unknown angle-bracket sentinel",
			value: "<unknown>",
			want:  true,
		},
		{
			name: "nested map contains sentinel",
			value: map[string]any{
				"outer": map[string]any{"inner": "<unknown>"},
			},
			want: true,
		},
		{
			name: "nested slice contains sentinel",
			value: []any{
				"ok",
				[]any{"still-ok", "74D93920-ED26-11E3-AC10-0800200C9A66"},
			},
			want: true,
		},
		{
			name: "no sentinel present",
			value: map[string]any{
				"field": []any{"value", 42, true},
			},
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ContainsUnknownSentinel(tt.value)
			if got != tt.want {
				t.Fatalf("ContainsUnknownSentinel() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestStringSliceFromList(t *testing.T) {
	ctx := context.Background()

	t.Run("null list returns nil without diagnostics", func(t *testing.T) {
		got, diags := StringSliceFromList(ctx, types.ListNull(types.StringType), "labels")
		if diags.HasError() {
			t.Fatalf("expected no diagnostics, got %v", diags)
		}
		if got != nil {
			t.Fatalf("expected nil, got %#v", got)
		}
	})

	t.Run("unknown list returns nil without diagnostics", func(t *testing.T) {
		got, diags := StringSliceFromList(ctx, types.ListUnknown(types.StringType), "labels")
		if diags.HasError() {
			t.Fatalf("expected no diagnostics, got %v", diags)
		}
		if got != nil {
			t.Fatalf("expected nil, got %#v", got)
		}
	})

	t.Run("mixed values skip null and map unknown", func(t *testing.T) {
		list, diags := types.ListValue(types.StringType, []attr.Value{
			types.StringValue("a"),
			types.StringUnknown(),
			types.StringNull(),
			types.StringValue("b"),
		})
		if diags.HasError() {
			t.Fatalf("unexpected diagnostics creating list: %v", diags)
		}

		got, diags := StringSliceFromList(ctx, list, "labels")
		if diags.HasError() {
			t.Fatalf("unexpected diagnostics decoding list: %v", diags)
		}

		want := []string{"a", "<unknown>", "b"}
		if !reflect.DeepEqual(got, want) {
			t.Fatalf("StringSliceFromList() = %#v, want %#v", got, want)
		}
	})
}

func TestListStringsToValue(t *testing.T) {
	ctx := context.Background()

	t.Run("empty slice returns null list", func(t *testing.T) {
		list, diags := ListStringsToValue(ctx, []string{})
		if diags.HasError() {
			t.Fatalf("expected no diagnostics, got %v", diags)
		}
		if !list.IsNull() {
			t.Fatalf("expected null list, got %#v", list)
		}
	})

	t.Run("non-empty slice round-trips", func(t *testing.T) {
		input := []string{"first", "second"}
		list, diags := ListStringsToValue(ctx, input)
		if diags.HasError() {
			t.Fatalf("expected no diagnostics, got %v", diags)
		}

		var got []string
		decoded := list.ElementsAs(ctx, &got, false)
		if decoded.HasError() {
			t.Fatalf("unexpected decode diagnostics: %v", decoded)
		}

		if !reflect.DeepEqual(got, input) {
			t.Fatalf("decoded list = %#v, want %#v", got, input)
		}
	})
}

func TestIsSetHelpers(t *testing.T) {
	t.Run("string", func(t *testing.T) {
		if !IsSet(types.StringValue("x")) {
			t.Fatal("expected IsSet true for known string")
		}
		if IsSet(types.StringNull()) {
			t.Fatal("expected IsSet false for null string")
		}
		if IsSet(types.StringUnknown()) {
			t.Fatal("expected IsSet false for unknown string")
		}
	})

	t.Run("int64", func(t *testing.T) {
		if !IsSetInt64(types.Int64Value(1)) {
			t.Fatal("expected IsSetInt64 true for known int64")
		}
		if IsSetInt64(types.Int64Null()) {
			t.Fatal("expected IsSetInt64 false for null int64")
		}
		if IsSetInt64(types.Int64Unknown()) {
			t.Fatal("expected IsSetInt64 false for unknown int64")
		}
	})

	t.Run("bool", func(t *testing.T) {
		if !IsSetBool(types.BoolValue(true)) {
			t.Fatal("expected IsSetBool true for known bool")
		}
		if IsSetBool(types.BoolNull()) {
			t.Fatal("expected IsSetBool false for null bool")
		}
		if IsSetBool(types.BoolUnknown()) {
			t.Fatal("expected IsSetBool false for unknown bool")
		}
	})
}

func TestIsKnownHelper(t *testing.T) {
	if !IsKnown(types.StringValue("ok")) {
		t.Fatal("expected IsKnown true for known value")
	}
	if IsKnown(types.StringNull()) {
		t.Fatal("expected IsKnown false for null value")
	}
	if IsKnown(types.StringUnknown()) {
		t.Fatal("expected IsKnown false for unknown value")
	}
}
