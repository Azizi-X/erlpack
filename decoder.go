package erlpack

import (
	"bytes"
	"compress/zlib"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"math/big"
	"slices"
	"unsafe"

	"github.com/segmentio/encoding/json"
)

const (
	SMALL_INTEGER_EXT = 97
	INTEGER_EXT       = 98
	FLOAT_EXT         = 99
	ATOM_EXT          = 100
	SMALL_ATOM_EXT    = 115
	SMALL_TUPLE_EXT   = 104
	LARGE_TUPLE_EXT   = 105
	NIL_EXT           = 106
	STRING_EXT        = 107
	LIST_EXT          = 108
	MAP_EXT           = 116
	BINARY_EXT        = 109
	SMALL_BIG_EXT     = 110
	LARGE_BIG_EXT     = 111
	REFERENCE_EXT     = 101
	NEW_REFERENCE_EXT = 114
	PORT_EXT          = 102
	PID_EXT           = 103
	EXPORT_EXT        = 113
	NEW_FLOAT_EXT     = 70
	COMPRESSED        = 80

	FORMAT_VERSION = 131
)

type Decoder struct {
	data   []byte
	offset int
}

func NewDecoder(data []byte) (*Decoder, error) {
	if len(data) == 0 || data[0] != FORMAT_VERSION {
		return nil, errors.New("invalid or missing format version")
	}
	return &Decoder{data: data[1:]}, nil
}

func (d *Decoder) read(n int) ([]byte, error) {
	if d.offset+n > len(d.data) {
		return nil, errors.New("read past end of buffer")
	}
	val := d.data[d.offset : d.offset+n]
	d.offset += n
	return val, nil
}

func (d *Decoder) readUint8() (uint8, error) {
	bytes, err := d.read(1)
	return bytes[0], err
}

func (d *Decoder) readUint16() (uint16, error) {
	bytes, err := d.read(2)
	return binary.BigEndian.Uint16(bytes), err
}

func (d *Decoder) readUint32() (uint32, error) {
	bytes, err := d.read(4)
	return binary.BigEndian.Uint32(bytes), err
}

func (d *Decoder) readUint64() (uint64, error) {
	bytes, err := d.read(8)
	return binary.BigEndian.Uint64(bytes), err
}

func (d *Decoder) DecodeToBytes() ([]byte, error) {
	decoded, err := d.Decode()
	if err != nil {
		return nil, err
	}

	return json.Marshal(decoded)
}

func (d *Decoder) Decode() (any, error) {
	tag, err := d.readUint8()
	if err != nil {
		return nil, err
	}

	switch tag {
	case SMALL_INTEGER_EXT:
		return d.readUint8()
	case INTEGER_EXT:
		bytes, err := d.read(4)
		if err != nil {
			return nil, err
		}
		return int32(binary.BigEndian.Uint32(bytes)), nil
	case FLOAT_EXT:
		raw, _ := d.read(31)
		var f float64
		_, err := fmt.Sscanf(string(bytes.Trim(raw, "\x00")), "%f", &f)
		return f, err
	case NEW_FLOAT_EXT:
		bits, _ := d.readUint64()
		return float64FromBits(bits), nil
	case ATOM_EXT, SMALL_ATOM_EXT:
		var length int
		if tag == ATOM_EXT {
			l, _ := d.readUint16()
			length = int(l)
		} else {
			l, _ := d.readUint8()
			length = int(l)
		}
		raw, _ := d.read(length)
		str := string(raw)
		switch str {
		case "true":
			return true, nil
		case "false":
			return false, nil
		case "nil", "null":
			return nil, nil
		default:
			return str, nil
		}
	case STRING_EXT:
		length, _ := d.readUint16()
		raw, _ := d.read(int(length))
		return string(raw), nil
	case BINARY_EXT:
		length, _ := d.readUint32()
		raw, _ := d.read(int(length))
		return string(raw), nil
	case NIL_EXT:
		return []any{}, nil
	case LIST_EXT:
		length, _ := d.readUint32()
		list := make([]any, length)
		for i := range length {
			item, _ := d.Decode()
			list[i] = item
		}
		_, _ = d.readUint8()
		return list, nil
	case SMALL_TUPLE_EXT:
		length, _ := d.readUint8()
		return d.decodeTuple(int(length))
	case LARGE_TUPLE_EXT:
		length, _ := d.readUint32()
		return d.decodeTuple(int(length))
	case MAP_EXT:
		length, _ := d.readUint32()
		m := make(map[string]any)
		for range length {
			key, _ := d.Decode()
			val, _ := d.Decode()
			m[fmt.Sprint(key)] = val
		}
		return m, nil
	case SMALL_BIG_EXT:
		return d.decodeBigInt()
	case COMPRESSED:
		return d.decodeCompressed()
	default:
		return nil, fmt.Errorf("unsupported tag: %d", tag)
	}
}

func (d *Decoder) decodeTuple(length int) ([]any, error) {
	tuple := make([]any, length)
	for i := range length {
		val, err := d.Decode()
		if err != nil {
			return nil, err
		}
		tuple[i] = val
	}
	return tuple, nil
}

func (d *Decoder) decodeBigInt() (*big.Int, error) {
	n, _ := d.readUint8()
	sign, _ := d.readUint8()
	raw, _ := d.read(int(n))
	slices.Reverse(raw)
	bi := new(big.Int).SetBytes(raw)
	if sign != 0 {
		bi = bi.Neg(bi)
	}
	return bi, nil
}

func (d *Decoder) decodeCompressed() (any, error) {
	uncompressedSize, _ := d.readUint32()
	comp := d.data[d.offset:]
	reader, err := zlib.NewReader(bytes.NewReader(comp))
	if err != nil {
		return nil, err
	}
	defer reader.Close()

	uncomp := make([]byte, uncompressedSize)
	_, err = io.ReadFull(reader, uncomp)
	if err != nil {
		return nil, err
	}

	child, err := NewDecoder(uncomp)
	if err != nil {
		return nil, err
	}
	return child.Decode()
}

func float64FromBits(bits uint64) float64 {
	return *(*float64)(unsafe.Pointer(&bits))
}
