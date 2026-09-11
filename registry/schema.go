package registry

import (
	"encoding/json"
	"fmt"
	"sort"
)

// Контракт действия сервиса. Движок читает его из metadata по ключу
// "<action>_schema" (engine/internal/api/handlers/registry.go) и отдаёт
// в GET /api/v1/registry/services/{name}/actions полем "schema";
// модельер использует schema.input для валидации и автозаполнения
// маппинга service task.

// Property описывает одно поле входа или выхода действия.
type Property struct {
	Type        string    `json:"type"`
	Description string    `json:"description,omitempty"`
	Items       *Property `json:"items,omitempty"`
	Enum        []string  `json:"enum,omitempty"`
}

// ObjectSchema — объектная схема входа или выхода.
type ObjectSchema struct {
	Type       string              `json:"type"`
	Required   []string            `json:"required,omitempty"`
	Properties map[string]Property `json:"properties"`
}

// ActionSchema — полный контракт одного действия.
type ActionSchema struct {
	Input  ObjectSchema `json:"input"`
	Output ObjectSchema `json:"output"`
}

// Object собирает ObjectSchema, проставляя type и сортируя required: порядок
// ключей должен быть стабильным между рестартами, иначе каждый heartbeat
// выглядит как изменение регистрации сервиса.
func Object(required []string, properties map[string]Property) ObjectSchema {
	if properties == nil {
		properties = map[string]Property{}
	}
	sorted := append([]string(nil), required...)
	sort.Strings(sorted)
	return ObjectSchema{Type: "object", Required: sorted, Properties: properties}
}

// ApplyActionSchemas добавляет в metadata по ключу "<action>_schema" JSON
// контракта каждого действия. Схемы действий, которых нет в metadata,
// игнорируются: реестр не должен рекламировать то, что сервис не заявил.
// Возвращает ту же карту, чтобы вызов можно было встроить в цепочку.
func ApplyActionSchemas(metadata map[string]string, schemas map[string]ActionSchema) map[string]string {
	if metadata == nil {
		return nil
	}
	for action, schema := range schemas {
		if _, declared := metadata[action]; !declared {
			continue
		}
		encoded, err := json.Marshal(schema)
		if err != nil {
			// Схемы статические: ошибка здесь означает битую декларацию в коде.
			panic(fmt.Sprintf("registry: marshal schema for action %q: %v", action, err))
		}
		metadata[action+"_schema"] = string(encoded)
	}
	return metadata
}
