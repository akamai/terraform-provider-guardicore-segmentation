package importer

import "text/template"

var labelDataTemplate = template.Must(template.New("label_data").Parse(`data "guardicore_label" "{{.Name}}" {
  key   = "{{.Key}}"
  value = "{{.Value}}"
}
`))

var labelTemplate = template.Must(template.New("label").Parse(`resource "guardicore_label" "{{.Name}}" {
  key   = "{{.Key}}"
  value = "{{.Value}}"
{{- if .Criteria}}

  criteria = [
{{- range $i, $c := .Criteria}}
    {
{{- if $c.IsCompound}}
      compound_criteria = [
{{- range $j, $cc := $c.CompoundCriteria}}
        {
          field    = "{{$cc.Field}}"
          op       = "{{$cc.Op}}"
          argument = "{{$cc.Argument}}"
        },
{{- end}}
      ]
{{- else}}
      field    = "{{$c.Field}}"
      op       = "{{$c.Op}}"
      argument = "{{$c.Argument}}"
{{- end}}
    },
{{- end}}
  ]
{{- end}}
}

import {
  to = guardicore_label.{{.Name}}
  id = "{{.ID}}"
}
`))

var labelGroupTemplate = template.Must(template.New("label_group").Parse(`resource "guardicore_label_group" "{{.Name}}" {
  key      = "{{.Key}}"
  value    = "{{.Value}}"
{{- if .Comments}}
  comments = "{{.Comments}}"
{{- end}}
{{- if .Include}}
  include = {{.Include}}
{{- end}}
{{- if .Exclude}}
  exclude = {{.Exclude}}
{{- end}}
}

import {
  to = guardicore_label_group.{{.Name}}
  id = "{{.ID}}"
}
`))

var policyRuleTemplate = template.Must(template.New("policy_rule").Parse(`resource "guardicore_policy_rule" "{{.Name}}" {
{{.BodyHCL}}
{{- if .WorksiteExpression}}

  worksite_id = {{.WorksiteExpression}}
{{- end}}
}

import {
  to = guardicore_policy_rule.{{.Name}}
  id = "{{.ID}}"
}
`))

// LabelTemplateData holds data for rendering a label resource block.
type LabelTemplateData struct {
	Name     string
	ID       string
	Key      string
	Value    string
	Criteria []CriteriaData
}

// CriteriaData holds data for rendering a criteria block.
type CriteriaData struct {
	Field            string
	Op               string
	Argument         string
	IsCompound       bool
	CompoundCriteria []CompoundCriteriaData
}

type CompoundCriteriaData struct {
	Field    string
	Op       string
	Argument string
}

// LabelGroupTemplateData holds data for rendering a label group resource block.
type LabelGroupTemplateData struct {
	Name     string
	ID       string
	Key      string
	Value    string
	Comments string
	Include  string
	Exclude  string
}

// PolicyRuleTemplateData holds data for rendering a policy rule resource block.
type PolicyRuleTemplateData struct {
	Name               string
	ID                 string
	BodyHCL            string
	WorksiteExpression string
}

var dnsSecurityTemplate = template.Must(template.New("dns_security").Parse(`resource "guardicore_dns_security" "{{.Name}}" {
  name = "{{.ResourceName}}"
  type = "{{.Type}}"
{{- if .Domains}}
  domains = [
{{- range .Domains}}
    "{{.}}",
{{- end}}
  ]
{{- end}}
  enabled = {{.Enabled}}
}

import {
  to = guardicore_dns_security.{{.Name}}
  id = "{{.ID}}"
}
`))

var dnsSecurityDataTemplate = template.Must(template.New("dns_security_data").Parse(`data "guardicore_dns_security" "{{.Name}}" {
  id = "{{.ID}}"
}
`))

// DnsSecurityTemplateData holds data for rendering a DNS security resource block.
type DnsSecurityTemplateData struct {
	Name         string
	ID           string
	ResourceName string
	Type         string
	Domains      []string
	Enabled      bool
}

var incidentTemplate = template.Must(template.New("incident").Parse(`# NOTE: Incidents are immutable and cannot be imported into Terraform.
# This block documents incident {{.ID}} for reference only.
# resource "guardicore_incident" "{{.Name}}" {
#   type        = "{{.IncidentType}}"
#   severity    = "{{.Severity}}"
#   time        = {{.StartTime}}
#   description = "Imported incident"
#   summary     = "Imported incident {{.ID}}"
{{- if .Tags}}
#   tags        = [{{range $i, $t := .Tags}}{{if $i}}, {{end}}"{{$t}}"{{end}}]
{{- else}}
#   tags        = ["imported"]
{{- end}}
#
#   affected_assets_json = jsonencode(
{{.CommentedAffectedAssetsJSON}}
#   )
# }
`))

// IncidentTemplateData holds data for rendering an incident resource block.
type IncidentTemplateData struct {
	Name                        string
	ID                          string
	IncidentType                string
	Severity                    string
	StartTime                   int64
	Tags                        []string
	CommentedAffectedAssetsJSON string
}

var worksiteDataTemplate = template.Must(template.New("worksite_data").Parse(`data "guardicore_worksite" "{{.Name}}" {
  name = {{.ResourceNameHCL}}
}
`))

