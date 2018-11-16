package main

import (
	"log"
	"sync"

	"./sea"
	"github.com/golang/protobuf/proto"
)

type LSMgr struct {
	LSData  []sea.FrameData
	Frames  map[int][]sea.ActionMsg
	TurnNum uint32
	IsEnd   bool
	pCount  int
	sync.Mutex
}

func NewLsMgr() *LSMgr {
	lsMgr := &LSMgr{
		LSData:  make([]sea.FrameData, 0),
		Frames:  make(map[int][]sea.ActionMsg),
		TurnNum: 0,
		IsEnd:   false,
		pCount:  0,
	}
	return lsMgr
}

func (this *LSMgr) RecvFrameHandle(frame <-chan sea.FrameData, quits chan bool) {
	for {
		select {
		case fd := <-frame:
			if len(fd.Msgs) > 0 {
				//TODO
				_ = fd.FrmID
				for _, msg := range fd.Msgs {
					this.AddFrame(int(msg.GetPlayerID()), *msg)
				}
			}
		case <-quits:
			log.Println("LockStep Quit")
			return
		}
	}
}

func (this *LSMgr) AddFrame(id int, action sea.ActionMsg) {
	this.Lock()
	defer this.Unlock()
	if id >= this.pCount {
		log.Println("GID out of Range")
		return
	}
	if this.Frames[id] == nil {
		this.Frames[id] = []sea.ActionMsg{action}
	} else {
		repeat := false
		for i, item := range this.Frames[id] {
			if item.GetType() == action.GetType() {
				this.Frames[id][i] = action
				repeat = true
				break
			}
		}
		if !repeat {
			this.Frames[id] = append(this.Frames[id], action)
		}
	}

}

func (this *LSMgr) EndGame() {
	this.Lock()
	this.IsEnd = true
	this.Unlock()
}

func (this *LSMgr) GetTurnFrames() *sea.FrameData {
	this.Lock()
	frs := &sea.FrameData{
		FrmID: proto.Uint32(this.TurnNum),
		Msgs:  make([]*sea.ActionMsg, 0),
		IsEnd: proto.Bool(this.IsEnd),
	}
	for _, v := range this.Frames {
		for _, action := range v {
			frs.Msgs = append(frs.Msgs, &action)
		}

	}
	this.Frames = make(map[int][]sea.ActionMsg)
	this.NextTurnFrames(*frs)
	this.Unlock()
	return frs
}

func (this *LSMgr) GetDoubleTurnFrames() *sea.FrameDataList {
	currentFrame := this.GetTurnFrames()
	frames := make([]*sea.FrameData, 2)
	frames[0] = &this.LSData[this.TurnNum-2]
	frames[1] = currentFrame
	frameList := &sea.FrameDataList{Frames: frames}
	return frameList
}

func (this *LSMgr) NextTurnFrames(frame sea.FrameData) {
	this.LSData = append(this.LSData, frame)
	this.TurnNum++
}

func (this *LSMgr) GetRequestFrameList(start int, end int) []*sea.FrameData {
	if start < 0 || start > end || len(this.LSData) <= end {
		return nil
	}
	frameList := make([]*sea.FrameData, end-start+1)
	c := 0
	this.Lock()
	for i := start; i <= end; i++ {
		frameList[c] = &this.LSData[i]
		c++
	}
	this.Unlock()
	return frameList
}
