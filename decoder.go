package erlpack

import (
	"encoding/binary"
	"errors"
	"fmt"
	"math"
	"strconv"
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
	NEW_FLOAT_EXT     = 70

	FORMAT_VERSION = 131
)

type Decoder struct {
	data   []byte
	offset int
	buf    []byte
}

func NewDecoder(bufSize int) *Decoder {
	return &Decoder{
		buf: make([]byte, 0, bufSize),
	}
}

func (d *Decoder) resetState() { d.offset = 0; d.data = nil }

func (d *Decoder) read8() (uint8, error) {
	if d.offset+1 > len(d.data) {
		return 0, errors.New("read8 out of bounds")
	}
	v := d.data[d.offset]
	d.offset++
	return v, nil
}

func (d *Decoder) read16() (uint16, error) {
	if d.offset+2 > len(d.data) {
		return 0, errors.New("read16 out of bounds")
	}
	v := binary.BigEndian.Uint16(d.data[d.offset:])
	d.offset += 2
	return v, nil
}

func (d *Decoder) read32() (uint32, error) {
	if d.offset+4 > len(d.data) {
		return 0, errors.New("read32 out of bounds")
	}
	v := binary.BigEndian.Uint32(d.data[d.offset:])
	d.offset += 4
	return v, nil
}

func (d *Decoder) read64() (uint64, error) {
	if d.offset+8 > len(d.data) {
		return 0, errors.New("read64 out of bounds")
	}
	v := binary.BigEndian.Uint64(d.data[d.offset:])
	d.offset += 8
	return v, nil
}

func (d *Decoder) readBytes(n uint32) ([]byte, error) {
	if d.offset+int(n) > len(d.data) {
		return nil, errors.New("readBytes out of bounds")
	}
	b := d.data[d.offset : d.offset+int(n)]
	d.offset += int(n)
	return b, nil
}

func (d *Decoder) writeQuotedASCII(s []byte) {
	d.buf = append(d.buf, '"')
	for _, c := range s {
		if c != '\\' && c != '"' {
			d.buf = append(d.buf, c)
		} else {
			d.buf = append(d.buf, '\\', c)
		}
	}
	d.buf = append(d.buf, '"')
}

func (d *Decoder) writeAtom(b []byte) {
	switch len(b) {
	case 3:
		if b[0] == 'n' && b[1] == 'i' && b[2] == 'l' {
			d.buf = append(d.buf, 'n', 'u', 'l', 'l')
			return
		}
	case 4:
		if b[0] == 't' && b[1] == 'r' && b[2] == 'u' && b[3] == 'e' {
			d.buf = append(d.buf, 't', 'r', 'u', 'e')
			return
		}
		if b[0] == 'n' && b[1] == 'u' && b[2] == 'l' && b[3] == 'l' {
			d.buf = append(d.buf, 'n', 'u', 'l', 'l')
			return
		}
	case 5:
		if b[0] == 'f' && b[1] == 'a' && b[2] == 'l' && b[3] == 's' && b[4] == 'e' {
			d.buf = append(d.buf, 'f', 'a', 'l', 's', 'e')
			return
		}
	}
	d.writeQuotedASCII(b)
}

func (d *Decoder) decodeSmallInteger() error {
	v, err := d.read8()
	if err != nil {
		return err
	}
	d.buf = strconv.AppendUint(d.buf, uint64(v), 10)
	return nil
}

func (d *Decoder) decodeInteger() error {
	v, err := d.read32()
	if err != nil {
		return err
	}
	d.buf = strconv.AppendUint(d.buf, uint64(v), 10)
	return nil
}

func (d *Decoder) decodeNewFloat() error {
	v, err := d.read64()
	if err != nil {
		return err
	}
	d.buf = strconv.AppendFloat(d.buf, math.Float64frombits(v), 'f', -1, 64)
	return nil
}

func (d *Decoder) decodeAtom() error {
	l, err := d.read16()
	if err != nil {
		return err
	}
	b, err := d.readBytes(uint32(l))
	if err != nil {
		return err
	}
	d.writeAtom(b)
	return nil
}

func (d *Decoder) decodeSmallAtom() error {
	l, err := d.read8()
	if err != nil {
		return err
	}
	b, err := d.readBytes(uint32(l))
	if err != nil {
		return err
	}
	d.writeAtom(b)
	return nil
}

func (d *Decoder) decodeString() error {
	l, err := d.read16()
	if err != nil {
		return err
	}
	b, err := d.readBytes(uint32(l))
	if err != nil {
		return err
	}
	d.writeQuotedASCII(b)
	return nil
}

func (d *Decoder) decodeBinary() error {
	l, err := d.read32()
	if err != nil {
		return err
	}
	b, err := d.readBytes(l)
	if err != nil {
		return err
	}
	d.writeQuotedASCII(b)
	return nil
}

