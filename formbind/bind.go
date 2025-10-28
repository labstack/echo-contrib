// SPDX-License-Identifier: MIT
// SPDX-FileCopyrightText: © 2017 LabStack and Echo contributors

package formbind

import (
	"errors"
	"fmt"
	"net/url"
	"reflect"
	"strconv"
	"strings"
	"time"
)

const (
	// maxSliceIndex defines the maximum allowed slice index to prevent memory exhaustion attacks
	maxSliceIndex = 1000000
)

type BindError struct {
	Field string
	Err   error
}

func (e *BindError) Error() string {
	return fmt.Sprintf("bind error on field %s: %v", e.Field, e.Err)
}

type ParseError struct {
	Value string
	Type  string
	Err   error
}

func (e *ParseError) Error() string {
	return fmt.Sprintf("parse error: cannot parse %q as %s: %v", e.Value, e.Type, e.Err)
}

func Bind(dst interface{}, data url.Values) error {
	val := reflect.ValueOf(dst)
	if val.Kind() != reflect.Ptr {
		return fmt.Errorf("destination must be a pointer")
	}

	val = val.Elem()
	if val.Kind() != reflect.Struct {
		return fmt.Errorf("destination must be a pointer to a struct")
	}

	typ := val.Type()

	// Collect array grouping information first
	groups := collectArrayGroups(data)

	// First pass: handle flat fields
	for key, values := range data {
		if !strings.Contains(key, ".") && !strings.Contains(key, "[") {
			if err := setField(val, typ, key, values[0]); err != nil {
				return &BindError{Field: key, Err: err}
			}
		}
	}

	// Second pass: handle nested fields with group-aware parsing
	for key, values := range data {
		if strings.Contains(key, ".") || strings.Contains(key, "[") {
			if err := bindNestedFormFieldWithGroups(val, typ, key, values, groups); err != nil {
				return &BindError{Field: key, Err: err}
			}
		}
	}

	return nil
}

// parseFieldPath parses a field path like "group.items[0].name" into parts.
// groupMap provides mapping from grouping keys to array indices.
func parseFieldPathWithGroups(key string, groups map[string]*groupInfo) []interface{} {
	var parts []interface{}
	start := 0
	
	for i := 0; i < len(key); i++ {
		switch key[i] {
		case '.':
			if i > start {
				parts = append(parts, key[start:i])
			}
			start = i + 1
		case '[':
			if i > start {
				fieldName := key[start:i]
				parts = append(parts, fieldName)
				
				// Find the closing bracket
				j := i + 1
				for j < len(key) && key[j] != ']' {
					j++
				}
				if j < len(key) && j > i+1 {
					groupKey := key[i+1 : j]
					
					// Look up the group info for this field
					arrayFieldPath := strings.Join(getStringParts(parts), ".")
					if group, exists := groups[arrayFieldPath]; exists {
						if idx, found := group.keyToIdx[groupKey]; found {
							parts = append(parts, idx)
						} else {
							// Group key not found, return empty to ignore
							return []interface{}{}
						}
					} else {
						// No group info, fall back to numeric parsing with limits
						if index, err := strconv.Atoi(groupKey); err == nil && index >= 0 && index < maxSliceIndex {
							parts = append(parts, index)
						} else {
							return []interface{}{}
						}
					}
					
					i = j
					start = j + 1
				} else {
					return []interface{}{}
				}
			}
		}
	}
	
	if start < len(key) {
		parts = append(parts, key[start:])
	}
	
	return parts
}

// getStringParts extracts only string parts from mixed interface slice
func getStringParts(parts []interface{}) []string {
	var stringParts []string
	for _, part := range parts {
		if s, ok := part.(string); ok {
			stringParts = append(stringParts, s)
		}
	}
	return stringParts
}

// parseFieldPath parses a field path like "group.items[0].name" into parts.
// Returns empty slice if path contains invalid indices (negative or too large).
func parseFieldPath(key string) []interface{} {
	return parseFieldPathWithGroups(key, nil) // Use legacy behavior when no groups provided
}

// bindNestedFormField binds a nested form field to the struct.
func bindNestedFormField(val reflect.Value, typ reflect.Type, key string, values []string) error {
	parts := parseFieldPath(key)
	if len(parts) == 0 {
		// Invalid path, ignore silently
		return nil
	}
	return setValueByParts(val, typ, parts, values[0])
}

