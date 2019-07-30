// Copyright 2018 The ChuBao Authors.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or
// implied. See the License for the specific language governing
// permissions and limitations under the License.
package encoding


import (
	"bytes"
	"encoding/binary"
	"fmt"
	"math"
	"time"

	"github.com/pkg/errors"
)

const (
	encodedNull = 0x00
	// A marker greater than NULL but lower than any other value.
	// This value is not actually ever present in a stored key, but
	// it's used in keys used as span boundaries for index scans.
	encodedNotNull = 0x01

	floatNaN     = encodedNotNull + 1
	floatNeg     = floatNaN + 1
	floatZero    = floatNeg + 1
	floatPos     = floatZero + 1
	floatNaNDesc = floatPos + 1 // NaN encoded descendingly

	// The gap between floatNaNDesc and bytesMarker was left for
	// compatibility reasons.
	bytesMarker          byte = 0x12
	bytesDescMarker      byte = bytesMarker + 1
	timeMarker           byte = bytesDescMarker + 1
	durationBigNegMarker byte = timeMarker + 1 // Only used for durations < MinInt64 nanos.
	durationMarker       byte = durationBigNegMarker + 1
	durationBigPosMarker byte = durationMarker + 1 // Only used for durations > MaxInt64 nanos.

	decimalNaN              = durationBigPosMarker + 1 // 24
	decimalNegativeInfinity = decimalNaN + 1
	decimalNegLarge         = decimalNegativeInfinity + 1
	decimalNegMedium        = decimalNegLarge + 11
	decimalNegSmall         = decimalNegMedium + 1
	decimalZero             = decimalNegSmall + 1
	decimalPosSmall         = decimalZero + 1
	decimalPosMedium        = decimalPosSmall + 1
	decimalPosLarge         = decimalPosMedium + 11
	decimalInfinity         = decimalPosLarge + 1
	decimalNaNDesc          = decimalInfinity + 1 // NaN encoded descendingly
	decimalTerminator       = 0x00

	jsonInvertedIndex = decimalNaNDesc + 1
	jsonEmptyArray    = jsonInvertedIndex + 1
	jsonEmptyObject   = jsonEmptyArray + 1

	// IntMin is chosen such that the range of int tags does not overlap the
	// ascii character set that is frequently used in testing.
	IntMin      = 0x80 // 128
	intMaxWidth = 8
	intZero     = IntMin + intMaxWidth           // 136
	intSmall    = IntMax - intZero - intMaxWidth // 109
	// IntMax is the maximum int tag value.
	IntMax = 0xfd // 253
)

// Direction for ordering results.
type Direction int

// Direction values.
const (
	_ Direction = iota
	Ascending
	Descending
)

// Reverse returns the opposite direction.
func (d Direction) Reverse() Direction {
	switch d {
	case Ascending:
		return Descending
	case Descending:
		return Ascending
	default:
		panic(fmt.Sprintf("Invalid direction %d", d))
	}
}

// EncodeUint8Ascending encodes the byte value using 1 byte
// representation. The bytes are appended to the supplied buffer and
// the final buffer is returned.
func EncodeByteAscending(b []byte, v byte) []byte {
	return append(b, v)
}

// DecodeUint8Ascending decodes a byte from the input buffer, treating
// the input as 1 byte byte representation. The remainder
// of the input buffer and the decoded byte are returned.
func DecodeByteAscending(b []byte) ([]byte, byte, error) {
	if len(b) < 1 {
		return nil, 0, errors.Errorf("insufficient bytes to decode uint8 int value")
	}
	v := uint8(b[0])
	return b[1:], v, nil
}


// EncodeUint8Ascending encodes the uint8 value using 1 byte
// representation. The bytes are appended to the supplied buffer and
// the final buffer is returned.
func EncodeUint8Ascending(b []byte, v uint8) []byte {
	return append(b, byte(v))
}

// DecodeUint8Ascending decodes a uint8 from the input buffer, treating
// the input 1 byte uint8 representation. The remainder
// of the input buffer and the decoded uint8 are returned.
func DecodeUint8Ascending(b []byte) ([]byte, uint8, error) {
	if len(b) < 1 {
		return nil, 0, errors.Errorf("insufficient bytes to decode uint8 int value")
	}
	v := uint8(b[0])
	return b[1:], v, nil
}

// EncodeUint16Ascending encodes the uint32 value using a big-endian 2 byte
// representation. The bytes are appended to the supplied buffer and
// the final buffer is returned.
func EncodeUint16Ascending(b []byte, v uint16) []byte {
	return append(b, byte(v>>8), byte(v))
}

