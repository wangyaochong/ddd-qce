package aggregate

import (
	"encoding/json"
	"fmt"
	"reflect"
	"strings"
)

// MarshalAggregate serializes an aggregate to JSON, merging the base
// AggregateRoot fields with the aggregate's own domain fields.
// It uses reflection to extract fields tagged with "json" that are not
// part of the embedded base types.
func MarshalAggregate[T AggregateRef](agg T) ([]byte, error) {
	root := agg.GetAggregateRoot()
	baseMap, err := structToMap(root.ToJSON())
	if err != nil {
		return nil, fmt.Errorf("marshal aggregate base: %w", err)
	}
	v := reflect.ValueOf(agg)
	if v.Kind() == reflect.Ptr {
		v = v.Elem()
	}
	domainMap, err := extractDomainFields(v)
	if err != nil {
		return nil, fmt.Errorf("marshal aggregate domain fields: %w", err)
	}
	for k, val := range domainMap {
		baseMap[k] = val
	}
	return json.Marshal(baseMap)
}

// UnmarshalAggregate deserializes JSON into an aggregate, restoring both the base
// AggregateRoot fields and the aggregate's domain fields via reflection.
func UnmarshalAggregate[T AggregateRef](data []byte, agg T) error {
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		return fmt.Errorf("unmarshal aggregate: %w", err)
	}
	root := agg.GetAggregateRoot()
	baseKeys, err := baseJSONKeys(root.ToJSON())
	if err != nil {
		return fmt.Errorf("unmarshal aggregate base keys: %w", err)
	}
	baseRaw := make(map[string]json.RawMessage)
	for _, k := range baseKeys {
		if v, ok := raw[k]; ok {
			baseRaw[k] = v
		}
	}
	baseData, err := json.Marshal(baseRaw)
	if err != nil {
		return fmt.Errorf("unmarshal aggregate base marshal: %w", err)
	}
	var baseJSON AggregateRootJSON
	if err := json.Unmarshal(baseData, &baseJSON); err != nil {
		return fmt.Errorf("unmarshal aggregate base: %w", err)
	}
	root.FromJSON(baseJSON)
	v := reflect.ValueOf(agg)
	if v.Kind() == reflect.Ptr {
		v = v.Elem()
	}
	t := v.Type()
	for i := 0; i < t.NumField(); i++ {
		field := t.Field(i)
		if !field.IsExported() {
			continue
		}
		if isEmbeddedBase(field) {
			continue
		}
		tag := field.Tag.Get("json")
		if tag == "" || tag == "-" {
			continue
		}
		jsonKey := strings.Split(tag, ",")[0]
		rawMsg, ok := raw[jsonKey]
		if !ok {
			continue
		}
		fieldVal := v.Field(i)
		fieldPtr := fieldVal.Addr().Interface()
		if err := json.Unmarshal(rawMsg, fieldPtr); err != nil {
			return fmt.Errorf("unmarshal aggregate field %s: %w", jsonKey, err)
		}
	}
	return nil
}

func structToMap(v interface{}) (map[string]interface{}, error) {
	data, err := json.Marshal(v)
	if err != nil {
		return nil, err
	}
	var m map[string]interface{}
	if err := json.Unmarshal(data, &m); err != nil {
		return nil, err
	}
	return m, nil
}

func extractDomainFields(v reflect.Value) (map[string]interface{}, error) {
	result := make(map[string]interface{})
	t := v.Type()
	for i := 0; i < t.NumField(); i++ {
		field := t.Field(i)
		if !field.IsExported() {
			continue
		}
		if isEmbeddedBase(field) {
			continue
		}
		tag := field.Tag.Get("json")
		if tag == "" || tag == "-" {
			continue
		}
		jsonKey := strings.Split(tag, ",")[0]
		fieldVal := v.Field(i)
		result[jsonKey] = fieldVal.Interface()
	}
	return result, nil
}

func isEmbeddedBase(f reflect.StructField) bool {
	if !f.Anonymous {
		return false
	}
	t := f.Type
	if t.Kind() == reflect.Ptr {
		t = t.Elem()
	}
	if t.Kind() != reflect.Struct {
		return false
	}
	name := t.Name()
	return name == "AggregateRoot" || name == "Entity" || name == "AuditableEntity" || name == "SoftDeletableEntity"
}

func baseJSONKeys(base AggregateRootJSON) ([]string, error) {
	m, err := structToMap(base)
	if err != nil {
		return nil, err
	}
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	return keys, nil
}
