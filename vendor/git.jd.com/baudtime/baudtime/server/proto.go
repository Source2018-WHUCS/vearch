package server

import (
	"bufio"
	"encoding/binary"
	"io"
	"net"

	"github.com/gogo/protobuf/proto"
	"github.com/pkg/errors"
)

type MsgType uint8

type MsgTypeHandler interface {
	Type(proto.Message) (MsgType, error)
	Make(MsgType) (proto.Message, error)
}

const BadMsgType MsgType = 255

var BadMsgTypeError = errors.New("bad message type")

type ProtoCodec struct {
	msgWBuf []byte
	msgRBuf []byte
	io.Reader
	io.Writer
	MsgTypeHandler
}

func NewProtoCodec(rw io.ReadWriter, msgTypeHandler MsgTypeHandler) *ProtoCodec {
	return NewProtoCodecSize(rw, msgTypeHandler, 0)
}

func NewProtoCodecSize(rw io.ReadWriter, msgTypeHandler MsgTypeHandler, size int) *ProtoCodec {
	if size <= 0 {
		return &ProtoCodec{
			msgWBuf:        make([]byte, 5+10240),
			msgRBuf:        make([]byte, 5+10240),
			Reader:         rw,
			Writer:         rw,
			MsgTypeHandler: msgTypeHandler,
		}
	} else {
		return &ProtoCodec{
			msgWBuf:        make([]byte, 5+10240),
			msgRBuf:        make([]byte, 5+10240),
			Reader:         bufio.NewReaderSize(rw, size),
			Writer:         bufio.NewWriterSize(rw, size),
			MsgTypeHandler: msgTypeHandler,
		}
	}
}

type ProtoConn struct {
	*ProtoCodec
	net.Conn
}

func NewProtoConn(conn net.Conn, msgTypeHandler MsgTypeHandler) *ProtoConn {
	codec := NewProtoCodec(conn, msgTypeHandler)
	return &ProtoConn{
		ProtoCodec: codec,
		Conn:       conn,
	}
}

func (codec *ProtoCodec) WriteMsg(msg proto.Message) error {
	mType, err := codec.Type(msg)
	if err != nil {
		return err
	}
	codec.msgWBuf[0] = byte(mType)

	b, err := proto.Marshal(msg)
	if err != nil {
		return err
	}

	msgLen := len(b)
	binary.BigEndian.PutUint32(codec.msgWBuf[1:5], uint32(msgLen))
	codec.msgWBuf = append(codec.msgWBuf[:5], b...)

	_, err = codec.Writer.Write(codec.msgWBuf[:5+msgLen])
	return err
}

func (codec *ProtoCodec) ReadMsg() (proto.Message, error) {
	_, err := io.ReadFull(codec.Reader, codec.msgRBuf[:5])
	if err != nil {
		return nil, err
	}

	//get message type
	msgType := MsgType(codec.msgRBuf[0])

	//get message length
	msgLen := int(binary.BigEndian.Uint32(codec.msgRBuf[1:5]))

	//get message content bytes
	if msgLen > cap(codec.msgRBuf) {
		codec.msgRBuf = make([]byte, msgLen, 2*msgLen)
	}
	msgBytes := codec.msgRBuf[:msgLen]
	_, err = io.ReadFull(codec.Reader, msgBytes)
	if err != nil {
		return nil, err
	}

	msg, err := codec.Make(msgType)
	if err != nil {
		return nil, err
	}

	err = proto.Unmarshal(msgBytes, msg)
	if err != nil {
		return nil, err
	}

	return msg, nil
}
