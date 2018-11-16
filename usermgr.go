package main

import (
	"sync"
	"time"

	"./sea"
	"github.com/golang/protobuf/proto"
)

var _userMsg *UserMgr

func UserMgr_Ins() *UserMgr {
	if _userMsg == nil {
		_userMsg = &UserMgr{
			users:     make(map[int]*User),
			wait:      make(map[int]int),
			modes:     make(map[int]*Set),
			ModeCount: GAMEMODE_COUNT,
		}
		for i := 0; i < _userMsg.ModeCount; i++ {
			_userMsg.modes[i] = NewSet()
		}
		go _userMsg.MatchGameHandle()
	}
	return _userMsg
}

type UserMgr struct {
	users     map[int]*User
	wait      map[int]int
	modes     map[int]*Set
	ModeCount int
	sync.Mutex
}

func (this *UserMgr) Add(u *User) *User {

	this.Lock()
	this.users[u.Id] = u
	this.Unlock()
	return u
}

func (this *UserMgr) Del(u *User) {
	this.Lock()
	mode, ok := this.wait[u.Id]
	if ok {
		delete(this.wait, u.Id)
		this.modes[mode].Remove(u.Id)
		this.NoticePlayerChange(mode)
	}
	//	if this.wait.Has(u.Id) {
	//		this.wait.Remove(u.Id)
	//	}
	delete(this.users, u.Id)
	this.Unlock()
}

func (this *UserMgr) Get(id int) *User {

	this.Lock()
	u, ok := this.users[id]
	this.Unlock()
	if ok {
		return u
	}
	return nil
}

//func (this *UserMgr) SendAllOther(msg string, eid int) {
//	this.Lock()
//	for _, u := range this.users {
//		//		log.Println("user id " + strconv.Itoa(int(u.Id)))
//		if u.Id == eid {
//			continue
//		}
//		u.Write(msg)
//	}
//	this.Unlock()
//}

//func (this *UserMgr) SendAll(msg string) {
//	this.Lock()
//	for _, u := range this.users {
//		u.Write(msg)
//	}
//	this.Unlock()
//}

func (this *UserMgr) AddToWait(id int, gameMode int) {
	this.Lock()
	if gameMode < this.ModeCount {
		this.wait[id] = gameMode
		this.modes[gameMode].Add(id)
		this.NoticePlayerChange(gameMode)
	}
	this.Unlock()
}

//通知匹配的人数修改
func (this *UserMgr) NoticePlayerChange(mode int) {
	list := this.modes[mode].GetItems(GameModeConfig[mode].PlayerNum)

	l := len(list)
	if l == 0 {
		return
	}
	if l >= GameModeConfig[mode].PlayerNum {
		return
	}
	playerChange := &sea.MatchPlayerChange{PlayerNum: proto.Uint32(uint32(l))}
	data, _ := proto.Marshal(playerChange)
	netData := &sea.NetProto{
		Route: proto.String("MatchPlayerChange"),
		Data:  data,
	}
	for _, v := range list {
		u, ok := this.users[v]
		if ok {
			u.WriteByProto(*netData)
		}
	}
}

//func (this *UserMgr) RmFromWait(id int) {
//	this.Lock()
//	this.wait.Remove(id)
//	this.Unlock()
//}

//根据不同模式匹配游戏
func (this *UserMgr) MatchGameHandle() {
	for {
		time.Sleep(time.Millisecond * 200)
		this.Lock()
		for mode, set := range this.modes {
			if set.Len() >= GameModeConfig[mode].PlayerNum {
				list := set.GetAndRmItem(GameModeConfig[mode].PlayerNum)
				if len(list) == GameModeConfig[mode].PlayerNum {
					for _, v := range list {
						delete(this.wait, v)
					}
					go RoomHandle(list, mode)
				}
			}
		}
		this.Unlock()
	}
}
