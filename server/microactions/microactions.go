package microactions

import (
	"context"
	"fmt"
	"sync"

	"github.com/xeipuuv/gojsonschema"
)

// ActionHandler is the function type that handles an action execution
type ActionHandler func(ctx context.Context, payload map[string]any) (map[string]any, error)

// Action represents a registered action with its handler and schemas
type Action struct {
	Name         string
	Description  string         // Human readable description of what the action does
	Handler      ActionHandler
	InputSchema  map[string]any // JSON Schema for input validation
	OutputSchema map[string]any // JSON Schema for output validation
	Permissions  []string       // Required permissions to execute this action
}

// Service manages the registration and execution of actions
type Service struct {
	mu      sync.RWMutex
	actions map[string]Action
}

// New creates a new microactions service
func New() *Service {
	return &Service{
		actions: make(map[string]Action),
	}
}

// RegisterAction registers a new action with the given name, description, handler, schemas and required permissions
func (s *Service) RegisterAction(name string, description string, handler ActionHandler, inputSchema, outputSchema map[string]any, permissions []string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, exists := s.actions[name]; exists {
		return fmt.Errorf("action %s already registered", name)
	}

	// Validate that the schemas are valid JSON Schema
	if _, err := gojsonschema.NewSchema(gojsonschema.NewGoLoader(inputSchema)); err != nil {
		return fmt.Errorf("invalid input schema: %w", err)
	}

	if _, err := gojsonschema.NewSchema(gojsonschema.NewGoLoader(outputSchema)); err != nil {
		return fmt.Errorf("invalid output schema: %w", err)
	}

	s.actions[name] = Action{
		Name:         name,
		Description:  description,
		Handler:      handler,
		InputSchema:  inputSchema,
		OutputSchema: outputSchema,
		Permissions:  permissions,
	}

	return nil
}

// ExecuteAction executes a registered action with the given payload and user ID
func (s *Service) ExecuteAction(ctx context.Context, name string, payload map[string]any, userID string) (map[string]any, error) {
	s.mu.RLock()
	action, exists := s.actions[name]
	s.mu.RUnlock()

	if !exists {
		return nil, fmt.Errorf("action %s not found", name)
	}

	// Validate input payload
	inputLoader := gojsonschema.NewGoLoader(action.InputSchema)
	documentLoader := gojsonschema.NewGoLoader(payload)

	result, err := gojsonschema.Validate(inputLoader, documentLoader)
	if err != nil {
		return nil, fmt.Errorf("input validation error: %w", err)
	}

	if !result.Valid() {
		var errors []string
		for _, err := range result.Errors() {
			errors = append(errors, err.String())
		}
		return nil, fmt.Errorf("input validation failed: %v", errors)
	}

	// Add userID to context
	ctx = context.WithValue(ctx, "user_id", userID)

	// Execute the action 
	output, err := action.Handler(ctx, payload)
	if err != nil {
		return nil, fmt.Errorf("action execution error: %w", err)
	}

	// Validate output
	outputLoader := gojsonschema.NewGoLoader(action.OutputSchema)
	resultLoader := gojsonschema.NewGoLoader(output)

	result, err = gojsonschema.Validate(outputLoader, resultLoader)
	if err != nil {
		return nil, fmt.Errorf("output validation error: %w", err)
	}

	if !result.Valid() {
		var errors []string
		for _, err := range result.Errors() {
			errors = append(errors, err.String())
		}
		return nil, fmt.Errorf("output validation failed: %v", errors)
	}

	return output, nil
}

// GetAction returns a registered action by name
func (s *Service) GetAction(name string) (Action, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	action, exists := s.actions[name]
	return action, exists
}

// ListActions returns all registered actions
func (s *Service) ListActions() []Action {
	s.mu.RLock()
	defer s.mu.RUnlock()

	actions := make([]Action, 0, len(s.actions))
	for _, action := range s.actions {
		actions = append(actions, action)
	}
	return actions
}

// UnregisterAction removes an action from the service
func (s *Service) UnregisterAction(name string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, exists := s.actions[name]; !exists {
		return fmt.Errorf("action %s not found", name)
	}

	delete(s.actions, name)
	return nil
}
