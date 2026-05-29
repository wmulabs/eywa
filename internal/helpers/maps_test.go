package helpers

import (
	"sort"
	"testing"
)

// ── GetString ────────────────────────────────────────────────────────────────

func TestGetString(t *testing.T) {
	m := map[string]any{"name": "alice", "age": 30}

	if v, ok := GetString(m, "name"); !ok || v != "alice" {
		t.Errorf("expected alice,true got %q,%v", v, ok)
	}
	if _, ok := GetString(m, "age"); ok {
		t.Error("int value should not cast to string")
	}
	if _, ok := GetString(m, "missing"); ok {
		t.Error("missing key should return false")
	}
	if _, ok := GetString(nil, "k"); ok {
		t.Error("nil map should return false")
	}
}

func TestGetStringOrDefault(t *testing.T) {
	m := map[string]any{"name": "bob"}

	if v := GetStringOrDefault(m, "name", "x"); v != "bob" {
		t.Errorf("got %q want bob", v)
	}
	if v := GetStringOrDefault(m, "missing", "default"); v != "default" {
		t.Errorf("got %q want default", v)
	}
}

// ── GetInt ───────────────────────────────────────────────────────────────────

func TestGetInt(t *testing.T) {
	m := map[string]any{
		"int":     42,
		"int64":   int64(99),
		"float64": float64(3.9),
		"strnum":  "7",
		"strbad":  "abc",
		"bool":    true,
	}

	cases := []struct {
		key  string
		want int
		ok   bool
	}{
		{"int", 42, true},
		{"int64", 99, true},
		{"float64", 3, true},
		{"strnum", 7, true},
		{"strbad", 0, false},
		{"bool", 0, false},
		{"missing", 0, false},
	}
	for _, c := range cases {
		v, ok := GetInt(m, c.key)
		if ok != c.ok || v != c.want {
			t.Errorf("GetInt(%q): got %d,%v want %d,%v", c.key, v, ok, c.want, c.ok)
		}
	}
	if _, ok := GetInt(nil, "k"); ok {
		t.Error("nil map should return false")
	}
}

func TestGetIntOrDefault(t *testing.T) {
	m := map[string]any{"n": 5}
	if v := GetIntOrDefault(m, "n", 0); v != 5 {
		t.Errorf("got %d want 5", v)
	}
	if v := GetIntOrDefault(m, "missing", 99); v != 99 {
		t.Errorf("got %d want 99", v)
	}
}

// ── GetInt64 ─────────────────────────────────────────────────────────────────

func TestGetInt64(t *testing.T) {
	m := map[string]any{
		"int64":   int64(1000),
		"int":     int(42),
		"float64": float64(2.9),
		"str":     "x",
	}

	cases := []struct {
		key  string
		want int64
		ok   bool
	}{
		{"int64", 1000, true},
		{"int", 42, true},
		{"float64", 2, true},
		{"str", 0, false},
		{"missing", 0, false},
	}
	for _, c := range cases {
		v, ok := GetInt64(m, c.key)
		if ok != c.ok || v != c.want {
			t.Errorf("GetInt64(%q): got %d,%v want %d,%v", c.key, v, ok, c.want, c.ok)
		}
	}
	if _, ok := GetInt64(nil, "k"); ok {
		t.Error("nil map should return false")
	}
}

func TestGetInt64OrDefault(t *testing.T) {
	m := map[string]any{"n": int64(7)}
	if v := GetInt64OrDefault(m, "n", 0); v != 7 {
		t.Errorf("got %d want 7", v)
	}
	if v := GetInt64OrDefault(m, "x", int64(42)); v != 42 {
		t.Errorf("got %d want 42", v)
	}
}

// ── GetFloat64 ───────────────────────────────────────────────────────────────

func TestGetFloat64(t *testing.T) {
	m := map[string]any{
		"f64": float64(1.5),
		"f32": float32(2.5),
		"int": int(3),
		"i64": int64(4),
		"str": "x",
	}

	cases := []struct {
		key  string
		want float64
		ok   bool
	}{
		{"f64", 1.5, true},
		{"f32", float64(float32(2.5)), true},
		{"int", 3.0, true},
		{"i64", 4.0, true},
		{"str", 0, false},
		{"missing", 0, false},
	}
	for _, c := range cases {
		v, ok := GetFloat64(m, c.key)
		if ok != c.ok || v != c.want {
			t.Errorf("GetFloat64(%q): got %f,%v want %f,%v", c.key, v, ok, c.want, c.ok)
		}
	}
	if _, ok := GetFloat64(nil, "k"); ok {
		t.Error("nil map should return false")
	}
}

