package main

import "container/ring"

type Stat struct {
	liteStorage *ring.Ring
}


func InitStat() (stat *Stat) {
	stat = &Stat{ring.New(1000)}
	return
}


func (s *Stat) AddStat(f float32) {
	s.liteStorage.Value = f
	s.liteStorage = s.liteStorage.Next()
}