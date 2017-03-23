package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"os/exec"
	"strings"
	"time"
	//"golang.org/x/crypto/poly1305"
	"crypto/sha256"
	"encoding/base64"
	//"log/syslog"
	"crypto/rand"
	"encoding/hex"
	"reflect"
	//"minio/blake2b-simd"

	MQTT "github.com/eclipse/paho.mqtt.golang"
)

type Message struct {
	Command     string
	Data        string
	Filename    string
	Timestamp   int64
	Checksum    string
	Uid         string
	Permissions uint32
}

var (
	MessagesInQueue  = make(chan Message, 4096)
	MessagesOutQueue = make(chan Message, 4096)
)

func MessagesInProcess(client MQTT.Client, msg MQTT.Message) {
	var m Message
	json.Unmarshal(msg.Payload(), &m)
	h := sha256.New()
	h.Write([]byte(m.Filename))
	computed_hash := hex.EncodeToString(h.Sum(nil))
	fmt.Println(computed_hash)
	if computed_hash == m.Checksum {
		MessagesInQueue <- m
	} else {
		fmt.Println("checksum error")
	}
}

func MessagesOutProcess(m Message) []byte {
	//m := <-MessagesOutQueue
	r, _ := json.Marshal(m)
	return r
}
func (m *Message) Exec() []byte {
	bin := strings.Split(m.Filename, " ")[0]
	args := strings.Split(m.Filename, " ")[1:]
	cmd := exec.Command(bin, args...)
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		log.Fatal(err)
	}
	if err := cmd.Start(); err != nil {
		log.Fatal(err)
	}
	str, _ := ioutil.ReadAll(stdout)
	return str
}

//func get
func (m *Message) Get() []byte {
	file, _ := ioutil.ReadFile(m.Filename)
	return file
}

//func put

func (m *Message) Put() {
	base64Bytes, _ := base64.StdEncoding.DecodeString(m.Data)
	ioutil.WriteFile(m.Data, base64Bytes, os.FileMode(m.Permissions))
}

func (m *Message) Delete() {
	err := os.Remove(m.Filename)
	fmt.Println(err)
}

func InQueueProcess(m Message) {
	var data []reflect.Value
	f := reflect.ValueOf(&m).MethodByName(m.Command)
	if f.IsValid() {
		//f.Call([]reflect.Value{})
		data = f.Call(nil)
		m.Command = "ok"
	} else {
		m.Command = "Unknow Method"
	}
	m.Data = base64.StdEncoding.EncodeToString(data[0].Bytes())
	//
	h := sha256.New()
	h.Write([]byte(m.Data))
	m.Checksum = hex.EncodeToString(h.Sum(nil))
	//
	m.Timestamp = time.Now().Unix()
	MessagesOutQueue <- m
}

func main() {
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
	opts.SetClientID("server_" + hex.EncodeToString(b))
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
	fmt.Println("Sample Subscriber Disconnected")
}
