package main

import (
	"encoding/json"
	"fmt"
	"github.com/boltdb/bolt"
	tb "gopkg.in/tucnak/telebot.v2"
	"log"
	"net/http"
	"os"
	"strconv"
	"sync"
	"time"
)

var tokenEnvVar = "TELETOKEN"

const databaseName = "users.db"
const usersBucket = "users"
const chatsBucket = "chats"
const sendDateBucket = "sendDateBucket"
const sendDateKey = "sendDateKey"

const apiUrlBTC = "https://api.coinbase.com/v2/prices/BTC-USD/buy"
const apiUrlETH = "https://api.coinbase.com/v2/prices/ETH-USD/buy"

var hourToSend = 6

type Users struct {
	usersMap map[int64]tb.User
	mut      sync.Mutex
}
type Chats struct {
	chatsMap map[int64]tb.Chat
	mut      sync.Mutex
}

//

func (u *Users) AddUser(user tb.User) {
	u.mut.Lock()
	defer u.mut.Unlock()
	fmt.Printf("Add user: %s\n", user.Username)
	if _, ok := u.usersMap[user.ID]; !ok {
		u.usersMap[user.ID] = user
		db, err := bolt.Open(databaseName, 0600, nil)
		if err != nil {
			log.Fatal(err)
		}
		defer db.Close()
		err = db.Update(func(tx *bolt.Tx) error {
			b := tx.Bucket([]byte(usersBucket))
			err := b.Put([]byte(strconv.FormatInt(user.ID, 10)), []byte(user.Username))
			return err
		})

	}

}

func (u *Users) RemoveUser(id int64) {
	u.mut.Lock()
	defer u.mut.Unlock()
	delete(u.usersMap, id)
	db, err := bolt.Open(databaseName, 0600, nil)
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()

	db.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(usersBucket))
		err := b.Delete([]byte(strconv.FormatInt(id, 10)))
		return err
	})

}

func (chats *Chats) AddChat(newchat tb.Chat) {
	chats.mut.Lock()
	defer chats.mut.Unlock()
	fmt.Printf("Add chat: %i\n", newchat.ID)
	if _, ok := chats.chatsMap[newchat.ID]; !ok {
		chats.chatsMap[newchat.ID] = newchat
		db, err := bolt.Open(databaseName, 0600, nil)
		if err != nil {
			log.Fatal(err)
		}
		defer db.Close()
		db.Update(func(tx *bolt.Tx) error {
			b := tx.Bucket([]byte(chatsBucket))
			val, _ := json.Marshal(newchat)
			err := b.Put([]byte(strconv.FormatInt(newchat.ID, 10)), val)
			return err
		})

	}
}

func (chats *Chats) RemoveChat(id int64) {
	chats.mut.Lock()
	defer chats.mut.Unlock()
	delete(chats.chatsMap, id)
	db, err := bolt.Open(databaseName, 0600, nil)
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()

	db.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(chatsBucket))
		err := b.Delete([]byte(strconv.FormatInt(id, 10)))
		return err
	})

}

func InitChats() (chats *Chats) {
	chats = &Chats{chatsMap: make(map[int64]tb.Chat)}
	db, err := bolt.Open(databaseName, 0600, nil)
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()

	db.Update(func(tx *bolt.Tx) error {
		_, err := tx.CreateBucketIfNotExists([]byte(chatsBucket))
		if err != nil {
			return fmt.Errorf("create bucket: %s", err)
		}
		_, err = tx.CreateBucketIfNotExists([]byte(sendDateBucket))
		if err != nil {
			return fmt.Errorf("create bucket: %s", err)
		}

		return nil
	})
	db.View(func(tx *bolt.Tx) error {
		if b := tx.Bucket([]byte(chatsBucket)); b != nil {
			c := b.Cursor()
			for k, v := c.First(); k != nil; k, v = c.Next() {
				id, err := strconv.ParseInt(string(k), 10, 64)
				if err != nil {
					continue
				}
				fmt.Printf("key=%s, value=%s\n", k, v)
				var newChat tb.Chat
				if err := json.Unmarshal(v, &newChat); err == nil {
					chats.chatsMap[id] = newChat
				}
			}
		}
		return nil
	})

	return
}

