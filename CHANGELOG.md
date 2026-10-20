## 0.4.0

BUG FIXES:

- `guardicore_policy_rule`: publishing a policy revision is now scoped to the
  rulesets the change actually touched. Previously the provider sent an empty
  `rulesets` list, which the API treats as "publish every pending change" — so a
  `terraform apply` also published unrelated draft rules, including ones created
  in the UI by other users. The publish scope is now derived from the rules in
  the batch. ([#9](https://github.com/akamai/terraform-provider-guardicore-segmentation/pull/9))

NOTES:

- Rules with no `ruleset_name` map to the `undefined` sentinel, which the API
  expands to every pending rule with no ruleset. The provider logs a warning when
  that happens; set `ruleset_name` on all Terraform-managed policy rules to keep
  the publish scope narrow.
- Within a published ruleset, all pending changes are published regardless of who
  made them. This matches the Web UI, where the ruleset is the unit of publish
  granularity.

## 0.3.1

BUG FIXES:

- `guardicore_asset`: `labels[].id` is now optional. The API resolves a label by
  `key`+`value` and ignores any supplied `id`, so the provider computes `id` from
  the server response after apply. Setting `id` explicitly still works and enables
  stricter reference validation. ([#3](https://github.com/akamai/terraform-provider-guardicore-segmentation/pull/3))

DEPENDENCIES:

- Bump `golang.org/x/crypto` to 0.55.0 in the root module and 0.52.0 in `tools` ([#5](https://github.com/akamai/terraform-provider-guardicore-segmentation/pull/5), [#6](https://github.com/akamai/terraform-provider-guardicore-segmentation/pull/6))
- Bump `golang.org/x/net` to 0.58.0 ([#4](https://github.com/akamai/terraform-provider-guardicore-segmentation/pull/4))
- Bump `google.golang.org/grpc` to 1.83.2 ([#8](https://github.com/akamai/terraform-provider-guardicore-segmentation/pull/8))

## 0.3.0

## 0.2.0

## 0.1.0

- Initial version
