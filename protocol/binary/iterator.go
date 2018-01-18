package binary

import (
	"fmt"
	"github.com/thrift-iterator/go/protocol"
	"math"
	"io"
)

type Iterator struct {
	buf []byte
	err error
}

func NewIterator(buf []byte) *Iterator {
	return &Iterator{buf: buf}
}

func (iter *Iterator) Error() error {
	return iter.err
}

func (iter *Iterator) ReportError(operation string, err string) {
	if iter.err == nil {
		iter.err = fmt.Errorf("%s: %s", operation, err)
	}
}

func (iter *Iterator) Reset(reader io.Reader, buf []byte) {
	iter.buf = buf
	iter.err = nil
}

func (iter *Iterator) ReadMessageHeader() protocol.MessageHeader {
	versionAndMessageType := iter.ReadInt32()
	messageType := protocol.TMessageType(versionAndMessageType & 0x0ff)
	version := protocol.Version(int64(int64(versionAndMessageType) & 0xffff0000))
	messageName := iter.ReadString()
	seqId := protocol.SeqId(iter.ReadInt32())
	return protocol.MessageHeader{
		Version:     version,
		MessageName: messageName,
		MessageType: messageType,
		SeqId:       seqId,
	}
}

func (iter *Iterator) ReadMessage() protocol.Message {
	header := iter.ReadMessageHeader()
	return protocol.Message{
		MessageHeader: header,
		Arguments:     iter.ReadStruct(),
	}
}

func (iter *Iterator) ReadStructCB(cb func(fieldType protocol.TType, fieldId protocol.FieldId)) {
	for iter.buf[0] != 0 {
		fieldType := iter.buf[0]
		fieldId := uint16(iter.buf[2]) | uint16(iter.buf[1])<<8
		iter.buf = iter.buf[3:]
		cb(protocol.TType(fieldType), protocol.FieldId(fieldId))
	}
	iter.buf = iter.buf[1:]
}

func (iter *Iterator) ReadStructHeader() {
	// noop
}

func (iter *Iterator) ReadStructField() (protocol.TType, protocol.FieldId) {
	fieldType := iter.buf[0]
	if fieldType == 0 {
		iter.buf = iter.buf[1:]
		return protocol.TType(fieldType), 0
	}
	fieldId := uint16(iter.buf[2]) | uint16(iter.buf[1])<<8
	iter.buf = iter.buf[3:]
	return protocol.TType(fieldType), protocol.FieldId(fieldId)
}

func (iter *Iterator) ReadListHeader() (protocol.TType, int) {
	b := iter.buf
	elemType := b[0]
	length := uint32(b[4]) | uint32(b[3])<<8 | uint32(b[2])<<16 | uint32(b[1])<<24
	iter.buf = iter.buf[5:]
	return protocol.TType(elemType), int(length)
}

func (iter *Iterator) ReadMapHeader() (protocol.TType, protocol.TType, int) {
	b := iter.buf
	keyType := b[0]
	elemType := b[1]
	length := uint32(b[5]) | uint32(b[4])<<8 | uint32(b[3])<<16 | uint32(b[2])<<24
	iter.buf = iter.buf[6:]
	return protocol.TType(keyType), protocol.TType(elemType), int(length)
}

func (iter *Iterator) ReadBool() bool {
	return iter.ReadUInt8() == 1
}

func (iter *Iterator) ReadUInt8() uint8 {
	b := iter.buf
	value := b[0]
	iter.buf = iter.buf[1:]
	return value
}

func (iter *Iterator) ReadInt8() int8 {
	return int8(iter.ReadUInt8())
}

func (iter *Iterator) ReadUInt16() uint16 {
	b := iter.buf
	value := uint16(b[1]) | uint16(b[0])<<8
	iter.buf = iter.buf[2:]
	return value
}

func (iter *Iterator) ReadInt16() int16 {
	return int16(iter.ReadUInt16())
}

func (iter *Iterator) ReadUInt32() uint32 {
	b := iter.buf
	value := uint32(b[3]) | uint32(b[2])<<8 | uint32(b[1])<<16 | uint32(b[0])<<24
	iter.buf = iter.buf[4:]
	return value
}

