package metrics

import (
	"fmt"
	"io"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/go-chi/chi/v5"
	chimiddleware "github.com/go-chi/chi/v5/middleware"
	"github.com/martinrgarciap/vaultforge/apps/api/internal/buildinfo"
)

const (
	unmatchedRoute     = "unmatched"
	maxRoutePatternLen = 256
)

type httpMetricKey struct {
	method      string
	route       string
	statusClass string
}

type httpMetricValue struct {
	count               uint64
	durationNanoseconds uint64
}

type Registry struct {
	info      buildinfo.Info
	startedAt time.Time
	now       func() time.Time
	inFlight  atomic.Int64

	mu      sync.Mutex
	records map[httpMetricKey]httpMetricValue
}

type metricSnapshot struct {
	key   httpMetricKey
	value httpMetricValue
}

func New(info buildinfo.Info) *Registry {
	now := time.Now

	return &Registry{
		info:      buildinfo.New(info.Version, info.Commit),
		startedAt: now(),
		now:       now,
		records:   make(map[httpMetricKey]httpMetricValue),
	}
}

func (registry *Registry) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if registry == nil {
			next.ServeHTTP(w, r)
			return
		}

		startedAt := registry.now()
		registry.inFlight.Add(1)

		responseWriter := chimiddleware.NewWrapResponseWriter(w, r.ProtoMajor)

		defer func() {
			registry.inFlight.Add(-1)

			status := responseWriter.Status()
			if status == 0 {
				status = http.StatusOK
			}

			registry.record(
				normalizeMethod(r.Method),
				normalizedRoutePattern(r),
				normalizeStatusClass(status),
				registry.now().Sub(startedAt),
			)
		}()

		next.ServeHTTP(responseWriter, r)
	})
}

func (registry *Registry) ServeHTTP(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "text/plain; version=0.0.4; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")

	snapshots := registry.snapshots()

	var output strings.Builder

	output.WriteString("# HELP vaultforge_build_info Sanitized VaultForge build information.\n")
	output.WriteString("# TYPE vaultforge_build_info gauge\n")

	_, _ = fmt.Fprintf(
		&output,
		"vaultforge_build_info{version=%s,commit=%s} 1\n",
		prometheusLabel(registry.info.Version),
		prometheusLabel(registry.info.Commit),
	)

	output.WriteString("# HELP vaultforge_process_uptime_seconds Process uptime in seconds.\n")
	output.WriteString("# TYPE vaultforge_process_uptime_seconds gauge\n")

	uptime := registry.now().Sub(registry.startedAt).Seconds()
	if uptime < 0 {
		uptime = 0
	}

	_, _ = fmt.Fprintf(
		&output,
		"vaultforge_process_uptime_seconds %s\n",
		strconv.FormatFloat(uptime, 'f', 3, 64),
	)

	output.WriteString("# HELP vaultforge_http_requests_in_flight Current HTTP requests being handled.\n")
	output.WriteString("# TYPE vaultforge_http_requests_in_flight gauge\n")

	_, _ = fmt.Fprintf(&output, "vaultforge_http_requests_in_flight %d\n", registry.inFlight.Load())

	output.WriteString("# HELP vaultforge_http_requests_total Completed HTTP requests.\n")
	output.WriteString("# TYPE vaultforge_http_requests_total counter\n")

	output.WriteString("# HELP vaultforge_http_request_duration_seconds HTTP request duration.\n")
	output.WriteString("# TYPE vaultforge_http_request_duration_seconds summary\n")

	for _, snapshot := range snapshots {
		labels := metricLabels(snapshot.key)

		_, _ = fmt.Fprintf(
			&output,
			"vaultforge_http_requests_total%s %d\n",
			labels,
			snapshot.value.count,
		)

		seconds := float64(snapshot.value.durationNanoseconds) / float64(time.Second)

		_, _ = fmt.Fprintf(
			&output,
			"vaultforge_http_request_duration_seconds_sum%s %s\n",
			labels,
			strconv.FormatFloat(seconds, 'f', 6, 64),
		)

		_, _ = fmt.Fprintf(
			&output,
			"vaultforge_http_request_duration_seconds_count%s %d\n",
			labels,
			snapshot.value.count,
		)
	}

	_, _ = io.WriteString(w, output.String())
}

func (registry *Registry) record(method string, route string, statusClass string, duration time.Duration) {
	if duration < 0 {
		duration = 0
	}

	key := httpMetricKey{
		method:      method,
		route:       route,
		statusClass: statusClass,
	}

	registry.mu.Lock()
	defer registry.mu.Unlock()

	value := registry.records[key]
	value.count++
	value.durationNanoseconds += uint64(duration)
	registry.records[key] = value
}

func (registry *Registry) snapshots() []metricSnapshot {
	registry.mu.Lock()
	defer registry.mu.Unlock()

	snapshots := make([]metricSnapshot, 0, len(registry.records))

	for key, value := range registry.records {
		snapshots = append(snapshots, metricSnapshot{
			key:   key,
			value: value,
		})
	}

	sort.Slice(snapshots, func(first int, second int) bool {
		left := snapshots[first].key
		right := snapshots[second].key

		if left.method != right.method {
			return left.method < right.method
		}

		if left.route != right.route {
			return left.route < right.route
		}

		return left.statusClass < right.statusClass
	})

	return snapshots
}

func normalizedRoutePattern(r *http.Request) string {
	if r == nil {
		return unmatchedRoute
	}

	routeContext := chi.RouteContext(r.Context())
	if routeContext == nil {
		return unmatchedRoute
	}

	pattern := strings.TrimSpace(routeContext.RoutePattern())

	if pattern == "" || len(pattern) > maxRoutePatternLen {
		return unmatchedRoute
	}

	return pattern
}

func normalizeMethod(method string) string {
	switch strings.ToUpper(method) {
	case http.MethodGet:
		return http.MethodGet
	case http.MethodHead:
		return http.MethodHead
	case http.MethodPost:
		return http.MethodPost
	case http.MethodPut:
		return http.MethodPut
	case http.MethodPatch:
		return http.MethodPatch
	case http.MethodDelete:
		return http.MethodDelete
	case http.MethodConnect:
		return http.MethodConnect
	case http.MethodOptions:
		return http.MethodOptions
	case http.MethodTrace:
		return http.MethodTrace
	default:
		return "OTHER"
	}
}

func normalizeStatusClass(status int) string {
	switch {
	case status >= 100 && status <= 199:
		return "1xx"
	case status >= 200 && status <= 299:
		return "2xx"
	case status >= 300 && status <= 399:
		return "3xx"
	case status >= 400 && status <= 499:
		return "4xx"
	case status >= 500 && status <= 599:
		return "5xx"
	default:
		return "other"
	}
}

func metricLabels(key httpMetricKey) string {
	return fmt.Sprintf(
		"{method=%s,route=%s,status_class=%s}",
		prometheusLabel(key.method),
		prometheusLabel(key.route),
		prometheusLabel(key.statusClass),
	)
}

func prometheusLabel(value string) string {
	replacer := strings.NewReplacer(`\`, `\\`, "\n", `\n`, `"`, `\"`)

	return `"` + replacer.Replace(value) + `"`
}
