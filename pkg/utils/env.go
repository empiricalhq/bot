package utils

import (
	"os"
	"strconv"
	"strings"
	"time"
)

func GetEnv(key, defaultValue string) string {
	if value, exists := os.LookupEnv(key); exists {
		return value
	}
	return defaultValue
}

func GetEnvInt(key string, defaultValue int) int {
	str := GetEnv(key, "")
	if str == "" {
		return defaultValue
	}
	value, err := strconv.Atoi(str)
	if err != nil {
		return defaultValue
	}
	return value
}

func GetEnvBool(key string, defaultValue bool) bool {
	str := GetEnv(key, "")
	if str == "" {
		return defaultValue
	}
	return strings.EqualFold(str, "true") || str == "1"
}

func GetEnvDuration(key string, defaultValue time.Duration) time.Duration {
	str := GetEnv(key, "")
	if str == "" {
		return defaultValue
	}
	duration, err := time.ParseDuration(str)
	if err != nil {
		return defaultValue
	}
	return duration
}
