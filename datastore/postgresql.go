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

type Store interface {
	Validate() error
	GetDB() *sql.DB
}

type DBStore struct {
	DB *sql.DB
}

func NewPostgresDB(cfg *DBConfig) (*DBStore, error) {
	dsn := fmt.Sprintf("host=%s port=%d user=%s password=%s dbname=%s sslmode=disable",
		cfg.Host, cfg.Port, cfg.User, cfg.Password, cfg.DBName)

	db, err := sql.Open("postgres", dsn)
	if err != nil {
		return nil, fmt.Errorf("failed to open database connection: %w", err)
	}

	if err = db.Ping(); err != nil {
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	if len(cfg.Scripts) > 0 {
		if err := runInitScripts(db, cfg.Scripts); err != nil {
			return nil, fmt.Errorf("failed to run init scripts: %w", err)
		}
	}

	return &DBStore{DB: db}, nil
}

func NewStore(dbStore *DBStore) (Store, error) {
	if dbStore == nil || dbStore.DB == nil {
		return nil, fmt.Errorf("invalid database connection")
	}

	return &DBStore{
		DB: dbStore.DB,
	}, nil
}

// Add required methods
func (s *DBStore) Validate() error {
	if s.DB == nil {
		return fmt.Errorf("database connection is nil")
	}
	return s.DB.Ping()
}

func (s *DBStore) GetDB() *sql.DB {
	return s.DB
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
