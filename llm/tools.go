// Copyright (c) 2023-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package llm

import (
	"encoding/json"
	"errors"
	"fmt"

	"github.com/invopop/jsonschema"
)

// Tool represents a function that can be called by the language model during a conversation.
//
// Each tool has a name, description, and schema that defines its parameters. These are passed to the LLM for it to understand what capabilities it has.
// It is the Resolver function that implements the actual functionality.
//
// The Schema field should contain a JSONSchema that defines the expected structure of the tool's arguments.
// The Resolver function receives the conversation context and a way to access the parsed arguments,
// and returns either a result that will be passed to the LLM or an error.
type Tool struct {
	Name        string
	Description string
	Schema      *jsonschema.Schema
	Resolver    func(context *Context, argsGetter ToolArgumentGetter) (string, error)
}

// ToolCallStatus represents the current status of a tool call
type ToolCallStatus int

const (
	// ToolCallStatusPending indicates the tool is waiting for user approval/rejection
	ToolCallStatusPending ToolCallStatus = iota
	// ToolCallStatusAccepted indicates the user has accepted the tool call but it's not resolved yet
	ToolCallStatusAccepted
	// ToolCallStatusRejected indicates the user has rejected the tool call
	ToolCallStatusRejected
	// ToolCallStatusError indicates the tool call was accepted but errored during resolution
	ToolCallStatusError
	// ToolCallStatusSuccess indicates the tool call was accepted and resolved successfully
	ToolCallStatusSuccess
)

// ToolCall represents a tool call. An empty result indicates that the tool has not yet been resolved.
type ToolCall struct {
	ID          string          `json:"id"`
	Name        string          `json:"name"`
	Description string          `json:"description"`
	Arguments   json.RawMessage `json:"arguments"`
	Result      string          `json:"result"`
	Status      ToolCallStatus  `json:"status"`
}

type ToolArgumentGetter func(args any) error

type ToolStore struct {
	tools   map[string]Tool
	log     TraceLog
	doTrace bool
}

type TraceLog interface {
	Info(message string, keyValuePairs ...any)
}

// NewJSONSchemaFromStruct creates a JSONSchema from a Go struct using reflection
// It's a helper function for tool providers that currently define schemas as structs
func NewJSONSchemaFromStruct(schemaStruct interface{}) *jsonschema.Schema {
	reflector := jsonschema.Reflector{
		Anonymous:      true,
		ExpandedStruct: true,
	}

	return reflector.Reflect(schemaStruct)
}

func NewNoTools() *ToolStore {
	return &ToolStore{
		tools:   make(map[string]Tool),
		log:     nil,
		doTrace: false,
	}
}

func NewToolStore(log TraceLog, doTrace bool) *ToolStore {
	return &ToolStore{
		tools:   make(map[string]Tool),
		log:     log,
		doTrace: doTrace,
	}
}

func (s *ToolStore) AddTools(tools []Tool) {
	for _, tool := range tools {
		s.tools[tool.Name] = tool
	}
}

func (s *ToolStore) ResolveTool(name string, argsGetter ToolArgumentGetter, context *Context) (string, error) {
	tool, ok := s.tools[name]
	if !ok {
		s.TraceUnknown(name, argsGetter)
		return "", errors.New("unknown tool " + name)
	}
	results, err := tool.Resolver(context, argsGetter)
	s.TraceResolved(name, argsGetter, results)
	return results, err
}

func (s *ToolStore) GetTools() []Tool {
	result := make([]Tool, 0, len(s.tools))
	for _, tool := range s.tools {
		result = append(result, tool)
	}
	return result
}

func (s *ToolStore) TraceUnknown(name string, argsGetter ToolArgumentGetter) {
	if s.log != nil && s.doTrace {
		args := ""
		var raw json.RawMessage
		if err := argsGetter(&raw); err != nil {
			args = fmt.Sprintf("failed to get tool args: %v", err)
		} else {
			args = string(raw)
		}
		s.log.Info("unknown tool called", "name", name, "args", args)
	}
}

func (s *ToolStore) TraceResolved(name string, argsGetter ToolArgumentGetter, result string) {
	if s.log != nil && s.doTrace {
		args := ""
		var raw json.RawMessage
		if err := argsGetter(&raw); err != nil {
			args = fmt.Sprintf("failed to get tool args: %v", err)
		} else {
			args = string(raw)
		}
		s.log.Info("tool resolved", "name", name, "args", args, "result", result)
	}
}
