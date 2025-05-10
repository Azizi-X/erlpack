package erlpack

import (
	"bytes"
	"compress/zlib"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"math"
	"strconv"
)

var (
	AtomTrue  = []byte("true")
	AtomFalse = []byte("false")
	AtomNil   = []byte("nil")
	AtomNull  = []byte("null")
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

func NewDecoder() *Decoder {
	return &Decoder{}
}

func (d *Decoder) reset() {
	d.offset = 0
	d.data = []byte{}
}

func (d *Decoder) read8() (uint8, error) {
	if d.offset+1 > len(d.data) {
		return 0, fmt.Errorf("reading a byte exceeds buffer size")
	}
	val := d.data[d.offset]
	d.offset++
	return val, nil
}

func (d *Decoder) read16() (uint16, error) {
	if d.offset+2 > len(d.data) {
		return 0, fmt.Errorf("reading two bytes exceeds buffer size")
	}
	val := binary.BigEndian.Uint16(d.data[d.offset:])
	d.offset += 2
	return val, nil
}

func (d *Decoder) read32() (uint32, error) {
	if d.offset+4 > len(d.data) {
		return 0, fmt.Errorf("reading four bytes exceeds buffer size")
	}
	val := binary.BigEndian.Uint32(d.data[d.offset:])
	d.offset += 4
	return val, nil
}

func (d *Decoder) read64() (uint64, error) {
	if d.offset+8 > len(d.data) {
		return 0, fmt.Errorf("reading eight bytes exceeds buffer size")
	}
	val := binary.BigEndian.Uint64(d.data[d.offset:])
	d.offset += 8
	return val, nil
}

func (d *Decoder) decodeSmallInteger() (int, error) {
	val, err := d.read8()
	if err != nil {
		return 0, err
	}
	return int(val), nil
}

func (d *Decoder) decodeInteger() (int32, error) {
	val, err := d.read32()
	if err != nil {
		return 0, err
	}
	return int32(val), nil
}

func (d *Decoder) readString(length uint32) (string, error) {
	if int(d.offset)+int(length) > len(d.data) {
		return "", fmt.Errorf("reading sequence past the end of the buffer")
	}

	bytes := d.data[d.offset : d.offset+int(length)]
	d.offset += int(length)
	return string(bytes), nil
}

func (d *Decoder) decodeFloat() (float64, error) {
	const floatLength = 31

	str, err := d.readString(floatLength)
	if err != nil {
		return 0, err
	}

	if i := bytes.IndexByte([]byte(str), 0); i >= 0 {
		str = str[:i]
	}

	number, err := strconv.ParseFloat(str, 64)
	if err != nil {
		return 0, fmt.Errorf("invalid float encoded: %w", err)
	}

	return number, nil
}

func (d *Decoder) decodeNewFloat() (float64, error) {
	ui64, err := d.read64()
	if err != nil {
		return 0, err
	}

	return math.Float64frombits(ui64), nil
}

func (d *Decoder) processAtom(atom string, length uint16) (any, error) {
	if atom == "" {
		return nil, nil
	}

	if length >= 3 && length <= 5 {
		switch {
		case length == 3 && atom == "nil":
			return nil, nil
		case length == 4 && atom == "null":
			return nil, nil
		case length == 4 && atom == "true":
			return true, nil
		case length == 5 && atom == "false":
			return false, nil
		}
	}

	return atom, nil
}

func (d *Decoder) decodeAtom() (any, error) {
	length, err := d.read16()
	if err != nil {
		return nil, err
	}
	atom, err := d.readString(uint32(length))
	if err != nil {
		return nil, err
	}

	return d.processAtom(atom, length)
}

func (d *Decoder) decodeSmallAtom() (any, error) {
	length, err := d.read8()
	if err != nil {
		return nil, err
	}
	atom, err := d.readString(uint32(length))
	if err != nil {
		return nil, err
	}

	return d.processAtom(atom, uint16(length))
}

func (d *Decoder) decodeArray(length uint32) ([]any, error) {
	array := make([]any, length)
	for i := uint32(0); i < length; i++ {
		value, err := d.decode()
		if err != nil {
			return nil, err
		}
		array[i] = value
	}
	return array, nil
}

func (d *Decoder) decodeTuple(length uint32) ([]any, error) {
	return d.decodeArray(length)
}

func (d *Decoder) decodeSmallTuple() ([]any, error) {
	length, err := d.read8()
	if err != nil {
		return nil, err
	}
	return d.decodeTuple(uint32(length))
}

func (d *Decoder) decodeLargeTuple() ([]any, error) {
	length, err := d.read32()
	if err != nil {
		return nil, err
	}
	return d.decodeTuple(length)
}

func (d *Decoder) decodeNil() ([]any, error) {
	return []any{}, nil
}

func (d *Decoder) decodeStringAsList() ([]any, error) {
	length, err := d.read16()
	if err != nil {
		return nil, err
	}

	if d.offset+int(length) > len(d.data) {
		return nil, fmt.Errorf("reading sequence past the end of the buffer (1)")
	}

	result := make([]any, length)

	for i := uint16(0); i < length; i++ {
		value, err := d.decodeSmallInteger()
		if err != nil {
			return nil, err
		}
		result[i] = value
	}

	return result, nil
}

func (d *Decoder) decodeList() (any, error) {
	length, err := d.read32()
	if err != nil {
		return nil, err
	}

	array, err := d.decodeArray(length)
	if err != nil {
		return nil, err
	}

	tailMarker, err := d.read8()
	if err != nil {
		return nil, err
	}

	if tailMarker != NIL_EXT {
		return nil, fmt.Errorf("list doesn't end with a tail marker, but it must")
	}

	return array, nil
}

func (d *Decoder) decodeMap() (map[string]any, error) {
	length, err := d.read32()
	if err != nil {
		return nil, err
	}

	resultMap := make(map[string]any, length)

	for i := uint32(0); i < length; i++ {
		key, err := d.decode()
		if err != nil {
			return nil, err
		}

		value, err := d.decode()
		if err != nil {
			return nil, err
		}

		resultMap[fmt.Sprint(key)] = value
	}

	return resultMap, nil
}

func (d *Decoder) decodeBinaryAsString() (string, error) {
	length, err := d.read32()
	if err != nil {
		return "", err
	}

	str, err := d.readString(uint32(length))
	if err != nil {
		return "", err
	}

	return str, nil
}

func (d *Decoder) decodeBig(digits uint32) (any, error) {
	sign, err := d.read8()
	if err != nil {
		return nil, err
	}

	if digits > 8 {
		return nil, fmt.Errorf("Unable to decode big ints larger than 8 bytes")
	}

	var value uint64
	var b uint64 = 1
	for i := uint32(0); i < digits; i++ {
		digit, err := d.read8()
		if err != nil {
			return nil, err
		}
		value += uint64(digit) * b
		b <<= 8
	}

	if digits <= 4 {
		if sign == 0 {
			return uint32(value), nil
		}

		if value&(1<<31) == 0 {
			negativeValue := -int32(value)
			return negativeValue, nil
		}
	}

	var outBuffer string
	if sign == 0 {
		outBuffer = fmt.Sprintf("%d", value)
	} else {
		outBuffer = fmt.Sprintf("-%d", value)
	}

	return outBuffer, nil
}

func (d *Decoder) decodeSmallBig() (any, error) {
	bytes, err := d.read8()
	if err != nil {
		return nil, err
	}
	return d.decodeBig(uint32(bytes))
}

func (d *Decoder) decodeLargeBig() (any, error) {
	bytes, err := d.read32()
	if err != nil {
		return nil, err
	}
	return d.decodeBig(bytes)
}

func (d *Decoder) unpack(data []byte) (any, error) {
	if len(data) == 0 || data[0] != FORMAT_VERSION {
		return nil, errors.New("invalid or missing format version")
	}
	d.data = data[1:]
	bytes, err := d.decode()
	d.reset()
	return bytes, err
}

func (d *Decoder) decode() (any, error) {
	tag, err := d.read8()
	if err != nil {
		return nil, err
	}

	switch tag {
	case SMALL_INTEGER_EXT:
		return d.decodeSmallInteger()
	case INTEGER_EXT:
		return d.decodeInteger()
	case FLOAT_EXT:
		return d.decodeFloat()
	case NEW_FLOAT_EXT:
		return d.decodeNewFloat()
	case ATOM_EXT:
		return d.decodeAtom()
	case SMALL_ATOM_EXT:
		return d.decodeSmallAtom()
	case SMALL_TUPLE_EXT:
		return d.decodeSmallTuple()
	case LARGE_TUPLE_EXT:
		return d.decodeLargeTuple()
	case NIL_EXT:
		return d.decodeNil()
	case STRING_EXT:
		return d.decodeStringAsList()
	case LIST_EXT:
		return d.decodeList()
	case MAP_EXT:
		return d.decodeMap()
	case BINARY_EXT:
		return d.decodeBinaryAsString()
	case SMALL_BIG_EXT:
		return d.decodeSmallBig()
	case LARGE_BIG_EXT:
		return d.decodeLargeBig()
	case COMPRESSED:
		return d.decodeCompressed()
	default:
		return nil, fmt.Errorf("unsupported tag: %d", tag)
	}
}

func (d *Decoder) decodeCompressed() (any, error) {
	uncompressedSize, err := d.read32()
	if err != nil {
		return nil, err
	}

	outBuffer := make([]byte, uncompressedSize)

	reader, err := zlib.NewReader(bytes.NewReader(d.data[d.offset:]))
	if err != nil {
		return nil, fmt.Errorf("failed to create zlib reader: %w", err)
	}
	defer reader.Close()

	_, err = io.ReadFull(reader, outBuffer)
	if err != nil {
		return nil, fmt.Errorf("failed to decompress data: %w", err)
	}

	d.offset += len(outBuffer)

	value, err := d.unpack(outBuffer)
	if err != nil {
		return nil, err
	}

	return value, nil
}