var worksiteTemplate = template.Must(template.New("worksite").Parse(`resource "guardicore_worksite" "{{.Name}}" {
  name = {{.ResourceNameHCL}}
{{- if .CommentHCL}}
  comment = {{.CommentHCL}}
{{- end}}
}

import {
  to = guardicore_worksite.{{.Name}}
  id = "{{.ID}}"
}
`))

// WorksiteTemplateData holds data for rendering a worksite resource block.
type WorksiteTemplateData struct {
	Name            string
	ID              string
	ResourceNameHCL string
	CommentHCL      string
}

var userGroupTemplate = template.Must(template.New("user_group").Parse(`resource "guardicore_user_group" "{{.Name}}" {
  title = "{{.Title}}"

  orchestrations_groups = [
{{- range $i, $og := .OrchestrationsGroups}}
    {
      orchestration_id = "{{$og.OrchestrationID}}"
      groups           = [{{range $j, $g := $og.Groups}}{{if $j}}, {{end}}"{{$g}}"{{end}}]
    },
{{- end}}
  ]
}

import {
  to = guardicore_user_group.{{.Name}}
  id = "{{.ID}}"
}
`))

var userGroupDataTemplate = template.Must(template.New("user_group_data").Parse(`data "guardicore_user_group" "{{.Name}}" {
  title = "{{.Title}}"
}
`))

// UserGroupTemplateData holds data for rendering a user group resource block.
type UserGroupTemplateData struct {
	Name                 string
	ID                   string
	Title                string
	OrchestrationsGroups []OrchestrationGroupData
}

// OrchestrationGroupData holds data for rendering an orchestration group block.
type OrchestrationGroupData struct {
	OrchestrationID string
	Groups          []string
}

var assetTemplate = template.Must(template.New("asset").Parse(`resource "guardicore_asset" "{{.Name}}" {
  name                 = "{{.ResourceName}}"
{{- if .OrchObjIDComment}}
  # {{.OrchObjIDComment}}
{{- end}}
  orchestration_obj_id = "{{.OrchestrationObjID}}"
{{- if .InstanceID}}
  instance_id          = "{{.InstanceID}}"
{{- end}}
{{- if .HwUUID}}
  hw_uuid              = "{{.HwUUID}}"
{{- end}}
{{- if .Comments}}
  comments = "{{.Comments}}"
{{- end}}
{{- if .Status}}
  status = "{{.Status}}"
{{- end}}
{{- if .WorksiteExpression}}
  worksite_id = {{.WorksiteExpression}}
{{- end}}
{{- if .Labels}}

  labels = [
{{- range .Labels}}
    {
      id    = {{.IDExpression}}
      key   = {{.KeyExpression}}
      value = {{.ValueExpression}}
    },
{{- end}}
  ]
{{- end}}

  nics = [
{{- range $i, $nic := .Nics}}
    {
      ip_addresses = [{{range $j, $ip := $nic.IPAddresses}}{{if $j}}, {{end}}"{{$ip}}"{{end}}]
{{- if $nic.MacAddress}}
      mac_address  = "{{$nic.MacAddress}}"
{{- end}}
{{- if $nic.VifID}}
      vif_id = "{{$nic.VifID}}"
{{- end}}
{{- if $nic.NetworkName}}
      network_name = "{{$nic.NetworkName}}"
{{- end}}
    },
{{- end}}
  ]
}

import {
  to = guardicore_asset.{{.Name}}
  id = "{{.ID}}"
}
`))

var assetSkippedTemplate = template.Must(template.New("asset_skipped").Parse(`# NOTE: Asset "{{.ResourceName}}" has no valid NICs and cannot be managed by Terraform.
# The guardicore_asset resource requires at least one NIC with at least one ip_address.
# Agent-reported assets may have NICs with empty ip_addresses that the provider filters out.
# resource "guardicore_asset" "{{.Name}}" {
#   name                 = "{{.ResourceName}}"
#   orchestration_obj_id = "{{.OrchestrationObjID}}"
# }
`))

// AssetTemplateData holds data for rendering an asset resource block.
type AssetTemplateData struct {
	Name               string
	ID                 string
	ResourceName       string
	OrchestrationObjID string
	OrchObjIDComment   string
	InstanceID         string
	HwUUID             string
	Comments           string
	Status             string
	Labels             []AssetLabelRefData
	Nics               []AssetNICData
	WorksiteExpression string
}

// AssetLabelRefData holds data for rendering a label reference in an asset.
type AssetLabelRefData struct {
	IDExpression    string // HCL expression: guardicore_label.foo.id or "hardcoded-uuid"
	KeyExpression   string // HCL expression: guardicore_label.foo.key or "hardcoded-key"
	ValueExpression string // HCL expression: guardicore_label.foo.value or "hardcoded-value"
}

// AssetNICData holds data for rendering a NIC block within an asset.
type AssetNICData struct {
	IPAddresses []string
	MacAddress  string
	VifID       string
	NetworkName string
}

// Agent aggregator template (data source only — aggregators are system-managed)

var agentAggregatorDataTemplate = template.Must(template.New("agent_aggregator_data").Parse(`data "guardicore_agent_aggregator" "{{.Name}}" {
  hostname = "{{.Hostname}}"
}
`))

// AgentAggregatorTemplateData holds data for rendering an agent aggregator data source block.
type AgentAggregatorTemplateData struct {
	Name     string
	Hostname string
}