func TestGetFloat64OrDefault(t *testing.T) {
	m := map[string]any{"f": float64(9.9)}
	if v := GetFloat64OrDefault(m, "f", 0); v != 9.9 {
		t.Errorf("got %f want 9.9", v)
	}
	if v := GetFloat64OrDefault(m, "x", 1.1); v != 1.1 {
		t.Errorf("got %f want 1.1", v)
	}
}

// ── GetBool ───────────────────────────────────────────────────────────────────

func TestGetBool(t *testing.T) {
	m := map[string]any{"t": true, "f": false, "str": "true"}

	if v, ok := GetBool(m, "t"); !ok || !v {
		t.Errorf("expected true,true")
	}
	if v, ok := GetBool(m, "f"); !ok || v {
		t.Errorf("expected false,true")
	}
	if _, ok := GetBool(m, "str"); ok {
		t.Error("string should not cast to bool")
	}
	if _, ok := GetBool(m, "missing"); ok {
		t.Error("missing key should return false")
	}
	if _, ok := GetBool(nil, "k"); ok {
		t.Error("nil map should return false")
	}
}

func TestGetBoolOrDefault(t *testing.T) {
	m := map[string]any{"b": true}
	if v := GetBoolOrDefault(m, "b", false); !v {
		t.Error("expected true")
	}
	if v := GetBoolOrDefault(m, "x", true); !v {
		t.Error("expected default true")
	}
}

// ── GetMap ────────────────────────────────────────────────────────────────────

func TestGetMap(t *testing.T) {
	nested := map[string]any{"x": 1}
	m := map[string]any{"inner": nested, "str": "hello"}

	if v, ok := GetMap(m, "inner"); !ok || v["x"] != 1 {
		t.Errorf("expected nested map")
	}
	if _, ok := GetMap(m, "str"); ok {
		t.Error("string should not cast to map")
	}
	if _, ok := GetMap(m, "missing"); ok {
		t.Error("missing key should return false")
	}
	if _, ok := GetMap(nil, "k"); ok {
		t.Error("nil map should return false")
	}
}

func TestGetMapOrDefault(t *testing.T) {
	def := map[string]any{"default": true}
	m := map[string]any{"inner": map[string]any{"a": 1}}
	if v := GetMapOrDefault(m, "inner", def); v["a"] != 1 {
		t.Error("expected inner map")
	}
	if v := GetMapOrDefault(m, "missing", def); v["default"] != true {
		t.Error("expected default map")
	}
}

// ── GetSlice ──────────────────────────────────────────────────────────────────

func TestGetSlice(t *testing.T) {
	sl := []any{"a", "b"}
	m := map[string]any{"items": sl, "str": "x"}

	if v, ok := GetSlice(m, "items"); !ok || len(v) != 2 {
		t.Errorf("expected slice of 2")
	}
	if _, ok := GetSlice(m, "str"); ok {
		t.Error("string should not cast to slice")
	}
	if _, ok := GetSlice(m, "missing"); ok {
		t.Error("missing key should return false")
	}
	if _, ok := GetSlice(nil, "k"); ok {
		t.Error("nil map should return false")
	}
}

func TestGetSliceOrDefault(t *testing.T) {
	def := []any{"default"}
	m := map[string]any{"items": []any{"a"}}
	if v := GetSliceOrDefault(m, "items", def); len(v) != 1 {
		t.Error("expected slice of 1")
	}
	if v := GetSliceOrDefault(m, "missing", def); len(v) != 1 || v[0] != "default" {
		t.Error("expected default slice")
	}
}

// ── GetStringSlice ────────────────────────────────────────────────────────────

func TestGetStringSlice(t *testing.T) {
	m := map[string]any{
		"typed": []string{"a", "b"},
		"iface": []any{"x", "y"},
		"mixed": []any{"ok", 123},
		"str":   "not a slice",
	}

	if v, ok := GetStringSlice(m, "typed"); !ok || len(v) != 2 || v[0] != "a" {
		t.Errorf("typed []string: got %v,%v", v, ok)
	}
	if v, ok := GetStringSlice(m, "iface"); !ok || len(v) != 2 || v[0] != "x" {
		t.Errorf("[]any of strings: got %v,%v", v, ok)
	}
	if _, ok := GetStringSlice(m, "mixed"); ok {
		t.Error("mixed slice should return false")
	}
	if _, ok := GetStringSlice(m, "str"); ok {
		t.Error("plain string should return false")
	}
	if _, ok := GetStringSlice(m, "missing"); ok {
		t.Error("missing key should return false")
	}
	if _, ok := GetStringSlice(nil, "k"); ok {
		t.Error("nil map should return false")
	}
}