// DecodeUint16Ascending decodes a uint32 from the input buffer, treating
// the input as a big-endian 4 byte uint32 representation. The remainder
// of the input buffer and the decoded uint32 are returned.
func DecodeUint16Ascending(b []byte) ([]byte, uint16, error) {
	if len(b) < 2 {
		return nil, 0, errors.Errorf("insufficient bytes to decode uint16 int value")
	}
	v := binary.BigEndian.Uint16(b)
	return b[2:], v, nil
}

// EncodeUint32Ascending encodes the uint32 value using a big-endian 4 byte
// representation. The bytes are appended to the supplied buffer and
// the final buffer is returned.
func EncodeUint32Ascending(b []byte, v uint32) []byte {
	return append(b, byte(v>>24), byte(v>>16), byte(v>>8), byte(v))
}

// DecodeUint32Ascending decodes a uint32 from the input buffer, treating
// the input as a big-endian 4 byte uint32 representation. The remainder
// of the input buffer and the decoded uint32 are returned.
func DecodeUint32Ascending(b []byte) ([]byte, uint32, error) {
	if len(b) < 4 {
		return nil, 0, errors.Errorf("insufficient bytes to decode uint32 int value")
	}
	v := binary.BigEndian.Uint32(b)
	return b[4:], v, nil
}

// EncodeUint64Ascending encodes the uint64 value using a big-endian 8 byte
// representation. The bytes are appended to the supplied buffer and
// the final buffer is returned.
func EncodeUint64Ascending(b []byte, v uint64) []byte {
	return append(b,
		byte(v>>56), byte(v>>48), byte(v>>40), byte(v>>32),
		byte(v>>24), byte(v>>16), byte(v>>8), byte(v))
}

// DecodeUint64Ascending decodes a uint64 from the input buffer, treating
// the input as a big-endian 8 byte uint64 representation. The remainder
// of the input buffer and the decoded uint64 are returned.
func DecodeUint64Ascending(b []byte) ([]byte, uint64, error) {
	if len(b) < 8 {
		return nil, 0, errors.Errorf("insufficient bytes to decode uint64 int value")
	}
	v := binary.BigEndian.Uint64(b)
	return b[8:], v, nil
}

const (
	// <term>     -> \x00\x01
	// \x00       -> \x00\xff
	escape                   byte = 0x00
	escapedTerm              byte = 0x01
	escaped00                byte = 0xff
	escapedFF                byte = 0x00
)

type escapes struct {
	escape      byte
	escapedTerm byte
	escaped00   byte
	escapedFF   byte
	marker      byte
}

var (
	ascendingEscapes  = escapes{escape, escapedTerm, escaped00, escapedFF, bytesMarker}
)

// EncodeBytesAscending encodes the []byte value using an escape-based
// encoding. The encoded value is terminated with the sequence
// "\x00\x01" which is guaranteed to not occur elsewhere in the
// encoded value. The encoded bytes are append to the supplied buffer
// and the resulting buffer is returned.
func EncodeBytesAscending(b []byte, data []byte) []byte {
	return encodeBytesAscendingWithTerminatorAndPrefix(b, data, ascendingEscapes.escapedTerm, bytesMarker)
}

func EncodePrefixBytesAscending(b []byte, data []byte) []byte {
	return encodeBytesAscendingWithPrefixWithoutTerminator(b, data, bytesMarker)
}

// encodeBytesAscendingWithTerminatorAndPrefix encodes the []byte value using an escape-based
// encoding. The encoded value is terminated with the sequence
// "\x00\terminator". The encoded bytes are append to the supplied buffer
// and the resulting buffer is returned. The terminator allows us to pass
// different terminators for things such as JSON key encoding.
func encodeBytesAscendingWithTerminatorAndPrefix(
	b []byte, data []byte, terminator byte, prefix byte,
) []byte {
	b = append(b, prefix)
	return encodeBytesAscendingWithTerminator(b, data, terminator)
}

func encodeBytesAscendingWithPrefixWithoutTerminator(
	b []byte, data []byte, prefix byte,
) []byte {
	b = append(b, prefix)
	return encodeBytesAscendingWithoutTerminatorOrPrefix(b, data)
}

// encodeBytesAscendingWithTerminator encodes the []byte value using an escape-based
// encoding. The encoded value is terminated with the sequence
// "\x00\terminator". The encoded bytes are append to the supplied buffer
// and the resulting buffer is returned. The terminator allows us to pass
// different terminators for things such as JSON key encoding.
func encodeBytesAscendingWithTerminator(b []byte, data []byte, terminator byte) []byte {
	bs := encodeBytesAscendingWithoutTerminatorOrPrefix(b, data)
	return append(bs, escape, terminator)
}

// encodeBytesAscendingWithoutTerminatorOrPrefix encodes the []byte value using an escape-based
// encoding.
func encodeBytesAscendingWithoutTerminatorOrPrefix(b []byte, data []byte) []byte {
	for {
		// IndexByte is implemented by the go runtime in assembly and is
		// much faster than looping over the bytes in the slice.
		i := bytes.IndexByte(data, escape)
		if i == -1 {
			break
		}
		b = append(b, data[:i]...)
		b = append(b, escape, escaped00)
		data = data[i+1:]
	}
	return append(b, data...)
}


