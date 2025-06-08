package myexternalpkg

// ExternalConfig holds external configuration.
type ExternalConfig struct {
	// URL for the external service.
	ExternalURL string
	// Retry count for external service.
	ExternalRetryCount int
}

// ExternalEmbedded holds fields to be embedded from external package.
type ExternalEmbedded struct {
	// Flag from external package.
	IsRemote bool `env:"IS_REMOTE_TAG"`
}

// PointerPkgConfig is an external struct often used as a pointer.
type PointerPkgConfig struct {
	// APIKey for external service.
	APIKey string `env:"API_KEY_TAG"`
}
