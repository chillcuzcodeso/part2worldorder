package config

import (
	"os"
	"strings"
)

const defaultServer = "https://api.mycompany.com"

// ServerBase returns the C2 base URL from SYS_AGENT_SERVER, or the default placeholder.
func ServerBase() string {
	if v := strings.TrimSpace(os.Getenv("SYS_AGENT_SERVER")); v != "" {
		return strings.TrimRight(v, "/")
	}
	return defaultServer
}
