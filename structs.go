package erlpack

// https://github.com/fatih/structs

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"reflect"
	"slices"
	"strings"
)

type Struct struct {
	raw      any
	value    reflect.Value
	TagName  string
	Flattern bool
}

func NewStruct(s any) *Struct {
	return &Struct{
		raw:      s,
		value:    strctVal(s),
		TagName:  "json",
		Flattern: true,
	}
}

func strctVal(s any) reflect.Value {
	v := reflect.ValueOf(s)

	for v.Kind() == reflect.Pointer {
		v = v.Elem()
	}

	if v.Kind() != reflect.Struct {
		panic("not struct")
	}

	return v
}

func (s *Struct) structFields() []reflect.StructField {
	t := s.value.Type()

	var f []reflect.StructField

	for i := range t.NumField() {
		field := t.Field(i)
		if field.PkgPath != "" {
			continue
		}

		if tag := field.Tag.Get(s.TagName); tag == "-" {
			continue
		}

		f = append(f, field)
	}

	return f
}

type tagOptions []string

func (t tagOptions) Has(opt string) bool {
	return slices.Contains(t, opt)
}

func parseTag(tag string) (string, tagOptions) {
	res := strings.Split(tag, ",")
	return res[0], res[1:]
}

func (s *Struct) FillMap(out map[string]any) {
	if out == nil {
		return
	}

	fields := s.structFields()

	for _, field := range fields {
		name := field.Name
		val := s.value.FieldByName(name)
		isSubStruct := false
		var finalVal any

		if val.CanInterface() {
			if marshaler, ok := val.Interface().(json.Marshaler); ok {
				if b, err := marshaler.MarshalJSON(); err == nil {
					var temp any
					if err := json.Unmarshal(b, &temp); err == nil {
						val = reflect.ValueOf(temp)
					}
				}
			}
		}

		tagName, tagOpts := parseTag(field.Tag.Get(s.TagName))
		if tagName != "" {
			name = tagName
		}

		if tagOpts.Has("omitempty") {
			zero := reflect.Zero(val.Type()).Interface()
			current := val.Interface()

			if reflect.DeepEqual(current, zero) {
				continue
			}
		}

		if tagOpts.Has("omitzero") {
			if s.isZeroValue(val) {
				continue
			}
		}

		if !tagOpts.Has("omitnested") {
			if s.isNilValue(val) {
				finalVal = nil
			} else {
				finalVal = s.nested(val)
				v := reflect.ValueOf(val.Interface())
				if v.Kind() == reflect.Pointer {
					v = v.Elem()
				}

				switch v.Kind() {
				case reflect.Map, reflect.Struct:
					isSubStruct = true
				}
			}
		} else {
			finalVal = val.Interface()
		}

		if tagOpts.Has("string") {
			s, ok := val.Interface().(fmt.Stringer)
			if ok {
				out[name] = s.String()
			}
			continue
		}

		if isSubStruct && tagOpts.Has("flatten") || (s.Flattern && field.Anonymous && field.Type.Kind() == reflect.Struct && len(tagOpts) == 0) {
			for k := range finalVal.(map[string]any) {
				out[k] = finalVal.(map[string]any)[k]
			}
		} else {
			out[name] = finalVal
		}
	}
}

func (s *Struct) isZeroValue(v reflect.Value) bool {
	if !v.IsValid() {
		return true
	}

	for v.Kind() == reflect.Pointer || v.Kind() == reflect.Interface {
		if v.IsNil() {
			return true
		}
		v = v.Elem()
	}

	if v.CanInterface() {
		if method := v.MethodByName("IsZero"); method.IsValid() && method.Type().NumIn() == 0 && method.Type().NumOut() == 1 {
			if method.Type().Out(0).Kind() == reflect.Bool {
				result := method.Call(nil)
				return result[0].Bool()
			}
		}
	}

	switch v.Kind() {
	case reflect.Pointer, reflect.Interface:
		if v.IsNil() {
			return true
		}
		return s.isZeroValue(v.Elem())
	case reflect.Struct:
		for i := range v.NumField() {
			field := v.Field(i)
			if !s.isZeroValue(field) {
				return false
			}
		}
		return true
	case reflect.Array, reflect.Slice:
		for i := range v.Len() {
			if !s.isZeroValue(v.Index(i)) {
				return false
			}
		}
		return true
	case reflect.Map:
		return v.Len() == 0
	default:
		return v.IsZero()
	}
}