func TestGetStringSliceOrDefault(t *testing.T) {
	def := []string{"default"}
	m := map[string]any{"tags": []string{"go", "test"}}
	if v := GetStringSliceOrDefault(m, "tags", def); len(v) != 2 {
		t.Errorf("expected 2, got %d", len(v))
	}
	if v := GetStringSliceOrDefault(m, "missing", def); len(v) != 1 || v[0] != "default" {
		t.Error("expected default slice")
	}
}

// ── Has ───────────────────────────────────────────────────────────────────────

func TestHas(t *testing.T) {
	m := map[string]any{"k": nil}
	if !Has(m, "k") {
		t.Error("key with nil value should return true")
	}
	if Has(m, "missing") {
		t.Error("missing key should return false")
	}
	if Has(nil, "k") {
		t.Error("nil map should return false")
	}
}

// ── GetNested ─────────────────────────────────────────────────────────────────

func TestGetNested(t *testing.T) {
	m := map[string]any{
		"a": map[string]any{
			"b": map[string]any{
				"c": "deep",
			},
		},
		"flat": "value",
		"a.b":  "dotted-key", // exact key with dot
	}

	// exact dotted key takes priority
	if v, ok := GetNested(m, "a.b"); !ok || v != "dotted-key" {
		t.Errorf("expected dotted-key, got %v,%v", v, ok)
	}
	// deep traversal when exact key not found
	if v, ok := GetNested(m, "a.b.c"); !ok || v != "deep" {
		t.Errorf("expected deep, got %v,%v", v, ok)
	}
	// flat key
	if v, ok := GetNested(m, "flat"); !ok || v != "value" {
		t.Errorf("expected value, got %v,%v", v, ok)
	}
	// missing path
	if _, ok := GetNested(m, "a.z.c"); ok {
		t.Error("missing path should return false")
	}
	// non-map intermediate
	m2 := map[string]any{"a": "string"}
	if _, ok := GetNested(m2, "a.b"); ok {
		t.Error("non-map intermediate should return false")
	}
	// nil map
	if _, ok := GetNested(nil, "a"); ok {
		t.Error("nil map should return false")
	}
}

func TestGetNestedString(t *testing.T) {
	m := map[string]any{
		"user": map[string]any{"name": "alice"},
	}
	if v, ok := GetNestedString(m, "user.name"); !ok || v != "alice" {
		t.Errorf("expected alice, got %q,%v", v, ok)
	}
	if _, ok := GetNestedString(m, "user.missing"); ok {
		t.Error("expected false for missing key")
	}
	// value is not a string
	m2 := map[string]any{"n": 42}
	if _, ok := GetNestedString(m2, "n"); ok {
		t.Error("int value should not cast to string")
	}
}

func TestGetNestedStringOrDefault(t *testing.T) {
	m := map[string]any{
		"cfg": map[string]any{"env": "prod"},
	}
	if v := GetNestedStringOrDefault(m, "cfg.env", "dev"); v != "prod" {
		t.Errorf("got %q want prod", v)
	}
	if v := GetNestedStringOrDefault(m, "cfg.missing", "default"); v != "default" {
		t.Errorf("got %q want default", v)
	}
}

// ── MustGet* ──────────────────────────────────────────────────────────────────

func TestMustGetString(t *testing.T) {
	m := map[string]any{"k": "v"}
	if v := MustGetString(m, "k"); v != "v" {
		t.Errorf("got %q want v", v)
	}
}

func TestMustGetString_Panics(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Error("expected panic for missing key")
		}
	}()
	MustGetString(map[string]any{}, "missing")
}

func TestMustGetInt(t *testing.T) {
	m := map[string]any{"n": 42}
	if v := MustGetInt(m, "n"); v != 42 {
		t.Errorf("got %d want 42", v)
	}
}

func TestMustGetInt_Panics(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Error("expected panic for missing key")
		}
	}()
	MustGetInt(map[string]any{}, "missing")
}

func TestMustGetMap(t *testing.T) {
	inner := map[string]any{"a": 1}
	m := map[string]any{"inner": inner}
	if v := MustGetMap(m, "inner"); v["a"] != 1 {
		t.Error("expected inner map")
	}
}

