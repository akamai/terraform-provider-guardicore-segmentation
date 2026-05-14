package client

import "time"

const (
	DefaultRequestTimeout         = 30
	DefaultImporterRequestTimeout = 120
	DefaultPaginationPageSize     = 100
	DefaultPaginationMaxPages     = 10_000
	DefaultBatchSize              = 100
	DefaultBatchFlushDelay        = 100 * time.Millisecond
	DefaultRequestMaxRetries      = 2
	DefaultRequestRetryBaseDelay  = 200 * time.Millisecond
	DefaultRequestRetryMaxDelay   = 2 * time.Second
	DefaultAuthMaxRetries         = 2
	DefaultAuthRetryBaseDelay     = 500 * time.Millisecond
)

var (
	defaultPageSize    = DefaultPaginationPageSize
	maxPaginationPages = DefaultPaginationMaxPages
	maxRequestRetries  = DefaultRequestMaxRetries
	retryBaseDelay     = DefaultRequestRetryBaseDelay
	retryMaxDelay      = DefaultRequestRetryMaxDelay
	authMaxRetries     = DefaultAuthMaxRetries
	authRetryBaseDelay = DefaultAuthRetryBaseDelay
)

type BatcherTuning struct {
	BatchSize  int
	FlushDelay time.Duration
}

type ResourceBatcherTuning struct {
	Create BatcherTuning
	Update BatcherTuning
	Delete BatcherTuning
}

type ClientBatchersTuning struct {
	Label       ResourceBatcherTuning
	PolicyRule  ResourceBatcherTuning
	LabelGroup  ResourceBatcherTuning
	UserGroup   ResourceBatcherTuning
	Asset       ResourceBatcherTuning
	DnsSecurity ResourceBatcherTuning
	Incident    ResourceBatcherTuning
	Worksite    ResourceBatcherTuning
}

type RuntimeSettings struct {
	PaginationPageSize    int
	PaginationMaxPages    int
	RequestMaxRetries     int
	RequestRetryBaseDelay time.Duration
	RequestRetryMaxDelay  time.Duration
	AuthMaxRetries        int
	AuthRetryBaseDelay    time.Duration
	MaxConcurrentRequests int
	Batchers              ClientBatchersTuning
}

func DefaultRuntimeSettings() RuntimeSettings {
	batch := BatcherTuning{
		BatchSize:  DefaultBatchSize,
		FlushDelay: DefaultBatchFlushDelay,
	}

	return RuntimeSettings{
		PaginationPageSize:    defaultPageSize,
		PaginationMaxPages:    maxPaginationPages,
		RequestMaxRetries:     maxRequestRetries,
		RequestRetryBaseDelay: retryBaseDelay,
		RequestRetryMaxDelay:  retryMaxDelay,
		AuthMaxRetries:        authMaxRetries,
		AuthRetryBaseDelay:    authRetryBaseDelay,
		MaxConcurrentRequests: 0,
		Batchers: ClientBatchersTuning{
			Label:       ResourceBatcherTuning{Create: batch, Update: batch, Delete: batch},
			PolicyRule:  ResourceBatcherTuning{Create: batch, Update: batch, Delete: batch},
			LabelGroup:  ResourceBatcherTuning{Create: batch, Update: batch, Delete: batch},
			UserGroup:   ResourceBatcherTuning{Create: batch, Update: batch, Delete: batch},
			Asset:       ResourceBatcherTuning{Create: batch, Update: batch, Delete: batch},
			DnsSecurity: ResourceBatcherTuning{Create: batch, Update: batch, Delete: batch},
			Incident:    ResourceBatcherTuning{Create: batch, Update: batch, Delete: batch},
			Worksite:    ResourceBatcherTuning{Create: batch, Update: batch, Delete: batch},
		},
	}
}

