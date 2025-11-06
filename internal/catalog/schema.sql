-- Catalog database schema
-- Stores only complete, installable ServiceTemplates from k0rdent catalog

-- Apps table: One row per application with ServiceTemplates
CREATE TABLE IF NOT EXISTS apps (
    slug TEXT PRIMARY KEY,              -- Directory name (e.g., "minio")
    title TEXT NOT NULL,                -- Display title
    summary TEXT,                       -- Description
    tags TEXT,                          -- JSON array: ["storage", "s3"]
    validated_platforms TEXT            -- JSON array: ["aws", "azure"]
);

-- ServiceTemplates table: One row per chart version with actual manifest
CREATE TABLE IF NOT EXISTS service_templates (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    app_slug TEXT NOT NULL,             -- References apps(slug)
    chart_name TEXT NOT NULL,           -- Helm chart name
    version TEXT NOT NULL,              -- Chart version
    service_template_path TEXT NOT NULL,-- Relative path to service-template.yaml
    helm_repository_path TEXT,          -- Relative path to helm-repository.yaml (optional)
    UNIQUE(app_slug, chart_name, version),
    FOREIGN KEY (app_slug) REFERENCES apps(slug) ON DELETE CASCADE
);

CREATE INDEX IF NOT EXISTS idx_service_templates_app ON service_templates(app_slug);

-- Metadata table: Cache management
CREATE TABLE IF NOT EXISTS metadata (
    key TEXT PRIMARY KEY,
    value TEXT
);

-- Initial metadata (only insert if not exists - don't overwrite existing values)
INSERT OR IGNORE INTO metadata (key, value) VALUES
    ('schema_version', '1'),
    ('catalog_sha', ''),
    ('indexed_at', ''),
    ('catalog_url', ''),
    ('index_timestamp', '');
