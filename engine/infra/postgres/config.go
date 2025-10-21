package postgres

import "time"

// Config holds PostgreSQL connection settings for the driver.
// Prefer providing a DSN via ConnString. When empty, a DSN will be
// synthesized from the individual fields.
type Config struct {
	ConnString         string
	Host               string
	Port               string
	User               string
	Password           string
	DBName             string
	SSLMode            string
	MaxOpenConns       int
	MaxIdleConns       int
	ConnMaxLifetime    time.Duration
	ConnMaxIdleTime    time.Duration
	PingTimeout        time.Duration
	HealthCheckTimeout time.Duration
	HealthCheckPeriod  time.Duration
	ConnectTimeout     time.Duration
}