func ResolveRuntimeSettings(override *RuntimeSettings) RuntimeSettings {
	settings := DefaultRuntimeSettings()
	if override == nil || isZeroRuntimeSettings(*override) {
		return settings
	}

	if override.PaginationPageSize > 0 {
		settings.PaginationPageSize = override.PaginationPageSize
	}
	if override.PaginationMaxPages > 0 {
		settings.PaginationMaxPages = override.PaginationMaxPages
	}
	if override.RequestMaxRetries >= 0 {
		settings.RequestMaxRetries = override.RequestMaxRetries
	}
	if override.RequestRetryBaseDelay > 0 {
		settings.RequestRetryBaseDelay = override.RequestRetryBaseDelay
	}
	if override.RequestRetryMaxDelay > 0 {
		settings.RequestRetryMaxDelay = override.RequestRetryMaxDelay
	}
	if override.AuthMaxRetries >= 0 {
		settings.AuthMaxRetries = override.AuthMaxRetries
	}
	if override.AuthRetryBaseDelay > 0 {
		settings.AuthRetryBaseDelay = override.AuthRetryBaseDelay
	}
	if override.MaxConcurrentRequests > 0 {
		settings.MaxConcurrentRequests = override.MaxConcurrentRequests
	}

	settings.Batchers.Label.Create = resolveBatcherTuning(settings.Batchers.Label.Create, override.Batchers.Label.Create)
	settings.Batchers.Label.Update = resolveBatcherTuning(settings.Batchers.Label.Update, override.Batchers.Label.Update)
	settings.Batchers.Label.Delete = resolveBatcherTuning(settings.Batchers.Label.Delete, override.Batchers.Label.Delete)

	settings.Batchers.PolicyRule.Create = resolveBatcherTuning(settings.Batchers.PolicyRule.Create, override.Batchers.PolicyRule.Create)
	settings.Batchers.PolicyRule.Update = resolveBatcherTuning(settings.Batchers.PolicyRule.Update, override.Batchers.PolicyRule.Update)
	settings.Batchers.PolicyRule.Delete = resolveBatcherTuning(settings.Batchers.PolicyRule.Delete, override.Batchers.PolicyRule.Delete)

	settings.Batchers.LabelGroup.Create = resolveBatcherTuning(settings.Batchers.LabelGroup.Create, override.Batchers.LabelGroup.Create)
	settings.Batchers.LabelGroup.Update = resolveBatcherTuning(settings.Batchers.LabelGroup.Update, override.Batchers.LabelGroup.Update)
	settings.Batchers.LabelGroup.Delete = resolveBatcherTuning(settings.Batchers.LabelGroup.Delete, override.Batchers.LabelGroup.Delete)

	settings.Batchers.UserGroup.Create = resolveBatcherTuning(settings.Batchers.UserGroup.Create, override.Batchers.UserGroup.Create)
	settings.Batchers.UserGroup.Update = resolveBatcherTuning(settings.Batchers.UserGroup.Update, override.Batchers.UserGroup.Update)
	settings.Batchers.UserGroup.Delete = resolveBatcherTuning(settings.Batchers.UserGroup.Delete, override.Batchers.UserGroup.Delete)

	settings.Batchers.Asset.Create = resolveBatcherTuning(settings.Batchers.Asset.Create, override.Batchers.Asset.Create)
	settings.Batchers.Asset.Update = resolveBatcherTuning(settings.Batchers.Asset.Update, override.Batchers.Asset.Update)
	settings.Batchers.Asset.Delete = resolveBatcherTuning(settings.Batchers.Asset.Delete, override.Batchers.Asset.Delete)

	settings.Batchers.DnsSecurity.Create = resolveBatcherTuning(settings.Batchers.DnsSecurity.Create, override.Batchers.DnsSecurity.Create)
	settings.Batchers.DnsSecurity.Update = resolveBatcherTuning(settings.Batchers.DnsSecurity.Update, override.Batchers.DnsSecurity.Update)
	settings.Batchers.DnsSecurity.Delete = resolveBatcherTuning(settings.Batchers.DnsSecurity.Delete, override.Batchers.DnsSecurity.Delete)

	settings.Batchers.Incident.Create = resolveBatcherTuning(settings.Batchers.Incident.Create, override.Batchers.Incident.Create)
	settings.Batchers.Incident.Update = resolveBatcherTuning(settings.Batchers.Incident.Update, override.Batchers.Incident.Update)
	settings.Batchers.Incident.Delete = resolveBatcherTuning(settings.Batchers.Incident.Delete, override.Batchers.Incident.Delete)

	settings.Batchers.Worksite.Create = resolveBatcherTuning(settings.Batchers.Worksite.Create, override.Batchers.Worksite.Create)
	settings.Batchers.Worksite.Update = resolveBatcherTuning(settings.Batchers.Worksite.Update, override.Batchers.Worksite.Update)
	settings.Batchers.Worksite.Delete = resolveBatcherTuning(settings.Batchers.Worksite.Delete, override.Batchers.Worksite.Delete)

	if settings.RequestRetryMaxDelay < settings.RequestRetryBaseDelay {
		settings.RequestRetryMaxDelay = settings.RequestRetryBaseDelay
	}

	return settings
}

func resolveBatcherTuning(def BatcherTuning, override BatcherTuning) BatcherTuning {
	if override.BatchSize > 0 {
		def.BatchSize = override.BatchSize
	}
	if override.FlushDelay > 0 {
		def.FlushDelay = override.FlushDelay
	}
	return def
}

func isZeroRuntimeSettings(s RuntimeSettings) bool {
	return s.PaginationPageSize == 0 &&
		s.PaginationMaxPages == 0 &&
		s.RequestMaxRetries == 0 &&
		s.RequestRetryBaseDelay == 0 &&
		s.RequestRetryMaxDelay == 0 &&
		s.AuthMaxRetries == 0 &&
		s.AuthRetryBaseDelay == 0 &&
		s.MaxConcurrentRequests == 0 &&
		s.Batchers == (ClientBatchersTuning{})
}
