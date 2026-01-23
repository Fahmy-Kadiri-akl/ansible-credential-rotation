package config

import (
	"fmt"
	"net"
	"os"

	"github.com/rs/zerolog/log"
)

// Config holds the application configuration
type Config struct {
	Port              string
	AkeylessGateway   string
	AkeylessAccessID  string
	AkeylessAccessKey string
	AllowedAccessIDs  string
	SyncURLBase       string
}

// Load loads configuration from environment variables
func Load() *Config {
	port := GetEnv("PORT", "8080")

	// Determine sync URL base - auto-detect if not explicitly set
	syncURLBase := os.Getenv("CRED_SERVER_SYNC_URL")
	if syncURLBase == "" {
		syncURLBase = autoDetectSyncURL(port)
	}

	return &Config{
		Port:              port,
		AkeylessGateway:   GetEnv("AKEYLESS_GATEWAY_URL", "https://api.akeyless.io"),
		AkeylessAccessID:  GetEnv("AKEYLESS_ACCESS_ID", ""),
		AkeylessAccessKey: GetEnv("AKEYLESS_ACCESS_KEY", ""),
		AllowedAccessIDs:  GetEnv("ALLOWED_ACCESS_IDS", ""),
		SyncURLBase:       syncURLBase,
	}
}

// autoDetectSyncURL determines the sync URL based on the server's IP address
func autoDetectSyncURL(port string) string {
	ip := getOutboundIP()
	if ip == "" {
		// Fallback to localhost if we can't determine IP
		log.Warn().Msg("Could not auto-detect IP address, using localhost for sync URL")
		return fmt.Sprintf("http://localhost:%s", port)
	}

	syncURL := fmt.Sprintf("http://%s:%s", ip, port)
	log.Info().Str("sync_url", syncURL).Msg("Auto-detected sync URL")
	return syncURL
}

// getOutboundIP gets the preferred outbound IP of this machine
func getOutboundIP() string {
	// First check if POD_IP is set (Kubernetes environment)
	if podIP := os.Getenv("POD_IP"); podIP != "" {
		return podIP
	}

	// Try to determine the outbound IP by dialing a known address
	conn, err := net.Dial("udp", "8.8.8.8:80")
	if err != nil {
		return ""
	}
	defer conn.Close()

	localAddr := conn.LocalAddr().(*net.UDPAddr)
	return localAddr.IP.String()
}

// GetEnv returns the value of an environment variable or a default value
func GetEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}
