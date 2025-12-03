package log

import (
	"context"
	"database/sql"
	"time"
)

// RequestLog represents a single HTTP request log entry
type RequestLog struct {
	ID          int64
	Timestamp   time.Time
	ProjectID   string // identifier for the website/project
	IP          string
	Method      string
	Scheme      string
	Proto       string
	Path        string
	Query       string
	StatusPtr   *int // nullable status
	Size        int
	Duration    time.Duration
	Browser     string
	BrowserVer  string
	Engine      string
	OS          string
	Device      string
	DeviceModel string
	CPUArch     string
	IsBot       bool
	UserAgent   string
	Location    string
	Referer     string
	AcceptLang  string
	AcceptEnc   string
	ContentType string
	ContentLen  string
	Host        string
	TLS         string
	RequestID   string
	CreatedAt   time.Time
}

// InitDB creates the request_logs table if it doesn't exist
func InitDB(db *sql.DB) error {
	schema := `
    CREATE TABLE IF NOT EXISTS request_logs (
        id BIGSERIAL PRIMARY KEY,
        timestamp TIMESTAMP NOT NULL,
        project_id VARCHAR(255),
        ip VARCHAR(45) NOT NULL,
        method VARCHAR(10) NOT NULL,
        scheme VARCHAR(10),
        proto VARCHAR(20),
        path TEXT,
        query TEXT,
        status INTEGER,
        size INTEGER,
        duration BIGINT,
        browser VARCHAR(50),
        browser_ver VARCHAR(50),
        engine VARCHAR(50),
        os VARCHAR(50),
        device VARCHAR(50),
        device_model VARCHAR(100),
        cpu_arch VARCHAR(50),
        is_bot BOOLEAN,
        user_agent TEXT,
        location TEXT,
        referer TEXT,
        accept_lang TEXT,
        accept_enc TEXT,
        content_type TEXT,
        content_len VARCHAR(20),
        host VARCHAR(255),
        tls VARCHAR(100),
        request_id VARCHAR(255),
        created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
    );
    CREATE INDEX IF NOT EXISTS idx_project_id ON request_logs (project_id);
    CREATE INDEX IF NOT EXISTS idx_ip ON request_logs (ip);
    CREATE INDEX IF NOT EXISTS idx_timestamp ON request_logs (timestamp);
    CREATE INDEX IF NOT EXISTS idx_method ON request_logs (method);
    CREATE INDEX IF NOT EXISTS idx_status ON request_logs (status);
    CREATE INDEX IF NOT EXISTS idx_browser ON request_logs (browser);
    CREATE INDEX IF NOT EXISTS idx_os ON request_logs (os);
    CREATE INDEX IF NOT EXISTS idx_is_bot ON request_logs (is_bot);
    `
	_, err := db.Exec(schema)
	return err
}

// StoreLog saves a request log to PostgreSQL
func StoreLog(db *sql.DB, log *RequestLog) error {
	query := `
    INSERT INTO request_logs (
        timestamp, project_id, ip, method, scheme, proto, path, query, status, size, duration,
        browser, browser_ver, engine, os, device, device_model, cpu_arch, is_bot,
        user_agent, location, referer, accept_lang, accept_enc, content_type,
        content_len, host, tls, request_id
    ) VALUES (
        $1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11,
        $12, $13, $14, $15, $16, $17, $18, $19,
        $20, $21, $22, $23, $24, $25,
        $26, $27, $28, $29
    ) RETURNING id, created_at
    `

	err := db.QueryRow(
		query,
		log.Timestamp, log.ProjectID, log.IP, log.Method, log.Scheme, log.Proto, log.Path, log.Query,
		log.StatusPtr,
		log.Size, log.Duration.Nanoseconds(),
		log.Browser, log.BrowserVer, log.Engine, log.OS, log.Device, log.DeviceModel,
		log.CPUArch, log.IsBot, log.UserAgent, log.Location, log.Referer, log.AcceptLang,
		log.AcceptEnc, log.ContentType, log.ContentLen, log.Host, log.TLS, log.RequestID,
	).Scan(&log.ID, &log.CreatedAt)

	return err
}

// GetLogByID retrieves a single request log by ID
func GetLogByID(db *sql.DB, id int64) (*RequestLog, error) {
	query := `
    SELECT id, timestamp, project_id, ip, method, scheme, proto, path, query, status, size, duration,
           browser, browser_ver, engine, os, device, device_model, cpu_arch, is_bot,
           user_agent, location, referer, accept_lang, accept_enc, content_type,
           content_len, host, tls, request_id, created_at
    FROM request_logs
    WHERE id = $1
    `

	log := &RequestLog{}
	var durationNano int64

	err := db.QueryRow(query, id).Scan(
		&log.ID, &log.Timestamp, &log.ProjectID, &log.IP, &log.Method, &log.Scheme, &log.Proto, &log.Path,
		&log.Query, &log.StatusPtr, &log.Size, &durationNano, &log.Browser, &log.BrowserVer,
		&log.Engine, &log.OS, &log.Device, &log.DeviceModel, &log.CPUArch, &log.IsBot,
		&log.UserAgent, &log.Location, &log.Referer, &log.AcceptLang, &log.AcceptEnc,
		&log.ContentType, &log.ContentLen, &log.Host, &log.TLS, &log.RequestID, &log.CreatedAt,
	)

	if err != nil {
		return nil, err
	}

	log.Duration = time.Duration(durationNano)
	return log, nil
}

