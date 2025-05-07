package erlpack

import (
	"encoding/binary"
	"encoding/json"
	"fmt"
	"math"
	"reflect"
)

type Encoder struct{}

func NewEncoder() *Encoder {
	return &Encoder{}
}

func (e *Encoder) AppendByte(b byte) []byte {
	return append([]byte{}, b)
}

func (e *Encoder) AppendUint32(val uint32) []byte {
	buf := make([]byte, 4)
	binary.BigEndian.PutUint32(buf, val)
	return buf
}

func (e *Encoder) AppendUint16(val uint16) []byte {
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

func (e *Encoder) AppendFloat64(f float64) []byte {
	buf := make([]byte, 8)
	binary.BigEndian.PutUint64(buf, math.Float64bits(f))
	return buf
}

func (e *Encoder) AppendInt(v int) []byte {
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

func (e *Encoder) AppendInt32(v int32) []byte {
	buf := make([]byte, 4)
	binary.BigEndian.PutUint32(buf, uint32(v))
	return buf
}

func (e *Encoder) AppendMap(m map[string]any) []byte {
	result := e.AppendByte(MAP_EXT)
	result = append(result, e.AppendUint32(uint32(len(m)))...)
	for key, val := range m {
		result = append(result, e.AppendBinary(key)...)
		result = append(result, e.rawPack(val)...)
	}
	return result
}

func (e *Encoder) pack(value any) []byte {
	return append([]byte{FORMAT_VERSION}, e.rawPack(value)...)
}

func (e *Encoder) rawPack(value any) []byte {
	var result []byte

	switch v := value.(type) {
	case int:
		result = append(result, e.AppendInt(v)...)
	case int32:
		result = append(result, e.AppendInt32(v)...)
	case float64:
		result = append(result, e.AppendByte(NEW_FLOAT_EXT)...)
		result = append(result, e.AppendFloat64(v)...)
	case string:
		result = append(result, e.AppendBinary(v)...)
	case bool:
		if v {
			result = append(result, e.AppendBinary("true")...)
		} else {
			result = append(result, e.AppendBinary("false")...)
		}
	case nil:
		result = append(result, e.AppendByte(MAP_EXT)...)
		result = append(result, e.AppendUint32(0)...)
	case []any:
		result = append(result, e.AppendByte(LIST_EXT)...)
		result = append(result, e.AppendUint32(uint32(len(v)))...)
		for i := range v {
			result = append(result, e.pack(v[i])...)
		}
		result = append(result, e.AppendByte(NIL_EXT)...)
	case map[string]any:
		result = append(result, e.AppendMap(v)...)
	default:
		t := reflect.TypeOf(v)
		val := reflect.ValueOf(v)

		for t.Kind() == reflect.Ptr {
			t = t.Elem()
			val = val.Elem()
		}

		switch t.Kind() {
		case reflect.Struct:
			var data map[string]any
			bytes, err := json.Marshal(v)
			if err != nil {
				panic(err)
			} else if err := json.Unmarshal(bytes, &data); err != nil {
				panic(err)
			}
			result = append(result, e.pack(data)...)

		case reflect.Slice, reflect.Array:
			result = append(result, e.AppendByte(LIST_EXT)...)
			result = append(result, e.AppendUint32(uint32(val.Len()))...)
			for i := range val.Len() {
				item := val.Index(i).Interface()
				result = append(result, e.pack(item)...)
			}
			result = append(result, e.AppendByte(NIL_EXT)...)
		default:
			fmt.Printf("Unsupported type: %T\n", v)
		}
	}

	return result
}
