package mibig

import (
	"bytes"
	"encoding/json"
	"reflect"
	"slices"
)

var citationType = reflect.TypeFor[Citation]()

// walk calls visit for v and every value reachable from it, with the JSON
// path of the value. Raw JSON values are not descended into.
func walk(rv reflect.Value, path string, visit func(rv reflect.Value, path string)) {
	switch rv.Kind() {
	case reflect.Pointer, reflect.Interface:
		if !rv.IsNil() {
			walk(rv.Elem(), path, visit)
		}
	case reflect.Struct:
		visit(rv, path)
		info := getStructInfo(rv.Type())
		for i := range info.fields {
			fi := &info.fields[i]
			walk(rv.FieldByIndex(fi.index), join(path, fi.name), visit)
		}
	case reflect.Slice:
		if rv.Type().Elem().Kind() == reflect.Uint8 {
			return
		}
		for i := range rv.Len() {
			walk(rv.Index(i), index(path, i), visit)
		}
	default:
		visit(rv, path)
	}
}

// validateExtras reports the unknown keys, preserved empty values and missing
// required keys found anywhere in the entry. It runs after the validation of
// the types, and does not report missing or empty required values where they
// already reported an error.
func validateExtras(v *validator, root any) {
	reported := map[string]bool{}
	for _, issue := range v.issues {
		if issue.Severity == SeverityError {
			reported[issue.Path] = true
		}
	}
	walk(reflect.ValueOf(root), "", func(rv reflect.Value, path string) {
		if rv.Kind() != reflect.Struct {
			return
		}
		info := getStructInfo(rv.Type())
		// Optional lists that are present must not be empty (minItems 1 in
		// the schema), but the data has many, so these are only warnings.
		for i := range info.fields {
			fi := &info.fields[i]
			fv := rv.FieldByIndex(fi.index)
			if fi.nonEmpty && fv.Kind() == reflect.Slice && !fv.IsNil() && fv.Len() == 0 && !reported[join(path, fi.name)] {
				v.warnf(join(path, fi.name), CodeEmptyValue, "empty value [], remove the key")
			}
		}
		if info.extras == nil {
			return
		}
		extras := rv.FieldByIndex(info.extras).Interface().(Extras)
		for _, p := range extras.preserved {
			fieldPath := join(path, p.key)
			fi := &info.fields[info.byKey[p.field]]
			switch {
			case !isZero(rv.FieldByIndex(fi.index)):
				// the field was set after decoding
			case p.value == nil:
				if !fi.optional && !reported[fieldPath] {
					v.errorf(fieldPath, CodeRequired, "required key %q is missing", p.key)
				}
			case reported[fieldPath]:
				// the validation of the type reported an error
			case fi.omitZero || fi.optional:
				v.warnf(fieldPath, CodeEmptyValue, "empty value %s, remove the key", p.value)
			default:
				// Only values the zero value does not reproduce are kept,
				// e.g. null for a number or an object.
				v.errorf(fieldPath, CodeRequired, "required key %q has the empty value %s", p.key, p.value)
			}
		}
		for _, unknown := range extras.Unknown {
			fieldPath := join(path, unknown.Key)
			if isEmptyJSON(unknown.Value) {
				v.warnf(fieldPath, CodeUnknownField, "unknown key %q with empty value %s, remove it", unknown.Key, unknown.Value)
			} else {
				v.errorf(fieldPath, CodeUnknownField, "unknown key %q", unknown.Key)
			}
		}
	})
}

// isEmptyJSON reports whether raw is null, "", [] or {}.
func isEmptyJSON(raw json.RawMessage) bool {
	var compacted bytes.Buffer
	if json.Compact(&compacted, raw) != nil {
		return false
	}
	switch compacted.String() {
	case "", "null", `""`, "[]", "{}":
		return true
	}
	return false
}

// References returns all citations used anywhere in the entry, without
// duplicates and sorted by database and identifier.
func (e *Entry) References() []Citation {
	var citations []Citation
	walk(reflect.ValueOf(e), "", func(rv reflect.Value, _ string) {
		if rv.Type() == citationType {
			citations = append(citations, rv.Interface().(Citation))
		}
	})
	slices.SortFunc(citations, func(a, b Citation) int {
		if c := compareStrings(a.Database(), b.Database()); c != 0 {
			return c
		}
		return compareStrings(a.ID(), b.ID())
	})
	return slices.Compact(citations)
}

func compareStrings(a, b string) int {
	switch {
	case a < b:
		return -1
	case a > b:
		return 1
	}
	return 0
}
