package main

import (
	"flag"
	"fmt"
	"log"
	"net/url"
	"os"
	"os/signal"
	"time"

	"github.com/gorilla/websocket"
)

// var addr = flag.String("addr", "localhost:8080", "http service address")

func clientHandler(c *websocket.Conn, done chan struct{}, num int) {
	// defer func() { done <- struct{}{} }()
	go func() {
		defer func() {
			log.Println("channel closed")
			close(done)
		}()
		for {
			_, message, err := c.ReadMessage()
			if err != nil {
				log.Println("read:", err)
				return
			}
			log.Printf("recv: %s", message)
		}
	}()

	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()

	clientStartMessage := fmt.Sprintf("Client %d: ", num)
	for {
		select {
		case <-done:
			return
		case t := <-ticker.C:
			err := c.WriteMessage(websocket.TextMessage, []byte(clientStartMessage+t.String()))
			if err != nil {
				log.Println("write:", err)
				return
			}
		}
	}
}

func main() {
	var numClients int
	var addr string
	var path string

	flag.IntVar(&numClients, "num", 10, "number of clients")
	flag.StringVar(&addr, "url", "localhost:8080", "URL")
	flag.StringVar(&path, "path", "/ws", "Path")

	flag.Parse()

	log.SetFlags(0)

	interrupt := make(chan os.Signal, 1)
	signal.Notify(interrupt, os.Interrupt)

	u := url.URL{Scheme: "ws", Host: addr, Path: path}

	// numClients := 10

	dones := make([]chan struct{}, numClients)
	connections := make([]*websocket.Conn, numClients)
	initFailed := false
	for i := 0; i < numClients; i++ {
		dones[i] = make(chan struct{})

		log.Printf("Connecting %d client to %s", i, u.String())
		c, _, err := websocket.DefaultDialer.Dial(u.String(), nil)
		if err != nil {
			log.Fatal("dial:", err)
			initFailed = true
			break
		}
		defer c.Close()

		connections[i] = c

		go clientHandler(c, dones[i], i)
		log.Printf("Client %d created", i)
	}

	destroyConnections := func() {
		for i := 0; i < numClients; i++ {
			c := connections[i]
			if c == nil {
				continue
			}

			done := dones[i]

			// Cleanly close the connection by sending a close message and then
			// waiting (with timeout) for the server to close the connection.
			err := c.WriteMessage(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseNormalClosure, ""))
			if err != nil {
				log.Println("write close:", err)
				continue
			}
			select {
			case <-done:
			case <-time.After(time.Second):
			}
		}
	}

	if initFailed {
		log.Println("Not all clients created, destroying all clients...")
		destroyConnections()
		return
	}

	for {
		select {
		case <-interrupt:
			log.Println("interrupt")
			destroyConnections()
			return
		}
	}
}
