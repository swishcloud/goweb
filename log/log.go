package log

import (
	"crypto/tls"
	"encoding/json"
	"io"
	stdlog "log"
	"net"
	"net/http"
	"runtime/debug"
	"strconv"
	"strings"
	"time"

	"github.com/swishcloud/goweb"
)

// LoggingMiddleware holds logging configuration and provides middleware functionality
type LoggingMiddleware struct {
	ProjectID string
	Logger    Logger
}

// NewLoggingMiddleware creates a new logging middleware with the given configuration
func NewLoggingMiddleware(projectID string, logger Logger) *LoggingMiddleware {
	return &LoggingMiddleware{
		ProjectID: projectID,
		Logger:    logger,
	}
}

// Handler returns the middleware handler function
func (lm *LoggingMiddleware) Handler(c *goweb.Context) {
	if lm.ProjectID == "" {
		stdlog.Println("WARNING: LoggingMiddleware ProjectID not set")
	}
	if lm.Logger == nil {
		stdlog.Println("WARNING: LoggingMiddleware Logger not set")
	}

	start := time.Now()
	r := c.Request

	ip := clientIP(r)
	ua := r.UserAgent()
	browser, browserVersion := detectBrowserAndVersion(ua)
	browserEngine := detectEngine(ua)
	os := detectOS(ua)
	device := detectDevice(ua)
	deviceModel := detectDeviceModel(ua)
	cpuArch := detectCPUArch(ua)
	isBot := detectBot(ua)

	location := ""

	orig := c.Writer
	rec := &httpResponseRecorder{ResponseWriter: orig}
	c.Writer = rec

	defer func() {
		var panicked interface{}
		if p := recover(); p != nil {
			panicked = p
			stdlog.Printf("PANIC in handler: %v\n%s", p, debug.Stack())
		}

		c.Writer = orig

		tlsInfo := tlsSummary(r.TLS)

		scheme := r.URL.Scheme
		if scheme == "" {
			if r.TLS != nil {
				scheme = "https"
			} else {
				scheme = "http"
			}
		}
		proto := r.Proto

		referer := r.Referer()
		acceptLang := r.Header.Get("Accept-Language")
		acceptEnc := r.Header.Get("Accept-Encoding")
		contentType := r.Header.Get("Content-Type")
		contentLength := r.Header.Get("Content-Length")
		host := r.Host
		requestID := r.Header.Get("X-Request-ID")
		userAgent := ua

		size := rec.size

		requestLog := &RequestLog{
			Timestamp:   start,
			ProjectID:   lm.ProjectID,
			IP:          ip,
			Method:      r.Method,
			Scheme:      scheme,
			Proto:       proto,
			Path:        r.URL.Path,
			Query:       r.URL.RawQuery,
			StatusPtr:   rec.status,
			Size:        size,
			Duration:    time.Since(start),
			Browser:     browser,
			BrowserVer:  browserVersion,
			Engine:      browserEngine,
			OS:          os,
			Device:      device,
			DeviceModel: deviceModel,
			CPUArch:     cpuArch,
			IsBot:       isBot,
			UserAgent:   userAgent,
			Location:    location,
			Referer:     referer,
			AcceptLang:  acceptLang,
			AcceptEnc:   acceptEnc,
			ContentType: contentType,
			ContentLen:  contentLength,
			Host:        host,
			TLS:         tlsInfo,
			RequestID:   requestID,
		}

		if lm.Logger != nil {
			go func(rl *RequestLog) {
				if err := lm.Logger.Log(rl); err != nil {
					stdlog.Printf("ERROR logging request: %v", err)
				}
			}(requestLog)
		}

		if panicked != nil {
			panic(panicked)
		}
	}()

	c.Next()
}

// httpResponseRecorder wraps http.ResponseWriter to capture status code and bytes written.
type httpResponseRecorder struct {
	http.ResponseWriter
	status *int
	size   int
}

func (h *httpResponseRecorder) Write(b []byte) (int, error) {
	n, err := h.ResponseWriter.Write(b)
	h.size += n
	return n, err
}

func (h *httpResponseRecorder) WriteHeader(statusCode int) {
	if h.status == nil {
		h.status = &statusCode
	} else {
		panic("the status code has already set")
	}
	h.ResponseWriter.WriteHeader(statusCode)
}

func clientIP(r *http.Request) string {
	// standard proxy headers
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		// X-Forwarded-For can contain multiple IPs, the left-most is the original client
		parts := strings.Split(xff, ",")
		if len(parts) > 0 {
			return strings.TrimSpace(parts[0])
		}
	}
	if xr := r.Header.Get("X-Real-IP"); xr != "" {
		return strings.TrimSpace(xr)
	}
	// fallback to remote address
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}