func (*Struct) isNilValue(v reflect.Value) bool {
	if !v.IsValid() {
		return true
	}

	switch v.Kind() {
	case reflect.Pointer, reflect.Map, reflect.Slice,
		reflect.Interface, reflect.Func, reflect.Chan:
		return v.IsNil()
	}

	return false
}

func (s *Struct) Exported() map[string]any {
	return collectExportedFields(reflect.TypeOf(s.raw))
}

func collectExportedFields(t reflect.Type) map[string]any {
	for t.Kind() == reflect.Pointer {
		t = t.Elem()
	}

	if t.Kind() != reflect.Struct {
		return nil
	}

	result := make(map[string]any)

	for i := 0; i < t.NumField(); i++ {
		f := t.Field(i)

		if !f.IsExported() {
			continue
		}

		tag := f.Tag.Get("json")
		parts := strings.Split(tag, ",")
		if len(parts) == 0 {
			continue
		}

		jsonTag := parts[0]
		if jsonTag == "-" || jsonTag == "" {
			continue
		}

		ft := f.Type
		for ft.Kind() == reflect.Pointer {
			ft = ft.Elem()
		}

		if ft.Kind() == reflect.Struct {
			m := collectExportedFields(ft)
			if len(m) == 0 {
				result[jsonTag] = nil
			} else {
				result[jsonTag] = m
			}
		} else {
			result[jsonTag] = nil
		}
	}

	return result
}

func (s *Struct) Hash() string {
	var b strings.Builder
	s.hashStruct(reflect.TypeOf(s.raw), &b)
	sum := sha256.Sum256([]byte(b.String()))
	return hex.EncodeToString(sum[:])[:16]
}

func (s *Struct) hashStruct(t reflect.Type, b *strings.Builder) {
	for t.Kind() == reflect.Pointer {
		t = t.Elem()
	}

	for i := range t.NumField() {
		f := t.Field(i)

		if !f.IsExported() {
			continue
		}

		parts := strings.Split(f.Tag.Get("json"), ",")
		if len(parts) == 0 {
			continue
		}

		jsonTag := parts[0]
		if jsonTag == "-" || jsonTag == "" {
			continue
		}

		fmt.Fprintf(b, "%s:%s:%s;", f.Name, f.Type.String(), jsonTag)

		ft := f.Type
		for ft.Kind() == reflect.Pointer {
			ft = ft.Elem()
		}

		if ft.Kind() == reflect.Struct {
			s.hashStruct(ft, b)
		}
	}
}

func (s *Struct) Map() map[string]any {
	out := make(map[string]any)
	s.FillMap(out)
	return out
}

func (s *Struct) nested(val reflect.Value) any {
	var finalVal any

	v := reflect.ValueOf(val.Interface())
	if v.Kind() == reflect.Pointer {
		v = v.Elem()
	}

	switch v.Kind() {
	case reflect.Struct:
		n := NewStruct(val.Interface())
		n.TagName = s.TagName
		m := n.Map()

		if len(m) == 0 {
			finalVal = val.Interface()
		} else {
			finalVal = m
		}
	case reflect.Map:
		mapElem := val.Type()
		switch val.Type().Kind() {
		case reflect.Pointer, reflect.Array, reflect.Map,
			reflect.Slice, reflect.Chan:
			mapElem = val.Type().Elem()
			if mapElem.Kind() == reflect.Pointer {
				mapElem = mapElem.Elem()
			}
		}

		if mapElem.Kind() == reflect.Struct ||
			(mapElem.Kind() == reflect.Slice &&
				mapElem.Elem().Kind() == reflect.Struct) {
			m := make(map[string]any, val.Len())
			for _, k := range val.MapKeys() {
				m[k.String()] = s.nested(val.MapIndex(k))
			}
			finalVal = m
			break
		}

		finalVal = val.Interface()
	case reflect.Slice, reflect.Array:
		if val.Type().Kind() == reflect.Interface {
			finalVal = val.Interface()
			break
		}

		if val.Type().Elem().Kind() != reflect.Struct &&
			!(val.Type().Elem().Kind() == reflect.Pointer &&
				val.Type().Elem().Elem().Kind() == reflect.Struct) {
			finalVal = val.Interface()
			break
		}

		slices := make([]any, val.Len())
		for x := range val.Len() {
			slices[x] = s.nested(val.Index(x))
		}
		finalVal = slices
	default:
		finalVal = val.Interface()
	}

	return finalVal
}
