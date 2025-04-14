// Copyright (c) 2023-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package llm

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// MockToolResolver provides a mock implementation for testing
type MockToolResolver struct {
	mock.Mock
}

func (m *MockToolResolver) Resolve(ctx *Context, argsGetter ToolArgumentGetter) (string, error) {
	called := m.Called(ctx, argsGetter)
	return called.String(0), called.Error(1)
}

// MockLogger is a simple mock for testing
type MockLogger struct {
	mock.Mock
}

func (m *MockLogger) Info(msg string, keyvals ...interface{}) {
	m.Called(msg, keyvals)
}

// Test direct tool resolution
func TestResolveTool(t *testing.T) {
	// Set up mock resolver and store
	mockResolver := &MockToolResolver{}
	toolStore := NewToolStore(nil, true)

	// Add the mock tool to the store
	toolStore.AddTools([]Tool{
		{
			Name:        "test_tool",
			Description: "Test tool for unit testing",
			Resolver:    mockResolver.Resolve,
		},
	})

	// Set up expected arguments and return values
	expectedResult := "success result"

	// Mock the execution of the tool - it should be called with any Context and ToolArgumentGetter
	mockResolver.On("Resolve", mock.Anything, mock.Anything).Return(expectedResult, nil)

	// Create a raw argument to pass to the tool
	rawArgs := json.RawMessage(`{"param":"value"}`)
	argsGetter := func(args any) error {
		*(args.(*json.RawMessage)) = rawArgs
		return nil
	}

	// Call ResolveTool directly
	ctx := &Context{}
	result, err := toolStore.ResolveTool("test_tool", argsGetter, ctx)

	// Assertions
	assert.NoError(t, err, "ResolveTool should not return an error")
	assert.Equal(t, expectedResult, result, "Result should match the expected value")
	mockResolver.AssertExpectations(t)
}

// Test tool call status enum
func TestToolCallStatus(t *testing.T) {
	// Create a tool call with default (Pending) status
	toolCall := ToolCall{
		ID:          "test_id",
		Name:        "test_tool",
		Description: "Test tool",
		Arguments:   json.RawMessage(`{"param":"value"}`),
	}

	// Verify default status is Pending
	assert.Equal(t, ToolCallStatusPending, toolCall.Status, "Default status should be Pending")

	// Test changing to Accepted
	toolCall.Status = ToolCallStatusAccepted
	assert.Equal(t, ToolCallStatusAccepted, toolCall.Status, "Status should be Accepted")

	// Test changing to Rejected
	toolCall.Status = ToolCallStatusRejected
	assert.Equal(t, ToolCallStatusRejected, toolCall.Status, "Status should be Rejected")

	// Test changing to Error
	toolCall.Status = ToolCallStatusError
	assert.Equal(t, ToolCallStatusError, toolCall.Status, "Status should be Error")

	// Test changing to Success
	toolCall.Status = ToolCallStatusSuccess
	assert.Equal(t, ToolCallStatusSuccess, toolCall.Status, "Status should be Success")
}