func TestMustGetMap_Panics(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Error("expected panic for missing key")
		}
	}()
	MustGetMap(map[string]any{}, "missing")
}

// ── Merge ─────────────────────────────────────────────────────────────────────

func TestMerge(t *testing.T) {
	a := map[string]any{"x": 1, "y": 2}
	b := map[string]any{"y": 99, "z": 3}
	result := Merge(a, b)

	if result["x"] != 1 {
		t.Error("expected x=1")
	}
	if result["y"] != 99 {
		t.Error("later map should override: expected y=99")
	}
	if result["z"] != 3 {
		t.Error("expected z=3")
	}
	// originals unchanged
	if a["y"] != 2 {
		t.Error("original map a should be unchanged")
	}
}

func TestMerge_Empty(t *testing.T) {
	result := Merge()
	if len(result) != 0 {
		t.Error("merging no maps should return empty map")
	}
}

// ── Pick ──────────────────────────────────────────────────────────────────────

func TestPick(t *testing.T) {
	m := map[string]any{"a": 1, "b": 2, "c": 3}
	result := Pick(m, "a", "c", "missing")

	if len(result) != 2 {
		t.Errorf("expected 2 keys, got %d", len(result))
	}
	if result["a"] != 1 || result["c"] != 3 {
		t.Error("wrong values in result")
	}
	if _, ok := result["b"]; ok {
		t.Error("b should not be in result")
	}
}

// ── Omit ──────────────────────────────────────────────────────────────────────

func TestOmit(t *testing.T) {
	m := map[string]any{"a": 1, "b": 2, "c": 3}
	result := Omit(m, "b")

	if len(result) != 2 {
		t.Errorf("expected 2 keys, got %d", len(result))
	}
	if _, ok := result["b"]; ok {
		t.Error("b should be omitted")
	}
	if result["a"] != 1 || result["c"] != 3 {
		t.Error("wrong values in result")
	}
}

// ── Keys / StringMapKeys / Values ─────────────────────────────────────────────

func TestKeys(t *testing.T) {
	m := map[string]any{"b": 2, "a": 1}
	keys := Keys(m)
	sort.Strings(keys)
	if len(keys) != 2 || keys[0] != "a" || keys[1] != "b" {
		t.Errorf("got %v", keys)
	}
}

func TestKeys_Empty(t *testing.T) {
	if keys := Keys(map[string]any{}); len(keys) != 0 {
		t.Error("expected empty slice")
	}
}

func TestStringMapKeys(t *testing.T) {
	m := map[string]string{"x": "1", "y": "2"}
	keys := StringMapKeys(m)
	sort.Strings(keys)
	if len(keys) != 2 || keys[0] != "x" || keys[1] != "y" {
		t.Errorf("got %v", keys)
	}
}

func TestValues(t *testing.T) {
	m := map[string]any{"a": 1, "b": 2}
	vals := Values(m)
	if len(vals) != 2 {
		t.Errorf("expected 2 values, got %d", len(vals))
	}
}

// ── GetMapFromSlice / GetFirstMapFromSlice ─────────────────────────────────────

func TestGetMapFromSlice(t *testing.T) {
	sl := []any{
		map[string]any{"n": 1},
		map[string]any{"n": 2},
		"not a map",
	}

	m, err := GetMapFromSlice(sl, 0)
	if err != nil || m["n"] != 1 {
		t.Errorf("index 0: got %v, %v", m, err)
	}

	m, err = GetMapFromSlice(sl, 1)
	if err != nil || m["n"] != 2 {
		t.Errorf("index 1: got %v, %v", m, err)
	}

	if _, err := GetMapFromSlice(sl, -1); err == nil {
		t.Error("negative index should error")
	}
	if _, err := GetMapFromSlice(sl, 10); err == nil {
		t.Error("out-of-bounds index should error")
	}
	if _, err := GetMapFromSlice(sl, 2); err == nil {
		t.Error("non-map element should error")
	}
}

func TestGetFirstMapFromSlice(t *testing.T) {
	sl := []any{map[string]any{"first": true}}
	m, err := GetFirstMapFromSlice(sl)
	if err != nil || m["first"] != true {
		t.Errorf("got %v, %v", m, err)
	}

	if _, err := GetFirstMapFromSlice(nil); err == nil {
		t.Error("empty slice should error")
	}
	if _, err := GetFirstMapFromSlice([]any{}); err == nil {
		t.Error("empty slice should error")
	}
}
