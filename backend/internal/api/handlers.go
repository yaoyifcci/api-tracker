package api

import (
	"context"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/yaoyifcci/api-tracker/internal/config"
	"github.com/yaoyifcci/api-tracker/internal/storage"
)

type API struct {
	store *storage.Store
	cfg   *config.Config
}

func NewAPI(store *storage.Store, cfg *config.Config) *API {
	return &API{store: store, cfg: cfg}
}

// parseTime accepts either Unix milliseconds or an RFC3339 timestamp.
func parseTime(v string) (time.Time, bool) {
	if v == "" {
		return time.Time{}, false
	}
	if ms, err := strconv.ParseInt(v, 10, 64); err == nil {
		return time.UnixMilli(ms), true
	}
	if t, err := time.Parse(time.RFC3339, v); err == nil {
		return t, true
	}
	return time.Time{}, false
}

func (a *API) ListRequests(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "20"))

	var f storage.ListFilter
	f.Provider = c.Query("provider")
	if v, err := strconv.Atoi(c.Query("status_code")); err == nil {
		f.StatusCode = v
	}
	switch c.Query("status_class") {
	case "2xx", "3xx", "4xx", "5xx":
		f.StatusClass = c.Query("status_class")
	}
	if t, ok := parseTime(c.Query("start_time")); ok {
		f.StartTime = t
	}
	if t, ok := parseTime(c.Query("end_time")); ok {
		f.EndTime = t
	}
	f.SessionID = c.Query("session_id")

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	result, err := a.store.List(ctx, f, page, limit)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, result)
}

// ListEndpoints returns the configured endpoint names for use as filter options.
func (a *API) ListEndpoints(c *gin.Context) {
	out := make([]gin.H, 0, len(a.cfg.Endpoints))
	for i := range a.cfg.Endpoints {
		ep := &a.cfg.Endpoints[i]
		out = append(out, gin.H{"name": ep.Name, "type": ep.ResolvedType()})
	}
	c.JSON(http.StatusOK, gin.H{"data": out})
}

func (a *API) GetRequest(c *gin.Context) {
	id := c.Param("id")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	r, err := a.store.GetByID(ctx, id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	if r == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "not found"})
		return
	}
	c.JSON(http.StatusOK, r)
}

func (a *API) GetStats(c *gin.Context) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	result, err := a.store.GetStats(ctx)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, result)
}
