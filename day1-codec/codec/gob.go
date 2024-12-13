package codec

import (
	"bufio"
	"encoding/gob"
	"io"
	"log"
)

type GobCodec struct {
	conn io.ReadWriteCloser // 连接 --建立服务的连接的connect
	buf  *bufio.Writer      // 带有缓冲的writer
	dec  *gob.Decoder       // 解码器
	enc  *gob.Encoder       // 编码器
} // 实现Codec接口方法

var _ Codec = (*GobCodec)(nil) // 匿名变量接口

// 创建结构体返回为接口对象
func NewGobCodec(conn io.ReadWriteCloser) Codec {
	buf := bufio.NewWriter(conn)
	return &GobCodec{
		conn: conn,
		buf:  buf,
		dec:  gob.NewDecoder(conn),
		enc:  gob.NewEncoder(buf), // 对writer的缓冲区编码
	}
}

func (g *GobCodec) ReadHeader(h *Header) error { // 读取头部
	return g.dec.Decode(h) // gob解码
}

func (g *GobCodec) ReadBody(body interface{}) error { // 读取消息体
	return g.dec.Decode(body) // 解码
}

func (g *GobCodec) Write(h *Header, body interface{}) error {
	// 编码写入
	defer func() {
		err := g.buf.Flush() // 刷新缓冲区--将缓冲区中尚未写入的数据立即刷新到目标中
		if err != nil {
			_ = g.Close() // 写入完成后 关闭连接
		}

	}()

	if err := g.enc.Encode(h); err != nil { // 编码header
		log.Println("rpc codec: gob error encoding header", err)
		return err
	}
	if err := g.enc.Encode(body); err != nil {
		log.Println("rpc codec: gob error encoding body", err)
		return err
	}
	return nil // 编码成功
}

func (g *GobCodec) Close() error { // 关闭连接
	return g.conn.Close()
}