// GetLogsByIP retrieves all logs for a specific IP address
func GetLogsByIP(db *sql.DB, ip string, limit int) ([]RequestLog, error) {
	query := `
    SELECT id, timestamp, project_id, ip, method, scheme, proto, path, query, status, size, duration,
           browser, browser_ver, engine, os, device, device_model, cpu_arch, is_bot,
           user_agent, location, referer, accept_lang, accept_enc, content_type,
           content_len, host, tls, request_id, created_at
    FROM request_logs
    WHERE ip = $1
    ORDER BY created_at DESC
    LIMIT $2
    `

	return scanLogs(db.QueryContext(context.Background(), query, ip, limit))
}

// GetLogsByBrowser retrieves all logs for a specific browser
func GetLogsByBrowser(db *sql.DB, browser string, limit int) ([]RequestLog, error) {
	query := `
    SELECT id, timestamp, project_id, ip, method, scheme, proto, path, query, status, size, duration,
           browser, browser_ver, engine, os, device, device_model, cpu_arch, is_bot,
           user_agent, location, referer, accept_lang, accept_enc, content_type,
           content_len, host, tls, request_id, created_at
    FROM request_logs
    WHERE browser = $1
    ORDER BY created_at DESC
    LIMIT $2
    `

	return scanLogs(db.QueryContext(context.Background(), query, browser, limit))
}

// GetLogsByOS retrieves all logs for a specific operating system
func GetLogsByOS(db *sql.DB, os string, limit int) ([]RequestLog, error) {
	query := `
    SELECT id, timestamp, project_id, ip, method, scheme, proto, path, query, status, size, duration,
           browser, browser_ver, engine, os, device, device_model, cpu_arch, is_bot,
           user_agent, location, referer, accept_lang, accept_enc, content_type,
           content_len, host, tls, request_id, created_at
    FROM request_logs
    WHERE os = $1
    ORDER BY created_at DESC
    LIMIT $2
    `

	return scanLogs(db.QueryContext(context.Background(), query, os, limit))
}

// GetLogsByStatus retrieves all logs with a specific HTTP status code
func GetLogsByStatus(db *sql.DB, status int, limit int) ([]RequestLog, error) {
	query := `
    SELECT id, timestamp, project_id, ip, method, scheme, proto, path, query, status, size, duration,
           browser, browser_ver, engine, os, device, device_model, cpu_arch, is_bot,
           user_agent, location, referer, accept_lang, accept_enc, content_type,
           content_len, host, tls, request_id, created_at
    FROM request_logs
    WHERE status = $1
    ORDER BY created_at DESC
    LIMIT $2
    `

	return scanLogs(db.QueryContext(context.Background(), query, status, limit))
}

// GetLogsTimeRange retrieves logs within a time range
func GetLogsTimeRange(db *sql.DB, startTime, endTime time.Time, limit int) ([]RequestLog, error) {
	query := `
    SELECT id, timestamp, project_id, ip, method, scheme, proto, path, query, status, size, duration,
           browser, browser_ver, engine, os, device, device_model, cpu_arch, is_bot,
           user_agent, location, referer, accept_lang, accept_enc, content_type,
           content_len, host, tls, request_id, created_at
    FROM request_logs
    WHERE timestamp BETWEEN $1 AND $2
    ORDER BY created_at DESC
    LIMIT $3
    `

	return scanLogs(db.QueryContext(context.Background(), query, startTime, endTime, limit))
}

// GetBotLogs retrieves all bot/crawler requests
func GetBotLogs(db *sql.DB, limit int) ([]RequestLog, error) {
	query := `
    SELECT id, timestamp, project_id, ip, method, scheme, proto, path, query, status, size, duration,
           browser, browser_ver, engine, os, device, device_model, cpu_arch, is_bot,
           user_agent, location, referer, accept_lang, accept_enc, content_type,
           content_len, host, tls, request_id, created_at
    FROM request_logs
    WHERE is_bot = true
    ORDER BY created_at DESC
    LIMIT $1
    `

	return scanLogs(db.QueryContext(context.Background(), query, limit))
}

// GetLogsByPath retrieves all logs for a specific request path
func GetLogsByPath(db *sql.DB, path string, limit int) ([]RequestLog, error) {
	query := `
    SELECT id, timestamp, project_id, ip, method, scheme, proto, path, query, status, size, duration,
           browser, browser_ver, engine, os, device, device_model, cpu_arch, is_bot,
           user_agent, location, referer, accept_lang, accept_enc, content_type,
           content_len, host, tls, request_id, created_at
    FROM request_logs
    WHERE path = $1
    ORDER BY created_at DESC
    LIMIT $2
    `

	return scanLogs(db.QueryContext(context.Background(), query, path, limit))
}

