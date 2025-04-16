package main

import (
	"os"
	"math/big"
	"crypto/rand"
	"log"
	"net"
	"time"
	"strings"
	"fmt"
)

var commands [3]string = [3]string{"/help", "/login", "/quit"};

type msgType int

const (
	msgConnect msgType = iota + 1
	msgLogin
	msgText
	msgQuit
)

type Client struct {
	UserName string
	LastMsgTime time.Time
	Request msgType
	Strike int
	Banned   bool
	BanEnd time.Time
	Conn     net.Conn
	Text string
}

const (
	msgCoolDownTimeSec = 1
	banLimit = 5
	banTimeoutSec = 180
	Port = "6969"
)

func addClient(conn net.Conn, Client_q chan Client) {
	// loginPrompt := "Who are you?\n> "
	// _, err := conn.Write([]byte(loginPrompt))
	// if err != nil {
	// 	log.Printf("[ERROR] Could not send login prompt to user %s: %s\n",
	// 		conn.RemoteAddr().String(), err)
	// }
	readBuf := make([]byte, 512)
	for {
		n, err := conn.Read(readBuf)
		if n == 0 {
			Client_q <- Client {
				Request: msgQuit,
				Conn: conn,
			}
			return;
		}
		if n > 0 {
			Client_q <- Client {
				Request: msgText,
				Conn: conn,
				Text: string(readBuf[:n]),
			}
		}
		if err != nil {
			log.Printf("Could not read message from client %s: %s\n", conn.RemoteAddr().String(), err)
			conn.Close()
			return
		}
	}
}

func canMessage(client *Client) bool {
	if !client.Banned {
		diff := time.Now().Sub(client.LastMsgTime).Seconds();
		if diff <= msgCoolDownTimeSec {
			client.Strike += 1;
			if client.Strike >= banLimit {
				client.Banned = true;
				client.BanEnd = time.Now().Add(banTimeoutSec * time.Second)
			}
			return false;
		}
		return true
	}
	banTimeRemaining := client.BanEnd.Sub(time.Now()).Seconds();
	if  banTimeRemaining >= 0.0 {
		banTimeRemainingStr := fmt.Sprintf("You're banned. Try again in %.0f seconds.\n", banTimeRemaining)
		client.Conn.Write([]byte(banTimeRemainingStr));
		return false;
	} 
	client.Strike = 0;
	client.Banned = false;
	return true;
}

func checkForDuplicateUN(needle string, heystack map[string]Client) bool {
	for _, client := range heystack {
		if client.UserName == needle { return true }
	}
	return false;
}

func server(Client_q chan Client) {
	clientsOnline := make(map[string]Client)
	clientsOffline := make(map[string]Client)
	for {
		client := <-Client_q
		keyString := client.Conn.RemoteAddr().String();
		switch client.Request {
		case msgConnect:
			// TODO: implement rate limit for connection requests
			log.Printf("Got login request from %s\n", keyString);
			clientsOffline[keyString] = client;
		case msgQuit:
			author, ok := clientsOnline[keyString]
			if ok {
				log.Printf("%s logged out.\n", author.UserName);
				author.Conn.Close();
				delete(clientsOnline, keyString);
			}
		case msgText:
			clientOffline, ok := clientsOffline[keyString];
			if ok {
				if !canMessage(&clientOffline) {
					clientsOffline[keyString] = clientOffline
					break;
				}
				clientOffline.UserName = strings.TrimRight(client.Text, "\r\n");
				clientOffline.LastMsgTime = time.Now();
				clientsOffline[keyString] = clientOffline
				if checkForDuplicateUN(clientOffline.UserName, clientsOnline) {
					_, err := clientsOffline[keyString].Conn.Write([]byte("UserName already exists; Try something else."));
					if err != nil {
						log.Printf("Could not send message to client %s\n", keyString)
					}
					break;
				}
				log.Printf("logging in %s\n", clientOffline.UserName);
				clientsOnline[keyString] = clientOffline;
				delete(clientsOffline, keyString);
				_, err := clientsOnline[keyString].Conn.Write([]byte("Welcome " + clientsOnline[keyString].UserName + "\n"));
				if err != nil {
					log.Printf("Could not send message to client %s\n", clientsOnline[keyString].UserName)
				}
				break;
			}

			author, ok := clientsOnline[keyString];
			if !ok {
				log.Fatal("cannot find client\n");
			}
			if !canMessage(&author) {
				clientsOnline[keyString] = author
				break;
			}
			author.LastMsgTime = time.Now()
			author.Text = client.Text;
			clientsOnline[keyString] = author
			for _, value := range clientsOnline {
				// if value.Conn == author.Conn {
				// 	continue
				// }
				_, err := value.Conn.Write([]byte(author.UserName + ": " + author.Text))
				if err != nil {
					log.Printf("Could not send message to client %s\n", value.UserName)
				}
			}
		}
	}
}

func genToken() {
	max := big.NewInt(0xF)
	var randInt *big.Int
	var err error
	var tokenStr string
	for range [32]int{} {
		randInt, err = rand.Int(rand.Reader, max)
		if err != nil {
			log.Fatalf("Could not generate random number: %s\n", err)
		}
		tokenStr = fmt.Sprintf(tokenStr + "%X", randInt)
	}
	tokenFile, err := os.Create("TOKEN")
	if err != nil {
		log.Fatalf("Could not create token file: %s\n", err)
	}
	_, err = tokenFile.WriteString(tokenStr)
	if err != nil {
		log.Fatalf("Could not write token file: %s\n", err)
	}
}

func main() {
	genToken()
	return;
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
