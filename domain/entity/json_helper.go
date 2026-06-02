package entity

import (
	"encoding/json"
	"fmt"
	"reflect"
	"strings"
)

// MarshalEntity serializes an entity to JSON, merging the base entity fields
// (Entity, AuditableEntity, or SoftDeletableEntity) with the entity's own domain fields.
// It uses reflection to extract fields tagged with "json" that are not
// part of the embedded base types.
func MarshalEntity[T any](ent T) ([]byte, error) {
	v := reflect.ValueOf(ent)
	if v.Kind() == reflect.Ptr {
		v = v.Elem()
	}
	baseVal, baseKeys, err := extractBase(v)
	if err != nil {
		return nil, fmt.Errorf("marshal entity base: %w", err)
	}
	baseMap, err := structToMap(baseVal)
	if err != nil {
		return nil, fmt.Errorf("marshal entity base to map: %w", err)
	}
	domainMap, err := extractDomainFields(v, baseKeys)
	if err != nil {
		return nil, fmt.Errorf("marshal entity domain fields: %w", err)
	}
	for k, val := range domainMap {
		baseMap[k] = val
	}
	return json.Marshal(baseMap)
}

// UnmarshalEntity deserializes JSON into an entity, restoring both the base
// entity fields and the entity's domain fields via reflection.
func UnmarshalEntity[T any](data []byte, ent T) error {
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		return fmt.Errorf("unmarshal entity: %w", err)
	}
	v := reflect.ValueOf(ent)
	if v.Kind() == reflect.Ptr {
		v = v.Elem()
	}
	baseVal, baseKeys, err := extractBase(v)
	if err != nil {
		return fmt.Errorf("unmarshal entity base extract: %w", err)
	}
	baseRaw := make(map[string]json.RawMessage)
	for _, k := range baseKeys {
		if rv, ok := raw[k]; ok {
			baseRaw[k] = rv
		}
	}
	baseData, err := json.Marshal(baseRaw)
	if err != nil {
		return fmt.Errorf("unmarshal entity base marshal: %w", err)
	}
	basePtr := reflect.New(reflect.TypeOf(baseVal))
	basePtr.Elem().Set(reflect.ValueOf(baseVal))
	if err := json.Unmarshal(baseData, basePtr.Interface()); err != nil {
		return fmt.Errorf("unmarshal entity base: %w", err)
	}
	applyBaseFromJSON(v, basePtr.Elem().Interface())
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
			return fmt.Errorf("unmarshal entity field %s: %w", jsonKey, err)
		}
	}
	return nil
}

func extractBase(v reflect.Value) (interface{}, []string, error) {
	t := v.Type()
	for i := 0; i < t.NumField(); i++ {
		field := t.Field(i)
		if !field.Anonymous {
			continue
		}
		ft := field.Type
		if ft.Kind() == reflect.Ptr {
			ft = ft.Elem()
		}
		if ft.Kind() != reflect.Struct {
			continue
		}
		fieldVal := v.Field(i)
		switch ft.Name() {
		case "SoftDeletableEntity":
			sd, ok := fieldVal.Interface().(SoftDeletableEntity)
			if !ok {
				if fieldVal.CanAddr() {
					sd = *fieldVal.Addr().Interface().(*SoftDeletableEntity)
				}
			}
			jsonVal := sd.ToJSON()
			keys, _ := jsonKeysFromMap(jsonVal)
			return jsonVal, keys, nil
		case "AuditableEntity":
			ae, ok := fieldVal.Interface().(AuditableEntity)
			if !ok {
				if fieldVal.CanAddr() {
					ae = *fieldVal.Addr().Interface().(*AuditableEntity)
				}
			}
			jsonVal := ae.ToJSON()
			keys, _ := jsonKeysFromMap(jsonVal)
			return jsonVal, keys, nil
		case "Entity":
			e, ok := fieldVal.Interface().(Entity)
			if !ok {
				if fieldVal.CanAddr() {
					e = *fieldVal.Addr().Interface().(*Entity)
				}
			}
			jsonVal := e.ToJSON()
			keys, _ := jsonKeysFromMap(jsonVal)
			return jsonVal, keys, nil
		}
	}
	return nil, nil, fmt.Errorf("no embedded Entity base found")
}

func applyBaseFromJSON(v reflect.Value, baseVal interface{}) {
	t := v.Type()
	for i := 0; i < t.NumField(); i++ {
		field := t.Field(i)
		if !field.Anonymous {
			continue
		}
		ft := field.Type
		if ft.Kind() == reflect.Ptr {
			ft = ft.Elem()
		}
		if ft.Kind() != reflect.Struct {
			continue
		}
		fieldVal := v.Field(i)
		switch ft.Name() {
		case "SoftDeletableEntity":
			sd := fieldVal.Addr().Interface().(*SoftDeletableEntity)
			sd.FromJSON(baseVal.(SoftDeletableEntityJSON))
			return
		case "AuditableEntity":
			ae := fieldVal.Addr().Interface().(*AuditableEntity)
			ae.FromJSON(baseVal.(AuditableEntityJSON))
			return
		case "Entity":
			e := fieldVal.Addr().Interface().(*Entity)
			e.FromJSON(baseVal.(EntityJSON))
			return
		}
	}
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

func extractDomainFields(v reflect.Value, baseKeys []string) (map[string]interface{}, error) {
	result := make(map[string]interface{})
	baseKeySet := make(map[string]bool, len(baseKeys))
	for _, k := range baseKeys {
		baseKeySet[k] = true
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
		if baseKeySet[jsonKey] {
			continue
		}
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
	return name == "Entity" || name == "AuditableEntity" || name == "SoftDeletableEntity"
}

func jsonKeysFromMap(v interface{}) ([]string, error) {
	m, err := structToMap(v)
	if err != nil {
		return nil, err
	}
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	return keys, nil
}
