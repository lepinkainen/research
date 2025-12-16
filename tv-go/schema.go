package main

import (
	"github.com/pocketbase/dbx"
	"github.com/pocketbase/pocketbase"
	"github.com/pocketbase/pocketbase/forms"
	"github.com/pocketbase/pocketbase/models"
	"github.com/pocketbase/pocketbase/models/schema"
	"github.com/pocketbase/pocketbase/tools/types"
)

func ensureCollections(app *pocketbase.PocketBase) error {
	// Check if collections already exist
	collections, err := app.Dao().FindCollectionsByNames("channels", "series", "programs", "fetch_logs")

	if len(collections) == 4 {
		// All collections exist
		return nil
	}

	// Create collections
	if err := createChannelsCollection(app); err != nil {
		return err
	}

	if err := createSeriesCollection(app); err != nil {
		return err
	}

	if err := createProgramsCollection(app); err != nil {
		return err
	}

	if err := createFetchLogsCollection(app); err != nil {
		return err
	}

	return nil
}

func createChannelsCollection(app *pocketbase.PocketBase) error {
	collection := &models.Collection{}
	form := forms.NewCollectionUpsert(app, collection)

	form.Name = "channels"
	form.Type = models.CollectionTypeBase
	form.Schema = schema.NewSchema(
		&schema.SchemaField{
			Name:     "name",
			Type:     schema.FieldTypeText,
			Required: true,
			Options: &schema.TextOptions{
				Min: types.Pointer(1),
				Max: types.Pointer(100),
			},
		},
		&schema.SchemaField{
			Name:     "show_order",
			Type:     schema.FieldTypeNumber,
			Required: true,
			Options: &schema.NumberOptions{
				Min: types.Pointer(0.0),
			},
		},
		&schema.SchemaField{
			Name:     "category",
			Type:     schema.FieldTypeSelect,
			Required: false,
			Options: &schema.SelectOptions{
				MaxSelect: 1,
				Values: []string{
					"public", "commercial", "sports", "movies",
					"kids", "music", "international", "documentary", "other",
				},
			},
		},
		&schema.SchemaField{
			Name:     "logo_url",
			Type:     schema.FieldTypeUrl,
			Required: false,
		},
		&schema.SchemaField{
			Name:     "active",
			Type:     schema.FieldTypeBool,
			Required: true,
		},
	)

	// API rules - public read access
	form.ListRule = types.Pointer("active = true")
	form.ViewRule = types.Pointer("")

	return form.Submit()
}

func createSeriesCollection(app *pocketbase.PocketBase) error {
	collection := &models.Collection{}
	form := forms.NewCollectionUpsert(app, collection)

	form.Name = "series"
	form.Type = models.CollectionTypeBase
	form.Schema = schema.NewSchema(
		&schema.SchemaField{
			Name:     "name",
			Type:     schema.FieldTypeText,
			Required: true,
			Options: &schema.TextOptions{
				Min: types.Pointer(1),
				Max: types.Pointer(200),
			},
		},
		&schema.SchemaField{
			Name:     "description",
			Type:     schema.FieldTypeEditor,
			Required: false,
			Options: &schema.EditorOptions{
				ConvertUrls: false,
			},
		},
		&schema.SchemaField{
			Name:     "episode_count",
			Type:     schema.FieldTypeNumber,
			Required: false,
			Options: &schema.NumberOptions{
				Min: types.Pointer(0.0),
			},
		},
		&schema.SchemaField{
			Name:     "first_seen",
			Type:     schema.FieldTypeDate,
			Required: false,
		},
		&schema.SchemaField{
			Name:     "last_seen",
			Type:     schema.FieldTypeDate,
			Required: false,
		},
		&schema.SchemaField{
			Name:     "active",
			Type:     schema.FieldTypeBool,
			Required: true,
		},
	)

	form.ListRule = types.Pointer("")
	form.ViewRule = types.Pointer("")

	return form.Submit()
}

