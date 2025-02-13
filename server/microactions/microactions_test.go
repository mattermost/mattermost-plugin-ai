package microactions

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewService(t *testing.T) {
	s := New()
	require.NotNil(t, s)
	require.NotNil(t, s.actions)
}

func TestRegisterAction(t *testing.T) {
	s := New()

	handler := func(ctx context.Context, payload map[string]any) (map[string]any, error) {
		return payload, nil
	}

	inputSchema := map[string]any{
		"type": "object",
		"required": []string{"test"},
		"properties": map[string]any{
			"test": map[string]any{
				"type": "string",
			},
		},
	}

	outputSchema := map[string]any{
		"type": "object",
		"properties": map[string]any{
			"test": map[string]any{
				"type": "string",
			},
		},
	}

	t.Run("successful registration", func(t *testing.T) {
		err := s.RegisterAction("test_action", "Test action description", handler, inputSchema, outputSchema, []string{"permission1"})
		require.NoError(t, err)

		action, exists := s.GetAction("test_action")
		require.True(t, exists)
		assert.Equal(t, "test_action", action.Name)
		assert.NotNil(t, action.Handler)
		assert.Equal(t, inputSchema, action.InputSchema)
		assert.Equal(t, outputSchema, action.OutputSchema)
		assert.Equal(t, []string{"permission1"}, action.Permissions)
	})

	t.Run("duplicate registration", func(t *testing.T) {
		err := s.RegisterAction("test_action", "Test action description", handler, inputSchema, outputSchema, []string{"permission1"})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "already registered")
	})

	t.Run("invalid input schema", func(t *testing.T) {
		err := s.RegisterAction("invalid_schema", "Invalid schema test", handler, map[string]any{
			"type": "invalid",
		}, outputSchema, []string{})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "invalid input schema")
	})
}

func TestExecuteAction(t *testing.T) {
	s := New()
	ctx := context.Background()

	handler := func(ctx context.Context, payload map[string]any) (map[string]any, error) {
		return map[string]any{
			"test": payload["test"],
		}, nil
	}

	inputSchema := map[string]any{
		"type": "object",
		"required": []string{"test"},
		"properties": map[string]any{
			"test": map[string]any{
				"type": "string",
			},
		},
	}

	outputSchema := map[string]any{
		"type": "object",
		"required": []string{"test"},
		"properties": map[string]any{
			"test": map[string]any{
				"type": "string",
			},
		},
	}

	err := s.RegisterAction("test_action", "Test action description", handler, inputSchema, outputSchema, []string{})
	require.NoError(t, err)

	t.Run("successful execution", func(t *testing.T) {
		result, err := s.ExecuteAction(ctx, "test_action", map[string]any{
			"test": "value",
		}, "system")
		require.NoError(t, err)
		assert.Equal(t, "value", result["test"])
	})

	t.Run("action not found", func(t *testing.T) {
		_, err := s.ExecuteAction(ctx, "non_existent", map[string]any{}, "system")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "not found")
	})

	t.Run("invalid input", func(t *testing.T) {
		_, err := s.ExecuteAction(ctx, "test_action", map[string]any{
			"test": 123,
		}, "system")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "validation failed")
	})
}

func TestUnregisterAction(t *testing.T) {
	s := New()
	handler := func(ctx context.Context, payload map[string]any) (map[string]any, error) {
		return payload, nil
	}

	err := s.RegisterAction("test_action", "Test action description", handler, 
		map[string]any{"type": "object"}, 
		map[string]any{"type": "object"}, 
		[]string{})
	require.NoError(t, err)

	t.Run("successful unregistration", func(t *testing.T) {
		err := s.UnregisterAction("test_action")
		require.NoError(t, err)

		_, exists := s.GetAction("test_action")
		assert.False(t, exists)
	})

	t.Run("unregister non-existent action", func(t *testing.T) {
		err := s.UnregisterAction("non_existent")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "not found")
	})
}

func TestListActions(t *testing.T) {
	s := New()
	handler := func(ctx context.Context, payload map[string]any) (map[string]any, error) {
		return payload, nil
	}

	schemas := map[string]any{"type": "object"}
	
	require.NoError(t, s.RegisterAction("action1", "First test action", handler, schemas, schemas, []string{}))
	require.NoError(t, s.RegisterAction("action2", "Second test action", handler, schemas, schemas, []string{}))

	actions := s.ListActions()
	assert.Len(t, actions, 2)
	
	names := make([]string, 2)
	for i, action := range actions {
		names[i] = action.Name
	}
	assert.Contains(t, names, "action1")
	assert.Contains(t, names, "action2")
}