func (u *Users) getUsers() (ret []tb.User) {
	u.mut.Lock()
	defer u.mut.Unlock()
	for _, v := range u.usersMap {
		ret = append(ret, v)
	}
	return
}

func (chats *Chats) getChats() (ret []tb.Chat) {
	chats.mut.Lock()
	defer chats.mut.Unlock()
	for _, v := range chats.chatsMap {
		ret = append(ret, v)
	}
	return
}

// Try with closure
var _lastSendDate time.Time

func getLastSendDate() time.Time {
	if _lastSendDate.IsZero() {
		db, err := bolt.Open(databaseName, 0600, nil)
		if err != nil {
			log.Fatal(err)
		}
		defer db.Close()
		db.View(func(tx *bolt.Tx) error {
			b := tx.Bucket([]byte(sendDateBucket))
			val := b.Get([]byte(sendDateKey))
			if val != nil {
				fmt.Printf("saved time: %s\n", string(val))
				_lastSendDate, err = time.Parse(time.UnixDate, string(val))
				if err != nil {
					log.Println(err)
					_lastSendDate = time.Now()
				}
			}
			return nil
		})
	}
	return _lastSendDate
}
func updateLastSendDate(sendDate time.Time) {
	db, err := bolt.Open(databaseName, 0600, nil)
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()
	db.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(sendDateBucket))
		err := b.Put([]byte(sendDateKey), []byte(sendDate.Format(time.UnixDate)))
		log.Printf("Saved: %s", sendDate.Format(time.UnixDate))
		return err
	})
	_lastSendDate = sendDate
}

func (chats *Chats) SendToAllChats(b *tb.Bot, priceBTC float64, medianBTC float64, priceETH float64, medianETH float64, force bool) {

	now := time.Now()
	hours, _, _ := now.Clock()

	if force || hours == hourToSend {
		//check if we already sent today
		lastSendDate := getLastSendDate()
		fmt.Printf("Time diff in hours: %f", time.Since(lastSendDate).Hours())
		if force || time.Since(lastSendDate).Hours() > 23 {
			message := fmt.Sprintf("BTC price is: %.2f $, "+
				"Diff: %.2f%%"+
				"ETH price is: %.2f$, "+
				"Diff: %.2f%%\n",
				priceBTC, (priceBTC/medianBTC-1)*100,
				medianETH, (priceETH/medianETH-1)*100)
			for _, chat := range chats.getChats() {
				_, err := b.Send(&chat, message)
				if err != nil {
					switch err.Error() {
					case "api error: Bad Request: chat not found":
						chats.RemoveChat(chat.ID)
					default:
						fmt.Println(err)
					}
				}
			}
			if !force {
				updateLastSendDate(now)
			}
		}
	} else {
		log.Printf("Time hours(%d), not the time to send(%d)", hours, hourToSend)
	}
}

type JSVal struct {
	Time struct {
		Updated string `json:"updated"`
	}
	BPI Currency `json:"BPI"`
}

type Currency struct {
	USD BPI `json:"USD"`
}
type BPI struct {
	Code        string  `json:"code"`
	Symbol      string  `json:"symbol"`
	Rate        string  `json:"rate"`
	Description string  `json:"description"`
	Rf          float64 `json:"rate_float"`
}

type CoinBasev2JSVal struct {
	Data struct {
		Base     string  `json:"base"`
		Currency string  `json:"currency"`
		Amount   float64 `json:"amount,string"`
	} `json:"data"`
}

func updatePrice() (priceBTC float64, priceETH float64) {
	var myClient = &http.Client{Timeout: 30 * time.Second}
	res, err := myClient.Get(apiUrlBTC)
	if err == nil {
		dec := json.NewDecoder(res.Body)
		for dec.More() {
			var jval CoinBasev2JSVal
			err := dec.Decode(&jval)
			if err != nil {
				fmt.Println(err)
				break
			}
			priceBTC = jval.Data.Amount

		}
		res.Body.Close()
		res, err = myClient.Get(apiUrlETH)
		if err == nil {
			dec = json.NewDecoder(res.Body)
			for dec.More() {
				var jval CoinBasev2JSVal
				err := dec.Decode(&jval)
				if err != nil {
					fmt.Println(err)
					break
				}
				priceETH = jval.Data.Amount
			}
			res.Body.Close()
		}
		fmt.Printf("BTC price: %f\nETH price: %f\n", priceBTC, priceETH)

	} else {
		log.Println(err)
	}

	return
}

