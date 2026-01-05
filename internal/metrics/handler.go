package metrics

import (
	"encoding/json"
	"net/http"
)

func (c *Collector) Handler(strategy string) http.HandlerFunc {
    return func(w http.ResponseWriter, r *http.Request) {
        snap := c.metrics.Snapshot(strategy)
        
        w.Header().Set("Content-Type", "application/json")
        if err := json.NewEncoder(w).Encode(snap); err != nil {
            http.Error(w, err.Error(), http.StatusInternalServerError)
            return
        }
    }
}