package proxypb

import (
	"git.jd.com/baudtime/baudtime/server"
	"github.com/gogo/protobuf/proto"
)

const (
	InstantQueryRequestType server.MsgType = iota
	RangeQueryRequestType
	WriteRequestType
	QueryResponseType
	WriteResponseType
)

type MsgTypeHandler struct{}

func (h MsgTypeHandler) Type(msg proto.Message) (server.MsgType, error) {
	switch msg.(type) {
	case *InstantQueryRequest:
		return InstantQueryRequestType, nil
	case *RangeQueryRequest:
		return RangeQueryRequestType, nil
	case *WriteRequest:
		return WriteRequestType, nil
	case *QueryResponse:
		return QueryResponseType, nil
	case *WriteResponse:
		return WriteResponseType, nil

	}

	return server.BadMsgType, server.BadMsgTypeError
}

func (h MsgTypeHandler) Make(msgType server.MsgType) (proto.Message, error) {
	switch msgType {
	case InstantQueryRequestType:
		return new(InstantQueryRequest), nil
	case RangeQueryRequestType:
		return new(RangeQueryRequest), nil
	case WriteRequestType:
		return new(WriteRequest), nil
	case QueryResponseType:
		return new(QueryResponse), nil
	case WriteResponseType:
		return new(WriteResponse), nil
	}

	return nil, server.BadMsgTypeError
}