func createProgramsCollection(app *pocketbase.PocketBase) error {
	// Get channel and series collections for relations
	channelsCollection, err := app.Dao().FindCollectionByNameOrId("channels")
	if err != nil {
		return err
	}

	seriesCollection, err := app.Dao().FindCollectionByNameOrId("series")
	if err != nil {
		return err
	}

	collection := &models.Collection{}
	form := forms.NewCollectionUpsert(app, collection)

	form.Name = "programs"
	form.Type = models.CollectionTypeBase
	form.Schema = schema.NewSchema(
		&schema.SchemaField{
			Name:     "channel",
			Type:     schema.FieldTypeRelation,
			Required: true,
			Options: &schema.RelationOptions{
				CollectionId:  channelsCollection.Id,
				CascadeDelete: true,
				MaxSelect:     types.Pointer(1),
			},
		},
		&schema.SchemaField{
			Name:     "name",
			Type:     schema.FieldTypeText,
			Required: true,
			Options: &schema.TextOptions{
				Min: types.Pointer(1),
				Max: types.Pointer(200),
			},
		},
		&schema.SchemaField{
			Name:     "episode",
			Type:     schema.FieldTypeText,
			Required: false,
			Options: &schema.TextOptions{
				Max: types.Pointer(100),
			},
		},
		&schema.SchemaField{
			Name:     "description",
			Type:     schema.FieldTypeEditor,
			Required: false,
			Options: &schema.EditorOptions{
				ConvertUrls: false,
			},
		},
		&schema.SchemaField{
			Name:     "start_time",
			Type:     schema.FieldTypeDate,
			Required: true,
		},
		&schema.SchemaField{
			Name:     "end_time",
			Type:     schema.FieldTypeDate,
			Required: true,
		},
		&schema.SchemaField{
			Name:     "duration",
			Type:     schema.FieldTypeNumber,
			Required: true,
			Options: &schema.NumberOptions{
				Min: types.Pointer(0.0),
			},
		},
		&schema.SchemaField{
			Name:     "series",
			Type:     schema.FieldTypeRelation,
			Required: false,
			Options: &schema.RelationOptions{
				CollectionId:  seriesCollection.Id,
				CascadeDelete: false,
				MaxSelect:     types.Pointer(1),
			},
		},
		&schema.SchemaField{
			Name:     "age_limit",
			Type:     schema.FieldTypeNumber,
			Required: true,
			Options: &schema.NumberOptions{
				Min: types.Pointer(0.0),
			},
		},
		&schema.SchemaField{
			Name:     "rating",
			Type:     schema.FieldTypeNumber,
			Required: false,
			Options: &schema.NumberOptions{
				Min: types.Pointer(0.0),
			},
		},
		&schema.SchemaField{
			Name:     "is_series",
			Type:     schema.FieldTypeBool,
			Required: true,
		},
	)

	// Create indexes for performance
	form.Indexes = types.JsonArray[string]{
		"CREATE INDEX idx_programs_channel ON programs (channel)",
		"CREATE INDEX idx_programs_start_time ON programs (start_time)",
		"CREATE INDEX idx_programs_end_time ON programs (end_time)",
		"CREATE INDEX idx_programs_name ON programs (name)",
		"CREATE INDEX idx_programs_series ON programs (series)",
	}

	form.ListRule = types.Pointer("")
	form.ViewRule = types.Pointer("")

	return form.Submit()
}

func createFetchLogsCollection(app *pocketbase.PocketBase) error {
	channelsCollection, err := app.Dao().FindCollectionByNameOrId("channels")
	if err != nil {
		return err
	}

	collection := &models.Collection{}
	form := forms.NewCollectionUpsert(app, collection)

	form.Name = "fetch_logs"
	form.Type = models.CollectionTypeBase
	form.Schema = schema.NewSchema(
		&schema.SchemaField{
			Name:     "channel",
			Type:     schema.FieldTypeRelation,
			Required: false,
			Options: &schema.RelationOptions{
				CollectionId:  channelsCollection.Id,
				CascadeDelete: false,
				MaxSelect:     types.Pointer(1),
			},
		},
		&schema.SchemaField{
			Name:     "target_date",
			Type:     schema.FieldTypeText,
			Required: true,
			Options: &schema.TextOptions{
				Min: types.Pointer(8),
				Max: types.Pointer(8),
				Pattern: `^\d{8}$`,
			},
		},
		&schema.SchemaField{
			Name:     "success",
			Type:     schema.FieldTypeBool,
			Required: true,
		},
		&schema.SchemaField{
			Name:     "programs_count",
			Type:     schema.FieldTypeNumber,
			Required: false,
			Options: &schema.NumberOptions{
				Min: types.Pointer(0.0),
			},
		},
		&schema.SchemaField{
			Name:     "error_message",
			Type:     schema.FieldTypeText,
			Required: false,
			Options: &schema.TextOptions{
				Max: types.Pointer(500),
			},
		},
		&schema.SchemaField{
			Name:     "duration_ms",
			Type:     schema.FieldTypeNumber,
			Required: false,
			Options: &schema.NumberOptions{
				Min: types.Pointer(0.0),
			},
		},
	)

	form.Indexes = types.JsonArray[string]{
		"CREATE INDEX idx_fetch_logs_target_date ON fetch_logs (target_date)",
		"CREATE INDEX idx_fetch_logs_channel ON fetch_logs (channel)",
	}

	return form.Submit()
}

func cleanupOldData(app *pocketbase.PocketBase, daysOld int) error {
	// Delete old programs
	_, err := app.Dao().DB().NewQuery(`
		DELETE FROM programs
		WHERE datetime(start_time) < datetime('now', '-' || {:days} || ' days')
	`).Bind(dbx.Params{
		"days": daysOld,
	}).Execute()

	if err != nil {
		return err
	}

	// Delete old fetch logs
	_, err = app.Dao().DB().NewQuery(`
		DELETE FROM fetch_logs
		WHERE datetime(created) < datetime('now', '-' || {:days} || ' days')
	`).Bind(dbx.Params{
		"days": daysOld,
	}).Execute()

	return err
}
