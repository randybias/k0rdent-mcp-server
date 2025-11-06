package catalog

import (
	"database/sql"
	_ "embed"
	"encoding/json"
	"fmt"
	"sync"

	_ "modernc.org/sqlite"
)

//go:embed schema.sql
var schemaSQL string

// DB wraps a SQLite database connection for catalog storage operations.
type DB struct {
	db *sql.DB
	mu sync.RWMutex
}

// AppRow represents a single application entry in the database.
type AppRow struct {
	Slug               string
	Title              string
	Summary            string
	Tags               []string
	ValidatedPlatforms []string
}

// ServiceTemplateRow represents a single ServiceTemplate version in the database.
type ServiceTemplateRow struct {
	ID                  int64
	AppSlug             string
	ChartName           string
	Version             string
	ServiceTemplatePath string
	HelmRepositoryPath  string
}

// AppWithTemplates combines an app with all its ServiceTemplate versions.
type AppWithTemplates struct {
	App       AppRow
	Templates []ServiceTemplateRow
}

// OpenDB opens or creates a SQLite database at the specified path.
func OpenDB(path string) (*DB, error) {
	// Use mode=rwc (read-write-create) and shared cache
	dsn := fmt.Sprintf("file:%s?mode=rwc&cache=shared", path)
	sqlDB, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("open database: %w", err)
	}

	// Test connection
	if err := sqlDB.Ping(); err != nil {
		sqlDB.Close()
		return nil, fmt.Errorf("ping database: %w", err)
	}

	db := &DB{db: sqlDB}

	// Initialize schema
	if err := db.InitSchema(); err != nil {
		sqlDB.Close()
		return nil, fmt.Errorf("initialize schema: %w", err)
	}

	return db, nil
}

// InitSchema loads and executes the embedded schema.sql file.
func (db *DB) InitSchema() error {
	db.mu.Lock()
	defer db.mu.Unlock()

	_, err := db.db.Exec(schemaSQL)
	if err != nil {
		return fmt.Errorf("execute schema: %w", err)
	}

	return nil
}

// GetMetadata retrieves a metadata value by key.
func (db *DB) GetMetadata(key string) (string, error) {
	db.mu.RLock()
	defer db.mu.RUnlock()

	var value string
	query := "SELECT value FROM metadata WHERE key = ?"
	err := db.db.QueryRow(query, key).Scan(&value)
	if err != nil {
		if err == sql.ErrNoRows {
			return "", nil
		}
		return "", fmt.Errorf("query metadata: %w", err)
	}

	return value, nil
}

// SetMetadata stores or updates a metadata key-value pair.
func (db *DB) SetMetadata(key, value string) error {
	db.mu.Lock()
	defer db.mu.Unlock()

	query := "INSERT OR REPLACE INTO metadata (key, value) VALUES (?, ?)"
	_, err := db.db.Exec(query, key, value)
	if err != nil {
		return fmt.Errorf("set metadata: %w", err)
	}

	return nil
}

// UpsertApp inserts or updates an application entry.
func (db *DB) UpsertApp(app AppRow) error {
	db.mu.Lock()
	defer db.mu.Unlock()

	// Marshal string slices to JSON
	tagsJSON, err := json.Marshal(app.Tags)
	if err != nil {
		return fmt.Errorf("marshal tags: %w", err)
	}

	platformsJSON, err := json.Marshal(app.ValidatedPlatforms)
	if err != nil {
		return fmt.Errorf("marshal validated_platforms: %w", err)
	}

	query := `
		INSERT OR REPLACE INTO apps (slug, title, summary, tags, validated_platforms)
		VALUES (?, ?, ?, ?, ?)
	`
	_, err = db.db.Exec(query, app.Slug, app.Title, app.Summary, string(tagsJSON), string(platformsJSON))
	if err != nil {
		return fmt.Errorf("upsert app: %w", err)
	}

	return nil
}

// UpsertServiceTemplate inserts or updates a ServiceTemplate version entry.
func (db *DB) UpsertServiceTemplate(st ServiceTemplateRow) error {
	db.mu.Lock()
	defer db.mu.Unlock()

	query := `
		INSERT OR REPLACE INTO service_templates
		(app_slug, chart_name, version, service_template_path, helm_repository_path)
		VALUES (?, ?, ?, ?, ?)
	`
	result, err := db.db.Exec(query, st.AppSlug, st.ChartName, st.Version, st.ServiceTemplatePath, st.HelmRepositoryPath)
	if err != nil {
		return fmt.Errorf("upsert service template: %w", err)
	}

	// Set the ID if this was an insert
	if st.ID == 0 {
		id, err := result.LastInsertId()
		if err == nil {
			st.ID = id
		}
	}

	return nil
}

