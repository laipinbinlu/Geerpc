package geerpc

import (
	"encoding/json"
	"errors"
	"fmt"
	geerpc "geerpc/day1-codec"
	"geerpc/day1-codec/codec"
	"io"
	"log"
	"net"
	"sync"
)

type Call struct { // 使用call结构体来封装一个rpc函数调用应该具有的消息
	Seq           uint64
	ServiceMethod string      // 函数调用的方法
	Args          interface{} // 入参
	Reply         interface{} //回复消息
	Error         error
	Done          chan *Call // 使用chan来传递当前的call调用结束
}

func (call *Call) done() {
	call.Done <- call // 向当前的通道写入当前完成调用的call对象
}

type Client struct {
	cc       codec.Codec    // 编解码器--接口
	opt      *geerpc.Option // 校验消息--解码标识
	sending  sync.Mutex     // 互斥锁--保证请求是有序的发送
	header   codec.Header
	mu       sync.Mutex
	seq      uint64           //请求的ID,给发送的请求编号，每个请求拥有唯一编号
	pending  map[uint64]*Call //存储请求call
	closing  bool             // true  --当前的client不可用， 用户主动关闭
	shutdown bool             // true   --          ，被动错误而关闭
}

var _ io.Closer = (*Client)(nil)

var ErrShutdown = errors.New("connection is shut down")

// 关闭连接
func (client *Client) Close() error {
	client.mu.Lock()
	defer client.mu.Unlock()
	if client.closing {
		return ErrShutdown
	}
	client.closing = true
	return client.cc.Close()
}

// 用户端是否还在工作
func (client *Client) IsAvailable() bool {
	client.mu.Lock()
	defer client.mu.Unlock()
	return !client.shutdown && !client.closing
}

// 用户实现call相关方法

func (client *Client) registerCall(call *Call) (uint64, error) { // 注册服务
	client.mu.Lock()
	defer client.mu.Lock()
	if client.closing || client.shutdown { // 当前客户端已经关闭，则直接返回
		return 0, ErrShutdown
	}
	call.Seq = client.seq // 请求的ID
	client.pending[call.Seq] = call
	client.seq++
	return call.Seq, nil
}

// 删除客户端的请求
func (client *Client) removeCall(seq uint64) *Call {
	client.mu.Lock()
	defer client.mu.Unlock()
	call := client.pending[seq]
	delete(client.pending, seq)
	return call
}

// 客户端发生错误时，终止客户端目前所有的请求--》并且返回错误
func (client *Client) terminateCalls(err error) {
	client.sending.Lock()
	defer client.sending.Unlock()
	client.mu.Lock()
	defer client.mu.Unlock()
	client.shutdown = true
	for _, call := range client.pending { // 终止所有的请求
		call.Error = err
		call.done()
	}
}

// 客户端的两个主要的方法   --- 发送请求  与接受响应

// 接受响应
func (client *Client) receive() {
	var err error

	for err == nil { //  没有错误则就循环接受响应
		var h codec.Header
		if err = client.cc.ReadHeader(&h); err != nil { // 如果错误
			break
		}

		call := client.removeCall(h.Seq) // 移除相关的请求ID

		switch {
		case call == nil: // call不存在--可能请求被取消了  ，，但是服务端还是处理了
			err = client.cc.ReadBody(nil)
		case h.Error != "": // call 存在 ，但是服务端出错了，返回错误
			call.Error = fmt.Errorf(h.Error)
			err = client.cc.ReadBody(nil)
			call.done()
		default: // 正常情况--读取请求
			err = client.cc.ReadBody(call.Reply)
			if err != nil {
				call.Error = errors.New("ready body" + err.Error())
			}
			call.done()
		}
	}
	client.terminateCalls(err) // 终止所有的请求
}

// 创建客户端实例
func NewClient(conn net.Conn, opt *geerpc.Option) (*Client, error) {
	f := codec.NewCodecFuncMap[opt.CodecType] // 根据编码类型取出编解码函数
	if f == nil {
		err := fmt.Errorf("invalid codec type %s", opt.CodecType)
		log.Println("rpc client: option error:", err)
		return nil, err
	}
	if err := json.NewEncoder(conn).Encode(opt); err != nil {
		log.Println("rpc client : option error:", err)
		_ = conn.Close()
		return nil, err
	}
	// 只有编解码验证消息相同，才会开始通信
	return NewClientCodec(f(conn), opt), nil
}

func NewClientCodec(cc codec.Codec, opt *geerpc.Option) *Client {
	client := &Client{
		seq:     1, // 请求ID初始化为1
		cc:      cc,
		opt:     opt,
		pending: make(map[uint64]*Call),
	}
	go client.receive()
	return client
}

func parseOptions(opts ...*geerpc.Option) (*geerpc.Option, error) { // 解析option 参数
	if len(opts) == 0 || opts[0] == nil { // 默认参数即为默认的协议
		return geerpc.DefaultOption, nil
	}
	if len(opts) != 1 {
		return nil, errors.New("number of options is more than 1")
	}

	opt := opts[0]
	opt.MagicNumber = geerpc.DefaultOption.MagicNumber
	if opt.CodecType == "" {
		opt.CodecType = geerpc.DefaultOption.CodecType
	}
	return opt, nil
}

// 自己实现dial函数

func Dial(network, address string, opts ...*geerpc.Option) (client *Client, err error) {
	opt, err := parseOptions(opts...)
	if err != nil {
		return nil, err
	}
	conn, err := net.Dial(network, address)
	if err != nil {
		return nil, err
	}

	defer func() {
		if client == nil {
			_ = conn.Close()
		}
	}()
	return NewClient(conn, opt)
}

// 发送请求
func (client *Client) send(call *Call) {
	client.sending.Lock()
	defer client.sending.Unlock()

	seq, err := client.registerCall(call)
	if err != nil {
		call.Error = err
		call.done()
		return
	}

	// 请求头的东西
	client.header.ServiceMethod = call.ServiceMethod
	client.header.Seq = call.Seq
	client.header.Error = ""

	// 编码并且发送请求
	if err := client.cc.Write(&client.header, call.Args); err != nil {
		call := client.removeCall(seq)
		if call != nil {
			call.Error = err
			call.done()
		}
	}
}

func (client *Client) Go(serviceMethod string, args, reply interface{}, done chan *Call) *Call {
	if done == nil {
		done = make(chan *Call, 10)
	} else if cap(done) == 0 {
		log.Panic(":rpc client : done channel is unbuffered")
	}
	call := &Call{
		ServiceMethod: serviceMethod,
		Args:          args,
		Reply:         reply,
		Done:          done,
	}
	client.send(call)
	return call
}

func (client *Client) Call(serviceMethod string, args, reply interface{}) error {
	call := <-client.Go(serviceMethod, args, reply, make(chan *Call, 1)).Done
	return call.Error
}