// DecodeBytesAscending decodes a []byte value from the input buffer
// which was encoded using EncodeBytesAscending. The decoded bytes
// are appended to r. The remainder of the input buffer and the
// decoded []byte are returned.
func DecodeBytesAscending(b []byte, r []byte) ([]byte, []byte, error) {
	return decodeBytesInternal(b, r, ascendingEscapes, true)
}

func decodeBytesInternal(b []byte, r []byte, e escapes, expectMarker bool) ([]byte, []byte, error) {
	if expectMarker {
		if len(b) == 0 || b[0] != e.marker {
			return nil, nil, errors.Errorf("did not find marker %#x in buffer %#x", e.marker, b)
		}
		b = b[1:]
	}

	for {
		i := bytes.IndexByte(b, e.escape)
		if i == -1 {
			return nil, nil, errors.Errorf("did not find terminator %#x in buffer %#x", e.escape, b)
		}
		if i+1 >= len(b) {
			return nil, nil, errors.Errorf("malformed escape in buffer %#x", b)
		}
		v := b[i+1]
		if v == e.escapedTerm {
			if r == nil {
				r = b[:i]
			} else {
				r = append(r, b[:i]...)
			}
			return b[i+2:], r, nil
		}

		if v != e.escaped00 {
			return nil, nil, errors.Errorf("unknown escape sequence: %#x %#x", e.escape, v)
		}

		r = append(r, b[:i]...)
		r = append(r, e.escapedFF)
		b = b[i+2:]
	}
}

// Type represents the type of a value encoded by
// Encode{Null,NotNull,Varint,Uvarint,Float,Bytes}.
//go:generate stringer -type=Type
type Type int

// Type values.
// TODO(dan, arjun): Make this into a proto enum.
// The 'Type' annotations are necessary for producing stringer-generated values.
const (
	Unknown   Type = 0
	Null      Type = 1
	NotNull   Type = 2
	Int       Type = 3
	Float     Type = 4
	Decimal   Type = 5
	Bytes     Type = 6
	BytesDesc Type = 7 // Bytes encoded descendingly
	Time      Type = 8
	Duration  Type = 9
	True      Type = 10
	False     Type = 11
	UUID      Type = 12
	Array     Type = 13
	IPAddr    Type = 14
	// SentinelType is used for bit manipulation to check if the encoded type
	// value requires more than 4 bits, and thus will be encoded in two bytes. It
	// is not used as a type value, and thus intentionally overlaps with the
	// subsequent type value. The 'Type' annotation is intentionally omitted here.
	SentinelType      = 15
	JSON         Type = 15
	Tuple        Type = 16
)

// NoColumnID is a sentinel for the EncodeFooValue methods representing an
// invalid column id.
const NoColumnID uint32 = 0

// EncodeUntaggedBytesValue encodes a byte array value, appends it to the supplied
// buffer, and returns the final buffer.
func EncodeUntaggedBytesValue(appendTo []byte, data []byte) []byte {
	appendTo = EncodeNonsortingUvarint(appendTo, uint64(len(data)))
	return append(appendTo, data...)
}

// DecodeUntaggedBytesValue decodes a value encoded by EncodeUntaggedBytesValue.
func DecodeUntaggedBytesValue(b []byte) (remaining, data []byte, err error) {
	var i uint64
	b, _, i, err = DecodeNonsortingUvarint(b)
	if err != nil {
		return b, nil, err
	}
	return b[int(i):], b[:int(i)], nil
}


