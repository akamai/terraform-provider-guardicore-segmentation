package main

import (
	"context"
	"flag"
	"fmt"
	"os"

	"github.com/akamai/terraform-provider-guardicore-segmentation/internal/client"
	"github.com/akamai/terraform-provider-guardicore-segmentation/internal/importer"
)

func main() {
	baseURL := flag.String("base-url", os.Getenv("GUARDICORE_BASE_URL"), "Akamai Guardicore Segmentation API base URL (env: GUARDICORE_BASE_URL)")
	username := flag.String("username", os.Getenv("GUARDICORE_USERNAME"), "Akamai Guardicore Segmentation username (env: GUARDICORE_USERNAME)")
	password := flag.String("password", os.Getenv("GUARDICORE_PASSWORD"), "Akamai Guardicore Segmentation password (env: GUARDICORE_PASSWORD)")
	accessToken := flag.String("access-token", os.Getenv("GUARDICORE_ACCESS_TOKEN"), "Akamai Guardicore Segmentation access token (env: GUARDICORE_ACCESS_TOKEN)")
	refreshToken := flag.String("refresh-token", os.Getenv("GUARDICORE_REFRESH_TOKEN"), "Akamai Guardicore Segmentation refresh token (env: GUARDICORE_REFRESH_TOKEN)")
	insecure := flag.Bool("insecure", false, "Skip TLS certificate verification")
	requestTimeout := flag.Int64("request-timeout", 0, fmt.Sprintf("HTTP request timeout in seconds (default: %d)", client.DefaultRequestTimeout))
	outputDir := flag.String("output-dir", ".", "Output directory for generated .tf files")

	flag.Parse()

	if *baseURL == "" {
		fmt.Fprintln(os.Stderr, "Error: --base-url or GUARDICORE_BASE_URL is required")
		flag.Usage()
		os.Exit(1)
	}

	if *insecure {
		fmt.Fprintln(os.Stderr, "WARNING: TLS certificate verification is disabled (--insecure). Connections are susceptible to man-in-the-middle attacks.")
	}

	// Use a longer default timeout for the importer (120s) since it fetches entire datasets.
	timeout := *requestTimeout
	if timeout <= 0 {
		timeout = 120
	}

	config := client.Config{
		BaseURL:            *baseURL,
		Username:           *username,
		Password:           *password,
		AccessToken:        *accessToken,
		RefreshToken:       *refreshToken,
		InsecureSkipVerify: *insecure,
		RequestTimeout:     timeout,
	}

	apiClient, err := client.NewClient(config)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: failed to create API client: %v\n", err)
		os.Exit(1)
	}

	imp := &importer.Importer{
		Client:    apiClient,
		OutputDir: *outputDir,
	}

	ctx := context.Background()
	result, err := imp.Run(ctx)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: import failed: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Import complete!\n")
	fmt.Printf("  Labels:        %d → %s/labels.tf\n", result.Labels, *outputDir)
	fmt.Printf("  Label Groups:  %d → %s/label_groups.tf\n", result.LabelGroups, *outputDir)
	fmt.Printf("  Policy Rules:  %d → %s/policy_rules.tf\n", result.PolicyRules, *outputDir)
	fmt.Printf("  DNS Blocklists: %d → %s/dns_security.tf\n", result.DnsBlocklists, *outputDir)
	fmt.Printf("  Incidents:     %d → %s/incidents.tf\n", result.Incidents, *outputDir)
	fmt.Printf("  Worksites:     %d → %s/worksites.tf\n", result.Worksites, *outputDir)
	fmt.Printf("  User Groups:   %d → %s/user_groups.tf\n", result.UserGroups, *outputDir)
	fmt.Printf("  Assets:        %d → %s/assets.tf\n", result.Assets, *outputDir)
	fmt.Printf("\nNext steps:\n")
	fmt.Printf("  1. Review the generated .tf files\n")
	fmt.Printf("  2. Run: terraform init\n")
	fmt.Printf("  3. Run: terraform plan (should show no changes if import blocks match)\n")
}
