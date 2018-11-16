package main

import (
	"log"
	"math/rand"
	"strconv"
	"time"

	"./sea"
	"github.com/golang/protobuf/proto"
)

type Player struct {
	Uid      int
	Gid      int
	UserName string
}

func RoomHandle(list []int, mode int) {
	//初始化数据
	gnum := len(list)
	players := []*Player{}
	for i, uid := range list {
		p := UserMgr_Ins().Get(uid)
		if p != nil {
			players = append(players, &Player{Uid: uid, Gid: i, UserName: p.UserName})
		} else {
			players = append(players, &Player{Uid: uid, Gid: i, UserName: "未知"})
		}
	}

	time.Sleep(time.Second)

	//帧同步管理器
	lsMgr := NewLsMgr()
	lsMgr.pCount = gnum
	quits := make(chan int, gnum)
	lockquit := make(chan bool)
	frameMsg := make(chan sea.FrameData, gnum)
	//等待玩家准备完毕
	readys := make(chan int, gnum)
	log.Println("New Room Start....")
	//发送开始游戏信号
	r := rand.New(rand.NewSource(time.Now().UnixNano()))

	enter := &sea.EnterRoomMsg{
		Players:    make([]*sea.PlayerInfo, 0),
		RandomSeed: proto.Uint32(uint32(r.Intn(1000000))),
		GameMode:   proto.Int32(int32(mode)),
		GroupCount: proto.Uint32(uint32(GameModeConfig[mode].GroupNum)),
		GameTime:   proto.Uint32(GameModeConfig[mode].GameTime),
	}
	netData := &sea.NetProto{
		Route: proto.String("MatchSuccess"),
	}
	for i := 0; i < gnum; i++ {
		enter.Players = append(enter.Players, &sea.PlayerInfo{
			PlayerID:   proto.Uint32(uint32(players[i].Gid)),
			PlayerName: proto.String(players[i].UserName),
			GroupID:    proto.Uint32(uint32(GameModeConfig[mode].GroupID[i])),
			SpawnPos:   proto.Uint32(uint32(GameModeConfig[mode].SpawnPos[i])),
		})
	}
	for _, p := range players {
		enter.SelfID = proto.Uint32(uint32(p.Gid))
		netData.Data, _ = proto.Marshal(enter)
		//		log.Println(string(b[:]))
		u := UserMgr_Ins().Get(p.Uid)
		if u != nil {
			//初始化玩家信息
			u.WriteByProto(*netData)
			u.FrameChan = frameMsg
			u.Quit = quits
			u.ReadyGame = readys
			u.LockStep = lsMgr
			u.StarGame()
		}
	}
	//等待玩家加载游戏
	waitReady := time.NewTimer(time.Second * WAIT_READY)
	waitSuccess := make(chan bool)
	log.Println("renshu", gnum)
	go func(gnum int, readys <-chan int) {
		for i := 0; i < gnum; {
			select {
			case <-readys:
				i++
			case <-waitReady.C:
				waitSuccess <- false
				return
			}
		}
		waitSuccess <- true
		waitReady.Stop()
	}(gnum, readys)
	isReady := <-waitSuccess
	if isReady == false {
		//等待超时
		log.Println("Wait Fail Quit")
		EndGame(lsMgr, players)
		return
	}

	//处理客户端发回的帧数据
	go lsMgr.RecvFrameHandle(frameMsg, lockquit)
	//每隔一个关键帧向所有房间里的客户端广播上一帧的操作
	//帧同步定时
	ticker := time.NewTicker(66 * time.Millisecond)
	//游戏结束计时器
	gameOverTime := time.NewTimer(time.Second * time.Duration(GameModeConfig[mode].GameTime))
	for {
		select {
		case <-ticker.C:
			var isEnd bool
			var route *string
			var data []byte
			//使用双倍传输数据
			if Send_Double_Data && lsMgr.TurnNum > 1 {
				flist := lsMgr.GetDoubleTurnFrames()
				isEnd = flist.Frames[1].GetIsEnd()
				data, _ = proto.Marshal(flist)
				route = proto.String("FrameDataList")
			} else {
				fds := lsMgr.GetTurnFrames()
				isEnd = fds.GetIsEnd()
				data, _ = proto.Marshal(fds)
				route = proto.String("FrameData")
			}

			//使用Kcp和使用Tcp
			if Use_KCP {
				netData := &sea.KcpProto{
					Route: route,
					Data:  data,
				}
				for _, p := range players {
					u := UserMgr_Ins().Get(p.Uid)
					if u != nil {
						u.WriteProtoByKcp(*netData)
					}
				}
			} else {
				netData := &sea.NetProto{
					Route: route,
					Data:  data,
				}
				for _, p := range players {
					u := UserMgr_Ins().Get(p.Uid)
					if u != nil {
						u.WriteByProto(*netData)
					}
				}
			}

			//log.Println("zhen", string(b[:]))

			if isEnd {
				//游戏结束
				lockquit <- true
				ticker.Stop()
				log.Println("Game over")
				log.Println(lsMgr.TurnNum)
				for _, p := range players {
					u := UserMgr_Ins().Get(p.Uid)
					if u != nil {
						u.Quit = nil
						u.FrameChan = nil
						u.ReadyGame = nil
					}
				}
				return
			}
		case <-gameOverTime.C:
			//			EndGame(lsMgr, players)
			lsMgr.EndGame()

		case uid := <-quits:
			log.Println("client ->" + strconv.Itoa(uid) + "<- halfway exit game")
			//			lockquit <- true
			//			ticker.Stop()
			//			EndGame(lsMgr, players)
			//			log.Println("Room Quit")
			//			return
		}
	}

}

func EndGame(lsMgr *LSMgr, players []*Player) {

	overMsg := &sea.GameOverMsg{AllFrameCount: proto.Uint32(lsMgr.TurnNum)}
	data, _ := proto.Marshal(overMsg)
	netData := &sea.NetProto{
		Route: proto.String("GameOver"),
		Data:  data,
	}
	for _, p := range players {
		u := UserMgr_Ins().Get(p.Uid)
		if u != nil {
			u.WriteByProto(*netData)
			u.EndGame()
		}
	}
}
