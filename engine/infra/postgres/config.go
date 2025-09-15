package postgres

// Config holds PostgreSQL connection settings for the driver.
// Prefer providing a DSN via ConnString. When empty, a DSN will be
// synthesized from the individual fields.
type Config struct {
	ConnString string
	Host       string
	Port       string
	User       string
	Password   string
	DBName     string
	SSLMode    string
}
