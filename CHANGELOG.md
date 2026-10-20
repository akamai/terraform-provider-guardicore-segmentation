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
