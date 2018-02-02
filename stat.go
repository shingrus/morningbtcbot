package main

import (
	"container/ring"
	"github.com/montanaflynn/stats"
)

type Stat struct {
	liteStorage *ring.Ring
}

func InitStat() (stat *Stat) {
	stat = &Stat{ring.New(1000)}
	return
}

func (s *Stat) AddStat(f float32) {
	s.liteStorage.Value = float64(f)
	s.liteStorage = s.liteStorage.Next()
}

func (s *Stat) getMedian() (float64) {
	var values []float64
	s.liteStorage.Do(func(elem interface{}) {
		if elem != nil {
			values = append(values, elem.(float64))
		}
	})
	if len(values) > 0 {
		ret, err := stats.Median(values)
		if err != nil {
			return 0
		}
		return ret
	}
	return 0
}
