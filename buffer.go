package gokiwi

import (
	"errors"
	"math"
)

var ErrIndexOutOfBounds = errors.New("index out of bounds")

func NewBuffer(data []byte) *Buffer {
	return &Buffer{data: data, length: len(data)}
}

type Buffer struct {
	data   []byte
	idx    int
	length int
}

func (b *Buffer) ReadByte() (byte, error) {
	if b.idx+1 > len(b.data) {
		return 0, ErrIndexOutOfBounds
	}
	val := b.data[b.idx]
	b.idx++
	return val, nil
}

func (b *Buffer) ReadByteArray() ([]byte, error) {
	val, err := b.ReadVarUint()
	if err != nil {
		return nil, err
	}
	size := int(val)
	start := b.idx
	end := start + size
	if end > len(b.data) {
		return nil, ErrIndexOutOfBounds
	}
	b.idx = end
	return b.data[start:end], nil
}

func (b *Buffer) ReadVarUint() (uint, error) {
	value := 0
	shift := 0
	for {
		bVal, err := b.ReadByte()
		if err != nil {
			return 0, err
		}
		value |= (int(bVal) & 127) << shift
		shift += 7
		if bVal&128 == 0 || shift > 35 {
			break
		}
	}

	return uint(value), nil
}

func (b *Buffer) ReadVarFloat() (float64, error) {
	idx := b.idx
	data := b.data
	size := len(data)

	if idx+1 > size {
		return 0, ErrIndexOutOfBounds
	}
	first := data[idx]
	if first == 0 {
		b.idx++
		return 0, nil
	}

	// Endian-independent 32-bit read
	if idx+4 > size {
		return 0, ErrIndexOutOfBounds
	}

	bits := uint32(first) | (uint32(data[idx+1]) << 8) | (uint32(data[idx+2]) << 16) | (uint32(data[idx+3]) << 24)
	b.idx += 4

	bits = (bits << 23) | (bits >> 9)
	return float64(math.Float32frombits(bits)), nil
}

func (b *Buffer) ReadVarInt() (int, error) {
	value, err := b.ReadVarUint()
	if err != nil {
		return 0, err
	}
	value |= 0
	if value&1 == 0 {
		return int(value) >> 1, nil
	}
	return int(^(value >> 1)), nil
}

func (b *Buffer) ReadVarUint64() (uint64, error) {
	var value uint64 = 0
	var shift uint64 = 0
	var seven uint64 = 7
	var byteVal uint64

	for {
		theByte, err := b.ReadByte()
		if err != nil {
			return 0, err
		}
		if int(theByte)&128 != 0 && shift < 56 {
			value |= (uint64(theByte) & 127) << shift
			shift += seven
		} else {
			break
		}
	}

	value |= byteVal << shift
	return value, nil
}

func (b *Buffer) ReadVarInt64() (int64, error) {
	val, err := b.ReadVarUint64()
	if err != nil {
		return 0, err
	}
	value := int64(val)
	var one int64 = 1
	var sign = (value & one) != 0
	value >>= one
	if sign {
		return ^value, nil
	}

	return value, nil
}

func (b *Buffer) ReadString() (string, error) {
	var result []rune

	for {
		var cp rune

		ac, err := b.ReadByte()
		if err != nil {
			return "", err
		}
		if ac < 0xC0 {
			cp = rune(ac)
		} else {
			bc, err := b.ReadByte()
			if err != nil {
				return "", err
			}
			if ac < 0xE0 {
				cp = ((rune(ac) & 0x1F) << 6) | (rune(bc) & 0x3F)
			} else {
				cc, err := b.ReadByte()
				if err != nil {
					return "", err
				}
				if ac < 0xF0 {
					cp = ((rune(ac) & 0x0F) << 12) | ((rune(bc) & 0x3F) << 6) | (rune(cc) & 0x3F)
				} else {
					dc, err := b.ReadByte()
					if err != nil {
						return "", err
					}
					cp = ((rune(ac) & 0x07) << 18) | ((rune(bc) & 0x3F) << 12) | ((rune(cc) & 0x3F) << 6) | rune(dc&0x3F)
				}
			}
		}

		if cp == 0 {
			break
		}

		if cp < 0x1000 {
			result = append(result, cp)
		} else {
			cp -= 0x1000
			result = append(result, (cp>>10)+0xD800, (cp&((1<<10)-1))+0xDC00)
		}
	}

	return string(result), nil
}
