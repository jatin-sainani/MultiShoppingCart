package main

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

const (
	defaultPort              = "8080"
	defaultAWSRegion         = "us-east-1"
	defaultMySQLPort         = 3306
	defaultMySQLMaxOpenConns = 10
	defaultMySQLMaxIdleConns = 5
	defaultMySQLConnMaxIdle  = 5 * time.Minute
	defaultMySQLConnMaxLife  = 30 * time.Minute
)

type Config struct {
	Port string

	StoreBackend string
	AWSRegion    string

	MySQLHost            string
	MySQLPort            int
	MySQLDatabase        string
	MySQLUser            string
	MySQLPassword        string
	MySQLMaxOpenConns    int
	MySQLMaxIdleConns    int
	MySQLConnMaxIdleTime time.Duration
	MySQLConnMaxLifetime time.Duration

	DynamoDBTableName string
	DynamoDBStrong    bool
}

func loadConfig() (Config, error) {
	cfg := Config{
		Port:                 getEnv("PORT", defaultPort),
		StoreBackend:         strings.ToLower(getEnv("STORE_BACKEND", "memory")),
		AWSRegion:            getEnv("AWS_REGION", defaultAWSRegion),
		MySQLHost:            strings.TrimSpace(os.Getenv("MYSQL_HOST")),
		MySQLPort:            getEnvInt("MYSQL_PORT", defaultMySQLPort),
		MySQLDatabase:        strings.TrimSpace(os.Getenv("MYSQL_DATABASE")),
		MySQLUser:            strings.TrimSpace(os.Getenv("MYSQL_USER")),
		MySQLPassword:        os.Getenv("MYSQL_PASSWORD"),
		MySQLMaxOpenConns:    getEnvInt("MYSQL_MAX_OPEN_CONNS", defaultMySQLMaxOpenConns),
		MySQLMaxIdleConns:    getEnvInt("MYSQL_MAX_IDLE_CONNS", defaultMySQLMaxIdleConns),
		MySQLConnMaxIdleTime: getEnvDuration("MYSQL_CONN_MAX_IDLE_TIME", defaultMySQLConnMaxIdle),
		MySQLConnMaxLifetime: getEnvDuration("MYSQL_CONN_MAX_LIFETIME", defaultMySQLConnMaxLife),
		DynamoDBTableName:    strings.TrimSpace(os.Getenv("DYNAMODB_TABLE_NAME")),
		DynamoDBStrong:       getEnvBool("DYNAMODB_STRONG_READS", false),
	}

	switch cfg.StoreBackend {
	case "memory":
		return cfg, nil
	case "mysql":
		if cfg.MySQLHost == "" || cfg.MySQLDatabase == "" || cfg.MySQLUser == "" || cfg.MySQLPassword == "" {
			return Config{}, errorsf("MYSQL_HOST, MYSQL_DATABASE, MYSQL_USER, and MYSQL_PASSWORD are required when STORE_BACKEND=mysql")
		}
	case "dynamodb":
		if cfg.DynamoDBTableName == "" {
			return Config{}, errorsf("DYNAMODB_TABLE_NAME is required when STORE_BACKEND=dynamodb")
		}
	default:
		return Config{}, errorsf("unsupported STORE_BACKEND %q", cfg.StoreBackend)
	}

	return cfg, nil
}

func getEnv(key, fallback string) string {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback
	}
	return value
}

func getEnvInt(key string, fallback int) int {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback
	}
	parsed, err := strconv.Atoi(value)
	if err != nil {
		return fallback
	}
	return parsed
}

func getEnvBool(key string, fallback bool) bool {
	value := strings.TrimSpace(strings.ToLower(os.Getenv(key)))
	if value == "" {
		return fallback
	}
	switch value {
	case "1", "true", "yes", "y", "on":
		return true
	case "0", "false", "no", "n", "off":
		return false
	default:
		return fallback
	}
}

func getEnvDuration(key string, fallback time.Duration) time.Duration {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback
	}
	parsed, err := time.ParseDuration(value)
	if err != nil {
		return fallback
	}
	return parsed
}

func errorsf(format string, args ...any) error {
	return fmt.Errorf(format, args...)
}
