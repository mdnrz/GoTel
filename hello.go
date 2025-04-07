package main

import (
	"log"
	"net"
	"time"
	"slices"
)

type msgType int

const (
	msgConnect msgType = iota + 1
	msgLogin
	msgText
	msgQuit
)

type Client struct {
	UserName string
	Request msgType
	Strike   int
	Banned   bool
	BannedAt time.Time
	Conn     net.Conn
	Text string
}

const Port = "6969"

func client(conn net.Conn, Client_q chan Client) {
	loginPrompt := "Who are you?\n> "
	_, err := conn.Write([]byte(loginPrompt))
	if err != nil {
		log.Printf("[ERROR] Could not send login prompt to user %s: %s\n",
			conn.RemoteAddr().String(), err)
	}
	readBuf := make([]byte, 512)
	for {
		n, err := conn.Read(readBuf)
		if err != nil {
			log.Printf("Could not read message from client %s: %s\n", conn.RemoteAddr().String(), err)
			conn.Close()
			return
		}
		Client_q <- Client {
			Request: msgText,
			Conn: conn,
			Text: string(readBuf[:n]),
		}
	}
}

func server(Client_q chan Client) {
	clientsOnline := []Client{}
	clientsOffline := []Client{}
	for {
		client := <-Client_q
		switch client.Request {
		case msgConnect:
			log.Printf("Got login request from %s\n", client.Conn.RemoteAddr().String());
			clientsOffline = append(clientsOffline, client)
		case msgText:
			// TODO: This loop can be avoided by adding a `loggedIn` boolean to client
			for index, clientOffline := range clientsOffline {
				if clientOffline.Conn == client.Conn {
					clientOffline.UserName = client.Text;
					log.Printf("logging in %s\n", clientOffline.UserName);
					clientsOnline = append(clientsOnline, clientOffline);
					clientsOffline = slices.Delete(clientsOffline, index, index+1);
				}
			}
			for index, clientOnline := range clientsOnline {
				if clientOnline.Conn == client.Conn {
					continue
				}
				_, err := clientOnline.Conn.Write([]byte(clientOnline.UserName + ": " + client.Text))
				if err != nil {
					log.Printf("Could not send message to client %s\n", clientOnline.UserName)
				}
			}
		}
	}
}

func main() {
	ln, err := net.Listen("tcp", ":"+Port)
	if err != nil {
		log.Fatalf("Could not listen to port %s: %s\n", Port, err)
	}
	Client_q := make(chan Client)
	go server(Client_q)
	for {
		conn, err := ln.Accept()
		if err != nil {
			log.Printf("Could not accept the connection: %s\n", err)
			continue
		}
		log.Printf("Accepted connection from %s", conn.RemoteAddr())
		Client_q <- Client {
			Request: msgConnect,
			Conn: conn,
		}
		go client(conn, Client_q)
	}
}