// bindNestedFormFieldWithGroups binds a nested form field using group-aware parsing
func bindNestedFormFieldWithGroups(val reflect.Value, typ reflect.Type, key string, values []string, groups map[string]*groupInfo) error {
	parts := parseFieldPathWithGroups(key, groups)
	if len(parts) == 0 {
		// Invalid path, ignore silently
		return nil
	}
	return setValueByParts(val, typ, parts, values[0])
}

// groupInfo holds information about array group keys
type groupInfo struct {
	keys     []string // distinct keys found (e.g., ["0", "5", "10"])
	keyToIdx map[string]int // mapping from key to array index
}

// collectArrayGroups analyzes form data to collect array grouping information
func collectArrayGroups(data map[string][]string) map[string]*groupInfo {
	groups := make(map[string]*groupInfo)
	
	for key := range data {
		if !strings.Contains(key, "[") {
			continue
		}
		
		// Extract array field path and grouping key
		if arrayField, groupKey := extractArrayGroup(key); arrayField != "" && groupKey != "" {
			if groups[arrayField] == nil {
				groups[arrayField] = &groupInfo{
					keys:     []string{},
					keyToIdx: make(map[string]int),
				}
			}
			
			group := groups[arrayField]
			if _, exists := group.keyToIdx[groupKey]; !exists {
				group.keyToIdx[groupKey] = len(group.keys)
				group.keys = append(group.keys, groupKey)
			}
		}
	}
	
	return groups
}

// extractArrayGroup extracts array field path and grouping key from a form key
// e.g., "items[123].name" -> ("items", "123")
// e.g., "data[0].nested[5].value" -> ("data", "0") - only handles first level
func extractArrayGroup(key string) (arrayField, groupKey string) {
	start := strings.Index(key, "[")
	if start == -1 {
		return "", ""
	}
	
	end := strings.Index(key[start:], "]")
	if end == -1 {
		return "", ""
	}
	end += start
	
	arrayField = key[:start]
	groupKey = key[start+1 : end]
	
	// Validate grouping key (should be reasonable)
	if len(groupKey) == 0 || len(groupKey) > 20 {
		return "", ""
	}
	
	return arrayField, groupKey
}

// setValueByParts sets a value using the parsed field path parts.
func setValueByParts(val reflect.Value, typ reflect.Type, parts []interface{}, value string) error {
	if len(parts) == 0 {
		return nil
	}
	part := parts[0]
	switch v := part.(type) {
	case string:
		fieldIdx := -1
		for i := 0; i < typ.NumField(); i++ {
			ft := typ.Field(i)
			if ft.Tag.Get("form") == v || strings.EqualFold(ft.Name, v) {
				fieldIdx = i
				break
			}
		}
		if fieldIdx == -1 {
			return nil // Field not found, skip silently
		}
		fv := val.Field(fieldIdx)
		ft := typ.Field(fieldIdx)
		if fv.Kind() == reflect.Ptr {
			if fv.IsNil() {
				fv.Set(reflect.New(ft.Type.Elem()))
			}
			fv = fv.Elem()
			ft.Type = ft.Type.Elem()
		}
		if len(parts) == 1 {
			if err := setWithProperType(fv.Kind(), value, fv); err != nil {
				// Wrap standard errors with ParseError for consistency
				if _, ok := err.(*ParseError); !ok && fv.Kind() != reflect.Struct && fv.Kind() != reflect.Slice {
					return &ParseError{Value: value, Type: fv.Kind().String(), Err: err}
				}
				return err
			}
			return nil
		}
		return setValueByParts(fv, ft.Type, parts[1:], value)
	case int:
		if val.Kind() != reflect.Slice {
			return nil // Not a slice, skip silently
		}
		// Validate slice index for security
		if v < 0 || v >= maxSliceIndex {
			return nil // Skip invalid indices silently
		}
		for val.Len() <= v {
			val.Set(reflect.Append(val, reflect.Zero(val.Type().Elem())))
		}
		elem := val.Index(v)
		elemType := val.Type().Elem()

		if elemType.Kind() == reflect.Ptr {
			if elem.IsNil() {
				elem.Set(reflect.New(elemType.Elem()))
			}
			elem = elem.Elem()
			elemType = elemType.Elem()
		}

		if len(parts) == 1 {
			if err := setWithProperType(elem.Kind(), value, elem); err != nil {
				// Wrap standard errors with ParseError for consistency
				if _, ok := err.(*ParseError); !ok && elem.Kind() != reflect.Struct && elem.Kind() != reflect.Slice {
					return &ParseError{Value: value, Type: elem.Kind().String(), Err: err}
				}
				return err
			}
			return nil
		}

		return setValueByParts(elem, elemType, parts[1:], value)
	}
	return nil
}

