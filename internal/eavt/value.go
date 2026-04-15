package eavt

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

// ValueType represents the type tag for EAVT values.
type ValueType string

const (
	TypeString   ValueType = "s"
	TypeInt      ValueType = "i"
	TypeBool     ValueType = "b"
	TypeRef      ValueType = "r"
	TypeDateTime ValueType = "d"
	TypeEnum     ValueType = "e"
	TypeRefSet   ValueType = "rs"
)

// Value is a typed value stored in a datom.
type Value struct {
	Type ValueType
	Raw  any
}

// Constructors

func NewString(s string) Value {
	s = strings.ReplaceAll(s, "\r\n", "\n")
	return Value{Type: TypeString, Raw: s}
}
func NewInt(i int64) Value       { return Value{Type: TypeInt, Raw: i} }
func NewBool(b bool) Value       { return Value{Type: TypeBool, Raw: b} }
func NewRef(entityID int64) Value { return Value{Type: TypeRef, Raw: entityID} }
func NewEnum(s string) Value     { return Value{Type: TypeEnum, Raw: s} }

func NewDateTime(t time.Time) Value {
	return Value{Type: TypeDateTime, Raw: t.UTC().Format(time.RFC3339Nano)}
}

func NewRefSet(ids []int64) Value { return Value{Type: TypeRefSet, Raw: ids} }

// Encode serializes the value to a JSON string for storage in the datoms table.
func (v Value) Encode() (string, error) {
	var raw any
	switch v.Type {
	case TypeDateTime:
		raw = v.Raw // already a string
	case TypeRefSet:
		raw = v.Raw // []int64
	default:
		raw = v.Raw
	}
	m := map[string]any{"t": string(v.Type), "v": raw}
	b, err := json.Marshal(m)
	if err != nil {
		return "", fmt.Errorf("encode value: %w", err)
	}
	return string(b), nil
}

// DecodeValue deserializes a JSON-encoded value from the datoms table.
func DecodeValue(encoded string) (Value, error) {
	var m map[string]any
	if err := json.Unmarshal([]byte(encoded), &m); err != nil {
		return Value{}, fmt.Errorf("decode value: %w", err)
	}

	t, ok := m["t"].(string)
	if !ok {
		return Value{}, fmt.Errorf("decode value: missing or invalid type tag")
	}
	raw := m["v"]

	vt := ValueType(t)
	switch vt {
	case TypeString:
		s, ok := raw.(string)
		if !ok {
			return Value{}, fmt.Errorf("decode string: expected string, got %T", raw)
		}
		return NewString(s), nil
	case TypeInt:
		// JSON numbers are float64
		f, ok := raw.(float64)
		if !ok {
			return Value{}, fmt.Errorf("decode int: expected number, got %T", raw)
		}
		return NewInt(int64(f)), nil
	case TypeBool:
		b, ok := raw.(bool)
		if !ok {
			return Value{}, fmt.Errorf("decode bool: expected bool, got %T", raw)
		}
		return NewBool(b), nil
	case TypeRef:
		f, ok := raw.(float64)
		if !ok {
			return Value{}, fmt.Errorf("decode ref: expected number, got %T", raw)
		}
		return NewRef(int64(f)), nil
	case TypeDateTime:
		s, ok := raw.(string)
		if !ok {
			return Value{}, fmt.Errorf("decode datetime: expected string, got %T", raw)
		}
		return Value{Type: TypeDateTime, Raw: s}, nil
	case TypeEnum:
		s, ok := raw.(string)
		if !ok {
			return Value{}, fmt.Errorf("decode enum: expected string, got %T", raw)
		}
		return NewEnum(s), nil
	case TypeRefSet:
		arr, ok := raw.([]any)
		if !ok {
			return Value{}, fmt.Errorf("decode refset: expected array, got %T", raw)
		}
		ids := make([]int64, len(arr))
		for i, item := range arr {
			f, ok := item.(float64)
			if !ok {
				return Value{}, fmt.Errorf("decode refset[%d]: expected number, got %T", i, item)
			}
			ids[i] = int64(f)
		}
		return NewRefSet(ids), nil
	default:
		return Value{}, fmt.Errorf("decode value: unknown type %q", t)
	}
}

// AsString returns the value as a string, or an error if the type doesn't match.
func (v Value) AsString() (string, error) {
	if v.Type != TypeString && v.Type != TypeEnum && v.Type != TypeDateTime {
		return "", fmt.Errorf("value is %s, not string-like", v.Type)
	}
	s, ok := v.Raw.(string)
	if !ok {
		return "", fmt.Errorf("value raw is %T, not string", v.Raw)
	}
	return s, nil
}

// AsInt64 returns the value as int64.
func (v Value) AsInt64() (int64, error) {
	if v.Type != TypeInt && v.Type != TypeRef {
		return 0, fmt.Errorf("value is %s, not int-like", v.Type)
	}
	i, ok := v.Raw.(int64)
	if !ok {
		return 0, fmt.Errorf("value raw is %T, not int64", v.Raw)
	}
	return i, nil
}

// AsBool returns the value as bool.
func (v Value) AsBool() (bool, error) {
	if v.Type != TypeBool {
		return false, fmt.Errorf("value is %s, not bool", v.Type)
	}
	b, ok := v.Raw.(bool)
	if !ok {
		return false, fmt.Errorf("value raw is %T, not bool", v.Raw)
	}
	return b, nil
}

// AsRefSet returns the value as []int64.
func (v Value) AsRefSet() ([]int64, error) {
	if v.Type != TypeRefSet {
		return nil, fmt.Errorf("value is %s, not refset", v.Type)
	}
	ids, ok := v.Raw.([]int64)
	if !ok {
		return nil, fmt.Errorf("value raw is %T, not []int64", v.Raw)
	}
	return ids, nil
}
