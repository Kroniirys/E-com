package metrics

import (
	"fmt"
	"net/http"
	"sort"
	"strings"
	"sync"
	"time"

	"gorm.io/gorm"
)

var mu sync.Mutex
var values = map[string]float64{}

func Add(name string, value float64) { mu.Lock(); values[name] += value; mu.Unlock() }
func HTTP(route string, status int, elapsed time.Duration) {
	route = strings.ReplaceAll(route, "\"", "")
	Add(fmt.Sprintf("ecom_http_requests_total{route=%q,status=%q}", route, fmt.Sprint(status)), 1)
	Add(fmt.Sprintf("ecom_http_duration_seconds_sum{route=%q}", route), elapsed.Seconds())
	Add(fmt.Sprintf("ecom_http_duration_seconds_count{route=%q}", route), 1)
}
func Handler(db *gorm.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain; version=0.0.4")
		mu.Lock()
		keys := make([]string, 0, len(values))
		for k := range values {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, k := range keys {
			fmt.Fprintf(w, "%s %g\n", k, values[k])
		}
		mu.Unlock()
		pool, e := db.DB()
		if e == nil {
			stats := pool.Stats()
			fmt.Fprintf(w, "ecom_db_open_connections %d\necom_db_in_use_connections %d\n", stats.OpenConnections, stats.InUse)
		}
		var row struct {
			Pending int64
			Age     float64
		}
		if e := db.WithContext(r.Context()).Raw("SELECT count(*) AS pending, COALESCE(EXTRACT(EPOCH FROM now()-min(occurred_at)),0) AS age FROM outbox_events WHERE published_at IS NULL").Scan(&row).Error; e == nil {
			fmt.Fprintf(w, "ecom_outbox_pending %d\necom_outbox_oldest_seconds %g\n", row.Pending, row.Age)
		}
	}
}
