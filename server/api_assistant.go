package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
)

type Action struct {
	Action  string         `json:"action"`
	Payload map[string]any `json:"payload"`
}

type ExecuteActionsRequest struct {
	Actions []Action          `json:"actions"`
	Context map[string]string `json:"context"`
}

func (p *Plugin) handleExecuteActions(c *gin.Context) {
	userID := c.GetHeader("Mattermost-User-Id")
	if userID == "" {
		c.AbortWithStatus(http.StatusUnauthorized)
		return
	}

	var req ExecuteActionsRequest
	if err := c.BindJSON(&req); err != nil {
		c.AbortWithError(http.StatusBadRequest, fmt.Errorf("invalid request: %w", err))
		return
	}

	results := make([]map[string]any, 0, len(req.Actions))
	for i, action := range req.Actions {
		result, err := p.microactions.ExecuteAction(c.Request.Context(), action.Action, action.Payload, userID)
		if err != nil {
			c.AbortWithError(http.StatusInternalServerError, fmt.Errorf("failed to execute action %d (%s): %w", i, action.Action, err))
			return
		}

		// Store result for potential use in subsequent actions
		results = append(results, result)

		// Process any response references in subsequent actions
		if i < len(req.Actions)-1 {
			for j := i + 1; j < len(req.Actions); j++ {
				payload, err := json.Marshal(req.Actions[j].Payload)
				if err != nil {
					continue
				}

				// Replace any {response[n].field} references
				payloadStr := string(payload)
				for k, res := range results {
					for field, value := range res {
						placeholder := fmt.Sprintf(`"{response[%d].%s}"`, k, field)
						valueStr, err := json.Marshal(value)
						if err != nil {
							continue
						}
						payloadStr = strings.Replace(payloadStr, placeholder, string(valueStr), -1)
					}
				}

				var updatedPayload map[string]any
				if err := json.Unmarshal([]byte(payloadStr), &updatedPayload); err != nil {
					continue
				}
				req.Actions[j].Payload = updatedPayload
			}
		}
	}

	c.JSON(http.StatusOK, results)
}
