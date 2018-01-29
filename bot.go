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

var databaseName = "users.db"
var usersBucket = "users"
var tokenEnvVar = "TELETOKEN"

type Users struct {
	usersMap map[int]tb.User
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

func (u *Users) getUsers() (ret []tb.User) {
	u.mut.Lock()
	defer u.mut.Unlock()
	for _, v := range u.usersMap {
		ret = append(ret, v)
	}
	return
}

type JSVal struct {
	Time struct {
		Updated string `json:"updated"`
	}
	BPI Currency `json:"BPI"`
}

/*
 "code": "USD",
      "symbol": "&#36;",
      "rate": "11,202.9125",
      "description": "United States Dollar",
      "rate_float": 11202.9125
 */

type Currency struct {
	USD BPI `json:"USD"`
}
type BPI struct {
	Code        string  `json:"code"`
	Symbol      string  `json:"symbol"`
	Rate        string  `json:"rate"`
	Description string  `json:"description"`
	Rf          float32 `json:"rate_float"`
}

/*
This function check price on coindesk: https://api.coindesk.com/v1/bpi/currentprice.json
 and seep,
prise sends to channel
 */
func getPriceEveryNSeconds(priceChannel chan float32, b *tb.Bot, users *Users) {

	apiUrl := "https://api.coindesk.com/v1/bpi/currentprice.json"
	var myClient = &http.Client{Timeout: 30 * time.Second}

	for {

		res, err := myClient.Get(apiUrl)
		if err == nil {
			dec := json.NewDecoder(res.Body)
			//robots, err := ioutil.ReadAll(res.Body)
			for dec.More() {
				var jval JSVal
				err := dec.Decode(&jval)
				if err != nil {
					fmt.Println(err)
					break
				}
				fmt.Printf("Bitcoin to usd price: %f at %s\n", jval.BPI.USD.Rf, jval.Time.Updated)
				select {
				case priceChannel <- jval.BPI.USD.Rf:
					fmt.Printf("sent message to channel")
				default:
					fmt.Println("no price channel readers")
				}

				for _, u := range users.getUsers() {
					_, err := b.Send(&u, fmt.Sprintf("Bitcoin price: %.2f $", jval.BPI.USD.Rf))
					if err != nil {
						switch err.Error() {
						case "api error: Bad Request: chat not found":
							users.RemoveUser(u.ID)
						default:
							fmt.Println(err)
						}
					}
				}

			}
			res.Body.Close()
		} else {
			log.Println(err)
		}
		time.Sleep(time.Second * 60)
	}
}

func main() {

	priceChannel := make(chan float32)

	users := InitUsers()
	b, err := tb.NewBot(tb.Settings{
		Token:  os.Getenv(tokenEnvVar),
		Poller: &tb.LongPoller{Timeout: 10 * time.Second},
	})

	if err != nil {
		log.Fatal(err)
		return
	}

	b.Handle("/hello", func(m *tb.Message) {
		b.Send(m.Sender, "Morning")
	})

	b.Handle("/start", func(m *tb.Message) {
		b.Send(m.Sender, fmt.Sprintf("Hi, @%s!\nThis bot is under development. Please come a bit later", m.Sender.Username))
		users.AddUser(*m.Sender)
	})
	go getPriceEveryNSeconds(priceChannel, b, users)

	b.Start()
}
