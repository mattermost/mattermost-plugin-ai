// Copyright (c) 2023-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/bedrockagentruntime"
	"github.com/aws/aws-sdk-go-v2/service/bedrockagentruntime/types"
)

// RetrievalResult represents a result from the knowledge base
type RetrievalResult struct {
	Content    string  `json:"content"`
	SourceKey  string  `json:"sourceKey,omitempty"`
	SourceURI  string  `json:"sourceUri,omitempty"`
	Score      float64 `json:"score,omitempty"`
	Metadata   string  `json:"metadata,omitempty"`
}

// KnowledgeBaseResponse represents a response from the knowledge base
type KnowledgeBaseResponse struct {
	Results []RetrievalResult `json:"results"`
}

// BedrockKBClient handles interactions with AWS Bedrock Knowledge Bases
type BedrockKBClient struct {
	client *bedrockagentruntime.Client
	config *configuration
}

// NewBedrockKBClient creates a new client for AWS Bedrock Knowledge Base
func NewBedrockKBClient(config *configuration) (*BedrockKBClient, error) {
	if len(config.Config.BedrockKnowledgeBases) == 0 || config.Config.BedrockKBRegion == "" {
		return nil, errors.New("bedrock knowledge base configuration not found")
	}

	// Configure AWS SDK
	awsCfg, err := awsconfig.LoadDefaultConfig(context.Background(),
		awsconfig.WithRegion(config.Config.BedrockKBRegion),
		awsconfig.WithCredentialsProvider(credentials.NewStaticCredentialsProvider(
			config.Config.BedrockKBAPIKey,
			config.Config.BedrockKBAPISecret,
			"",
		)),
	)

	if err != nil {
		return nil, fmt.Errorf("error loading AWS config: %w", err)
	}

	// Create Bedrock Agent Runtime client
	client := bedrockagentruntime.NewFromConfig(awsCfg)

	return &BedrockKBClient{
		client: client,
		config: config,
	}, nil
}

// RetrieveFromKnowledgeBase queries a knowledge base and returns relevant information
func (b *BedrockKBClient) RetrieveFromKnowledgeBase(ctx context.Context, query string, kbID string, maxResults int) (string, error) {
	// Validate knowledge base ID exists in configuration
	var kbConfig *BedrockKnowledgeBaseConfig
	for i := range b.config.Config.BedrockKnowledgeBases {
		if b.config.Config.BedrockKnowledgeBases[i].ID == kbID {
			kbConfig = &b.config.Config.BedrockKnowledgeBases[i]
			break
		}
	}

	if kbConfig == nil {
		return "", fmt.Errorf("knowledge base with ID %s not found in configuration", kbID)
	}

	if !kbConfig.Enabled {
		return "", fmt.Errorf("knowledge base with ID %s is disabled", kbID)
	}

	// Set default max results if not provided
	if maxResults <= 0 {
		maxResults = kbConfig.MaxResults
		if maxResults <= 0 {
			maxResults = 5 // Default value
		}
	}

	// Cap max results to a reasonable limit
	if maxResults > 20 {
		maxResults = 20
	}

	// Create the Bedrock Knowledge Base retrieval request
	retrieveRequest := &bedrockagentruntime.RetrieveInput{
		KnowledgeBaseId: aws.String(kbID),
		RetrievalQuery: &types.KnowledgeBaseQuery{
			Text: aws.String(query),
		},
	}
	
	// For now, skip the retrieval configuration as the AWS SDK is evolving
	// We'll use the default configuration provided by AWS
	
	// Log the query (for debugging)
	fmt.Printf("Executing Bedrock KB Query: query=%s, kb=%s, maxResults=%d\n", query, kbID, maxResults)
	
	// Execute the API call
	retrieveResponse, err := b.client.Retrieve(ctx, retrieveRequest)
	if err != nil {
		return "", fmt.Errorf("failed to retrieve from knowledge base: %w", err)
	}
	
	// Process the results
	response := KnowledgeBaseResponse{
		Results: []RetrievalResult{},
	}
	
	// Convert AWS response to our internal structure
	for _, retrievalResult := range retrieveResponse.RetrievalResults {
		// Get content text
		var content string
		if retrievalResult.Content != nil && retrievalResult.Content.Text != nil {
			content = *retrievalResult.Content.Text
		}
		
		result := RetrievalResult{
			Content: content,
		}
		
		// Add metadata if available
		if retrievalResult.Location != nil && retrievalResult.Location.S3Location != nil {
			if retrievalResult.Location.S3Location.Uri != nil {
				result.SourceURI = *retrievalResult.Location.S3Location.Uri
			}
			// Get source key from URI
			if result.SourceURI != "" {
				// Extract filename from S3 URI
				parts := strings.Split(result.SourceURI, "/")
				if len(parts) > 0 {
					result.SourceKey = parts[len(parts)-1]
				}
			}
		}
		
		// Add score if available
		if retrievalResult.Score != nil {
			result.Score = *retrievalResult.Score
		}
		
		// For the simplified implementation, we'll just add basic metadata
		result.Metadata = fmt.Sprintf("Knowledge Base: %s, Query Time: %s", kbID, time.Now().Format(time.RFC3339))
		
		response.Results = append(response.Results, result)
	}

	// Format the results
	var formattedResults string
	for _, result := range response.Results {
		formattedResults += fmt.Sprintf("--- Result ---\n")
		formattedResults += fmt.Sprintf("Content: %s\n", result.Content)
		
		if result.SourceKey != "" || result.SourceURI != "" {
			formattedResults += fmt.Sprintf("Source: %s, URI: %s\n", result.SourceKey, result.SourceURI)
		}
		
		if result.Score > 0 {
			formattedResults += fmt.Sprintf("Relevance Score: %.2f\n", result.Score)
		}
		
		if result.Metadata != "" {
			formattedResults += fmt.Sprintf("Metadata: %s\n", result.Metadata)
		}
		
		formattedResults += "\n"
	}

	if formattedResults == "" {
		return "No relevant information found in the knowledge base.", nil
	}

	return formattedResults, nil
}

