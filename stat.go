package main

import (
	"github.com/montanaflynn/stats"
	"sync"
	"github.com/boltdb/bolt"
	"time"
	"fmt"
)

const statDBName = "stat.db"

const storegeSize = 60 * 24

type Stat struct {
	mut         sync.Mutex
	liteStorage []float64
	pointer     int
	length      int
	db          *bolt.DB
}

func InitStat() (stat *Stat) {
	stat = &Stat{liteStorage: make([]float64, storegeSize)}
	db, err := bolt.Open(statDBName, 0600, nil)
	if err == nil {
		stat.db = db
	}
	return

}

func (s *Stat) AddStat(f float64) {
	s.mut.Lock()
	defer s.mut.Unlock()
	s.liteStorage[s.pointer] = float64(f)
	if s.length < storegeSize {
		s.length++
	}
	s.pointer++
	s.pointer %= storegeSize
	if s.db != nil {
		s.db.Update(func(tx *bolt.Tx) error {
			now := time.Now()
			statBucket := fmt.Sprintf("%d/%d/%d", now.UTC().Year(), now.UTC().Month(), now.UTC().Day())
			b, err := tx.CreateBucketIfNotExists([]byte(statBucket))
			if err != nil {
				return fmt.Errorf("Can't create a bucket: %s", err)
			}
			err = b.Put([]byte(now.UTC().Format(time.UnixDate)), []byte(fmt.Sprintf("%.2f", f)))
			//log.Printf("Saved: %s -> %.2f", now.UTC().Format(time.UnixDate), f)
			return err
		})

	}

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
