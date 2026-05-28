package helpers

import (
	"fmt"
	"strconv"
)

// GetString retrieves a string value from m by key.
// Returns the value and true if the key exists and holds a string; otherwise returns "", false.
func GetString(m map[string]any, key string) (string, bool) {
	if m == nil {
		return "", false
	}
	val, exists := m[key]
	if !exists {
		return "", false
	}
	str, ok := val.(string)
	return str, ok
}

// GetStringOrDefault retrieves a string from m by key, returning defaultValue if absent or not a string.
func GetStringOrDefault(m map[string]any, key string, defaultValue string) string {
	str, ok := GetString(m, key)
	if !ok {
		return defaultValue
	}
	return str
}

// GetInt handles int, int64, float64, and numeric string conversions.
func GetInt(m map[string]any, key string) (int, bool) {
	if m == nil {
		return 0, false
	}
	val, exists := m[key]
	if !exists {
		return 0, false
	}

	switch v := val.(type) {
	case int:
		return v, true
	case int64:
		return int(v), true
	case float64:
		return int(v), true
	case string:
		if i, err := strconv.Atoi(v); err == nil {
			return i, true
		}
		return 0, false
	default:
		return 0, false
	}
}

// GetIntOrDefault retrieves an int from m by key, returning defaultValue if absent or not convertible.
func GetIntOrDefault(m map[string]any, key string, defaultValue int) int {
	val, ok := GetInt(m, key)
	if !ok {
		return defaultValue
	}
	return val
}

// GetInt64 retrieves an int64 from m by key.
// Handles int64, int, and float64 (the type encoding/json produces for JSON numbers).
// Returns the value and true on success; otherwise returns 0, false.
func GetInt64(m map[string]any, key string) (int64, bool) {
	if m == nil {
		return 0, false
	}
	val, exists := m[key]
	if !exists {
		return 0, false
	}

	switch v := val.(type) {
	case int64:
		return v, true
	case int:
		return int64(v), true
	case float64:
		return int64(v), true
	default:
		return 0, false
	}
}

// GetInt64OrDefault retrieves an int64 from m by key, returning defaultValue if absent or not convertible.
func GetInt64OrDefault(m map[string]any, key string, defaultValue int64) int64 {
	val, ok := GetInt64(m, key)
	if !ok {
		return defaultValue
	}
	return val
}

// GetFloat64 retrieves a float64 from m by key.
// Handles float64, float32, int, and int64 conversions.
// Returns the value and true on success; otherwise returns 0, false.
func GetFloat64(m map[string]any, key string) (float64, bool) {
	if m == nil {
		return 0, false
	}
	val, exists := m[key]
	if !exists {
		return 0, false
	}

	switch v := val.(type) {
	case float64:
		return v, true
	case float32:
		return float64(v), true
	case int:
		return float64(v), true
	case int64:
		return float64(v), true
	default:
		return 0, false
	}
}

// GetFloat64OrDefault retrieves a float64 from m by key, returning defaultValue if absent or not convertible.
func GetFloat64OrDefault(m map[string]any, key string, defaultValue float64) float64 {
	val, ok := GetFloat64(m, key)
	if !ok {
		return defaultValue
	}
	return val
}

// GetBool retrieves a bool from m by key.
// Returns the value and true if the key exists and holds a bool; otherwise returns false, false.
func GetBool(m map[string]any, key string) (bool, bool) {
	if m == nil {
		return false, false
	}
	val, exists := m[key]
	if !exists {
		return false, false
	}
	b, ok := val.(bool)
	return b, ok
}

// GetBoolOrDefault retrieves a bool from m by key, returning defaultValue if absent or not a bool.
func GetBoolOrDefault(m map[string]any, key string, defaultValue bool) bool {
	val, ok := GetBool(m, key)
	if !ok {
		return defaultValue
	}
	return val
}

// GetMap retrieves a nested map[string]any from m by key.
// Returns the map and true if the key exists and holds a map; otherwise returns nil, false.
func GetMap(m map[string]any, key string) (map[string]any, bool) {
	if m == nil {
		return nil, false
	}
	val, exists := m[key]
	if !exists {
		return nil, false
	}
	nested, ok := val.(map[string]any)
	return nested, ok
}

// GetMapOrDefault retrieves a nested map from m by key, returning defaultValue if absent or not a map.
func GetMapOrDefault(m map[string]any, key string, defaultValue map[string]any) map[string]any {
	nested, ok := GetMap(m, key)
	if !ok {
		return defaultValue
	}
	return nested
}

// GetSlice retrieves a []any from m by key.
// Returns the slice and true if the key exists and holds a []any; otherwise returns nil, false.
func GetSlice(m map[string]any, key string) ([]any, bool) {
	if m == nil {
		return nil, false
	}
	val, exists := m[key]
	if !exists {
		return nil, false
	}
	slice, ok := val.([]any)
	return slice, ok
}

// GetSliceOrDefault retrieves a []any from m by key, returning defaultValue if absent or not a []any.
func GetSliceOrDefault(m map[string]any, key string, defaultValue []any) []any {
	slice, ok := GetSlice(m, key)
	if !ok {
		return defaultValue
	}
	return slice
}

// GetStringSlice retrieves a string slice from m by key.
// Handles both []string and []any of strings (the latter is common after JSON round-trips).
// For []any values, returns nil, false if any element is not a string.
// For []string values, always returns true (type system guarantees all elements are strings).
func GetStringSlice(m map[string]any, key string) ([]string, bool) {
	if m == nil {
		return nil, false
	}
	val, exists := m[key]
	if !exists {
		return nil, false
	}

	if strSlice, ok := val.([]string); ok {
		return strSlice, true
	}

	if slice, ok := val.([]any); ok {
		result := make([]string, 0, len(slice))
		for _, item := range slice {
			if str, ok := item.(string); ok {
				result = append(result, str)
			} else {
				return nil, false
			}
		}
		return result, true
	}

	return nil, false
}

