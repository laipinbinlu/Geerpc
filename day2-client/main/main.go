package main

import (
	"fmt"
	geerpc "geerpc/day1-codec"
	geerpc2 "geerpc/day2-client"
	"log"
	"net"
	"sync"
	"time"
)

func startServer(add chan string) {
	l, err := net.Listen("tcp", ":0")
	if err != nil {
		log.Fatalf("network err: %v", err)
	}
	log.Printf("start rpc server on,%v", l.Addr().String())
	add <- l.Addr().String()
	geerpc.Accept(l)
}

func main() {
	log.SetFlags(0)
	add := make(chan string)
	go startServer(add) // 服务端监听

	// 用户开始连接服务端 ---客户端
	client, _ := geerpc2.Dial("tcp", <-add)
	defer func() { _ = client.Close() }()

	time.Sleep(time.Second)
	// send request & receive response
	var wg sync.WaitGroup
	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			args := fmt.Sprintf("geerpc req %d", i)
			var reply string
			if err := client.Call("Foo.Sum", args, &reply); err != nil {
				log.Fatal("call Foo.Sum error:", err)
			}
			log.Println("reply:", reply)
		}(i)
	}
	wg.Wait()
}