// EncodeNonsortingUvarint encodes a uint64, appends it to the supplied buffer,
// and returns the final buffer. The encoding used is similar to
// encoding/binary, but with the most significant bits first:
// - Unsigned integers are serialized 7 bits at a time, starting with the
//   most significant bits.
// - The most significant bit (msb) in each output byte indicates if there
//   is a continuation byte (msb = 1).
func EncodeNonsortingUvarint(appendTo []byte, x uint64) []byte {
	switch {
	case x < (1 << 7):
		return append(appendTo, byte(x))
	case x < (1 << 14):
		return append(appendTo, 0x80|byte(x>>7), 0x7f&byte(x))
	case x < (1 << 21):
		return append(appendTo, 0x80|byte(x>>14), 0x80|byte(x>>7), 0x7f&byte(x))
	case x < (1 << 28):
		return append(appendTo, 0x80|byte(x>>21), 0x80|byte(x>>14), 0x80|byte(x>>7), 0x7f&byte(x))
	case x < (1 << 35):
		return append(appendTo, 0x80|byte(x>>28), 0x80|byte(x>>21), 0x80|byte(x>>14), 0x80|byte(x>>7), 0x7f&byte(x))
	case x < (1 << 42):
		return append(appendTo, 0x80|byte(x>>35), 0x80|byte(x>>28), 0x80|byte(x>>21), 0x80|byte(x>>14), 0x80|byte(x>>7), 0x7f&byte(x))
	case x < (1 << 49):
		return append(appendTo, 0x80|byte(x>>42), 0x80|byte(x>>35), 0x80|byte(x>>28), 0x80|byte(x>>21), 0x80|byte(x>>14), 0x80|byte(x>>7), 0x7f&byte(x))
	case x < (1 << 56):
		return append(appendTo, 0x80|byte(x>>49), 0x80|byte(x>>42), 0x80|byte(x>>35), 0x80|byte(x>>28), 0x80|byte(x>>21), 0x80|byte(x>>14), 0x80|byte(x>>7), 0x7f&byte(x))
	case x < (1 << 63):
		return append(appendTo, 0x80|byte(x>>56), 0x80|byte(x>>49), 0x80|byte(x>>42), 0x80|byte(x>>35), 0x80|byte(x>>28), 0x80|byte(x>>21), 0x80|byte(x>>14), 0x80|byte(x>>7), 0x7f&byte(x))
	default:
		return append(appendTo, 0x80|byte(x>>63), 0x80|byte(x>>56), 0x80|byte(x>>49), 0x80|byte(x>>42), 0x80|byte(x>>35), 0x80|byte(x>>28), 0x80|byte(x>>21), 0x80|byte(x>>14), 0x80|byte(x>>7), 0x7f&byte(x))
	}
}

// DecodeNonsortingUvarint decodes a value encoded by EncodeNonsortingUvarint. It
// returns the length of the encoded varint and value.
func DecodeNonsortingUvarint(buf []byte) (remaining []byte, length int, value uint64, err error) {
	// TODO(dan): Handle overflow.
	for i, b := range buf {
		value += uint64(b & 0x7f)
		if b < 0x80 {
			return buf[i+1:], i + 1, value, nil
		}
		value <<= 7
	}
	return buf, 0, 0, nil
}


func EncodeUntaggedFloatValue(appendTo []byte, f float64) []byte {
	return EncodeUint64Ascending(appendTo, math.Float64bits(f))
}

// DecodeUntaggedFloatValue decodes a value encoded by EncodeUntaggedFloatValue.
func DecodeUntaggedFloatValue(b []byte) (remaining []byte, f float64, err error) {
	if len(b) < 8 {
		return b, 0, fmt.Errorf("float64 value should be exactly 8 bytes: %d", len(b))
	}
	var i uint64
	b, i, err = DecodeUint64Ascending(b)
	return b, math.Float64frombits(i), err
}


// EncodeUntaggedTimeValue encodes a time.Time value, appends it to the supplied buffer,
// and returns the final buffer.
func EncodeUntaggedTimeValue(appendTo []byte, t time.Time) []byte {
	appendTo = EncodeNonsortingStdlibVarint(appendTo, t.Unix())
	return EncodeNonsortingStdlibVarint(appendTo, int64(t.Nanosecond()))
}

// NonsortingVarintMaxLen is the maximum length of an EncodeNonsortingVarint
// encoded value.
const NonsortingVarintMaxLen = binary.MaxVarintLen64

// EncodeNonsortingStdlibVarint encodes an int value using encoding/binary, appends it
// to the supplied buffer, and returns the final buffer.
func EncodeNonsortingStdlibVarint(appendTo []byte, x int64) []byte {
	// Fixed size array to allocate this on the stack.
	var scratch [binary.MaxVarintLen64]byte
	i := binary.PutVarint(scratch[:binary.MaxVarintLen64], x)
	return append(appendTo, scratch[:i]...)
}

// DecodeUntaggedTimeValue decodes a value encoded by EncodeUntaggedTimeValue.
func DecodeUntaggedTimeValue(b []byte) (remaining []byte, t time.Time, err error) {
	var sec, nsec int64
	b, _, sec, err = DecodeNonsortingStdlibVarint(b)
	if err != nil {
		return b, time.Time{}, err
	}
	b, _, nsec, err = DecodeNonsortingStdlibVarint(b)
	if err != nil {
		return b, time.Time{}, err
	}
	return b, time.Unix(sec, nsec).UTC(), nil
}

// DecodeNonsortingStdlibVarint decodes a value encoded by EncodeNonsortingVarint. It
// returns the length of the encoded varint and value.
func DecodeNonsortingStdlibVarint(b []byte) (remaining []byte, length int, value int64, err error) {
	value, length = binary.Varint(b)
	if length <= 0 {
		return nil, 0, 0, fmt.Errorf("int64 varint decoding failed: %d", length)
	}
	return b[length:], length, value, nil
}