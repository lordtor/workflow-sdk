package registry

import (
	"encoding/json"
	"testing"
)

// Движок читает контракт из metadata["<action>_schema"] и кладёт разобранный
// JSON в поле "schema" ответа GET /registry/services/{name}/actions, а модельер
// берёт оттуда schema.input. Тест фиксирует именно эту форму.
func TestApplyActionSchemasProducesEngineReadableShape(t *testing.T) {
	metadata := map[string]string{
		"get_secret": "Retrieve a secret",
	}

	ApplyActionSchemas(metadata, map[string]ActionSchema{
		"get_secret": {
			Input: Object(
				[]string{"path"},
				map[string]Property{"path": {Type: "string"}},
			),
			Output: Object(
				nil,
				map[string]Property{"value": {Type: "string"}},
			),
		},
	})

	raw, ok := metadata["get_secret_schema"]
	if !ok {
		t.Fatal("ключ get_secret_schema не появился в metadata")
	}

	var decoded struct {
		Input struct {
			Type       string              `json:"type"`
			Required   []string            `json:"required"`
			Properties map[string]Property `json:"properties"`
		} `json:"input"`
		Output struct {
			Type       string              `json:"type"`
			Properties map[string]Property `json:"properties"`
		} `json:"output"`
	}
	if err := json.Unmarshal([]byte(raw), &decoded); err != nil {
		t.Fatalf("движок не разберёт схему: %v", err)
	}

	if decoded.Input.Type != "object" || decoded.Output.Type != "object" {
		t.Fatalf("type должен быть object, получено input=%q output=%q", decoded.Input.Type, decoded.Output.Type)
	}
	if len(decoded.Input.Required) != 1 || decoded.Input.Required[0] != "path" {
		t.Fatalf("required входа = %v, ожидалось [path]", decoded.Input.Required)
	}
	if decoded.Input.Properties["path"].Type != "string" {
		t.Fatalf("тип path = %q, ожидалось string", decoded.Input.Properties["path"].Type)
	}
	if decoded.Output.Properties["value"].Type != "string" {
		t.Fatalf("тип выхода value = %q, ожидалось string", decoded.Output.Properties["value"].Type)
	}
}

// Реестр не должен рекламировать то, чего сервис не заявил в metadata.
func TestApplyActionSchemasSkipsUndeclaredActions(t *testing.T) {
	metadata := map[string]string{"declared": "Заявленное действие"}

	ApplyActionSchemas(metadata, map[string]ActionSchema{
		"declared":   {Input: Object(nil, nil), Output: Object(nil, nil)},
		"undeclared": {Input: Object(nil, nil), Output: Object(nil, nil)},
	})

	if _, ok := metadata["declared_schema"]; !ok {
		t.Fatal("схема заявленного действия не записана")
	}
	if _, ok := metadata["undeclared_schema"]; ok {
		t.Fatal("записана схема действия, которого нет в metadata")
	}
}

// Порядок required не должен зависеть от порядка объявления: metadata уходит
// в реестр на каждом heartbeat, и болтанка выглядела бы как перерегистрация.
func TestObjectSortsRequiredForStableMetadata(t *testing.T) {
	first := Object([]string{"repo", "org", "path"}, nil)
	second := Object([]string{"path", "repo", "org"}, nil)

	a, _ := json.Marshal(first)
	b, _ := json.Marshal(second)
	if string(a) != string(b) {
		t.Fatalf("сериализация зависит от порядка required:\n%s\n%s", a, b)
	}
}

// Действие без параметров должно давать пустой объект, а не null: модельер
// отличает «нет входа» от «контракт не объявлен».
func TestObjectWithoutPropertiesEncodesEmptyObject(t *testing.T) {
	raw, err := json.Marshal(Object(nil, nil))
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if string(raw) != `{"type":"object","properties":{}}` {
		t.Fatalf("получено %s", raw)
	}
}
