package main

import (
	"github.com/montanaflynn/stats"
	"sync"
)

const storegeSize = 5

type Stat struct {
	mut         sync.Mutex
	liteStorage []float64
	pointer     int
	length      int
}

func InitStat() (stat *Stat) {
	return &Stat{liteStorage:make([]float64,storegeSize)}

}

func (s *Stat) AddStat(f float32) {
	s.mut.Lock()
	defer s.mut.Unlock()
	s.liteStorage[s.pointer] = float64(f)
	if(s.length <storegeSize) {
		s.length++
	}
	s.pointer++
	s.pointer%=storegeSize
}

func (s *Stat) getMedian() (float64) {

	s.mut.Lock()
	defer s.mut.Unlock()

	if s.length > 0 {
		ret, err := stats.Median(s.liteStorage[:s.length])
		if err != nil {
			return 0
		}
		return ret
	}
	return 0
}