func detectBrowserAndVersion(ua string) (string, string) {
	// Identify browser name (same logic as before) and try to extract a version token.
	name := detectBrowser(ua)
	version := extractVersionForBrowser(name, ua)
	// fallback: try common tokens
	if version == "" {
		version = extractAnyVersion(ua)
	}
	return name, version
}

func extractVersionForBrowser(name, ua string) string {
	switch name {
	case "Opera":
		// OPR/<ver> or Opera/<ver>
		if v := extractTokenVersion("OPR/", ua); v != "" {
			return v
		}
		return extractTokenVersion("Opera/", ua)
	case "Edge":
		if v := extractTokenVersion("Edg/", ua); v != "" {
			return v
		}
		return extractTokenVersion("Edge/", ua)
	case "Chrome":
		return extractTokenVersion("Chrome/", ua)
	case "Chromium":
		return extractTokenVersion("Chromium/", ua)
	case "Safari":
		// Safari/<ver> but Chrome also contains Safari token; already filtered
		return extractTokenVersion("Version/", ua)
	case "Firefox":
		return extractTokenVersion("Firefox/", ua)
	case "Internet Explorer":
		// MSIE <ver> or rv:<ver>
		if v := extractTokenVersion("MSIE ", ua); v != "" {
			return v
		}
		return extractTokenVersion("rv:", ua)
	default:
		return ""
	}
}

func extractTokenVersion(token, ua string) string {
	idx := strings.Index(ua, token)
	if idx == -1 {
		return ""
	}
	start := idx + len(token)
	// read until non-version char
	end := start
	for end < len(ua) {
		c := ua[end]
		if (c >= '0' && c <= '9') || c == '.' || c == '_' {
			end++
			continue
		}
		break
	}
	if end > start {
		return strings.ReplaceAll(ua[start:end], "_", ".")
	}
	return ""
}

func extractAnyVersion(ua string) string {
	// try a few common tokens
	tokens := []string{"Chrome/", "Firefox/", "Safari/", "OPR/", "Edg/", "Edge/", "Version/", "MSIE ", "rv:"}
	for _, t := range tokens {
		if v := extractTokenVersion(t, ua); v != "" {
			return v
		}
	}
	return ""
}

func detectBrowser(ua string) string {
	switch {
	case strings.Contains(ua, "OPR/") || strings.Contains(ua, "Opera"):
		return "Opera"
	case strings.Contains(ua, "Edg/") || strings.Contains(ua, "Edge"):
		return "Edge"
	case strings.Contains(ua, "Chrome/") && !strings.Contains(ua, "Chromium"):
		return "Chrome"
	case strings.Contains(ua, "Chromium"):
		return "Chromium"
	case strings.Contains(ua, "Safari/") && !strings.Contains(ua, "Chrome"):
		return "Safari"
	case strings.Contains(ua, "Firefox/"):
		return "Firefox"
	case strings.Contains(ua, "MSIE") || strings.Contains(ua, "Trident/"):
		return "Internet Explorer"
	default:
		return "Unknown"
	}
}

func detectEngine(ua string) string {
	switch {
	case strings.Contains(ua, "Trident"):
		return "Trident"
	case strings.Contains(ua, "Gecko") && strings.Contains(ua, "like Gecko"):
		// "like Gecko" is common in Chromium-based browsers
		return "Blink"
	case strings.Contains(ua, "Gecko") && !strings.Contains(ua, "like Gecko"):
		return "Gecko"
	case strings.Contains(ua, "AppleWebKit"):
		if strings.Contains(ua, "Chrome") || strings.Contains(ua, "Chromium") || strings.Contains(ua, "Edg") {
			return "Blink"
		}
		return "WebKit"
	default:
		return "Unknown"
	}
}

func detectOS(ua string) string {
	switch {
	case strings.Contains(ua, "Windows NT"):
		return "Windows"
	case strings.Contains(ua, "Macintosh") || strings.Contains(ua, "Mac OS X"):
		return "macOS"
	case strings.Contains(ua, "Android"):
		return "Android"
	case strings.Contains(ua, "iPhone") || strings.Contains(ua, "iPad"):
		return "iOS"
	case strings.Contains(ua, "Linux"):
		return "Linux"
	default:
		return "Unknown"
	}
}

func detectDevice(ua string) string {
	if strings.Contains(strings.ToLower(ua), "mobile") || strings.Contains(ua, "iPhone") || strings.Contains(ua, "Android") {
		return "Mobile"
	}
	if strings.Contains(ua, "iPad") || strings.Contains(ua, "Tablet") {
		return "Tablet"
	}
	return "Desktop"
}

