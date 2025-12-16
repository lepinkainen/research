package main

import (
	"net/http"
	"time"

	"github.com/labstack/echo/v5"
	"github.com/pocketbase/pocketbase"
	"github.com/pocketbase/pocketbase/apis"
	"github.com/pocketbase/pocketbase/core"
	"github.com/pocketbase/pocketbase/models"
)

func setupCustomRoutes(app *pocketbase.PocketBase, e *core.ServeEvent) error {
	// Health check endpoint
	e.Router.GET("/api/health", func(c echo.Context) error {
		return c.JSON(http.StatusOK, map[string]interface{}{
			"status":    "ok",
			"timestamp": time.Now().Format(time.RFC3339),
		})
	})

	// Get programs currently airing (what's on now)
	e.Router.GET("/api/tv/now", func(c echo.Context) error {
		now := time.Now().Format(time.RFC3339)

		records, err := app.Dao().FindRecordsByFilter(
			"programs",
			"start_time <= {:now} && end_time >= {:now}",
			"-start_time",
			100,
			0,
			map[string]any{"now": now},
		)

		if err != nil {
			return apis.NewApiError(500, "Failed to fetch programs", err)
		}

		// Expand channel relation
		expandedRecords := make([]map[string]any, 0, len(records))
		for _, record := range records {
			data := record.PublicExport()

			// Get channel info
			if channelID := record.GetString("channel"); channelID != "" {
				if channel, err := app.Dao().FindRecordById("channels", channelID); err == nil {
					data["expand"] = map[string]any{
						"channel": channel.PublicExport(),
					}
				}
			}

			expandedRecords = append(expandedRecords, data)
		}

		return c.JSON(http.StatusOK, expandedRecords)
	})

	// Get tonight's prime time programs (20:00-23:00)
	e.Router.GET("/api/tv/tonight", func(c echo.Context) error {
		today := time.Now()
		start := time.Date(today.Year(), today.Month(), today.Day(), 20, 0, 0, 0, today.Location())
		end := time.Date(today.Year(), today.Month(), today.Day(), 23, 0, 0, 0, today.Location())

		records, err := app.Dao().FindRecordsByFilter(
			"programs",
			"start_time >= {:start} && start_time <= {:end}",
			"start_time",
			200,
			0,
			map[string]any{
				"start": start.Format(time.RFC3339),
				"end":   end.Format(time.RFC3339),
			},
		)

		if err != nil {
			return apis.NewApiError(500, "Failed to fetch programs", err)
		}

		// Expand channel relation
		expandedRecords := make([]map[string]any, 0, len(records))
		for _, record := range records {
			data := record.PublicExport()

			if channelID := record.GetString("channel"); channelID != "" {
				if channel, err := app.Dao().FindRecordById("channels", channelID); err == nil {
					data["expand"] = map[string]any{
						"channel": channel.PublicExport(),
					}
				}
			}

			expandedRecords = append(expandedRecords, data)
		}

		return c.JSON(http.StatusOK, expandedRecords)
	})

	// Get schedule for a specific channel and date
	e.Router.GET("/api/tv/schedule/:channelId/:date", func(c echo.Context) error {
		channelID := c.PathParam("channelId")
		dateStr := c.PathParam("date") // Format: YYYY-MM-DD

		// Parse date
		date, err := time.Parse("2006-01-02", dateStr)
		if err != nil {
			return apis.NewBadRequestError("Invalid date format. Use YYYY-MM-DD", err)
		}

		start := time.Date(date.Year(), date.Month(), date.Day(), 0, 0, 0, 0, date.Location())
		end := start.AddDate(0, 0, 1)

		records, err := app.Dao().FindRecordsByFilter(
			"programs",
			"channel = {:channel} && start_time >= {:start} && start_time < {:end}",
			"start_time",
			200,
			0,
			map[string]any{
				"channel": channelID,
				"start":   start.Format(time.RFC3339),
				"end":     end.Format(time.RFC3339),
			},
		)

		if err != nil {
			return apis.NewApiError(500, "Failed to fetch programs", err)
		}

		// Export records
		exportedRecords := make([]map[string]any, 0, len(records))
		for _, record := range records {
			exportedRecords = append(exportedRecords, record.PublicExport())
		}

		return c.JSON(http.StatusOK, exportedRecords)
	})

	// Manual trigger for data collection (admin only)
	e.Router.POST("/api/admin/trigger/fetch", func(c echo.Context) error {
		admin, _ := c.Get(apis.ContextAdminKey).(*models.Admin)
		if admin == nil {
			return apis.NewForbiddenError("Admin authentication required", nil)
		}

		daysAhead := 7
		if days := c.QueryParam("days"); days != "" {
			// Parse days parameter if provided
			var d int
			if _, err := echo.QueryParamsBinder(c).Int("days", &d).BindError(); err == nil {
				daysAhead = d
			}
		}

		// Run in background
		go func() {
			collector := NewTVCollector(app)
			if err := collector.FetchAllPrograms(daysAhead); err != nil {
				app.Logger().Error("Manual fetch failed", "error", err)
			}
		}()

		return c.JSON(http.StatusOK, map[string]interface{}{
			"message":    "Fetch job triggered",
			"days_ahead": daysAhead,
		})
	})

	// Manual trigger for channel update (admin only)
	e.Router.POST("/api/admin/trigger/update-channels", func(c echo.Context) error {
		admin, _ := c.Get(apis.ContextAdminKey).(*models.Admin)
		if admin == nil {
			return apis.NewForbiddenError("Admin authentication required", nil)
		}

		go func() {
			collector := NewTVCollector(app)
			if err := collector.UpdateChannelList(); err != nil {
				app.Logger().Error("Channel update failed", "error", err)
			}
		}()

		return c.JSON(http.StatusOK, map[string]string{
			"message": "Channel update job triggered",
		})
	})

	// Manual trigger for cleanup (admin only)
	e.Router.POST("/api/admin/trigger/cleanup", func(c echo.Context) error {
		admin, _ := c.Get(apis.ContextAdminKey).(*models.Admin)
		if admin == nil {
			return apis.NewForbiddenError("Admin authentication required", nil)
		}

		days := 30
		if d := c.QueryParam("days"); d != "" {
			var parsedDays int
			if _, err := echo.QueryParamsBinder(c).Int("days", &parsedDays).BindError(); err == nil {
				days = parsedDays
			}
		}

		go func() {
			if err := cleanupOldData(app, days); err != nil {
				app.Logger().Error("Cleanup failed", "error", err)
			}
		}()

		return c.JSON(http.StatusOK, map[string]interface{}{
			"message": "Cleanup job triggered",
			"days":    days,
		})
	})

	// Get statistics
	e.Router.GET("/api/tv/stats", func(c echo.Context) error {
		stats := make(map[string]interface{})

		// Count programs
		programCount, err := app.Dao().DB().Select("count(*)").
			From("programs").
			Limit(1).
			Build().
			Execute()
		if err == nil {
			stats["total_programs"] = programCount
		}

		// Count channels
		channelCount, err := app.Dao().DB().Select("count(*)").
			From("channels").
			Where(app.Dao().DB().NewExp("active = {:active}", map[string]any{"active": true})).
			Limit(1).
			Build().
			Execute()
		if err == nil {
			stats["total_channels"] = channelCount
		}

		// Count series
		seriesCount, err := app.Dao().DB().Select("count(*)").
			From("series").
			Limit(1).
			Build().
			Execute()
		if err == nil {
			stats["total_series"] = seriesCount
		}

		return c.JSON(http.StatusOK, stats)
	})

	return nil
}
