package main

import (
	"encoding/json"
	"fmt"
	geerpc "geerpc/day1-codec"
	"geerpc/day1-codec/codec"
	"log"
	"net"
	"time"
)

func startServer(add chan string) {
	l, err := net.Listen("tcp", ":0")
	if err != nil {
		log.Fatalf("net err: %v", err)
	}
	log.Printf("start rpc server on,%v", l.Addr().String())
	add <- l.Addr().String()
	geerpc.Accept(l)
}

func main() {
	add := make(chan string)
	go startServer(add)

	// 用户开始连接服务端
	conn, _ := net.Dial("tcp", <-add)
	defer conn.Close()

	time.Sleep(time.Second)

	_ = json.NewEncoder(conn).Encode(geerpc.DefaultOption)
	cc := codec.NewGobCodec(conn)

	for i := 0; i < 5; i++ {
		h := &codec.Header{
			ServiceMethod: "Foo.Sum",
			Seq:           uint64(i),
		}
		_ = cc.Write(h, fmt.Sprintf("geerpc  req %d", i))
		_ = cc.ReadHeader(h)
		var reply string
		_ = cc.ReadBody(&reply)
		log.Println("reply:", reply)
	}
}
