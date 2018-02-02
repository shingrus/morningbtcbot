package main

import (
	"container/ring"
)

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

func (s *Stat)getAvg () float32 {
	var summ, count float32
	s.liteStorage.Do(func(elem interface{}) {
		if elem!=nil {
			summ += elem.(float32)
			count ++
		}
	})
	ret := (summ/count)
	return ret
}