// GetRecentLogs retrieves the most recent N logs
func GetRecentLogs(db *sql.DB, limit int) ([]RequestLog, error) {
	query := `
    SELECT id, timestamp, project_id, ip, method, scheme, proto, path, query, status, size, duration,
           browser, browser_ver, engine, os, device, device_model, cpu_arch, is_bot,
           user_agent, location, referer, accept_lang, accept_enc, content_type,
           content_len, host, tls, request_id, created_at
    FROM request_logs
    ORDER BY created_at DESC
    LIMIT $1
    `

	return scanLogs(db.QueryContext(context.Background(), query, limit))
}

// GetErrorLogs retrieves logs with error status codes (4xx, 5xx)
func GetErrorLogs(db *sql.DB, limit int) ([]RequestLog, error) {
	query := `
    SELECT id, timestamp, project_id, ip, method, scheme, proto, path, query, status, size, duration,
           browser, browser_ver, engine, os, device, device_model, cpu_arch, is_bot,
           user_agent, location, referer, accept_lang, accept_enc, content_type,
           content_len, host, tls, request_id, created_at
    FROM request_logs
    WHERE status >= 400
    ORDER BY created_at DESC
    LIMIT $1
    `

	return scanLogs(db.QueryContext(context.Background(), query, limit))
}

// GetStatsByBrowser returns stats grouped by browser
func GetStatsByBrowser(db *sql.DB) (map[string]int, error) {
	query := `
    SELECT browser, COUNT(*) as count
    FROM request_logs
    GROUP BY browser
    ORDER BY count DESC
    `

	rows, err := db.Query(query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	stats := make(map[string]int)
	for rows.Next() {
		var browser string
		var count int
		if err := rows.Scan(&browser, &count); err != nil {
			return nil, err
		}
		stats[browser] = count
	}

	return stats, rows.Err()
}

// GetStatsByOS returns stats grouped by operating system
func GetStatsByOS(db *sql.DB) (map[string]int, error) {
	query := `
    SELECT os, COUNT(*) as count
    FROM request_logs
    GROUP BY os
    ORDER BY count DESC
    `

	rows, err := db.Query(query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	stats := make(map[string]int)
	for rows.Next() {
		var os string
		var count int
		if err := rows.Scan(&os, &count); err != nil {
			return nil, err
		}
		stats[os] = count
	}

	return stats, rows.Err()
}

// GetStatsByDevice returns stats grouped by device type
func GetStatsByDevice(db *sql.DB) (map[string]int, error) {
	query := `
    SELECT device, COUNT(*) as count
    FROM request_logs
    GROUP BY device
    ORDER BY count DESC
    `

	rows, err := db.Query(query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	stats := make(map[string]int)
	for rows.Next() {
		var device string
		var count int
		if err := rows.Scan(&device, &count); err != nil {
			return nil, err
		}
		stats[device] = count
	}

	return stats, rows.Err()
}

// UpdateLogsLocationByIP updates the location field for logs with a specific IP
func UpdateLogsLocationByIP(db *sql.DB, ip string, location string) (int64, error) {
	if db == nil {
		return 0, nil
	}
	// Only update rows where location is NULL or empty string — never overwrite existing location info.
	query := `UPDATE request_logs SET location = $1 WHERE ip = $2 AND (location IS NULL OR location = '')`
	res, err := db.Exec(query, location, ip)
	if err != nil {
		return 0, err
	}
	return res.RowsAffected()
}

// GetLogsByProject retrieves logs for a specific project/website
func GetLogsByProject(db *sql.DB, projectID string, limit int) ([]RequestLog, error) {
	query := `
    SELECT id, timestamp, project_id, ip, method, scheme, proto, path, query, status, size, duration,
           browser, browser_ver, engine, os, device, device_model, cpu_arch, is_bot,
           user_agent, location, referer, accept_lang, accept_enc, content_type,
           content_len, host, tls, request_id, created_at
    FROM request_logs
    WHERE project_id = $1
    ORDER BY created_at DESC
    LIMIT $2
    `

	return scanLogs(db.Query(query, projectID, limit))
}

// scanLogs is a helper to scan rows into RequestLog slices
func scanLogs(rows *sql.Rows, err error) ([]RequestLog, error) {
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var logs []RequestLog
	for rows.Next() {
		log := RequestLog{}
		var durationNano int64

		err := rows.Scan(
			&log.ID, &log.Timestamp, &log.ProjectID, &log.IP, &log.Method, &log.Scheme, &log.Proto, &log.Path,
			&log.Query, &log.StatusPtr, &log.Size, &durationNano, &log.Browser, &log.BrowserVer,
			&log.Engine, &log.OS, &log.Device, &log.DeviceModel, &log.CPUArch, &log.IsBot,
			&log.UserAgent, &log.Location, &log.Referer, &log.AcceptLang, &log.AcceptEnc,
			&log.ContentType, &log.ContentLen, &log.Host, &log.TLS, &log.RequestID, &log.CreatedAt,
		)

		if err != nil {
			return nil, err
		}

		log.Duration = time.Duration(durationNano)
		logs = append(logs, log)
	}

	return logs, rows.Err()
}
