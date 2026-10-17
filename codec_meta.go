package mibig

import (
	"encoding/json"
	"fmt"
	"reflect"
	"strings"
	"sync"
)

// This file holds the codec's metadata: the struct tag options and the field
// information cached per struct type, and the discriminators of polymorphic
// objects.

// discriminator describes the key that selects the variant of a polymorphic
// object, e.g. "type": "adenylation". The first value is the one written; all
// values are accepted when reading.
type discriminator struct {
	key    string
	values []string
}

func (d *discriminator) accepts(value string) bool {
	for _, v := range d.values {
		if v == value {
			return true
		}
	}
	return false
}

// discriminators holds the discriminator of each variant struct type. It is
// filled by newUnion during package initialisation.
var discriminators = map[reflect.Type]*discriminator{}

type fieldInfo struct {
	name     string
	quoted   []byte // JSON encoded key, including quotes
	aliases  []string
	index    []int
	omitZero bool
	single   bool
	last     bool
	// optional fields are always written for new values, but are not
	// required.
	optional bool
	// nonempty lists must not be empty if present (minItems 1 in the schema).
	nonEmpty bool
}

type structInfo struct {
	fields []fieldInfo
	// order is the output order of the fields, as indices into fields.
	order  []int
	byKey  map[string]int
	extras []int // field index path of the Extras field, nil if there is none
	// disc is the discriminator written and checked for the type, nil if
	// there is none.
	disc *discriminator
}

var (
	structInfoCache sync.Map // reflect.Type -> *structInfo
	extrasType      = reflect.TypeFor[Extras]()
	rawMessageType  = reflect.TypeFor[json.RawMessage]()
	numberType      = reflect.TypeFor[json.Number]()
	marshalerType   = reflect.TypeFor[json.Marshaler]()
	unmarshalerType = reflect.TypeFor[json.Unmarshaler]()
)

func getStructInfo(t reflect.Type) *structInfo {
	if cached, ok := structInfoCache.Load(t); ok {
		return cached.(*structInfo)
	}
	info := &structInfo{byKey: map[string]int{}, disc: discriminators[t]}
	collectFields(t, nil, info)
	for i := range info.fields {
		if !info.fields[i].last {
			info.order = append(info.order, i)
		}
	}
	for i := range info.fields {
		if info.fields[i].last {
			info.order = append(info.order, i)
		}
	}
	cached, _ := structInfoCache.LoadOrStore(t, info)
	return cached.(*structInfo)
}

func collectFields(t reflect.Type, parent []int, info *structInfo) {
	for i := range t.NumField() {
		f := t.Field(i)
		index := append(append([]int(nil), parent...), i)
		if f.Type == extrasType {
			info.extras = index
			continue
		}
		tag, hasTag := f.Tag.Lookup("json")
		if f.Anonymous && !hasTag && f.Type.Kind() == reflect.Struct {
			collectFields(f.Type, index, info)
			continue
		}
		if !f.IsExported() || tag == "-" {
			continue
		}
		name, opts, _ := strings.Cut(tag, ",")
		if name == "" {
			name = f.Name
		}
		quoted, _ := marshalNoEscape(name)
		fi := fieldInfo{
			name:     name,
			quoted:   quoted,
			index:    index,
			omitZero: hasOption(opts, "omitzero"),
		}
		for _, opt := range strings.Split(f.Tag.Get("mibig"), ",") {
			switch {
			case opt == "single":
				fi.single = true
			case opt == "last":
				fi.last = true
			case opt == "optional":
				fi.optional = true
			case opt == "nonempty":
				fi.nonEmpty = true
			case strings.HasPrefix(opt, "alias="):
				fi.aliases = append(fi.aliases, strings.TrimPrefix(opt, "alias="))
			case opt == "":
			default:
				panic(fmt.Sprintf("mibig: unknown struct tag option %q on %s.%s", opt, t.Name(), f.Name))
			}
		}
		pos := len(info.fields)
		info.fields = append(info.fields, fi)
		info.byKey[name] = pos
		for _, alias := range fi.aliases {
			info.byKey[alias] = pos
		}
	}
}

func hasOption(opts, option string) bool {
	for _, opt := range strings.Split(opts, ",") {
		if opt == option {
			return true
		}
	}
	return false
}
