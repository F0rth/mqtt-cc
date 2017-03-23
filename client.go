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

	"github.com/protonmail/ui"

	MQTT "github.com/eclipse/paho.mqtt.golang"
)

type Message map[string]interface{}

var (
	MessagesInQueue  = make(chan Message, 4096)
	MessagesOutQueue = make(chan Message, 4096)
	labels           = make(map[interface{}]*ui.Label)
	entries          = make(map[interface{}]*ui.Entry)
	wg               sync.WaitGroup
	box              *ui.Box
)

var WindgetsMap = map[reflect.Kind]interface{}{
	reflect.Bool:   ui.NewCheckbox,
	reflect.String: ui.NewEntry,
}

func MessagesInProcess(client MQTT.Client, msg MQTT.Message) {
	var m Message
	json.Unmarshal(msg.Payload(), &m)
	MessagesInQueue <- m
	fmt.Println(m)
}

func MessagesOutProcess(m Message) []byte {
	//m := <-MessagesOutQueue
	r, _ := json.Marshal(m)
	fmt.Println(r)
	return r
}

func GUI() {
	wg.Add(1)
	_ = ui.Main(func() {
		send := ui.NewButton("send")
		box = ui.NewVerticalBox()
		box.SetPadded(true)
		box.Append(send, false)
		window := ui.NewWindow("Auto generated Form", 300, 10, false)
		window.SetChild(box)
		window.SetMargined(true)
		//window.SetContentSize(100, 100)
		window.OnClosing(func(*ui.Window) bool {
			ui.Quit()
			return true
		})
		send.OnClicked(func(*ui.Button) {
			r := make(Message)
			for n, v := range entries {
				r[n.(string)] = v.Text()
			}
			MessagesOutQueue <- r
			fmt.Println(r)
		})
		window.Show()

	})
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
}

func main() {
	go MessageRouter()
	GUI()
	wg.Wait()
	fmt.Println("Sample Subscriber Disconnected")
}