/*
This function check price on coindesk API 2
and sleep,

The price sent to the channel
And to all telegram subscribers
*/
func getPriceEvery60Seconds(stat *Stat, b *tb.Bot, chats *Chats) {

	for {
		priceBTC, priceETH := updatePrice()
		if priceBTC != 0 && priceETH != 0 {
			stat.AddStat(priceBTC, priceETH)
			medianBTC, medianETH := stat.getMedian()
			chats.SendToAllChats(b, priceBTC, medianBTC, priceETH, medianETH, false)
			fmt.Printf("BTC median: %.2f, diff: %.2f%%\nETH median %.2f, diff: %.2f%%",
				medianBTC, (1-float64(priceBTC)/medianBTC)*100, medianETH, (1-float64(priceETH)/medianETH)*100)
		}
		//wake up every 30 minute
		time.Sleep(time.Second * 60)

	}
}
func sendMedianPrice(b *tb.Bot, chatChannel chan *tb.Chat, stat *Stat) {
	for chat, ok := <-chatChannel; ok; chat, ok = <-chatChannel {
		priceBTC, priceETH := updatePrice()
		if priceBTC != 0 {
			log.Printf("Send update to %d", chat.ID)
			medianBTC, medianETH := stat.getMedian()
			message := fmt.Sprintf("BTC price is: %.2f$, "+
				"Diff: %.2f%%\n"+
				"ETH price is: %.2f$, "+
				"Diff: %.2f%%\n",
				priceBTC, (priceBTC/medianBTC-1)*100, medianETH, (priceETH/medianETH-1)*100)
			_, err := b.Send(chat, message)
			if err != nil {
				switch err.Error() {
				case "api error: Bad Request: no such user":
				default:
					fmt.Println(err)
				}
			}
		}
	}
}

func sendUserToChan(ch chan *tb.Chat, chat *tb.Chat) {
	ch <- chat
}

func main() {

	chats := InitChats()
	b, err := tb.NewBot(tb.Settings{
		Token:  os.Getenv(tokenEnvVar),
		Poller: &tb.LongPoller{Timeout: 10 * time.Second},
	})
	stat := InitStat()
	userChannel := make(chan *tb.Chat)
	go sendMedianPrice(b, userChannel, stat)

	if err != nil {
		log.Fatal(err)
		return
	}

	b.Handle("/hello", func(m *tb.Message) {
		b.Send(m.Sender, "Morning")
	})
	b.Handle("/update", func(m *tb.Message) {

		go sendUserToChan(userChannel, m.Chat)
	})
	b.Handle("/unsubscribe", func(m *tb.Message) {
		b.Send(m.Chat, fmt.Sprintf("Hey, @%s!\nThis chat was unsubscribed", m.Sender.Username))
		chats.RemoveChat(m.Chat.ID)
		//users.RemoveUser(m.Sender.ID)

	})
	b.Handle("/subscribe", func(m *tb.Message) {
		b.Send(m.Chat, fmt.Sprintf("Hi, @%s!\nI'm going to send btc price update daily to this chat", m.Sender.Username))
		chats.AddChat(*m.Chat)
	})
	//b.Handle("/sendall", func(m *tb.Message) {
	//		log.Println("Send to all")
	//		chats.SendToAllChats(b, 1, 1,true)
	//})
	b.Handle("/start", func(m *tb.Message) {
		b.Send(m.Chat, fmt.Sprintf("Hi, @%s!\nI'm going to send you price update daily", m.Sender.Username))
		chats.AddChat(*m.Chat)
	})

	go getPriceEvery60Seconds(stat, b, chats)
	b.Start()
}
