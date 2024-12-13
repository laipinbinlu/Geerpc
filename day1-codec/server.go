package geerpc

import (
	"encoding/json"
	"fmt"
	"geerpc/day1-codec/codec"
	"io"
	"log"
	"net"
	"reflect"
	"sync"
)

const MagicNumber = 0x3bef5c

type Option struct {
	MagicNumber int        // 固定的值 -->标识协议或者协议版本，只有该值相同才会处理数据
	CodecType   codec.Type // 不同的编码形式
}

var DefaultOption = &Option{ // 默认初始化状态
	MagicNumber: MagicNumber,
	CodecType:   codec.GobType,
}

// option 采用json编码 , header 和body 采用option指定的类型编码

type Server struct {
}

func NewServer() *Server {
	return &Server{}
}

var DefaultServer = NewServer() // 初始化一个server

func (s *Server) Accept(lis net.Listener) { // 服务器监听连接
	for {
		conn, err := lis.Accept()
		if err != nil {
			log.Println("lis accept err", err)
			return
		}
		// 连接成功--- 服务器服务客户端
		go s.ServeConn(conn)
	}
}

func Accept(lis net.Listener) { DefaultServer.Accept(lis) }

func (s *Server) ServeConn(conn io.ReadWriteCloser) {
	var opt Option
	// json.NewDecoder 从conn接口（io.reader）读取数据，进行解码->Decode 会把从 conn 中读取到的 JSON 数据解码到这个对象(&opt)中。(将json数据变成结构体对象)
	if err := json.NewDecoder(conn).Decode(&opt); err != nil { // 解码错误
		log.Println("rpc server : option decode error", err)
		return
	}

	if opt.MagicNumber != MagicNumber { // 协议不对--错误
		log.Printf("rpc server: invalid magic number %x", opt.MagicNumber)
		return
	}

	f := codec.NewCodecFuncMap[opt.CodecType] // 传入解码类型-->得到处理函数f-> 返回接口方法
	if f == nil {
		log.Printf("rpc server: invalid codec type %v", opt.CodecType)
		return
	}

	// 传入的消息都没有错误（校验） ---> 处理用户传入的消息request请求
	s.serveCodec(f(conn))

	defer func() { // 不管发生什么，都要关闭连接
		_ = conn.Close()
	}()
}

func (s *Server) serveCodec(cc codec.Codec) { // 处理请求 -> 1.先是读取用户请求 2.处理用户请求  3. 恢复请求==返回响应
	sending := new(sync.Mutex)
	wg := new(sync.WaitGroup)
	for {
		req, err := s.readRequest(cc) // 读取请求
		if err != nil {
			if req == nil {
				break // 空消息 --> 跳出循环
			}
			req.h.Error = err.Error()               // 将错误附加到头部 --->作为消息返回给用户
			s.sendResponse(cc, req.h, nil, sending) // 回复请求
			continue
		}
		wg.Add(1)
		go s.handleRequest(cc, req, sending, wg) // 处理请求
	}
	wg.Wait()
	_ = cc.Close()
}

func (s *Server) readRequest(cc codec.Codec) (*request, error) { // 1.读取请求内容
	h, err := s.readRequestHeader(cc) // 读取请求的头部
	if err != nil {
		return nil, err
	}
	req := &request{h: h}

	req.argv = reflect.New(reflect.TypeOf(""))               // 请求的body
	if err = cc.ReadBody(req.argv.Interface()); err != nil { // 读取body
		log.Println("rpc server : read argv err :", err)
		return nil, err
	}
	return req, nil
}

func (s *Server) readRequestHeader(cc codec.Codec) (*codec.Header, error) { // 读取请求中的头部
	var h codec.Header
	if err := cc.ReadHeader(&h); err != nil {
		if err != io.EOF && err != io.ErrUnexpectedEOF { // 忽略EOF错误 -->打印其他错误
			log.Println("rpc server: read header error", err)
		}
		return nil, err
	}
	return &h, nil // 返回头部结果
}

// 返回响应
func (s *Server) sendResponse(cc codec.Codec, h *codec.Header, reply interface{}, sending *sync.Mutex) {
	sending.Lock()
	defer sending.Unlock()
	if err := cc.Write(h, reply); err != nil {
		log.Println("rpc server : write response error:", err)
	}
}

// 处理请求
func (s *Server) handleRequest(cc codec.Codec, req *request, sending *sync.Mutex, wg *sync.WaitGroup) {
	defer wg.Done()
	log.Println(req.h, req.argv.Elem())
	req.replyv = reflect.ValueOf(fmt.Sprintf("geerpc resp %d", req.h.Seq)) // 回应内容
	s.sendResponse(cc, req.h, req.replyv.Interface(), sending)
}
