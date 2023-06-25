package main

import (
	"fmt"
	"github.com/boltdb/bolt"
	"github.com/montanaflynn/stats"
	"sync"
	"time"
)

const statDBName = "stat.db"

const storageSize = 60 * 24

type Stat struct {
	mut       sync.Mutex
	pricesBTC []float64
	pricesETH []float64
	pointer   int
	length    int
	db        *bolt.DB
}

func InitStat() (stat *Stat) {
	stat = &Stat{
		pricesBTC: make([]float64, storageSize),
		pricesETH: make([]float64, storageSize),
	}
	db, err := bolt.Open(statDBName, 0600, nil)
	if err == nil {
		stat.db = db
	}
	return

}

func (s *Stat) AddStat(priceBTC float64, priceETH float64) {
	s.mut.Lock()
	defer s.mut.Unlock()
	s.pricesBTC[s.pointer] = priceBTC
	s.pricesETH[s.pointer] = priceETH
	if s.length < storageSize {
		s.length++
	}
	s.pointer++
	s.pointer %= storageSize
	if s.db != nil {
		s.db.Update(func(tx *bolt.Tx) error {
			now := time.Now()
			statBucket := fmt.Sprintf("%d/%d/%d", now.UTC().Year(), now.UTC().Month(), now.UTC().Day())
			b, err := tx.CreateBucketIfNotExists([]byte(statBucket))
			if err != nil {
				return fmt.Errorf("Can't create a bucket: %s", err)
			}
			err = b.Put([]byte(now.UTC().Format(time.UnixDate)), []byte(fmt.Sprintf("%.2f", priceBTC)))
			//log.Printf("Saved: %s -> %.2f", now.UTC().Format(time.UnixDate), f)
			return err
		})

	}

}

func (s *Stat) getMedian() (priceBTC float64, priceETH float64) {

	s.mut.Lock()
	defer s.mut.Unlock()

	if s.length > 0 {
		var err error
		priceETH, err = stats.Median(s.pricesETH[:s.length])
		priceBTC, err = stats.Median(s.pricesBTC[:s.length])
		if err != nil {
			return
		}

	}
	return
}
