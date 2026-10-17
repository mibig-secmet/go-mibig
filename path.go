package mibig

import (
	"encoding/json"
	"errors"
	"strconv"
	"strings"
)

// This file builds and parses the JSON paths used in decode errors and
// validation issues, e.g. `biosynthesis.modules[2]["odd.key"]`.

// join appends an object key to a path.
func join(path, key string) string {
	return joinPath(path, pathKey(key))
}

// index appends an array index to a path.
func index(path string, i int) string {
	return path + "[" + strconv.Itoa(i) + "]"
}

// wrapKey prefixes the path of err with an object key.
func wrapKey(key string, err error) error {
	return wrapPath(pathKey(key), err)
}

// pathKey returns key as a path segment. Keys that are not plain words are
// quoted, e.g. ["a.b"], so that paths stay unambiguous.
func pathKey(key string) string {
	if key == "" || strings.ContainsFunc(key, func(r rune) bool {
		return !(r == '_' || r == '-' || r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9')
	}) {
		quoted, _ := marshalNoEscape(key)
		return "[" + string(quoted) + "]"
	}
	return key
}

// wrapIndex prefixes the path of err with an array index.
func wrapIndex(index int, err error) error {
	return wrapPath("["+strconv.Itoa(index)+"]", err)
}

func wrapPath(segment string, err error) error {
	if err == nil {
		return nil
	}
	var de *DecodeError
	if errors.As(err, &de) {
		return &DecodeError{Path: joinPath(segment, de.Path), Err: de.Err}
	}
	return &DecodeError{Path: segment, Err: err}
}

// joinPath joins two JSON path fragments, e.g. "a" + "b" = "a.b" and
// "a" + "[1]" = "a[1]".
func joinPath(prefix, suffix string) string {
	switch {
	case prefix == "":
		return suffix
	case suffix == "":
		return prefix
	case suffix[0] == '[':
		return prefix + suffix
	default:
		return prefix + "." + suffix
	}
}

// splitPath splits a path like `a.b[2]["odd.key"]` into its segments.
func splitPath(path string) []string {
	var segments []string
	for len(path) > 0 {
		switch {
		case path[0] == '.':
			path = path[1:]
		case strings.HasPrefix(path, `["`):
			dec := json.NewDecoder(strings.NewReader(path[1:]))
			var key string
			if err := dec.Decode(&key); err != nil {
				return append(segments, path)
			}
			segments = append(segments, key)
			path = strings.TrimPrefix(path[1+dec.InputOffset():], "]")
		case path[0] == '[':
			end := strings.IndexByte(path, ']')
			if end < 0 {
				return append(segments, path[1:])
			}
			segments = append(segments, path[1:end])
			path = path[end+1:]
		default:
			end := strings.IndexAny(path, ".[")
			if end < 0 {
				end = len(path)
			}
			segments = append(segments, path[:end])
			path = path[end:]
		}
	}
	return segments
}
