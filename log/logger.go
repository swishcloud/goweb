package log

import (
	"database/sql"
	"fmt"
	stdlog "log"
)

// Logger is an interface for handling request logs
type Logger interface {
	Log(requestLog *RequestLog) error
}

// ConsoleLogger logs directly to stdout/stderr
type ConsoleLogger struct{}

func (cl *ConsoleLogger) Log(requestLog *RequestLog) error {
	statusStr := "null"
	if requestLog.StatusPtr != nil {
		statusStr = fmt.Sprintf("%d", *requestLog.StatusPtr)
	}

	stdlog.Printf(
		"time=%s project_id=%s ip=%s method=%s scheme=%s proto=%s path=%s query=%q status=%s size=%d duration=%s browser=%s browser_ver=%s engine=%s os=%s device=%s device_model=%s cpu_arch=%s is_bot=%v agent=%q location=%s referer=%q accept_lang=%q accept_enc=%q content_type=%q content_len=%s host=%s tls=%s request_id=%s",
		requestLog.Timestamp.Format("2006-01-02T15:04:05Z07:00"),
		requestLog.ProjectID,
		requestLog.IP,
		requestLog.Method,
		requestLog.Scheme,
		requestLog.Proto,
		requestLog.Path,
		requestLog.Query,
		statusStr,
		requestLog.Size,
		requestLog.Duration.String(),
		requestLog.Browser,
		requestLog.BrowserVer,
		requestLog.Engine,
		requestLog.OS,
		requestLog.Device,
		requestLog.DeviceModel,
		requestLog.CPUArch,
		requestLog.IsBot,
		requestLog.UserAgent,
		requestLog.Location,
		requestLog.Referer,
		requestLog.AcceptLang,
		requestLog.AcceptEnc,
		requestLog.ContentType,
		requestLog.ContentLen,
		requestLog.Host,
		requestLog.TLS,
		requestLog.RequestID,
	)
	return nil
}

// DatabaseLogger stores logs in PostgreSQL
type DatabaseLogger struct {
	db *sql.DB
}

// NewDatabaseLogger creates a new database logger instance
func NewDatabaseLogger(db *sql.DB) *DatabaseLogger {
	return &DatabaseLogger{db: db}
}

// Log stores the request log to database
func (dl *DatabaseLogger) Log(requestLog *RequestLog) error {
	return StoreLog(dl.db, requestLog)
}

// MultiLogger logs to multiple loggers simultaneously
type MultiLogger struct {
	loggers []Logger
}

// NewMultiLogger creates a logger that writes to multiple loggers
func NewMultiLogger(loggers ...Logger) *MultiLogger {
	return &MultiLogger{loggers: loggers}
}

// Log writes to all configured loggers
func (ml *MultiLogger) Log(requestLog *RequestLog) error {
	for _, logger := range ml.loggers {
		if err := logger.Log(requestLog); err != nil {
			stdlog.Printf("ERROR in logger: %v", err)
		}
	}
	return nil
}