func (iter *Iterator) ReadInt32() int32 {
	return int32(iter.ReadUInt32())
}

func (iter *Iterator) ReadInt64() int64 {
	return int64(iter.ReadUInt64())
}

func (iter *Iterator) ReadUInt64() uint64 {
	b := iter.buf
	value := uint64(b[7]) | uint64(b[6])<<8 | uint64(b[5])<<16 | uint64(b[4])<<24 |
		uint64(b[3])<<32 | uint64(b[2])<<40 | uint64(b[1])<<48 | uint64(b[0])<<56
	iter.buf = iter.buf[8:]
	return value
}

func (iter *Iterator) ReadFloat64() float64 {
	return math.Float64frombits(iter.ReadUInt64())
}

func (iter *Iterator) ReadString() string {
	length := iter.ReadUInt32()
	value := string(iter.buf[:length])
	iter.buf = iter.buf[length:]
	return value
}

func (iter *Iterator) ReadBinary() []byte {
	length := iter.ReadUInt32()
	value := iter.buf[:length]
	iter.buf = iter.buf[length:]
	return value
}

func (iter *Iterator) ReadStruct() map[protocol.FieldId]interface{} {
	obj := map[protocol.FieldId]interface{}{}
	for {
		fieldType, fieldId := iter.ReadStructField()
		if fieldType == protocol.STOP {
			return obj
		}
		obj[fieldId] = iter.Read(fieldType)
	}
}

func (iter *Iterator) ReadList() []interface{} {
	var obj []interface{}
	elemType, length := iter.ReadListHeader()
	elemReader := iter.ReaderOf(elemType)
	for i := 0; i < length; i++ {
		obj = append(obj, elemReader())
	}
	return obj
}

func (iter *Iterator) ReadMap() map[interface{}]interface{} {
	obj := map[interface{}]interface{}{}
	keyType, elemType, length := iter.ReadMapHeader()
	keyReader := iter.ReaderOf(keyType)
	elemReader := iter.ReaderOf(elemType)
	for i := 0; i < length; i++ {
		obj[keyReader()] = elemReader()
	}
	return obj
}

func (iter *Iterator) Read(ttype protocol.TType) interface{} {
	switch ttype {
	case protocol.BOOL:
		return iter.ReadBool()
	case protocol.I08:
		return iter.ReadInt8()
	case protocol.I16:
		return iter.ReadInt16()
	case protocol.I32:
		return iter.ReadInt32()
	case protocol.I64:
		return iter.ReadInt64()
	case protocol.DOUBLE:
		return iter.ReadFloat64()
	case protocol.STRING:
		return iter.ReadString()
	case protocol.LIST:
		return iter.ReadList()
	case protocol.MAP:
		return iter.ReadMap()
	case protocol.STRUCT:
		return iter.ReadStruct()
	default:
		panic("unsupported type")
	}
}

func (iter *Iterator) ReaderOf(ttype protocol.TType) func() interface{} {
	switch ttype {
	case protocol.BOOL:
		return func() interface{} {
			return iter.ReadBool()
		}
	case protocol.I08:
		return func() interface{} {
			return iter.ReadInt8()
		}
	case protocol.I16:
		return func() interface{} {
			return iter.ReadInt16()
		}
	case protocol.I32:
		return func() interface{} {
			return iter.ReadInt32()
		}
	case protocol.I64:
		return func() interface{} {
			return iter.ReadInt64()
		}
	case protocol.DOUBLE:
		return func() interface{} {
			return iter.ReadFloat64()
		}
	case protocol.STRING:
		return func() interface{} {
			return iter.ReadString()
		}
	case protocol.LIST:
		return func() interface{} {
			return iter.ReadList()
		}
	case protocol.MAP:
		return func() interface{} {
			return iter.ReadMap()
		}
	case protocol.STRUCT:
		return func() interface{} {
			return iter.ReadStruct()
		}
	default:
		panic("unsupported type")
	}
}
