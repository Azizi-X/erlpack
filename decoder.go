package erlpack

import (
	"encoding/binary"
	"errors"
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

var (
	hexMap = [16]byte{'0', '1', '2', '3', '4', '5', '6', '7', '8', '9', 'a', 'b', 'c', 'd', 'e', 'f'}

	errInvalidFormat      = errors.New("invalid format")
	errListTailMissing    = errors.New("list tail missing")
	errUnsupportedTag     = errors.New("unsupported tag")
	errUnsupportedKeyTag  = errors.New("unsupported key tag")
	errTooBig             = errors.New("unable to decode big ints larger than 8 bytes")
	errRead8OutOfBound    = errors.New("read8 out of bounds")
	errRead16OutOfBound   = errors.New("read16 out of bounds")
	errRead32OutOfBound   = errors.New("read32 out of bounds")
	errRead64OutOfBound   = errors.New("read64 out of bounds")
	errReadByteOutOfBound = errors.New("ready byte out of bounds")

	MaxCap = 32 * 1024
)

type Decoder struct {
	data    []byte
	offset  int
	buf     []byte
	tempBuf []byte
}

func NewDecoder() *Decoder {
	return &Decoder{
		tempBuf: make([]byte, 0, 32),
		buf:     make([]byte, 0, MaxCap),
	}
}

func (d *Decoder) read8() (uint8, error) {
	if d.offset+1 > len(d.data) {
		return 0, errRead8OutOfBound
	}
	v := d.data[d.offset]
	d.offset++
	return v, nil
}

func (d *Decoder) read16() (uint16, error) {
	if d.offset+2 > len(d.data) {
		return 0, errRead16OutOfBound
	}
	v := binary.BigEndian.Uint16(d.data[d.offset:])
	d.offset += 2
	return v, nil
}

func (d *Decoder) read32() (uint32, error) {
	if d.offset+4 > len(d.data) {
		return 0, errRead32OutOfBound
	}
	v := binary.BigEndian.Uint32(d.data[d.offset:])
	d.offset += 4
	return v, nil
}

func (d *Decoder) read64() (uint64, error) {
	if d.offset+8 > len(d.data) {
		return 0, errRead64OutOfBound
	}
	v := binary.BigEndian.Uint64(d.data[d.offset:])
	d.offset += 8
	return v, nil
}

func (d *Decoder) readBytes(n uint32) ([]byte, error) {
	if d.offset+int(n) > len(d.data) {
		return nil, errReadByteOutOfBound
	}
	b := d.data[d.offset : d.offset+int(n)]
	d.offset += int(n)
	return b, nil
}

func (d *Decoder) writeJsonASCII(s []byte) {
	d.buf = append(d.buf, '"')
	for _, c := range s {
		switch c {
		case '\\', '"':
			d.buf = append(d.buf, '\\', c)
		case '\b':
			d.buf = append(d.buf, '\\', 'b')
		case '\f':
			d.buf = append(d.buf, '\\', 'f')
		case '\n':
			d.buf = append(d.buf, '\\', 'n')
		case '\r':
			d.buf = append(d.buf, '\\', 'r')
		case '\t':
			d.buf = append(d.buf, '\\', 't')
		default:
			if c < 0x20 {
				d.buf = append(d.buf, '\\', 'u', '0', '0',
					hexMap[c>>4], hexMap[c&0xF])
			} else {
				d.buf = append(d.buf, c)
			}
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
	d.writeJsonASCII(b)
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
	d.writeJsonASCII(b)
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
	d.writeJsonASCII(b)
	return nil
}

func (d *Decoder) decodeArray(n uint32) error {
	d.buf = append(d.buf, '[')
	for i := range n {
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
		return errListTailMissing
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
		d.writeJsonASCII(key)
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

		b := d.data[d.offset-1]
		d.tempBuf = d.tempBuf[:0]
		d.tempBuf = strconv.AppendUint(d.tempBuf, uint64(b), 10)

		return d.tempBuf, nil
	case SMALL_BIG_EXT:
		digits, err := d.read8()
		if err != nil {
			return nil, err
		}

		number, err := d.decodeBigRaw(uint32(digits))
		if err != nil {
			return nil, err
		}

		d.tempBuf = d.tempBuf[:0]
		d.tempBuf = strconv.AppendInt(d.tempBuf, number, 10)

		return d.tempBuf, nil
	default:
		return nil, errUnsupportedKeyTag
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
		return errUnsupportedTag
	}
}

func (d *Decoder) decodeBigRaw(digits uint32) (int64, error) {
	sign, err := d.read8()
	if err != nil {
		return 0, err
	}

	if digits > 8 {
		return 0, errTooBig
	}

	var value int64
	var b uint64 = 1
	for range digits {
		digit, err := d.read8()
		if err != nil {
			return 0, err
		}
		value += int64(uint64(digit) * b)
		b <<= 8
	}

	if digits <= 4 {
		if sign == 0 {
			return value, nil
		}
		return -value, nil
	}

	if sign != 0 {
		return -value, nil
	}

	return value, nil
}

func (d *Decoder) decodeBig(digits uint32) error {
	value, err := d.decodeBigRaw(digits)
	if err != nil {
		return err
	}

	if digits <= 4 {
		d.buf = strconv.AppendInt(d.buf, value, 10)
		return nil
	}

	d.buf = append(d.buf, '"')
	d.buf = strconv.AppendInt(d.buf, value, 10)
	d.buf = append(d.buf, '"')

	return nil
}

func (d *Decoder) Unpack(data []byte) ([]byte, error) {
	if len(data) == 0 || data[0] != FORMAT_VERSION {
		return nil, errInvalidFormat
	}

	d.offset = 0
	d.data = data[1:]

	if cap(d.buf) > MaxCap {
		d.buf = nil
		d.buf = make([]byte, 0, MaxCap)
	} else {
		d.buf = d.buf[:0]
	}

	if cap(d.buf) < len(d.data) {
		d.buf = make([]byte, 0, len(d.data)*2)
	}

	if err := d.decode(); err != nil {
		return nil, err
	}

	return d.buf, nil
}
