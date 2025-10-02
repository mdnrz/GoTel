package main

import (
	"fmt"
	"github.com/jroimartin/gocui"
	"log"
	"net"
	"strings"
)

type Command struct {
	Name        string
	Description string
	Signature   string
	Function    func(*gocui.View, []string) error
}

const commandCnt = 5

var commands [commandCnt]Command
var gui *gocui.Gui
var serverConn net.Conn

const serverAddr = "127.0.0.1:6969"
const initMsg = `This is a client for connecting to GoTel chat server.
=======================================================`

func main() {
	initCommands()
	gui, _ = gocui.NewGui(gocui.OutputNormal)
	// if err != nil {
	// 	log.Panicln(err)
	// }
	defer gui.Close()

	gui.Cursor = true
	gui.Mouse = true
	gui.SetManagerFunc(layout)

	if err := gui.SetKeybinding("", gocui.KeyCtrlC, gocui.ModNone, quit); err != nil {
		log.Panicln(err)
	}

	if err := gui.SetKeybinding("prompt", gocui.KeyEnter, gocui.ModNone, getInput); err != nil {
		log.Panicln(err)
	}

	if err := gui.MainLoop(); err != nil && err != gocui.ErrQuit {
		log.Panicln(err)
	}
}

func layout(g *gocui.Gui) error {
	maxX, maxY := g.Size()
	chatLogW := maxX - 2
	chatLogH := maxY - 10
	promptX1 := 1
	promptY1 := chatLogH + 1
	promptX2 := maxX - 2
	promptY2 := maxY - 2
	if chatLog, err := g.SetView("chatLog", 1, 1, chatLogW, chatLogH); err != nil {
		if err != gocui.ErrUnknownView {
			return err
		}
		chatLog.Wrap = true
		chatLog.Autoscroll = true
		fmt.Fprintln(chatLog, initMsg)
	}
	if prompt, promptErr := g.SetView("prompt", promptX1, promptY1, promptX2, promptY2); promptErr != nil {
		if promptErr != gocui.ErrUnknownView {
			return promptErr
		}
		prompt.Editable = true
		prompt.Wrap = true
		if _, err := g.SetCurrentView("prompt"); err != nil {
			return err
		}
	}
	return nil
}

func getInput(g *gocui.Gui, v *gocui.View) error {
	input := strings.TrimRight(v.Buffer(), "\r\n")
	items := strings.Split(input, " ")
	v.Clear()
	v.SetCursor(0, 0)
	chatLog, _ := g.View("chatLog")
	if strings.HasPrefix(items[0], "/") {
		for _, cmd := range commands {
			if strings.HasPrefix(cmd.Signature, items[0]) {
				cmd.Function(chatLog, items)
				return nil
			}
		}
		fmt.Fprintf(chatLog, "Invalid command: %s\n", items[0])
		return nil
	}
	if serverConn != nil {
		serverConn.Write([]byte(input))
	}
	return nil
}

func initCommands() {
	commands = [commandCnt]Command{

		Command{
			Name:        "help",
			Description: "Print help menu",
			Signature:   "/help [command]",
			Function:    printHelp,
		},

		Command{
			Name:        "join",
			Description: "Join the chat",
			Signature:   "/join <ipv4>:<port> <TOKEN>",
			Function:    sendJoin,
		},

		Command{
			Name:        "signup",
			Description: "Sign up to server",
			Signature:   "/signup <username> <password>",
			Function:    sendSignup,
		},

		Command{
			Name:        "login",
			Description: "Login to server",
			Signature:   "/login <username> <password>",
			Function:    sendLogin,
		},

		Command{
			Name:        "exit",
			Description: "Logout from server",
			Signature:   "/exit",
			Function:    sendQuit,
		},
	}
}

func printHelp(v *gocui.View, input []string) error {
	if len(input) == 1 {
		fmt.Fprintf(v, "Commands:\n")
		for _, cmd := range commands {
			fmt.Fprintf(v, "%s - %s\n", cmd.Signature, cmd.Description)
		}
		return nil
	}
	if len(input) == 2 {
		for _, cmd := range commands {
			if strings.HasPrefix(cmd.Signature, input[1]) {
				fmt.Fprintf(v, "%s - %s\n", cmd.Signature, cmd.Description)
				return nil
			}
		}
		fmt.Fprintf(v, "%s is not a valid command. Type /help to see the full list of commands.\n", input[1])
		return nil
	}
	fmt.Fprintln(v, "Too many arguments for /help command.")
	fmt.Fprintf(v, "%s - %s\n", commands[0].Signature, commands[0].Description)
	return nil
}

func sendJoin(v *gocui.View, input []string) error {
	var err error
	if len(input) != 3 {
		fmt.Fprintf(v, "Invalid join command.\n")
		input := []string{"/help", "/join"}
		printHelp(v, input)
		return nil
	}

	if len(input[2]) != 32 {
		fmt.Fprintf(v, "Invalid token: %s\nThe token should be a 32-character string.\n", input[1])
		return nil
	}

	serverConn, err = net.Dial("tcp", input[1])
	if err != nil {
		fmt.Fprintf(v, "Server is not responding.\n")
		return nil
	}
	_, err = serverConn.Write([]byte(input[0] + " " + input[2]))
	if err != nil {
		fmt.Fprintf(v, "Could not send join request to the server: %s\n", err)
		return nil
	}
	go getMsg(serverConn)
	return nil
}

func sendSignup(v *gocui.View, input []string) error {
	if len(input) != 3 {
		fmt.Fprintf(v, "Invalid signup command.\n")
		input := []string{"/help", "/signup"}
		printHelp(v, input)
		return nil
	}

	_, err := serverConn.Write([]byte(input[0] + " " + input[1] + " " + input[2]))
	if err != nil {
		fmt.Fprintf(v, "Could not send signup request to the server: %s\nTry join command first.", err)
	}
	return nil
}

func sendLogin(v *gocui.View, input []string) error {
	if len(input) != 3 {
		fmt.Fprintf(v, "Invalid login command.\n")
		input := []string{"/help", "/login"}
		printHelp(v, input)
		return nil
	}

	_, err := serverConn.Write([]byte(input[0] + " " + input[1] + " " + input[2]))
	if err != nil {
		fmt.Fprintf(v, "Could not send login request to the server: %s\nTry /join command first.", err)
	}
	return nil
}

func sendQuit(v *gocui.View, input []string) error {
	fmt.Fprintln(v, "Quiting from server")
	serverConn.Close()
	return nil
}

func getMsg(conn net.Conn) {
	readBuf := make([]byte, 512)
	for {
		n, readErr := conn.Read(readBuf)

		if readErr != nil {
			return
		}
		gui.Update(func(g *gocui.Gui) error {
			v, err := gui.View("chatLog")
			if err != nil {
				return err
			}
			fmt.Fprintln(v, string(readBuf[:n]))
			return nil
		})
	}
}

func quit(g *gocui.Gui, v *gocui.View) error {
	return gocui.ErrQuit
}
