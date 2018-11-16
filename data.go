package main

import (
	"bytes"
	"encoding/binary"
)

const TCP_SERVER_PORT = "9223"
const KCP_SERVER_PORT = "9224"

//使用KCP协议
const Use_KCP bool = true

//传输双倍数据，包括上一帧的数据
const Send_Double_Data = true

var GAMEMODE_COUNT int

type GameMode struct {
	GameTime  uint32
	PlayerNum int
	GroupNum  int
	GroupID   []int
	SpawnPos  []int
}

var GameModeConfig = make([]GameMode, 0, 10)

//游戏匹配成功等待玩家时间(s)

const WAIT_READY = 15

type Set struct {
	m map[int]bool
}

func NewSet() *Set {
	return &Set{
		m: map[int]bool{},
	}
}

func (s *Set) Add(item int) {
	s.m[item] = true
}

func (s *Set) Remove(item int) {
	delete(s.m, item)
}

func (s *Set) Has(item int) bool {
	_, ok := s.m[item]
	return ok
}

func (s *Set) Len() int {
	return len(s.m)
}

func (s *Set) Clear() {
	s.m = map[int]bool{}
}

func (s *Set) IsEmpty() bool {
	if s.Len() == 0 {
		return true
	}
	return false
}

func (s *Set) List() []int {
	list := []int{}
	for item := range s.m {
		list = append(list, item)
	}
	return list
}

func (s *Set) GetAndRmItem(num int) []int {
	list := []int{}
	if s.Len() >= num {
		i := 0
		for item := range s.m {
			list = append(list, item)
			s.Remove(item)
			i++
			if i >= num {
				break
			}
		}
	}
	return list
}

func (s *Set) GetItems(num int) []int {
	list := []int{}
	i := 0
	for item := range s.m {
		list = append(list, item)
		i++
		if i >= num {
			break
		}
	}
	return list
}

func IntToBytes(n int) []byte {
	tmp := int32(n)
	bytesBuffer := bytes.NewBuffer([]byte{})
	binary.Write(bytesBuffer, binary.LittleEndian, tmp)
	return bytesBuffer.Bytes()
}

func BytesToInt(b []byte) int {
	bytesBuffer := bytes.NewBuffer(b)
	var tmp int32
	binary.Read(bytesBuffer, binary.LittleEndian, &tmp)
	return int(tmp)
}