// ListApps retrieves all apps with their ServiceTemplates, optionally filtered by slug.
// If slugFilter is empty, all apps are returned.
func (db *DB) ListApps(slugFilter string) ([]AppWithTemplates, error) {
	db.mu.RLock()
	defer db.mu.RUnlock()

	// Build query with optional filter
	var rows *sql.Rows
	var err error

	if slugFilter != "" {
		query := "SELECT slug, title, summary, tags, validated_platforms FROM apps WHERE slug = ?"
		rows, err = db.db.Query(query, slugFilter)
	} else {
		query := "SELECT slug, title, summary, tags, validated_platforms FROM apps"
		rows, err = db.db.Query(query)
	}

	if err != nil {
		return nil, fmt.Errorf("query apps: %w", err)
	}
	defer rows.Close()

	var results []AppWithTemplates

	for rows.Next() {
		var app AppRow
		var tagsJSON, platformsJSON string

		if err := rows.Scan(&app.Slug, &app.Title, &app.Summary, &tagsJSON, &platformsJSON); err != nil {
			return nil, fmt.Errorf("scan app row: %w", err)
		}

		// Unmarshal JSON arrays
		if tagsJSON != "" {
			if err := json.Unmarshal([]byte(tagsJSON), &app.Tags); err != nil {
				return nil, fmt.Errorf("unmarshal tags: %w", err)
			}
		}

		if platformsJSON != "" {
			if err := json.Unmarshal([]byte(platformsJSON), &app.ValidatedPlatforms); err != nil {
				return nil, fmt.Errorf("unmarshal validated_platforms: %w", err)
			}
		}

		// Query ServiceTemplates for this app
		templates, err := db.getServiceTemplatesForApp(app.Slug)
		if err != nil {
			return nil, fmt.Errorf("get templates for app %s: %w", app.Slug, err)
		}

		results = append(results, AppWithTemplates{
			App:       app,
			Templates: templates,
		})
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate app rows: %w", err)
	}

	return results, nil
}

// getServiceTemplatesForApp retrieves all ServiceTemplate versions for a specific app.
// This is a helper method that assumes the caller already holds a read lock.
func (db *DB) getServiceTemplatesForApp(appSlug string) ([]ServiceTemplateRow, error) {
	query := `
		SELECT id, app_slug, chart_name, version, service_template_path, helm_repository_path
		FROM service_templates
		WHERE app_slug = ?
		ORDER BY chart_name, version
	`

	rows, err := db.db.Query(query, appSlug)
	if err != nil {
		return nil, fmt.Errorf("query service templates: %w", err)
	}
	defer rows.Close()

	var templates []ServiceTemplateRow

	for rows.Next() {
		var st ServiceTemplateRow
		if err := rows.Scan(&st.ID, &st.AppSlug, &st.ChartName, &st.Version, &st.ServiceTemplatePath, &st.HelmRepositoryPath); err != nil {
			return nil, fmt.Errorf("scan service template row: %w", err)
		}
		templates = append(templates, st)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate service template rows: %w", err)
	}

	return templates, nil
}

// GetServiceTemplate retrieves a specific ServiceTemplate by app slug, chart name, and version.
func (db *DB) GetServiceTemplate(appSlug, chartName, version string) (*ServiceTemplateRow, error) {
	db.mu.RLock()
	defer db.mu.RUnlock()

	query := `
		SELECT id, app_slug, chart_name, version, service_template_path, helm_repository_path
		FROM service_templates
		WHERE app_slug = ? AND chart_name = ? AND version = ?
	`

	var st ServiceTemplateRow
	err := db.db.QueryRow(query, appSlug, chartName, version).Scan(
		&st.ID, &st.AppSlug, &st.ChartName, &st.Version, &st.ServiceTemplatePath, &st.HelmRepositoryPath,
	)

	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("service template not found")
		}
		return nil, fmt.Errorf("query service template: %w", err)
	}

	return &st, nil
}

// ClearAll removes all data from apps and service_templates tables.
// This is used for cache invalidation when rebuilding the catalog index.
func (db *DB) ClearAll() error {
	db.mu.Lock()
	defer db.mu.Unlock()

	// Delete in correct order due to foreign key constraints
	queries := []string{
		"DELETE FROM service_templates",
		"DELETE FROM apps",
	}

	for _, query := range queries {
		if _, err := db.db.Exec(query); err != nil {
			return fmt.Errorf("clear tables: %w", err)
		}
	}

	return nil
}

// Close closes the database connection.
func (db *DB) Close() error {
	db.mu.Lock()
	defer db.mu.Unlock()

	if db.db != nil {
		return db.db.Close()
	}

	return nil
}
