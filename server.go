package main

import (
	"crypto/rand"
	"fmt"
	"log"
	"math/big"
	"net"
	"os"
	"strings"
	"time"
	"database/sql"
	_"github.com/mattn/go-sqlite3"
)

type msgType int

const (
	msgConnect msgType = iota + 1
	msgJoin
	msgSignup
	msgLogin
	msgText
	msgQuit
)

var RequestMap map[string]msgType
var userDB *sql.DB

type Msg struct {
	Author Client
	Type   msgType
	Args   []string
}

type loginStage int

const (
	verification loginStage = iota + 1
	username
	password
)

type Client struct {
	Stage       loginStage
	UserName    string
	Password    string
	LastMsgTime time.Time
	Strike      int
	Banned      bool
	BanEnd      time.Time
	Conn        net.Conn
}

const (
	msgCoolDownTimeSec = 1
	banLimit           = 5
	banTimeoutSec      = 180
	Port               = "6969"
)

func initReqMap() {
	RequestMap = make(map[string]msgType)
	RequestMap["/join"] = msgJoin
	RequestMap["/signup"] = msgSignup
	RequestMap["/login"] = msgLogin
}

func clientRoutine(conn net.Conn, Msg_q chan Msg) {
	Msg_q <- Msg{
		Type:         msgConnect,
		Author: Client{
			Conn:  conn,
			Stage: verification,
		},
	}
	readBuf := make([]byte, 512)
	for {
		n, err := conn.Read(readBuf)
		if n == 0 {
			Msg_q <- Msg{
				Type: msgQuit,
				Author: Client{
					Conn: conn,
				},
			}
			return
		}
		if n > 0 {
			if strings.HasPrefix(string(readBuf), "/") {
				items := strings.Split(string(readBuf[:n]), " ")
				Msg_q <- Msg{
					Type: RequestMap[items[0]],
					Args: items[1:len(items)],
					Author: Client{
						Conn: conn,
					},
				}
			} else {
				log.Printf("Got message from %s\n", conn.RemoteAddr().String())
				Msg_q <- Msg{
					Type: msgText,
					Args: []string{string(readBuf[:n])},
					Author: Client{
						Conn: conn,
					},
				}
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
		diff := time.Now().Sub(client.LastMsgTime).Seconds()
		if diff <= msgCoolDownTimeSec {
			client.Strike += 1
			if client.Strike >= banLimit {
				client.Banned = true
				client.BanEnd = time.Now().Add(banTimeoutSec * time.Second)
			}
			return false
		}
		return true
	}
	banTimeRemaining := client.BanEnd.Sub(time.Now()).Seconds()
	if banTimeRemaining >= 0.0 {
		banTimeRemainingStr := fmt.Sprintf("You're banned. Try again in %.0f seconds.\n", banTimeRemaining)
		client.Conn.Write([]byte(banTimeRemainingStr))
		return false
	}
	client.Strike = 0
	client.Banned = false
	return true
}

func checkForDuplicateUN(needle string, heystack map[string]Client) bool {
	for _, client := range heystack {
		if client.UserName == needle {
			return true
		}
	}
	return false
}

func verifyToken(input string) bool {
	tokenBytes := make([]byte, 32)
	tokenFile, err := os.Open("TOKEN")
	if err != nil {
		log.Fatalf("Could not open TOKEN file for authentication: %s\n", err)
	}
	n, err := tokenFile.Read(tokenBytes)
	if err != nil {
		log.Fatalf("Could not read TOKEN file: %s\n", err)
	}
	if n < 32 {
		log.Fatalf("TOKEN file is not valid.\n")
	}
	return input == string(tokenBytes)
}

func trimNewline(input rune) bool {
	if input == '\n' {
		return true
	}
	if input == '\r' {
		return true
	}
	return false
}

func login(client *Client, msg string) bool {
	if client.Stage == verification {
		if !verifyToken(strings.TrimRightFunc(msg, trimNewline)) {
			_, err := client.Conn.Write([]byte("Invalid token.\n"))
			if err != nil {
				log.Printf("Could not send invalid token message to client %s\n", client.Conn.RemoteAddr().String())
			}
			client.LastMsgTime = time.Now()
			return false
		}
		_, err := client.Conn.Write([]byte("Token verified successfully!\nEnter your user name"))
		if err != nil {
			log.Printf("Could not send verification message to %s\n", client.Conn.RemoteAddr().String())
		}
		client.Stage = username
		client.LastMsgTime = time.Now()
		return false
	}
	if client.Stage == username {
		client.UserName = strings.TrimRight(msg, "\r\n")
		client.LastMsgTime = time.Now()
		_, err := client.Conn.Write([]byte("Enter your password:\n"))
		if err != nil {
			log.Printf("Could not send passowrd message to %s\n", client.Conn.RemoteAddr().String())
		}
		client.Stage = password
		return false
	}
	client.Password = strings.TrimRight(msg, "\r\n")
	client.LastMsgTime = time.Now()
	_, err := client.Conn.Write([]byte("Welcome " + client.UserName + "\n"))
	if err != nil {
		log.Printf("Could not send welcome message to %s\n", client.Conn.RemoteAddr().String())
	}
	client.LastMsgTime = time.Now()
	return true
}

func isUserInDB (username string) (bool, error) {
	var count int
	check := "SELECT COUNT(*) FROM users WHERE username = ?"
	err := userDB.QueryRow(check, username).Scan(&count)
	if err != nil {
		return false, err
	}
	return count > 0, nil
}

func addUserToDB (client Client) error {
	insert := "INSERT INTO users (username, password, banned, banEnd) VALUES ($1, $2, $3, $4)"
	_, err := userDB.Exec(insert, client.UserName, client.Password, client.Banned, client.BanEnd)
	return err
}

func checkPassword(username string, inputPass string) (bool, error) {
	var dbPass string
	query := "SELECT password FROM users WHERE username = ?"
	err := userDB.QueryRow(query, username).Scan(&dbPass)
	if err != nil {
		return false, nil
	}
	return dbPass == inputPass, nil
}

func server(Msg_q chan Msg) {
	onlineList := make(map[string]Client)
	joinedList := make(map[string]Client)
	offlineList := make(map[string]Client)
	for {
		msg := <-Msg_q
		keyString := msg.Author.Conn.RemoteAddr().String()
		switch msg.Type {
		case msgConnect:
			// TODO: implement rate limit for connection requests
			log.Printf("Got connection request from %s\n", keyString)
			offlineList[keyString] = msg.Author
			break
		case msgQuit:
			author, ok := onlineList[keyString]
			if ok {
				log.Printf("%s logged out.\n", author.UserName)
				author.Conn.Close()
				delete(onlineList, keyString)
				break
			}
			break

		case msgJoin:
			log.Printf("server: Got join request from %s\n", keyString)
			_, offline := offlineList[keyString]
			_, joined := joinedList[keyString]
			if offline {
				if verifyToken(msg.Args[0]) {
					joinedList[keyString] = msg.Author
					delete(offlineList, keyString)
					msg.Author.Conn.Write([]byte("Authentication successfull."))
					break
				} 
				msg.Author.Conn.Write([]byte("Provided token is not valid."))
				break
			}
			if joined {
				msg.Author.Conn.Write([]byte("You are already joined the server.\nTry logging in or signing up."))
				break
			}
			msg.Author.Conn.Write([]byte("You are currently logged in."))
			break

		case msgSignup:
			_, offline := offlineList[keyString]
			_, joined := joinedList[keyString]
			_, online := onlineList[keyString]
			if online {
				msg.Author.Conn.Write([]byte("This username already exists."))
				break
			}
			if joined {
				yes, err := isUserInDB(msg.Args[0]) 
				if err != nil {
					msg.Author.Conn.Write([]byte("Database error: " + err.Error()))
					break
				}
				if yes {
					msg.Author.Conn.Write([]byte("This username already exists."))
					break
				}
				msg.Author.UserName = msg.Args[0]
				msg.Author.Password = msg.Args[1]
				msg.Author.LastMsgTime = time.Now()
				if err := addUserToDB(msg.Author); err != nil {
					msg.Author.Conn.Write([]byte("Could not signup user. Database error: " + err.Error()))
					break
				}
				onlineList[keyString] = msg.Author
				delete(joinedList, keyString)
				msg.Author.Conn.Write([]byte("Welcome " + msg.Author.UserName))
				break
			}
			if offline {
				msg.Author.Conn.Write([]byte("You should provide the token first with the /join command.\n"))
				break
			}
		case msgLogin:
			_, offline := offlineList[keyString]
			_, joined := joinedList[keyString]
			_, online := onlineList[keyString]
			if online {
				msg.Author.Conn.Write([]byte("You are currently logged in."))
				break
			}
			if offline {
				msg.Author.Conn.Write([]byte("You should provide the token first with the /join command."))
				break
			}
			if joined {
				yes, err := isUserInDB(msg.Args[0]) 
				if err != nil {
					msg.Author.Conn.Write([]byte("Database error: " + err.Error()))
					break
				}
				if !yes {
					msg.Author.Conn.Write([]byte("Username does not exist. You can create new user using /signup command."))
					break
				}
				passOK, err := checkPassword(msg.Args[0], msg.Args[1])
				if err != nil {
					msg.Author.Conn.Write([]byte("Database error: " + err.Error()))
					break
				}
				if !passOK {
					// TODO: limited retry
					msg.Author.Conn.Write([]byte("Incorrect password."))
					break
				}
				msg.Author.UserName = msg.Args[0]
				msg.Author.Password = msg.Args[1]
				msg.Author.LastMsgTime = time.Now()
				onlineList[keyString] = msg.Author
				delete(joinedList, keyString)
				msg.Author.Conn.Write([]byte("Welcome " + msg.Author.UserName))
				break
			}
			break

		case msgText:
			author, online := onlineList[keyString]
			if !online {
				msg.Author.Conn.Write([]byte("You must be logged in to send messages.\n"))
				break
			}
			if !canMessage(&author) {
				onlineList[keyString] = author // update the timings
				break
			}
			author.LastMsgTime = time.Now()
			onlineList[keyString] = author
			for _, client := range onlineList {
				_, err := client.Conn.Write([]byte(author.UserName + ": " + msg.Args[0]))
				if err != nil {
					log.Printf("Could not send message to client %s\n", client.UserName)
				}
			}
		default:
			log.Printf("server: Invalid request\n")
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
		tokenStr = fmt.Sprintf(tokenStr+"%X", randInt)
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
	userDB, _ = sql.Open("sqlite3", "./users.db")
	createTable := `CREATE TABLE IF NOT EXISTS users (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		username VARCHAR(50) UNIQUE NOT NULL,
		password VARCHAR(100) NOT NULL,
		banned BOOLEAN,
		banEnd TIMESTAMP
	);`
	_, err := userDB.Exec(createTable)
	if err != nil {
		log.Fatalf("Error creating users table: %s\n", err)
	}
	log.Printf("Users table created.\n")

	initReqMap()
	genToken()
	ln, err := net.Listen("tcp", ":"+Port)
	if err != nil {
		log.Fatalf("Could not listen to port %s: %s\n", Port, err)
	}
	Msg_q := make(chan Msg)
	go server(Msg_q)
	for {
		conn, err := ln.Accept()
		if err != nil {
			log.Printf("Could not accept the connection: %s\n", err)
			continue
		}
		log.Printf("Accepted connection from %s", conn.RemoteAddr())
		// Msg_q <- Msg{
		// 	Type:         msgConnect,
		// 	Author: Client{
		// 		Conn:  conn,
		// 		Stage: verification,
		// 	},
		// }
		go clientRoutine(conn, Msg_q)
	}
}
