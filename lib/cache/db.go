package cache

import (
	"database/sql"
	"fmt"
	"os"

	c "github.com/gookit/color" // nolint:misspell
)

type Cache struct {
	Path string
	DB   *sql.DB
}

func Open(path string) (*Cache, error) {
	// exists?
	if _, err := os.Stat(path); err == nil {
		c.Printf("Opening <magenta>%s</>...\n", path)
		db, err := sql.Open("sqlite3", path)
		if err != nil {
			return nil, fmt.Errorf("failed to open db %s: %w", path, err)
		}

		return &Cache{path, db}, nil
	}

	// create file
	c.Printf("Creating <magenta>%s</>...\n", path)
	if _, err := os.Create(path); err != nil {
		return nil, fmt.Errorf("failed to create db %s: %w", path, err)
	}
	db, err := sql.Open("sqlite3", path)
	if err != nil {
		return nil, fmt.Errorf("failed to open db %s: %w", path, err)
	}

	c.Printf("  table <white>issues</>...\n")
	if _, err = db.Exec(CreateIssuesTableSQL); err != nil {
		return nil, fmt.Errorf("failed to create issues table %s: %w", path, err)
	}

	c.Printf("  table <white>events</>...\n")
	if _, err = db.Exec(CreateEventsTableSQL); err != nil {
		return nil, fmt.Errorf("failed to create events table %s: %w", path, err)
	}

	return &Cache{path, db}, nil
}
