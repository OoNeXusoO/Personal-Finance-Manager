package config

import (
	"os"
	"strconv"
	"time"
)

type Config struct {
	ServerPort  string
	ServerHost  string
	TLSCertFile string
	TLSKeyFile  string

	DBHost     string
	DBPort     string
	DBUser     string
	DBPassword string
	DBName     string
	DBSSLMode  string
	DBMaxOpen  int
	DBMaxIdle  int
	DBTimeout  time.Duration

	JWTSecret     string
	JWTExpiration time.Duration

	GRPCPort         string
	CurrencyGRPCAddr string

	UploadDir     string
	MaxUploadSize int64
}

func Load() *Config {
	return &Config{
		ServerPort:  getEnv("SERVER_PORT", "8080"),
		ServerHost:  getEnv("SERVER_HOST", "0.0.0.0"),
		TLSCertFile: getEnv("TLS_CERT_FILE", ""),
		TLSKeyFile:  getEnv("TLS_KEY_FILE", ""),

		DBHost:     getEnv("DB_HOST", "localhost"),
		DBPort:     getEnv("DB_PORT", "5432"),
		DBUser:     getEnv("DB_USER", "postgres"),
		DBPassword: getEnv("DB_PASSWORD", "postgres"),
		DBName:     getEnv("DB_NAME", "finance"),
		DBSSLMode:  getEnv("DB_SSL_MODE", "disable"),
		DBMaxOpen:  getEnvInt("DB_MAX_OPEN", 25),
		DBMaxIdle:  getEnvInt("DB_MAX_IDLE", 5),
		DBTimeout:  getEnvDuration("DB_TIMEOUT", 5*time.Second),

		JWTSecret:     getEnv("JWT_SECRET", "change-this-secret-in-production-min-32chars"),
		JWTExpiration: getEnvDuration("JWT_EXPIRATION", 24*time.Hour),

		GRPCPort:         getEnv("GRPC_PORT", "50051"),
		CurrencyGRPCAddr: getEnv("CURRENCY_GRPC_ADDR", "localhost:50051"),

		UploadDir:     getEnv("UPLOAD_DIR", "./uploads"),
		MaxUploadSize: int64(getEnvInt("MAX_UPLOAD_SIZE_MB", 10)) * 1024 * 1024,
	}
}

func (c *Config) DSN() string {
	return "host=" + c.DBHost +
		" port=" + c.DBPort +
		" user=" + c.DBUser +
		" password=" + c.DBPassword +
		" dbname=" + c.DBName +
		" sslmode=" + c.DBSSLMode
}

func getEnv(key, defaultVal string) string {
	if v, ok := os.LookupEnv(key); ok {
		return v
	}
	return defaultVal
}

func getEnvInt(key string, defaultVal int) int {
	if v, ok := os.LookupEnv(key); ok {
		if i, err := strconv.Atoi(v); err == nil {
			return i
		}
	}
	return defaultVal
}

func getEnvDuration(key string, defaultVal time.Duration) time.Duration {
	if v, ok := os.LookupEnv(key); ok {
		if d, err := time.ParseDuration(v); err == nil {
			return d
		}
	}
	return defaultVal
}
