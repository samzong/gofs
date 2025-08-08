package constants

import "time"

const (
	DefaultHost        = "127.0.0.1"
	DefaultPort        = 8000
	DefaultMaxFileSize = 100 << 20

	DefaultRequestTimeout = 30 * time.Second
	ServerReadTimeout     = 30 * time.Second
	ServerWriteTimeout    = 30 * time.Second
	ServerIdleTimeout     = 2 * time.Minute

	UploadTimeout    = 5 * time.Minute
	FileServeTimeout = 30 * time.Second
	DirectoryTimeout = 10 * time.Second
	TemplateTimeout  = 5 * time.Second

	StaticAssetCacheMaxAge = 3600
	DefaultPathBufferSize  = 256
	ShutdownTimeout        = 5 * time.Second
	HealthCheckTimeout     = 5 * time.Second

	CSRFTokenExpiry     = 1 * time.Hour
	CSRFCleanupInterval = 5 * time.Minute

	// bcrypt constants
	BcryptCost = 12

	// File upload limits
	MaxUploadSize = 100 << 20
)
