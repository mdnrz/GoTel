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

type Packet struct {
	Type msgType
	Conn net.Conn
	Text string
}

type Client struct {
	userName string
	strike   int
	banned   bool
	bannedAt time.Time
	conn     net.Conn
}

const Port = "6969"

func client(conn net.Conn, packet_q chan Packet) {
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
		packet_q <- Packet{
			Type: msgText,
			Conn: conn,
			Text: string(readBuf[:n]),
		}
	}
}

func login(packet_q chan Packet) {
}

func server(packet_q chan Packet) {
	clientsOnline := []net.Conn{}
	clientsOffline := []net.Conn{}
	for {
		packet := <-packet_q
		switch packet.Type {
		case msgConnect:
			log.Printf("Got login request from %s\n", packet.Conn.RemoteAddr().String());
			clientsOffline = append(clientsOffline, packet.Conn)
		case msgText:
			for index, clientOffline := range clientsOffline {
				if clientOffline == packet.Conn {
					log.Printf("logging in clinet %s\n", packet.Conn.RemoteAddr().String());
					clientsOnline = append(clientsOnline, packet.Conn);
					clientsOffline = slices.Delete(clientsOffline, index, index+1);
				}
			}
			for _, clientOnline := range clientsOnline {
				if clientOnline == packet.Conn {
					continue
				}
				_, err := clientOnline.Write([]byte(packet.Text))
				if err != nil {
					log.Printf("Could not send message to client %s\n", clientOnline.RemoteAddr().String())
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
	packet_q := make(chan Packet)
	go server(packet_q)
	for {
		conn, err := ln.Accept()
		if err != nil {
			log.Printf("Could not accept the connection: %s\n", err)
			continue
		}
		log.Printf("Accepted connection from %s", conn.RemoteAddr())
		packet_q <- Packet{
			Type: msgConnect,
			Conn: conn,
		}
		go client(conn, packet_q)
	}
}
