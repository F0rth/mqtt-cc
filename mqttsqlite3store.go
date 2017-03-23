// client.go
package main

import (
	"encoding/json"
	"fmt"
	//"io/ioutil"
	//"log"
	"os"
	//"os/exec"
	//"strings"
	//"time"
	//"golang.org/x/crypto/poly1305"
	//"crypto/sha256"
	//"encoding/base64"
	//"log/syslog"
	"crypto/rand"
	"encoding/hex"
	"reflect"
	"sync"

	"github.com/jmoiron/sqlx"
	_ "github.com/mattn/go-sqlite3"

	MQTT "github.com/eclipse/paho.mqtt.golang"
)

type RawMessage struct {
	Duplicate bool   `db:"Duplicate"`
	MessageID uint16 `db:"MessageID"`
	Payload   []byte `db:"Payload"`
	Qos       byte   `db:"Qos"`
	Retained  bool   `db:"Retained"`
	Topic     string `db:"Topic"`
}

var (
	db                 *sqlx.DB
	RawMessagesInQueue = make(chan RawMessage, 4096)
	MessagesOutQueue   = make(chan Message, 4096)
	wg                 sync.WaitGroup
)

func DbSetup() {
	var err error
	db, err = sqlx.Open("sqlite3", "stockbot.db")
	lib.Check(err)
	//defer db.Close()
}

func InitDB() {
	var err error
	_, err = db.Exec("PRAGMA journal_mode=WAL")
	lib.Check(err)
	_, err = db.Exec("CREATE TABLE AllLog (Id INTEGER PRIMARY KEY AUTOINCREMENT NOT NULL, Isin TEXT NOT NULL, Date TEXT NOT NULL, Open REAL NOT NULL, High REAL NOT NULL, Low REAL NOT NULL, Close REAL NOT NULL, Volume INTEGER NOT NULL, FrDate TEXT NOT NULL)")
	lib.Check(err)
}

func MessagesInProcess(client MQTT.Client, msg MQTT.Message) {
	var m Message
	MessagesInQueue <- m
	fmt.Println(m)
	msg.Duplicate()
	msg.MessageID()
	msg.Payload()
	msg.Qos()
	msg.Retained()
	msg.Topic()
	tnx, err := db.Begin()
	lib.Check(err)
	stmt, err := tnx.Prepare(strings.Join([]string{"INSERT INTO AllLog (Isin,Date,Open,High,Low,Close,Volume,FrDate) VALUES(?, ?, ?, ?, ?, ?, ?, ?)"}, ""))
	lib.Check(err)
	for _, value := range input {
		_, err := stmt.Exec(value.Isin, lib.Time2Date(value.Date), value.Open, value.High, value.Low, value.Close, value.Volume, value.FrDate)
		lib.Check(err)
	}
	err = stmt.Close()
	lib.Check(err)
	err = tnx.Commit()
	lib.Check(err)
}

func MessagesOutProcess(m Message) []byte {
	//m := <-MessagesOutQueue
	r, _ := json.Marshal(m)
	fmt.Println(r)
	return r
}

func InQueueProcess(m Message) {
	// slice where elements are "append" or "delete" is in the 3'th field of the struct
	elementsInBox := reflect.ValueOf(box).Elem().Field(2).Len()
	fmt.Println("number of elements in box: ", elementsInBox)
	if elementsInBox > 1 {
		for i := elementsInBox; i > 1; i-- {
			fmt.Println(i - 1)
			box.Delete(i - 1)
		}
	}
	// clean entries and labels maps
	for k := range entries {
		delete(entries, k)
	}
	for k := range labels {
		delete(labels, k)
	}
	//box.Delete(0)
	fmt.Printf("%+v\n", m)
	for n, v := range m {
		fmt.Printf("index:%d  value:%v  kind:%s  type:%s\n", n, v, reflect.TypeOf(v).Kind(), reflect.TypeOf(v))
	}
	ui.QueueMain(func() {
		for n, v := range m {
			labels[n] = ui.NewLabel(n)
			entries[n] = ui.NewEntry()
			entries[n].SetText(fmt.Sprintf("%v", v))
			box.Append(labels[n], false)
			box.Append(entries[n], false)
		}
		fmt.Println("number of elements in box: ", reflect.ValueOf(box).Elem().Field(2).Len())
	})
}

func MessageRouter() {
	wg.Add(1)
	c := 3
	b := make([]byte, c)
	//_, _ := rand.Read(b)
	rand.Read(b)
	//var incomingMsg MQTT.Message
	hostname, _ := os.Hostname()
	store := "store"
	topicListen := hostname + "/" + "listen"
	topicRespond := hostname + "/" + "respond"
	qos := 2
	opts := MQTT.NewClientOptions()
	opts.AddBroker("ws://127.0.0.1:8000")
	opts.SetClientID(hostname + "_subscriber_" + hex.EncodeToString(b))
	opts.SetStore(MQTT.NewFileStore(store))
	opts.SetCleanSession(true)
	client := MQTT.NewClient(opts)

	if token := client.Connect(); token.Wait() && token.Error() != nil {
		panic(token.Error())
	}

	if token := client.Subscribe(topicListen, byte(qos), MessagesInProcess); token.Wait() && token.Error() != nil {
		fmt.Println(token.Error())
		os.Exit(1)
	}
	for {
		select {
		case out := <-MessagesOutQueue:
			if token := client.Publish(topicRespond, byte(qos), false, MessagesOutProcess(out)); token.Wait() && token.Error() != nil {
				fmt.Println(token.Error())
				os.Exit(1)
			}
		case in := <-MessagesInQueue:
			InQueueProcess(in)
		}
	}
	client.Disconnect(250)
	wg.Done()
}

func main() {

	go MessageRouter()
	wg.Wait()
	fmt.Println("Sample Subscriber Disconnected")
}