func (d *Decoder) decodeArray(n uint32) error {
	d.buf = append(d.buf, '[')
	for i := uint32(0); i < n; i++ {
		if i > 0 {
			d.buf = append(d.buf, ',')
		}
		if err := d.decode(); err != nil {
			return err
		}
	}
	d.buf = append(d.buf, ']')
	return nil
}

func (d *Decoder) decodeNil() error {
	d.buf = append(d.buf, '[', ']')
	return nil
}

func (d *Decoder) decodeList() error {
	l, err := d.read32()
	if err != nil {
		return err
	}
	if err := d.decodeArray(l); err != nil {
		return err
	}
	tail, err := d.read8()
	if err != nil || tail != NIL_EXT {
		return errors.New("list tail missing")
	}
	return nil
}

func (d *Decoder) decodeMap() error {
	l, err := d.read32()
	if err != nil {
		return err
	}
	d.buf = append(d.buf, '{')
	for i := range l {
		if i > 0 {
			d.buf = append(d.buf, ',')
		}
		key, err := d.decodeKey()
		if err != nil {
			return err
		}
		d.writeQuotedASCII(key)
		d.buf = append(d.buf, ':')
		if err := d.decode(); err != nil {
			return err
		}
	}
	d.buf = append(d.buf, '}')
	return nil
}

func (d *Decoder) decodeKey() ([]byte, error) {
	tag, err := d.read8()
	if err != nil {
		return nil, err
	}

	switch tag {
	case ATOM_EXT:
		l, _ := d.read16()
		return d.readBytes(uint32(l))
	case SMALL_ATOM_EXT:
		l, _ := d.read8()
		return d.readBytes(uint32(l))
	case BINARY_EXT:
		l, _ := d.read32()
		return d.readBytes(l)
	case STRING_EXT:
		l, _ := d.read16()
		return d.readBytes(uint32(l))
	case SMALL_INTEGER_EXT:
		_, err := d.read8()
		if err != nil {
			return nil, err
		}

		return d.data[d.offset-1 : d.offset], nil
	default:
		return nil, errors.New("unsupported key tag: " + strconv.Itoa(int(tag)))
	}
}

func (d *Decoder) decodeSmallBig() error {
	bytes, err := d.read8()
	if err != nil {
		return err
	}
	return d.decodeBig(uint32(bytes))
}

func (d *Decoder) decodeLargeBig() error {
	bytes, err := d.read32()
	if err != nil {
		return err
	}
	return d.decodeBig(bytes)
}

func (d *Decoder) decode() error {
	tag, err := d.read8()
	if err != nil {
		return err
	}
	switch tag {
	case SMALL_INTEGER_EXT:
		return d.decodeSmallInteger()
	case INTEGER_EXT:
		return d.decodeInteger()
	case NEW_FLOAT_EXT:
		return d.decodeNewFloat()
	case ATOM_EXT:
		return d.decodeAtom()
	case SMALL_ATOM_EXT:
		return d.decodeSmallAtom()
	case STRING_EXT:
		return d.decodeString()
	case LIST_EXT:
		return d.decodeList()
	case MAP_EXT:
		return d.decodeMap()
	case NIL_EXT:
		return d.decodeNil()
	case BINARY_EXT:
		return d.decodeBinary()
	case SMALL_BIG_EXT:
		return d.decodeSmallBig()
	case LARGE_BIG_EXT:
		return d.decodeLargeBig()
	default:
		return errors.New("unsupported tag: " + strconv.Itoa(int(tag)))
	}
}

func (d *Decoder) decodeBig(digits uint32) error {
	sign, err := d.read8()
	if err != nil {
		return err
	}

	if digits > 8 {
		return fmt.Errorf("unable to decode big ints larger than 8 bytes")
	}

	var value uint64
	var b uint64 = 1
	for range digits {
		digit, err := d.read8()
		if err != nil {
			return err
		}
		value += uint64(digit) * b
		b <<= 8
	}

	if digits <= 4 {
		if sign == 0 {
			d.buf = strconv.AppendUint(d.buf, value, 10)
			return nil
		}
		d.buf = strconv.AppendInt(d.buf, -int64(value), 10)
		return nil
	}

	d.buf = append(d.buf, '"')

	if sign != 0 {
		d.buf = append(d.buf, '-')
	}

	d.buf = strconv.AppendUint(d.buf, value, 10)
	d.buf = append(d.buf, '"')

	return nil
}

func (d *Decoder) unpack(data []byte) ([]byte, error) {
	if len(data) == 0 || data[0] != FORMAT_VERSION {
		return nil, errors.New("invalid format")
	}
	d.data = data[1:]
	d.offset = 0
	d.buf = d.buf[:0]
	if err := d.decode(); err != nil {
		d.resetState()
		return nil, err
	}
	return d.buf, nil
}