// ListAvailableKnowledgeBases returns a formatted list of available knowledge bases
func (b *BedrockKBClient) ListAvailableKnowledgeBases() string {
	if len(b.config.Config.BedrockKnowledgeBases) == 0 {
		return "No knowledge bases are configured."
	}

	// Only show knowledge bases that are enabled in the plugin configuration
	result := "Available Knowledge Bases:\n"
	for _, kb := range b.config.Config.BedrockKnowledgeBases {
		if kb.Enabled {
			result += fmt.Sprintf("- %s (ID: %s): %s\n", kb.Name, kb.ID, kb.Description)
		}
	}

	return result
}

// ListAllBedrockKnowledgeBases fetches all knowledge bases from configuration
func (b *BedrockKBClient) ListAllBedrockKnowledgeBases() []BedrockKnowledgeBaseConfig {
	// Return the configured knowledge bases
	return b.config.Config.BedrockKnowledgeBases
}

// StoreQueryResultInContext stores query and results in the context for follow-up queries
func StoreQueryResultInContext(query string, result string, customData map[string]interface{}) map[string]interface{} {
	if customData == nil {
		customData = make(map[string]interface{})
	}
	
	// Structure for storing KB queries in context
	type KBQueryEntry struct {
		Query  string `json:"query"`
		Result string `json:"result"`
	}
	
	// Get existing entries
	var storedQueries []KBQueryEntry
	
	// If there are existing entries, retrieve them
	if existingData, ok := customData["kb_queries"]; ok {
		if existingJSON, err := json.Marshal(existingData); err == nil {
			json.Unmarshal(existingJSON, &storedQueries) // Ignore error, will default to empty slice
		}
	}
	
	// Add the new entry
	storedQueries = append(storedQueries, KBQueryEntry{
		Query:  query,
		Result: result,
	})
	
	// Keep only the last 3 entries
	const maxStoredQueries = 3
	if len(storedQueries) > maxStoredQueries {
		storedQueries = storedQueries[len(storedQueries)-maxStoredQueries:]
	}
	
	// Store back in context
	customData["kb_queries"] = storedQueries
	
	return customData
}