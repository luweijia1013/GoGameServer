package main

import (
	"encoding/json"
	"io/ioutil"
	"log"
	"net"
	"os"

	"github.com/xtaci/kcp-go"
)

func main() {
	//读取配置文件
	f, err := os.Open("./config.json")
	if err != nil {
		log.Println("config not exist error:", err)
		return
	}
	b, err := ioutil.ReadAll(f)
	if err != nil {
		log.Println("config read error:", err)
		return
	}
	err = json.Unmarshal(b, &GameModeConfig)
	if err != nil {
		log.Println("config read error:", err)
		return
	}
	GAMEMODE_COUNT = len(GameModeConfig)
	//监听服务
	if Use_KCP {
		go KcpHandle()
	}
	TcpHandle()
}

func TcpHandle() {
	id := 0
	l, err := net.Listen("tcp", ":"+TCP_SERVER_PORT)
	if err != nil {
		log.Println("tcp server start error", err)
	}
	log.Println("start game server")
	for {
		conn, err := l.Accept()
		if err != nil {
			log.Println("client tcp connect fail", err)
		}
		id++
		if id < 0 {
			id = 0
		}
		go ClientHandle(conn, id)
	}
}

func KcpHandle() {
	l, err := kcp.ListenWithOptions(":"+KCP_SERVER_PORT, nil, 0, 0)
	if err != nil {
		log.Println("kcp server start error", err)
	}
	log.Println("start kcp server")
	for {
		conn, err := l.AcceptKCP()
		if err != nil {
			log.Println("client kcp connect fail", err)
		}
		go ClientKcpHandle(conn)
	}
}