func detectDeviceModel(ua string) string {
	// Try to extract device model from UA string
	ua = strings.ToLower(ua)

	// iPhone models
	if strings.Contains(ua, "iphone") {
		if strings.Contains(ua, "iphone 15") {
			return "iPhone15"
		}
		if strings.Contains(ua, "iphone 14") {
			return "iPhone14"
		}
		if strings.Contains(ua, "iphone 13") {
			return "iPhone13"
		}
		return "iPhone"
	}

	// iPad models
	if strings.Contains(ua, "ipad") {
		if strings.Contains(ua, "ipad pro") {
			return "iPadPro"
		}
		if strings.Contains(ua, "ipad air") {
			return "iPadAir"
		}
		return "iPad"
	}

	// Android devices (Samsung, Google Pixel, etc.)
	if strings.Contains(ua, "samsung") {
		return "Samsung"
	}
	if strings.Contains(ua, "pixel") {
		return "GooglePixel"
	}
	if strings.Contains(ua, "oneplus") {
		return "OnePlus"
	}
	if strings.Contains(ua, "xiaomi") {
		return "Xiaomi"
	}

	return ""
}

func detectCPUArch(ua string) string {
	ua = strings.ToLower(ua)

	switch {
	case strings.Contains(ua, "arm64"):
		return "ARM64"
	case strings.Contains(ua, "armv7"):
		return "ARMv7"
	case strings.Contains(ua, "arm"):
		return "ARM"
	case strings.Contains(ua, "x86_64") || strings.Contains(ua, "amd64"):
		return "x86_64"
	case strings.Contains(ua, "x86"):
		return "x86"
	case strings.Contains(ua, "ppc64"):
		return "PPC64"
	case strings.Contains(ua, "ppc"):
		return "PPC"
	default:
		return ""
	}
}

func detectBot(ua string) bool {
	ua = strings.ToLower(ua)
	botPatterns := []string{
		"bot", "crawler", "spider", "scraper", "curl", "wget",
		"googlebot", "bingbot", "slurp", "duckduckbot", "baiduspider",
		"yandexbot", "facebookexternalhit", "twitterbot", "linkedinbot",
		"whatsapp", "telegrambot", "slackbot", "discordbot",
		"applebot", "sogoubot", "exabot", "msiebot",
	}

	for _, pattern := range botPatterns {
		if strings.Contains(ua, pattern) {
			return true
		}
	}
	return false
}

type ipAPIResponse struct {
	Status  string  `json:"status"`
	Country string  `json:"country"`
	Region  string  `json:"regionName"`
	City    string  `json:"city"`
	Lat     float64 `json:"lat"`
	Lon     float64 `json:"lon"`
	ISP     string  `json:"isp"`
	Message string  `json:"message"`
}

// fetchLocation does a best-effort geolocation lookup using ip-api.com.
// It will return a single-line summary or an empty string on failure.
func fetchLocation(ip string) string {
	if ip == "" || ip == "127.0.0.1" || ip == "::1" {
		return "local"
	}

	url := "http://ip-api.com/json/" + ip + "?fields=status,country,regionName,city,lat,lon,isp,message"
	resp, err := http.Get(url)
	if err != nil {
		return ""
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return ""
	}

	var r ipAPIResponse
	if err := json.Unmarshal(body, &r); err != nil {
		return ""
	}
	if r.Status != "success" {
		if r.Message != "" {
			return r.Message
		}
		return ""
	}
	parts := []string{}
	if r.Country != "" {
		parts = append(parts, r.Country)
	}
	if r.Region != "" {
		parts = append(parts, r.Region)
	}
	if r.City != "" {
		parts = append(parts, r.City)
	}
	if r.ISP != "" {
		parts = append(parts, "isp:"+r.ISP)
	}
	coords := ""
	if r.Lat != 0 || r.Lon != 0 {
		coords = "(" + formatFloat(r.Lat) + "," + formatFloat(r.Lon) + ")"
	}
	if coords != "" {
		parts = append(parts, coords)
	}
	return strings.Join(parts, " | ")
}

func formatFloat(f float64) string {
	// keep short form for logs
	return strconv.FormatFloat(f, 'f', 4, 64)
}

func tlsSummary(t *tls.ConnectionState) string {
	if t == nil {
		return ""
	}
	ver := tlsVersionName(t.Version)
	cipher := strconv.FormatUint(uint64(t.CipherSuite), 10)
	return ver + " cipher:" + cipher
}

func tlsVersionName(v uint16) string {
	switch v {
	case tls.VersionSSL30:
		return "SSL3.0"
	case tls.VersionTLS10:
		return "TLS1.0"
	case tls.VersionTLS11:
		return "TLS1.1"
	case tls.VersionTLS12:
		return "TLS1.2"
	case tls.VersionTLS13:
		return "TLS1.3"
	default:
		return "UNKNOWN"
	}
}
