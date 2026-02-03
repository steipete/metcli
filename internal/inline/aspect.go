package inline

import (
	"os"
	"strconv"
	"strings"
)

func CellAspectRatio(envKey string, defaultValue float64) float64 {
	raw := strings.TrimSpace(os.Getenv(envKey))
	if raw == "" {
		return defaultValue
	}
	value, err := strconv.ParseFloat(raw, 64)
	if err != nil {
		return defaultValue
	}
	if value < 0.1 || value > 2 {
		return defaultValue
	}
	return value
}
