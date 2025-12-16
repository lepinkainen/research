package main

import (
	"log"
	"os"

	"github.com/pocketbase/pocketbase"
	"github.com/pocketbase/pocketbase/core"
	"github.com/pocketbase/pocketbase/plugins/cron"
	"github.com/pocketbase/pocketbase/plugins/migratecmd"
)

func main() {
	app := pocketbase.New()

	// Enable auto creation of migration files
	migratecmd.MustRegister(app, app.RootCmd, migratecmd.Config{
		Automigrate: isDevMode(),
	})

	// Setup collections on first run
	app.OnBeforeServe().Add(func(e *core.ServeEvent) error {
		// Initialize collections if they don't exist
		if err := ensureCollections(app); err != nil {
			return err
		}

		return nil
	})

	// Register job scheduler
	app.OnBeforeServe().Add(func(e *core.ServeEvent) error {
		scheduler := cron.New()

		// Job 1: Fetch TV program data daily at 01:00
		scheduler.MustAdd("fetch_programs", "0 1 * * *", func() {
			log.Println("🔄 Starting nightly program data fetch...")
			collector := NewTVCollector(app)
			if err := collector.FetchAllPrograms(7); err != nil {
				log.Printf("❌ Program fetch failed: %v", err)
			} else {
				log.Println("✅ Program fetch completed successfully")
			}
		})

		// Job 2: Clean up old programs daily at 02:00
		scheduler.MustAdd("cleanup_old_data", "0 2 * * *", func() {
			log.Println("🧹 Starting data cleanup...")
			if err := cleanupOldData(app, 30); err != nil {
				log.Printf("❌ Cleanup failed: %v", err)
			} else {
				log.Println("✅ Cleanup completed successfully")
			}
		})

		// Job 3: Update channel list weekly (Sunday at 03:00)
		scheduler.MustAdd("update_channels", "0 3 * * 0", func() {
			log.Println("📡 Updating channel list...")
			collector := NewTVCollector(app)
			if err := collector.UpdateChannelList(); err != nil {
				log.Printf("❌ Channel update failed: %v", err)
			} else {
				log.Println("✅ Channel list updated successfully")
			}
		})

		scheduler.Start()

		log.Println("✅ Job scheduler started:")
		log.Println("   - fetch_programs: Daily at 01:00")
		log.Println("   - cleanup_old_data: Daily at 02:00")
		log.Println("   - update_channels: Weekly on Sunday at 03:00")

		return nil
	})

	// Add custom API endpoints
	app.OnBeforeServe().Add(func(e *core.ServeEvent) error {
		return setupCustomRoutes(app, e)
	})

	if err := app.Start(); err != nil {
		log.Fatal(err)
	}
}

func isDevMode() bool {
	return os.Getenv("ENV") == "development"
}
