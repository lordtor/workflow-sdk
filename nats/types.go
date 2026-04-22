package nats

import (
	"time"
)

type ResponseStatus string

const (
	ResponseStatusSuccess ResponseStatus = "success"
	ResponseStatusError   ResponseStatus = "error"
	ResponseStatusTimeout ResponseStatus = "timeout"
)

type WorkflowCommand struct {
	CommandID      string                 `json:"command_id"`
	CommandType    string                 `json:"command_type"`
	InstanceID     string                 `json:"instance_id"`
	TokenID        string                 `json:"token_id"`
	NodeID         string                 `json:"node_id"`
	ServiceName    string                 `json:"service_name,omitempty"`
	Operation      string                 `json:"operation"`
	InputVariables map[string]interface{} `json:"input_variables"`
	CreatedAt      time.Time              `json:"created_at"`
	ExpiresAt      time.Time              `json:"expires_at"`
	RetryCount     int                    `json:"retry_count"`
	MaxRetries     int                    `json:"max_retries"`
}

type ServiceResponse struct {
	CommandID       string                 `json:"command_id"`
	InstanceID      string                 `json:"instance_id"`
	TokenID         string                 `json:"token_id"`
	NodeID          string                 `json:"node_id"`
	Status          ResponseStatus         `json:"status"`
	OutputVariables map[string]interface{} `json:"output_variables,omitempty"`
	ErrorMessage    string                 `json:"error_message,omitempty"`
	ErrorCode       string                 `json:"error_code,omitempty"`
	ProcessedAt     time.Time              `json:"processed_at"`
}
