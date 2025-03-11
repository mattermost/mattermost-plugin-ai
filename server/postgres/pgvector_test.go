// Copyright (c) 2023-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package postgres

import (
	"context"
	"fmt"
	"os"
	"testing"

	"github.com/jmoiron/sqlx"
	_ "github.com/lib/pq"
	"github.com/mattermost/mattermost/server/public/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/mattermost/mattermost-plugin-ai/server/embeddings"
)

// These tests require PostgreSQL with pgvector extension installed.
// Tests will fail if the database connection fails or if pgvector is not available.

// testDB creates a test database and returns a connection to it.
// This function will automatically create a temporary database for testing.
// If PG_ROOT_DSN environment variable is set, it will be used as the root connection.
// Default: "postgres://root:mostest@localhost:5432/postgres?sslmode=disable"
var rootDSN = "postgres://mmuser:mostest@localhost:5432/postgres?sslmode=disable"

func testDB(t *testing.T) *sqlx.DB {
	rootDB, err := sqlx.Connect("postgres", rootDSN)
	require.NoError(t, err, "Failed to connect to PostgreSQL. Is PostgreSQL running?")
	defer rootDB.Close()

	// Check if pgvector extension is available
	var hasVector bool
	err = rootDB.Get(&hasVector, "SELECT EXISTS(SELECT 1 FROM pg_available_extensions WHERE name = 'vector')")
	require.NoError(t, err, "Failed to check for vector extension")
	require.True(t, hasVector, "pgvector extension not available in PostgreSQL. Please install it to run these tests.")

	// Create a unique database name with a timestamp
	dbName := fmt.Sprintf("pgvector_test_%d", model.GetMillis())

	// Create the test database
	_, err = rootDB.Exec("CREATE DATABASE " + dbName)
	require.NoError(t, err, "Failed to create test database")
	t.Logf("Created test database: %s", dbName)

	// Connect to the new database
	testDSN := fmt.Sprintf("postgres://mmuser:mostest@localhost:5432/%s?sslmode=disable", dbName)
	db, err := sqlx.Connect("postgres", testDSN)
	if err != nil {
		// Try to clean up the database even if connection fails
		_, _ = rootDB.Exec("DROP DATABASE " + dbName)
		require.NoError(t, err, "Failed to connect to test database")
	}

	// Store the database name for cleanup
	t.Setenv("PGVECTOR_TEST_DB", dbName)

	// Enable the pgvector extension
	_, err = db.Exec("CREATE EXTENSION IF NOT EXISTS vector")
	if err != nil {
		db.Close()
		dropTestDB(t)
		require.NoError(t, err, "Failed to create vector extension in test database")
	}

	// Create a mock Posts table just for tests to satisfy the foreign key constraint
	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS Posts (
			Id TEXT PRIMARY KEY,
			CreateAt BIGINT NOT NULL
		)
	`)
	if err != nil {
		db.Close()
		dropTestDB(t)
		require.NoError(t, err, "Failed to create Posts table")
	}

	return db
}

// dropTestDB drops the temporary test database
func dropTestDB(t *testing.T) {
	dbName := os.Getenv("PGVECTOR_TEST_DB")
	if dbName == "" {
		return
	}

	rootDB, err := sqlx.Connect("postgres", rootDSN)
	require.NoError(t, err, "Failed to connect to PostgreSQL to drop test database")
	defer rootDB.Close()

	// Drop the test database
	if !t.Failed() {
		_, err = rootDB.Exec("DROP DATABASE " + dbName)
		require.NoError(t, err, "Failed to drop test database")
	}
}

// cleanupDB cleans up test database state and drops the database
func cleanupDB(t *testing.T, db *sqlx.DB) {
	if db == nil {
		return
	}

	err := db.Close()
	require.NoError(t, err, "Failed to close database connection")

	dropTestDB(t)
}

// addTestPosts adds test posts to the Posts table
func addTestPosts(t *testing.T, db *sqlx.DB, postIDs []string, createAts []int64) {
	for i, postID := range postIDs {
		_, err := db.Exec("INSERT INTO Posts (Id, CreateAt) VALUES ($1, $2) ON CONFLICT (Id) DO NOTHING",
			postID, createAts[i])
		require.NoError(t, err, "Failed to insert test post")
	}
}

func TestNewPGVector(t *testing.T) {
	t.Run("successfully creates PGVector instance and table", func(t *testing.T) {
		db := testDB(t)
		defer cleanupDB(t, db)

		config := PGVectorConfig{
			Dimensions: 1536,
		}

		pgVector, err := NewPGVector(db, config)
		require.NoError(t, err)
		assert.NotNil(t, pgVector)

		// Verify the table was created
		var count int
		err = db.Get(&count, "SELECT COUNT(*) FROM information_schema.tables WHERE table_name = 'llm_posts_embeddings'")
		require.NoError(t, err)
		assert.Equal(t, 1, count)
	})
}

func TestStore(t *testing.T) {
	t.Run("successfully stores documents and their embeddings", func(t *testing.T) {
		db := testDB(t)
		defer cleanupDB(t, db)

		// Set up PGVector
		config := PGVectorConfig{
			Dimensions: 3, // Small dimensions for test
		}
		pgVector, err := NewPGVector(db, config)
		require.NoError(t, err)

		// Create test data
		now := model.GetMillis()
		postIDs := []string{"post1", "post2"}
		createAts := []int64{now, now}
		addTestPosts(t, db, postIDs, createAts)

		docs := []embeddings.PostDocument{
			{
				Post: &model.Post{
					Id:       "post1",
					CreateAt: now,
				},
				TeamID:    "team1",
				ChannelID: "channel1",
				UserID:    "user1",
				Content:   "This is test content 1",
			},
			{
				Post: &model.Post{
					Id:       "post2",
					CreateAt: now,
				},
				TeamID:    "team1",
				ChannelID: "channel2",
				UserID:    "user2",
				Content:   "This is test content 2",
			},
		}

		embedVectors := [][]float32{
			{0.1, 0.2, 0.3},
			{0.4, 0.5, 0.6},
		}

		ctx := context.Background()

		// Store the documents
		err = pgVector.Store(ctx, docs, embedVectors)
		require.NoError(t, err)

		// Verify documents were stored
		var count int
		err = db.Get(&count, "SELECT COUNT(*) FROM llm_posts_embeddings")
		require.NoError(t, err)
		assert.Equal(t, 2, count)
	})
}

func TestStoreUpdate(t *testing.T) {
	t.Run("updates existing document when storing with same ID", func(t *testing.T) {
		db := testDB(t)
		defer cleanupDB(t, db)

		// Set up PGVector
		config := PGVectorConfig{
			Dimensions: 3, // Small dimensions for test
		}
		pgVector, err := NewPGVector(db, config)
		require.NoError(t, err)

		// Create test data
		now := model.GetMillis()
		postIDs := []string{"post1"}
		createAts := []int64{now}
		addTestPosts(t, db, postIDs, createAts)

		// First document version
		docs1 := []embeddings.PostDocument{
			{
				Post: &model.Post{
					Id:       "post1",
					CreateAt: now,
				},
				TeamID:    "team1",
				ChannelID: "channel1",
				UserID:    "user1",
				Content:   "Original content",
			},
		}

		embedVectors1 := [][]float32{
			{0.1, 0.2, 0.3},
		}

		// Updated document version
		docs2 := []embeddings.PostDocument{
			{
				Post: &model.Post{
					Id:       "post1",
					CreateAt: now,
				},
				TeamID:    "team1",
				ChannelID: "channel1",
				UserID:    "user1",
				Content:   "Updated content",
			},
		}

		embedVectors2 := [][]float32{
			{0.4, 0.5, 0.6},
		}

		ctx := context.Background()

		// Store the original document
		err = pgVector.Store(ctx, docs1, embedVectors1)
		require.NoError(t, err)

		// Store the updated document
		err = pgVector.Store(ctx, docs2, embedVectors2)
		require.NoError(t, err)

		// Verify we still have just one document (update instead of insert)
		var count int
		err = db.Get(&count, "SELECT COUNT(*) FROM llm_posts_embeddings")
		require.NoError(t, err)
		assert.Equal(t, 1, count)

		// Verify the content was updated
		var content string
		err = db.Get(&content, "SELECT content FROM llm_posts_embeddings WHERE post_id = 'post1'")
		require.NoError(t, err)
		assert.Equal(t, "Updated content", content)
	})
}

func TestSearch(t *testing.T) {
	// Setup test data
	setupSearchTest := func(t *testing.T) (context.Context, *PGVector, *sqlx.DB, []int64, []float32) {
		db := testDB(t)

		// Set up PGVector
		config := PGVectorConfig{
			Dimensions: 3, // Small dimensions for test
		}
		pgVector, err := NewPGVector(db, config)
		require.NoError(t, err)

		// Create test data
		now := model.GetMillis()
		postIDs := []string{"post1", "post2", "post3", "post4"}
		createAts := []int64{now - 2000, now - 1500, now - 1000, now - 500}
		addTestPosts(t, db, postIDs, createAts)

		docs := []embeddings.PostDocument{
			{
				Post: &model.Post{
					Id:       "post1",
					CreateAt: createAts[0],
				},
				TeamID:    "team1",
				ChannelID: "channel1",
				UserID:    "user1",
				Content:   "Content for team 1 channel 1",
			},
			{
				Post: &model.Post{
					Id:       "post2",
					CreateAt: createAts[1],
				},
				TeamID:    "team1",
				ChannelID: "channel2",
				UserID:    "user1",
				Content:   "Content for team 1 channel 2",
			},
			{
				Post: &model.Post{
					Id:       "post3",
					CreateAt: createAts[2],
				},
				TeamID:    "team2",
				ChannelID: "channel3",
				UserID:    "user2",
				Content:   "Content for team 2 channel 3",
			},
			{
				Post: &model.Post{
					Id:       "post4",
					CreateAt: createAts[3],
				},
				TeamID:    "team2",
				ChannelID: "channel4",
				UserID:    "user2",
				Content:   "Content for team 2 channel 4",
			},
		}

		// Create vectors with varying similarity to search vector [1, 1, 1]
		// The closer the vector is to [1, 1, 1], the higher the similarity
		embedVectors := [][]float32{
			{0.7, 0.7, 0.7}, // post1: somewhat similar
			{0.9, 0.9, 0.9}, // post2: very similar
			{0.2, 0.2, 0.2}, // post3: not very similar
			{0.5, 0.5, 0.5}, // post4: moderately similar
		}

		ctx := context.Background()

		// Store the documents
		err = pgVector.Store(ctx, docs, embedVectors)
		require.NoError(t, err)

		// Search vector
		searchVector := []float32{1.0, 1.0, 1.0}

		return ctx, pgVector, db, createAts, searchVector
	}

	t.Run("basic search with limit", func(t *testing.T) {
		ctx, pgVector, db, _, searchVector := setupSearchTest(t)
		defer cleanupDB(t, db)

		opts := embeddings.SearchOptions{
			Limit: 2,
		}

		results, err := pgVector.Search(ctx, searchVector, opts)
		require.NoError(t, err)
		assert.Len(t, results, 2)

		// Should return post2 and post1 in that order
		assert.Equal(t, "post2", results[0].Document.Post.Id)
		assert.Equal(t, "post1", results[1].Document.Post.Id)
	})

	t.Run("search with team filter", func(t *testing.T) {
		ctx, pgVector, db, _, searchVector := setupSearchTest(t)
		defer cleanupDB(t, db)

		opts := embeddings.SearchOptions{
			TeamID: "team1",
		}

		results, err := pgVector.Search(ctx, searchVector, opts)
		require.NoError(t, err)
		assert.Len(t, results, 2)
		for _, result := range results {
			assert.Equal(t, "team1", result.Document.TeamID)
		}
	})

	t.Run("search with channel filter", func(t *testing.T) {
		ctx, pgVector, db, _, searchVector := setupSearchTest(t)
		defer cleanupDB(t, db)

		opts := embeddings.SearchOptions{
			ChannelID: "channel3",
		}

		results, err := pgVector.Search(ctx, searchVector, opts)
		require.NoError(t, err)
		assert.Len(t, results, 1)
		assert.Equal(t, "post3", results[0].Document.Post.Id)
	})

	t.Run("search with min score filter", func(t *testing.T) {
		ctx, pgVector, db, _, searchVector := setupSearchTest(t)
		defer cleanupDB(t, db)

		opts := embeddings.SearchOptions{
			MinScore: 0.8, // Only include very similar vectors
		}

		results, err := pgVector.Search(ctx, searchVector, opts)
		require.NoError(t, err)
		assert.Len(t, results, 1)
		assert.Equal(t, "post2", results[0].Document.Post.Id)
	})

	t.Run("search with creation time filter", func(t *testing.T) {
		ctx, pgVector, db, createAts, searchVector := setupSearchTest(t)
		defer cleanupDB(t, db)

		opts := embeddings.SearchOptions{
			CreatedAfter: createAts[1], // After post2
		}

		results, err := pgVector.Search(ctx, searchVector, opts)
		require.NoError(t, err)
		assert.Len(t, results, 2)
		// Should contain post3 and post4
		ids := []string{results[0].Document.Post.Id, results[1].Document.Post.Id}
		assert.Contains(t, ids, "post3")
		assert.Contains(t, ids, "post4")
	})
}

func TestDelete(t *testing.T) {
	t.Run("successfully deletes documents by ID", func(t *testing.T) {
		db := testDB(t)
		defer cleanupDB(t, db)

		// Set up PGVector
		config := PGVectorConfig{
			Dimensions: 3, // Small dimensions for test
		}
		pgVector, err := NewPGVector(db, config)
		require.NoError(t, err)

		// Create test data
		now := model.GetMillis()
		postIDs := []string{"post1", "post2", "post3"}
		createAts := []int64{now, now, now}
		addTestPosts(t, db, postIDs, createAts)

		docs := []embeddings.PostDocument{
			{
				Post: &model.Post{
					Id:       "post1",
					CreateAt: now,
				},
				TeamID:    "team1",
				ChannelID: "channel1",
				UserID:    "user1",
				Content:   "Content 1",
			},
			{
				Post: &model.Post{
					Id:       "post2",
					CreateAt: now,
				},
				TeamID:    "team1",
				ChannelID: "channel1",
				UserID:    "user1",
				Content:   "Content 2",
			},
			{
				Post: &model.Post{
					Id:       "post3",
					CreateAt: now,
				},
				TeamID:    "team1",
				ChannelID: "channel1",
				UserID:    "user1",
				Content:   "Content 3",
			},
		}

		embedVectors := [][]float32{
			{0.1, 0.2, 0.3},
			{0.4, 0.5, 0.6},
			{0.7, 0.8, 0.9},
		}

		ctx := context.Background()

		// Store the documents
		err = pgVector.Store(ctx, docs, embedVectors)
		require.NoError(t, err)

		// Verify 3 documents were stored
		var count int
		err = db.Get(&count, "SELECT COUNT(*) FROM llm_posts_embeddings")
		require.NoError(t, err)
		assert.Equal(t, 3, count)

		// Delete 2 documents
		err = pgVector.Delete(ctx, []string{"post1", "post3"})
		require.NoError(t, err)

		// Verify only 1 document remains
		err = db.Get(&count, "SELECT COUNT(*) FROM llm_posts_embeddings")
		require.NoError(t, err)
		assert.Equal(t, 1, count)

		// Verify the remaining document is post2
		var postID string
		err = db.Get(&postID, "SELECT post_id FROM llm_posts_embeddings")
		require.NoError(t, err)
		assert.Equal(t, "post2", postID)
	})
}

func TestClear(t *testing.T) {
	t.Run("successfully clears all documents", func(t *testing.T) {
		db := testDB(t)
		defer cleanupDB(t, db)

		// Set up PGVector
		config := PGVectorConfig{
			Dimensions: 3, // Small dimensions for test
		}
		pgVector, err := NewPGVector(db, config)
		require.NoError(t, err)

		// Create test data
		now := model.GetMillis()
		postIDs := []string{"post1", "post2"}
		createAts := []int64{now, now}
		addTestPosts(t, db, postIDs, createAts)

		docs := []embeddings.PostDocument{
			{
				Post: &model.Post{
					Id:       "post1",
					CreateAt: now,
				},
				TeamID:    "team1",
				ChannelID: "channel1",
				UserID:    "user1",
				Content:   "Content 1",
			},
			{
				Post: &model.Post{
					Id:       "post2",
					CreateAt: now,
				},
				TeamID:    "team1",
				ChannelID: "channel1",
				UserID:    "user1",
				Content:   "Content 2",
			},
		}

		embedVectors := [][]float32{
			{0.1, 0.2, 0.3},
			{0.4, 0.5, 0.6},
		}

		ctx := context.Background()

		// Store the documents
		err = pgVector.Store(ctx, docs, embedVectors)
		require.NoError(t, err)

		// Verify 2 documents were stored
		var count int
		err = db.Get(&count, "SELECT COUNT(*) FROM llm_posts_embeddings")
		require.NoError(t, err)
		assert.Equal(t, 2, count)

		// Clear all documents
		err = pgVector.Clear(ctx)
		require.NoError(t, err)

		// Verify no documents remain
		err = db.Get(&count, "SELECT COUNT(*) FROM llm_posts_embeddings")
		require.NoError(t, err)
		assert.Equal(t, 0, count)
	})
}

// Stress test with a larger number of vectors
func TestStoreAndSearchMany(t *testing.T) {
	t.Run("handles large number of documents", func(t *testing.T) {
		if testing.Short() {
			t.Skip("Skipping stress test in short mode")
		}

		db := testDB(t)
		defer cleanupDB(t, db)

		// Set up PGVector
		config := PGVectorConfig{
			Dimensions: 3, // Small dimensions for test
		}
		pgVector, err := NewPGVector(db, config)
		require.NoError(t, err)

		// Create many test documents
		numDocs := 100
		docs := make([]embeddings.PostDocument, numDocs)
		embedVectors := make([][]float32, numDocs)
		postIDs := make([]string, numDocs)
		createAts := make([]int64, numDocs)
		now := model.GetMillis()

		for i := 0; i < numDocs; i++ {
			postID := fmt.Sprintf("post%d", i)
			postIDs[i] = postID
			createAts[i] = now - int64(i*100)

			docs[i] = embeddings.PostDocument{
				Post: &model.Post{
					Id:       postID,
					CreateAt: createAts[i],
				},
				TeamID:    fmt.Sprintf("team%d", i%5),
				ChannelID: fmt.Sprintf("channel%d", i%10),
				UserID:    fmt.Sprintf("user%d", i%20),
				Content:   fmt.Sprintf("Content for document %d", i),
			}

			// Create vectors with varying similarity to search vector [1, 1, 1]
			similarity := float32(i) / float32(numDocs)
			embedVectors[i] = []float32{similarity, similarity, similarity}
		}

		ctx := context.Background()

		// Add posts to the mock Posts table
		addTestPosts(t, db, postIDs, createAts)

		// Store all documents
		err = pgVector.Store(ctx, docs, embedVectors)
		require.NoError(t, err)

		// Verify all documents were stored
		var count int
		err = db.Get(&count, "SELECT COUNT(*) FROM llm_posts_embeddings")
		require.NoError(t, err)
		assert.Equal(t, numDocs, count)

		// Search vector
		searchVector := []float32{1.0, 1.0, 1.0}

		// Test search with limit
		opts := embeddings.SearchOptions{
			Limit: 10,
		}

		results, err := pgVector.Search(ctx, searchVector, opts)
		require.NoError(t, err)
		assert.Len(t, results, 10)

		// Results should be sorted by similarity (descending)
		for i := 1; i < len(results); i++ {
			assert.GreaterOrEqual(t, results[i-1].Score, results[i].Score)
		}
	})
}
