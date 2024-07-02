package ai

import (
	"encoding/json"
	"errors"
)

type Tool struct {
	Name         string
	Description  string
	Schema       any
	IsRawMessage bool
	HTTPMethod   string
	Resolver     func(name string, context ConversationContext, argsGetter ToolArgumentGetter) (string, error)
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

func NewNoTools() ToolStore {
	return ToolStore{
		tools:   make(map[string]Tool),
		log:     nil,
		doTrace: false,
	}
}

func NewToolStore(log TraceLog, doTrace bool) ToolStore {
	return ToolStore{
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

func (s *ToolStore) ResolveTool(name string, argsGetter ToolArgumentGetter, context ConversationContext) (string, error) {
	tool, ok := s.tools[name]
	if !ok {
		s.TraceUnknown(name, argsGetter)
		return "", errors.New("unknown tool " + name)
	}
	if tool.Resolver == nil {
		return "", errors.New("Tool resolver IS NIL")
	}
  results, err := tool.Resolver(name, context, argsGetter)
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
		var raw json.RawMessage
		argsGetter(raw)
		s.log.Info("unknown tool called", "name", name, "args", string(raw))
	}
}

func (s *ToolStore) TraceResolved(name string, argsGetter ToolArgumentGetter, result string) {
	if s.log != nil && s.doTrace {
		var raw json.RawMessage
		argsGetter(raw)
		s.log.Info("tool resolved", "name", name, "args", string(raw), "result", result)
	}
}
