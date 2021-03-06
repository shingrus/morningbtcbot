package main

import (
	"time"
	"log"
	tb "gopkg.in/tucnak/telebot.v2"
	"github.com/boltdb/bolt"
	"net/http"
	"fmt"
	"encoding/json"
	"sync"
	"strconv"
	"os"
)

var tokenEnvVar = "TELETOKEN"

const databaseName = "users.db"
const usersBucket = "users"
const chatsBucket = "chats"
const sendDateBucket = "sendDateBucket"
const sendDateKey = "sendDateKey"
const apiUrl = "https://api.coindesk.com/v1/bpi/currentprice.json"

//var picesBucket = "Price_BTC_USD"

var hourToSend = 6

type Users struct {
	usersMap map[int]tb.User
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
		db.Update(func(tx *bolt.Tx) error {
			b := tx.Bucket([]byte(usersBucket))
			err := b.Put([]byte(strconv.Itoa(user.ID)), []byte(user.Username))
			return err
		})

	}

}

func (u *Users) RemoveUser(id int) {
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
		err := b.Delete([]byte(strconv.Itoa(id)))
		return err
	})

}

func InitUsers() (users *Users) {
	users = &Users{usersMap: make(map[int]tb.User)}
	db, err := bolt.Open(databaseName, 0600, nil)
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()

	db.Update(func(tx *bolt.Tx) error {
		_, err := tx.CreateBucketIfNotExists([]byte(usersBucket))
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
		if b := tx.Bucket([]byte(usersBucket)); b != nil {
			c := b.Cursor()
			for k, v := c.First(); k != nil; k, v = c.Next() {
				id, err := strconv.Atoi(string(k))
				if err != nil {
					continue
				}
				fmt.Printf("key=%s, value=%s\n", k, v)
				users.usersMap[id] = tb.User{ID: id, Username: string(v)}

			}
		}
		return nil
	})

	return
}

func (chats *Chats) AddChat(newchat tb.Chat) {
	chats.mut.Lock()
	defer chats.mut.Unlock()
	fmt.Printf("Add chat: %s\n", newchat.ID)
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

//Try with closure
var _lastSendDate time.Time

func getLastSendDate() (time.Time) {
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

func (u *Users) SendToAllUsers(b *tb.Bot, price float64, median float64) {

	//hours, minutes, secs:= time.Now().Clock()
	now := time.Now()
	hours, _, _ := now.Clock()

	if hours == hourToSend {
		//check if we already sent today
		lastSendDate := getLastSendDate()
		fmt.Printf("Time diff in hours: %f", time.Since(lastSendDate).Hours())
		if time.Since(lastSendDate).Hours() > 23 {
			message := fmt.Sprintf("Bitcoin price is: %.2f $, "+
				"Diff: %.2f%%"+
				"\nSee more at https://www.coindesk.com/price/", price, (price/median-1)*100)
			for _, user := range u.getUsers() {
				_, err := b.Send(&user, message)
				if err != nil {
					switch err.Error() {
					case "api error: Bad Request: chat not found":
						u.RemoveUser(user.ID)
					default:
						fmt.Println(err)
					}
				}
			}
			updateLastSendDate(now)
		}
	} else {
		log.Printf("Time hours(%d), not the time to send(%d)", hours, hourToSend)
	}
}

func (chats *Chats) SendToAllChats(b *tb.Bot, price float64, median float64, force bool) {

	//hours, minutes, secs:= time.Now().Clock()
	now := time.Now()
	hours, _, _ := now.Clock()

	if force || hours == hourToSend {
		//check if we already sent today
		lastSendDate := getLastSendDate()
		fmt.Printf("Time diff in hours: %f", time.Since(lastSendDate).Hours())
		if force || time.Since(lastSendDate).Hours() > 23 {
			message := fmt.Sprintf("Bitcoin price is: %.2f $, "+
				"Diff: %.2f%%"+
				"\nSee more at https://www.coindesk.com/price/", price, (price/median-1)*100)
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

func updatePrice() (price float64) {
	var myClient = &http.Client{Timeout: 30 * time.Second}
	res, err := myClient.Get(apiUrl)
	if err == nil {
		dec := json.NewDecoder(res.Body)
		for dec.More() {
			var jval JSVal
			err := dec.Decode(&jval)
			if err != nil {
				fmt.Println(err)
				break
			}
			fmt.Printf("Bitcoin to usd price: %f at %s\n", jval.BPI.USD.Rf, jval.Time.Updated)
			price = jval.BPI.USD.Rf
		}
		res.Body.Close()
	} else {
		log.Println(err)
	}

	return
}

/*
This function check price on coindesk: https://api.coindesk.com/v1/bpi/currentprice.json
 and seep,
The prise is sent to the channel
And to all telegram subscribers
 */
func getPriceEvery60Seconds(stat *Stat, b *tb.Bot,  chats *Chats) {

	for {
		price := updatePrice()
		if price != 0 {
			stat.AddStat(price)
			median := stat.getMedian()
			//users.SendToAllUsers(b, price, median)
			chats.SendToAllChats(b, price, median, false)
			fmt.Printf("Median price: %.2f, diff: %.2f%%\n", median, (1-float64(price)/median)*100)
		}
		//wake up every 30 minutes
		time.Sleep(time.Second * 60)

	}
}
func sendMedianPrice(b *tb.Bot, chatChannel chan *tb.Chat, stat *Stat) {
	for chat, ok := <-chatChannel; ok; chat, ok = <-chatChannel {
		price := updatePrice()
		if price != 0 {
			log.Printf("Send update to %d", chat.ID)
			median := stat.getMedian()
			message := fmt.Sprintf("Bitcoin price is: %.2f $, "+
				"Diff: %.2f%%"+
				"\nSee more at https://www.coindesk.com/price/", price, (price/median-1)*100)
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

	//priceChannel := make(chan float32)

	//users := InitUsers()
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
		b.Send(m.Chat, fmt.Sprintf("Hi, @%s!\nI'm going to send btc price update daily to this chat", m.Sender.Username, ))
		chats.AddChat(*m.Chat)
	})
	//b.Handle("/sendall", func(m *tb.Message) {
	//		log.Println("Send to all")
	//		chats.SendToAllChats(b, 1, 1,true)
	//})
	b.Handle("/start", func(m *tb.Message) {
		b.Send(m.Chat, fmt.Sprintf("Hi, @%s!\nI'm going to send you price update daily", m.Sender.Username))
		//Todo change to chat
		//users.AddUser(*m.Sender)
		chats.AddChat(*m.Chat)
	})

	go getPriceEvery60Seconds(stat, b, chats)
	b.Start()
}
