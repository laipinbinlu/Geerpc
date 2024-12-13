package geerpc

import (
	"geerpc/day1-codec/codec"
	"reflect"
)

type request struct {
	h      *codec.Header // 头部
	argv   reflect.Value // body
	replyv reflect.Value
}
