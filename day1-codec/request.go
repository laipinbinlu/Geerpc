package geerpc

import (
	"geerpc/day1-codec/codec"
	"reflect"
)

type request struct {
	h *codec.Header // 头部
	//argv, replyv reflect.Value // body   -- 与下面的同理
	argv   reflect.Value
	replyv reflect.Value
}
