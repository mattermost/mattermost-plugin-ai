package ai

import (
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
	tools map[string]Tool
}

func NewToolStore() ToolStore {
	return ToolStore{
		tools: make(map[string]Tool),
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
		return "", errors.New("unknown tool " + name)
	}
	if tool.Resolver == nil {
		return "", errors.New("Tool resolver IS NIL")
	}
	return tool.Resolver(name, context, argsGetter)
}

func (s *ToolStore) GetTools() []Tool {
	result := make([]Tool, 0, len(s.tools))
	for _, tool := range s.tools {
		result = append(result, tool)
	}
	return result
}
