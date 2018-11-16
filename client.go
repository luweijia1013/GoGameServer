package main

import (
	"log"
	"net"
	"strconv"
	"sync/atomic"
	"time"

	"./sea"
	"github.com/golang/protobuf/proto"
	"github.com/xtaci/kcp-go"
)

type User struct {
	Id        int
	UserName  string
	conn      net.Conn
	Kcp       *kcp.UDPSession
	isClose   int32
	isGaming  int32
	FrameChan chan<- sea.FrameData
	Quit      chan<- int
	ReadyGame chan<- int
	LockStep  *LSMgr
}

func (this *User) IsClosed() bool {
	return atomic.LoadInt32(&this.isClose) == 0
}

func (this *User) Close() {
	if atomic.CompareAndSwapInt32(&this.isClose, 1, 0) {
		log.Println("client ->" + strconv.Itoa(this.Id) + "<- dis connect.")
		if this.conn != nil {
			this.conn.Close()
		}
		if this.Quit != nil {
			this.Quit <- this.Id
			this.Quit = nil
		}
		if this.ReadyGame != nil {
			this.ReadyGame = nil
		}
		if this.Kcp != nil {
			this.Kcp.Close()
		}
		this.EndGame()
		UserMgr_Ins().Del(this)
	}
}

func (this *User) IsGaming() bool {
	return atomic.LoadInt32(&this.isGaming) == 1
}

func (this *User) EndGame() {
	atomic.CompareAndSwapInt32(&this.isGaming, 1, 0)
	this.LockStep = nil
}

func (this *User) StarGame() {
	atomic.CompareAndSwapInt32(&this.isGaming, 0, 1)
}

func (this *User) Write(str string) (err error) {
	this.WriteBytes([]byte(str))
	return nil
}

func (this *User) WriteBytes(bs []byte) (err error) {
	if !this.IsClosed() {
		_, err = this.conn.Write(bs)
	}
	return nil
}

func (this *User) WriteByProto(netData sea.NetProto) (err error) {
	if netData.GetData() != nil {
		netData.DataCount = proto.Int32(int32(len(netData.GetData())))
	}
	b, err := proto.Marshal(&netData)
	if err != nil {
		log.Println("proto error", err)
		return err
	}
	this.WriteBytes(b)
	return nil
}

func (this *User) SendKcpMsg() {
	kcpInfo := &sea.KcpProto{TcpId: proto.Int64(int64(this.Id))}
	data, _ := proto.Marshal(kcpInfo)
	netData := &sea.NetProto{
		Route: proto.String("KcpInfo"),
		Data:  data,
	}
	this.WriteByProto(*netData)
}

func (this *User) WriteBytesByKcp(bs []byte) (err error) {
	if !this.IsClosed() && this.Kcp != nil {
		_, err = this.Kcp.Write(bs)
	}
	return nil
}

func (this *User) WriteProtoByKcp(netData sea.KcpProto) (err error) {
	if netData.GetData() != nil {
		netData.DataCount = proto.Int32(int32(len(netData.GetData())))
	}
	netData.TcpId = proto.Int64(int64(this.Id))
	b, err := proto.Marshal(&netData)
	if err != nil {
		log.Println("proto error", err)
		return err
	}
	this.WriteBytesByKcp(b)
	return nil
}

func ClientHandle(conn net.Conn, id int) {
	log.Println("client id-> " + strconv.Itoa(id) + " <-connect")
	this := NewUser(conn, id)
	//	UserMgr_Ins().Add(this)
	buf := make([]byte, 2048)
	//发送Kcp信息
	this.SendKcpMsg()
	for !this.IsClosed() {
		l, err := conn.Read(buf)
		if err != nil {
			this.Close()
			return
		}
		netData := &sea.NetProto{}
		err = proto.Unmarshal(buf[:l], netData)
		if err != nil {
			log.Println("proto error", err)
			continue
		}
		route := netData.GetRoute()
		if route != "Action" {
			log.Println("client ->", this.Id, "<- request route:", route)
		}
		switch {
		case route == "RequestKcpInfo":
			//没有收到KcpInfo，重传
			this.SendKcpMsg()
		case route == "StartMatch":
			if this.IsGaming() {
				this.EndGame()
			}
			requestEnter := &sea.RequestEnterMsg{}
			err := proto.Unmarshal(netData.GetData(), requestEnter)
			if err != nil {
				log.Println("proto error", err)
				break
			}
			log.Println("mode", int(requestEnter.GetGameMode()))
			this.UserName = requestEnter.GetUsername()
			UserMgr_Ins().AddToWait(id, int(requestEnter.GetGameMode()))
		case route == "Action":
			if !this.IsGaming() {
				break
			}
			action := &sea.FrameData{}
			err := proto.Unmarshal(netData.GetData(), action)
			if err != nil {
				log.Println("proto error", err)
				break
			}
			if this.FrameChan != nil {
				this.FrameChan <- *action
			}
		case route == "ReadyGame":
			if this.ReadyGame != nil {
				this.ReadyGame <- this.Id
			}
		case route == "RequestFrameData":
			//客户端获取丢失的帧数据
			requestFrame := &sea.RequestFrameData{}
			err := proto.Unmarshal(netData.GetData(), requestFrame)
			if err != nil {
				log.Println("proto error", err)
				break
			}
			if this.LockStep != nil {
				frameList := this.LockStep.GetRequestFrameList(int(requestFrame.GetStart()), int(requestFrame.GetEnd()))
				if frameList != nil {
					log.Println("client lost", this.Id, " len:", len(frameList), " start", frameList[0].GetFrmID())
					frames := &sea.FrameDataList{Frames: frameList}
					data, _ := proto.Marshal(frames)
					netData := &sea.NetProto{
						Route: proto.String("FrameDataList"),
						Data:  data,
					}
					this.WriteByProto(*netData)
				}

			}
		}
	}
}

func ClientKcpHandle(conn *kcp.UDPSession) {
	log.Println("kcp client connect", conn.RemoteAddr())
	conn.SetStreamMode(true)
	conn.SetWindowSize(256, 256)
	conn.SetNoDelay(1, 10, 2, 1)
	conn.SetACKNoDelay(false)
	conn.SetDSCP(46)
	conn.SetReadDeadline(time.Now().Add(time.Hour))
	conn.SetWriteDeadline(time.Now().Add(time.Hour))
	buf := make([]byte, 2048)
	for {
		l, err := conn.Read(buf)
		if err != nil {
			log.Println("client kcp dis connect")
			if conn != nil {
				conn.Close()
			}
			break
		}
		netData := &sea.KcpProto{}
		err = proto.Unmarshal(buf[:l], netData)
		if err != nil {
			log.Println("proto error", err)
			continue
		}
		route := netData.GetRoute()
		switch {
		case route == "Connect":
			u := UserMgr_Ins().Get(int(netData.GetTcpId()))
			if u != nil {
				u.Kcp = conn
			}
		case route == "Action":
			u := UserMgr_Ins().Get(int(netData.GetTcpId()))
			if u == nil {
				break
			}
			if !u.IsGaming() {
				break
			}
			action := &sea.FrameData{}
			err := proto.Unmarshal(netData.GetData(), action)
			if err != nil {
				log.Println("proto error", err)
				break
			}
			if u.FrameChan != nil {
				u.FrameChan <- *action
			}
		}
	}
}

func NewUser(conn net.Conn, id int) (u *User) {
	u = &User{
		conn:     conn,
		isClose:  1,
		isGaming: 0,
		Id:       id,
		LockStep: nil,
		Kcp:      nil,
	}
	UserMgr_Ins().Add(u)
	return u
}
