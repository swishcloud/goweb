package goweb

import (
	"compress/gzip"
	"fmt"
	"net/http"
	"strings"
	"time"
)

type Context struct {
	Engine   *Engine
	Request  *http.Request
	Writer   http.ResponseWriter
	CT       time.Time
	Signal   chan int
	Data     map[string]interface{}
	index    int
	handlers HandlersChain
	FuncMap  map[string]interface{}
	Err      error
}

func (c *Context) Next() {
	c.index++
	for c.index < len(c.handlers) {
		c.handlers[c.index](c)
		c.index++
	}
}

func (c *Context) Abort() {
	c.index = 10000000000000
}

func (c *Context) Success(data interface{}) {
	HandlerResult{Data: data}.Write(c.Writer)
}
func (c *Context) Failed(error string) {
	HandlerResult{Error: &error}.Write(c.Writer)
}

type ErrorPageFunc func(c *Context, status int, msg string)

func (c *Context) ShowErrorPage(status int, msg string) {
}

func (c *Context) String() string {
	return fmt.Sprintf("method:%s path:%s remote_ip:%s", c.Request.Method, c.Request.URL.Path, c.Request.RemoteAddr)
}

// gzipWriter wraps http.ResponseWriter and the gzip writer.
type gzipWriter struct {
	http.ResponseWriter
	gz *gzip.Writer
}

func (w *gzipWriter) Write(b []byte) (int, error) {
	if w.Header().Get("Content-Type") == "" {
		w.Header().Set("Content-Type", http.DetectContentType(b))
	}
	return w.gz.Write(b)
}

func (w *gzipWriter) WriteHeader(statusCode int) {
	w.ResponseWriter.WriteHeader(statusCode)
}

func (w *gzipWriter) Close() error {
	if w.gz != nil {
		return w.gz.Close()
	}
	return nil
}

func (w *gzipWriter) Flush() {
	// flush gzip writer first
	if w.gz != nil {
		_ = w.gz.Flush()
	}
	// then underlying flusher if available
	if f, ok := w.ResponseWriter.(http.Flusher); ok {
		f.Flush()
	}
}

// wrapGzip wraps c.Writer with a gzipWriter, sets the header and returns a cleanup
// function that should be deferred by the caller to ensure the gzip writer is closed.
func wrapGzip(c *Context) func() {
	c.Writer.Header().Set("Content-Encoding", "gzip")
	gz := gzip.NewWriter(c.Writer)
	gw := &gzipWriter{
		ResponseWriter: c.Writer,
		gz:             gz,
	}
	c.Writer = gw
	return func() {
		_ = gw.Close()
	}
}

// GzipMiddleware enables gzip compression and is compatible with RouterGroup.Use().
func GzipMiddleware(c *Context) {
	if !strings.Contains(c.Request.Header.Get("Accept-Encoding"), "gzip") {
		c.Next()
		return
	}
	cleanup := wrapGzip(c)
	defer cleanup()
	c.Next()
}

// CompressionMiddleware chooses compression (currently prefers gzip) and reuses wrapGzip.
func CompressionMiddleware(c *Context) {
	ae := c.Request.Header.Get("Accept-Encoding")
	if strings.Contains(ae, "gzip") {
		cleanup := wrapGzip(c)
		defer cleanup()
	}
	c.Next()
}
