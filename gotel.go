package main

import (
	"log"
	"net"
	"time"
	"strings"
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

func addClient(conn net.Conn, Client_q chan Client) {
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
	clientsOnline := make(map[string]Client)
	clientsOffline := make(map[string]Client)
	for {
		client := <-Client_q
		keyString := client.Conn.RemoteAddr().String();
		switch client.Request {
		case msgConnect:
			log.Printf("Got login request from %s\n", keyString);
			clientsOffline[keyString] = client;
		case msgText:
			clientOffline, ok := clientsOffline[keyString];
			if ok {
				clientOffline.UserName = strings.TrimRight(client.Text, "\r\n");
				log.Printf("logging in %s\n", clientOffline.UserName);
				clientsOnline[keyString] = clientOffline;
				delete(clientsOffline, keyString);
				_, err := clientsOnline[keyString].Conn.Write([]byte("Welcome " + clientsOnline[keyString].UserName + "\n\n"));
				if err != nil {
					log.Printf("Could not send message to client %s\n", clientsOnline[keyString].UserName)
				}
				break;
			}
			author, _ := clientsOnline[keyString];
			author.Text = client.Text;
			for _, value := range clientsOnline {
				if value.Conn == author.Conn {
					continue
				}
				_, err := value.Conn.Write([]byte(author.UserName + ": " + author.Text))
				if err != nil {
					log.Printf("Could not send message to client %s\n", value.UserName)
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
		go addClient(conn, Client_q)
	}
}
