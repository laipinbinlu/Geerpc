package codec

import "io"

type Header struct {
	ServiceMethod string // rpc服务名字和方法
	Seq           uint64 //请求的序号--ID
	Error         string //错误消息
}

type Codec interface { // 接口方法
	io.Closer
	ReadHeader(*Header) error
	ReadBody(interface{}) error
	Write(*Header, interface{}) error
}

type NewCodecFunc func(io.ReadWriteCloser) Codec // 将函数重命名为新类型方便操作

type Type string

const (
	GobType  Type = "application/gob"
	JsonType Type = "application/json" //编码类型
)

var NewCodecFuncMap map[Type]NewCodecFunc

func init() {
	NewCodecFuncMap = make(map[Type]NewCodecFunc)
	NewCodecFuncMap[GobType] = NewGobCodec
}
