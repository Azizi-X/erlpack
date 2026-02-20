package erlpack

import (
	"encoding/binary"
	"fmt"
	"math"
	"reflect"
)

type Encoder struct{}

func NewEncoder() *Encoder {
	return &Encoder{}
}

func (*Encoder) AppendByte(b byte) []byte {
	return append([]byte{}, b)
}

func (*Encoder) AppendUint32(val uint32) []byte {
	buf := make([]byte, 4)
	binary.BigEndian.PutUint32(buf, val)
	return buf
}

func (*Encoder) AppendUint16(val uint16) []byte {
	buf := make([]byte, 2)
	binary.BigEndian.PutUint16(buf, val)
	return buf
}

func (e *Encoder) AppendBinary(s string) []byte {
	result := e.AppendByte(BINARY_EXT)
	result = append(result, e.AppendUint32(uint32(len(s)))...)
	result = append(result, []byte(s)...)
	return result
}

func (*Encoder) AppendFloat64(f float64) []byte {
	buf := make([]byte, 8)
	binary.BigEndian.PutUint64(buf, math.Float64bits(f))
	return buf
}

func (e *Encoder) AppendInt(v int64) []byte {
	if v >= 0 && v <= 255 {
		result := e.AppendByte(SMALL_INTEGER_EXT)
		result = append(result, byte(v))
		return result
	} else {
		result := e.AppendByte(INTEGER_EXT)
		result = append(result, e.AppendInt32(int32(v))...)
		return result
	}
}

func (*Encoder) AppendInt32(v int32) []byte {
	buf := make([]byte, 4)
	binary.BigEndian.PutUint32(buf, uint32(v))
	return buf
}

func (*Encoder) AppendInt64(v int64) []byte {
	buf := make([]byte, 8)
	binary.BigEndian.PutUint64(buf, uint64(v))
	return buf
}

func (e *Encoder) AppendMap(m map[string]any) []byte {
	length := len(m)

	if length > math.MaxUint32-1 {
		panic("Dictionary has too many properties")
	}

	result := e.AppendByte(MAP_EXT)
	result = append(result, e.AppendUint32(uint32(length))...)
	for key, val := range m {
		result = append(result, e.AppendBinary(key)...)
		result = append(result, e.rawPack(val)...)
	}
	return result
}

func (*Encoder) AppendNil() []byte {
	return []byte{SMALL_ATOM_EXT, 3, 'n', 'i', 'l'}
}

func (*Encoder) AppendBool(v bool) []byte {
	if v {
		return []byte{SMALL_ATOM_EXT, 4, 't', 'r', 'u', 'e'}
	}

	return []byte{SMALL_ATOM_EXT, 5, 'f', 'a', 'l', 's', 'e'}
}

func (*Encoder) convertMap(x any) (map[string]any, bool) {
	v := reflect.ValueOf(x)
	if v.Kind() != reflect.Map {
		return nil, false
	}
	out := make(map[string]any, v.Len())
	for _, key := range v.MapKeys() {
		if key.Kind() != reflect.String {
			panic(fmt.Sprintf("expected string, got %s", key.Type()))
		}
		out[key.String()] = v.MapIndex(key).Interface()
	}
	return out, true
}

func (e *Encoder) Pack(value any) []byte {
	return append([]byte{FORMAT_VERSION}, e.rawPack(value)...)
}

func (e *Encoder) rawPack(value any) []byte {
	var result []byte

	switch v := value.(type) {
	case int:
		result = append(result, e.AppendInt(int64(v))...)
	case *int:
		if v == nil {
			result = append(result, e.AppendNil()...)
		} else {
			result = append(result, e.AppendInt(int64(*v))...)
		}
	case int32:
		result = append(result, e.AppendInt(int64(v))...)
	case int64:
		result = append(result, e.AppendInt(v)...)
	case float32:
		result = append(result, e.AppendByte(NEW_FLOAT_EXT)...)
		result = append(result, e.AppendFloat64(float64(v))...)
	case float64:
		result = append(result, e.AppendByte(NEW_FLOAT_EXT)...)
		result = append(result, e.AppendFloat64(v)...)
	case *string:
		if v == nil {
			result = append(result, e.AppendNil()...)
		} else {
			result = append(result, e.AppendBinary(*v)...)
		}
	case string:
		result = append(result, e.AppendBinary(v)...)
	case bool:
		result = append(result, e.AppendBool(v)...)
	case nil:
		result = append(result, e.AppendNil()...)
	case []any:
		result = append(result, e.AppendByte(LIST_EXT)...)
		result = append(result, e.AppendUint32(uint32(len(v)))...)
		for i := range v {
			result = append(result, e.rawPack(v[i])...)
		}
		result = append(result, e.AppendByte(NIL_EXT)...)
	case map[string]any:
		result = append(result, e.AppendMap(v)...)
	default:
		t := reflect.TypeOf(v)
		val := reflect.ValueOf(v)

		for t.Kind() == reflect.Pointer {
			if val.IsNil() {
				result = append(result, e.AppendNil()...)
				return result
			}

			t = t.Elem()
			val = val.Elem()
		}

		kind := t.Kind()

		if kind == reflect.Map {
			v, ok := e.convertMap(v)
			if ok {
				return e.AppendMap(v)
			}
		}

		switch kind {
		case reflect.Struct:
			var data = NewStruct(v).Map()
			result = append(result, e.rawPack(data)...)
		case reflect.Slice, reflect.Array:
			length := val.Len()

			if length == 0 {
				result = append(result, e.AppendByte(NIL_EXT)...)
				return result
			} else if length > math.MaxUint32-1 {
				panic("List is too large")
			}

			result = append(result, e.AppendByte(LIST_EXT)...)
			result = append(result, e.AppendUint32(uint32(val.Len()))...)
			for i := range val.Len() {
				item := val.Index(i).Interface()
				result = append(result, e.rawPack(item)...)
			}
			result = append(result, e.AppendByte(NIL_EXT)...)
		default:
			panic("Unsupported etf type: " + fmt.Sprintf("%T", v))
		}
	}

	return result
}