// setField sets a flat field value.
func setField(val reflect.Value, typ reflect.Type, key, value string) error {
	for i := 0; i < typ.NumField(); i++ {
		ft := typ.Field(i)
		tag := ft.Tag.Get("form")
		if tag == key || (tag == "" && strings.EqualFold(ft.Name, key)) {
			fv := val.Field(i)
			if fv.Kind() == reflect.Ptr {
				if fv.IsNil() {
					fv.Set(reflect.New(ft.Type.Elem()))
				}
				fv = fv.Elem()
			}
			if err := setWithProperType(fv.Kind(), value, fv); err != nil {
				// Wrap standard errors with ParseError for consistency
				if _, ok := err.(*ParseError); !ok && fv.Kind() != reflect.Struct && fv.Kind() != reflect.Slice {
					return &ParseError{Value: value, Type: fv.Kind().String(), Err: err}
				}
				return err
			}
			return nil
		}
	}
	return nil // Field not found, skip silently
}

// setWithProperType sets a value with the appropriate type conversion.
func setWithProperType(valueKind reflect.Kind, val string, structField reflect.Value) error {
	switch valueKind {
	case reflect.Ptr:
		return setWithProperType(structField.Elem().Kind(), val, structField.Elem())
	case reflect.Int:
		return setIntField(val, 0, structField)
	case reflect.Int8:
		return setIntField(val, 8, structField)
	case reflect.Int16:
		return setIntField(val, 16, structField)
	case reflect.Int32:
		return setIntField(val, 32, structField)
	case reflect.Int64:
		return setIntField(val, 64, structField)
	case reflect.Uint:
		return setUintField(val, 0, structField)
	case reflect.Uint8:
		return setUintField(val, 8, structField)
	case reflect.Uint16:
		return setUintField(val, 16, structField)
	case reflect.Uint32:
		return setUintField(val, 32, structField)
	case reflect.Uint64:
		return setUintField(val, 64, structField)
	case reflect.Bool:
		return setBoolField(val, structField)
	case reflect.Float32:
		return setFloatField(val, 32, structField)
	case reflect.Float64:
		return setFloatField(val, 64, structField)
	case reflect.String:
		structField.SetString(val)
	case reflect.Struct:
		return setTimeField(val, structField)
	case reflect.Slice:
		return setSliceField(val, structField)
	default:
		return errors.New("unknown type")
	}
	return nil
}

func setIntField(value string, bitSize int, field reflect.Value) error {
	if value == "" {
		value = "0"
	}
	intVal, err := strconv.ParseInt(value, 10, bitSize)
	if err == nil {
		field.SetInt(intVal)
	}
	return err
}

func setUintField(value string, bitSize int, field reflect.Value) error {
	if value == "" {
		value = "0"
	}
	uintVal, err := strconv.ParseUint(value, 10, bitSize)
	if err == nil {
		field.SetUint(uintVal)
	}
	return err
}

func setBoolField(value string, field reflect.Value) error {
	if value == "" {
		value = "false"
	}
	boolVal, err := strconv.ParseBool(value)
	if err == nil {
		field.SetBool(boolVal)
	}
	return err
}

func setFloatField(value string, bitSize int, field reflect.Value) error {
	if value == "" {
		value = "0.0"
	}
	floatVal, err := strconv.ParseFloat(value, bitSize)
	if err == nil {
		field.SetFloat(floatVal)
	}
	return err
}

func setTimeField(value string, field reflect.Value) error {
	if field.Type() != reflect.TypeOf(time.Time{}) {
		return fmt.Errorf("unsupported struct type: %v", field.Type())
	}

	if value == "" {
		field.Set(reflect.ValueOf(time.Time{}))
		return nil
	}

	// Try common time formats
	formats := []string{
		time.RFC3339,
		time.RFC3339Nano,
		"2006-01-02T15:04:05",
		"2006-01-02 15:04:05",
		"2006-01-02",
		"15:04:05",
	}

	for _, format := range formats {
		if t, err := time.Parse(format, value); err == nil {
			field.Set(reflect.ValueOf(t))
			return nil
		}
	}

	return &ParseError{Value: value, Type: "time.Time", Err: fmt.Errorf("unknown time format")}
}

func setSliceField(value string, field reflect.Value) error {
	slice := reflect.MakeSlice(field.Type(), 0, 1)
	elemType := field.Type().Elem()
	
	elem := reflect.New(elemType).Elem()
	if err := setWithProperType(elemType.Kind(), value, elem); err != nil {
		return err
	}
	
	slice = reflect.Append(slice, elem)
	field.Set(slice)
	return nil
}