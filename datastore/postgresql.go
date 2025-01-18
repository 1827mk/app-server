package datastore

import (
	"database/sql"
	"fmt"
	"os"

	_ "github.com/lib/pq"
)

type DBConfig struct {
	Host     string
	Port     int
	User     string
	Password string
	DBName   string
	Scripts  []string
}

type DBStore struct {
	DB *sql.DB
}

func NewPostgresDB(cfg *DBConfig) (*sql.DB, error) {
	// Construct the connection string
	dsn := fmt.Sprintf("host=%s port=%d user=%s password=%s dbname=%s sslmode=disable",
		cfg.Host, cfg.Port, cfg.User, cfg.Password, cfg.DBName)

	// Open a connection to the database
	db, err := sql.Open("postgres", dsn)
	if err != nil {
		return nil, fmt.Errorf("failed to open database connection: %w", err)
	}

	// Verify the connection is valid
	if err = db.Ping(); err != nil {
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	// Run the init script if provided
	if len(cfg.Scripts) > 0 {
		err := runInitScripts(db, cfg.Scripts)
		if err != nil {
			return nil, fmt.Errorf("failed to run init script: %w", err)
		}
	}

	return db, nil
}

func runInitScripts(db *sql.DB, scripts []string) error {
	// Loop through each script and execute
	for _, scriptPath := range scripts {
		// Read the content of the init script file
		scriptContent, err := os.ReadFile(scriptPath) // Using os.ReadFile instead of ioutil.ReadFile
		if err != nil {
			return fmt.Errorf("failed to read init script %s: %w", scriptPath, err)
		}

		// Execute the script
		_, err = db.Exec(string(scriptContent))
		if err != nil {
			return fmt.Errorf("failed to execute init script %s: %w", scriptPath, err)
		}
	}

	return nil
}
