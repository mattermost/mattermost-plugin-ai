// Copyright (c) 2023-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package database

import (
	"fmt"

	"github.com/jmoiron/sqlx"
)

// SetupTables creates all necessary database tables for the AI plugin
func SetupTables(db *sqlx.DB) error {
	if err := createLLMPostMetaTable(db); err != nil {
		return fmt.Errorf("failed to create tables: %w", err)
	}

	if err := migrateOldTables(db); err != nil {
		return fmt.Errorf("failed to migrate old tables: %w", err)
	}

	return nil
}

// createLLMPostMetaTable creates the LLM_PostMeta table
func createLLMPostMetaTable(db *sqlx.DB) error {
	if _, err := db.Exec(`
		CREATE TABLE IF NOT EXISTS LLM_PostMeta (
			RootPostID TEXT NOT NULL REFERENCES Posts(ID) ON DELETE CASCADE PRIMARY KEY,
			Title TEXT NOT NULL
		);
	`); err != nil {
		return fmt.Errorf("can't create llm postmeta table: %w", err)
	}

	return nil
}

// migrateOldTables handles migration from older table structures
func migrateOldTables(db *sqlx.DB) error {
	// This fixes data retention issues when a post is deleted for an older version of the postmeta table.
	// Migrate from the old table using `"INSERT INTO LLM_PostMeta(RootPostID, Title) SELECT RootPostID, Title from LLM_Threads"`
	if _, err := db.Exec(`ALTER TABLE IF EXISTS LLM_Threads DROP CONSTRAINT IF EXISTS llm_threads_rootpostid_fkey;`); err != nil {
		return fmt.Errorf("failed to migrate constraint: %w", err)
	}

	return nil
}