// GetStringSliceOrDefault retrieves a string slice from m by key, returning defaultValue if absent, not a slice, or contains non-string elements.
func GetStringSliceOrDefault(m map[string]any, key string, defaultValue []string) []string {
	slice, ok := GetStringSlice(m, key)
	if !ok {
		return defaultValue
	}
	return slice
}

// Has reports whether m contains the given key. Returns false for nil maps.
func Has(m map[string]any, key string) bool {
	if m == nil {
		return false
	}
	_, exists := m[key]
	return exists
}

// GetNested retrieves a value using dot-notation path traversal.
// Example: GetNested(m, "user.profile.name") traverses m["user"]["profile"]["name"].
func GetNested(m map[string]any, path string) (any, bool) {
	if m == nil {
		return nil, false
	}

	if val, exists := m[path]; exists {
		return val, true
	}

	return getNestedRecursive(m, path)
}

func getNestedRecursive(m map[string]any, path string) (any, bool) {
	dotIndex := -1
	for i, c := range path {
		if c == '.' {
			dotIndex = i
			break
		}
	}

	if dotIndex == -1 {
		val, exists := m[path]
		return val, exists
	}

	firstKey := path[:dotIndex]
	remainingPath := path[dotIndex+1:]

	val, exists := m[firstKey]
	if !exists {
		return nil, false
	}

	nested, ok := val.(map[string]any)
	if !ok {
		return nil, false
	}

	return getNestedRecursive(nested, remainingPath)
}

// GetNestedString retrieves a string using dot-notation path traversal.
// Combines GetNested with string type assertion. Returns "", false if the path is absent or not a string.
func GetNestedString(m map[string]any, path string) (string, bool) {
	val, exists := GetNested(m, path)
	if !exists {
		return "", false
	}
	str, ok := val.(string)
	return str, ok
}

// GetNestedStringOrDefault retrieves a string using dot-notation path traversal,
// returning defaultValue if the path is absent or the value is not a string.
func GetNestedStringOrDefault(m map[string]any, path string, defaultValue string) string {
	str, ok := GetNestedString(m, path)
	if !ok {
		return defaultValue
	}
	return str
}

// MustGetString panics if the key is missing or not a string.
// Use only when a missing value indicates programmer error, not runtime input.
func MustGetString(m map[string]any, key string) string {
	str, ok := GetString(m, key)
	if !ok {
		panic(fmt.Sprintf("required string key '%s' not found or invalid type", key))
	}
	return str
}

// MustGetInt panics if the key is missing or not convertible to int.
func MustGetInt(m map[string]any, key string) int {
	val, ok := GetInt(m, key)
	if !ok {
		panic(fmt.Sprintf("required int key '%s' not found or invalid type", key))
	}
	return val
}

// MustGetMap panics if the key is missing or not a map[string]any.
func MustGetMap(m map[string]any, key string) map[string]any {
	nested, ok := GetMap(m, key)
	if !ok {
		panic(fmt.Sprintf("required map key '%s' not found or invalid type", key))
	}
	return nested
}

// Merge combines multiple maps into a new map. Later maps override earlier ones on duplicate keys.
func Merge(maps ...map[string]any) map[string]any {
	result := make(map[string]any)
	for _, m := range maps {
		for k, v := range m {
			result[k] = v
		}
	}
	return result
}

// Pick returns a new map containing only the keys listed in keys.
// Keys absent from m are silently omitted.
func Pick(m map[string]any, keys ...string) map[string]any {
	result := make(map[string]any)
	for _, key := range keys {
		if val, exists := m[key]; exists {
			result[key] = val
		}
	}
	return result
}

// Omit returns a new map containing all keys from m except those listed in keys.
func Omit(m map[string]any, keys ...string) map[string]any {
	omitSet := make(map[string]bool)
	for _, key := range keys {
		omitSet[key] = true
	}

	result := make(map[string]any)
	for k, v := range m {
		if !omitSet[k] {
			result[k] = v
		}
	}
	return result
}

// Keys returns the keys of m in unspecified order.
func Keys(m map[string]any) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	return keys
}

// StringMapKeys returns the keys of a map[string]string in unspecified order.
func StringMapKeys(m map[string]string) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	return keys
}

// Values returns the values of m in unspecified order.
func Values(m map[string]any) []any {
	values := make([]any, 0, len(m))
	for _, v := range m {
		values = append(values, v)
	}
	return values
}

// GetMapFromSlice returns the element at index from slice, asserting it is a map[string]any.
// Returns an error if index is out of bounds or the element is not a map.
func GetMapFromSlice(slice []any, index int) (map[string]any, error) {
	if index < 0 || index >= len(slice) {
		return nil, fmt.Errorf("index %d out of bounds for slice of length %d", index, len(slice))
	}

	m, ok := slice[index].(map[string]any)
	if !ok {
		return nil, fmt.Errorf("element at index %d is not a map[string]any", index)
	}

	return m, nil
}

// GetFirstMapFromSlice returns the first element of slice as a map[string]any.
// Returns an error if the slice is empty or the first element is not a map.
func GetFirstMapFromSlice(slice []any) (map[string]any, error) {
	if len(slice) == 0 {
		return nil, fmt.Errorf("slice is empty")
	}
	return GetMapFromSlice(slice, 0)
}
