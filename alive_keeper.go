package withdrawal_service

import (
	"os"
)

func getEndpoint() string {
	endpoint := os.Getenv("KEEP_ALIVE_ENDPOINT")
	if endpoint != "" {
		return endpoint
	}

	return "http://localhost:8082/v4/withdrawal/health"
}
