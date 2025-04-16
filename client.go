package main

import (
	"fmt"
	"net"
	"log"
	"strings"

	"github.com/jroimartin/gocui"
)

type Command struct {
	Name string
	Description string
	Signature string
	Function func(*gocui.View, string) error
}

const commandCnt = 3
var commands [commandCnt]Command
var gui *gocui.Gui
var serverConn net.Conn

const serverAddr = "127.0.0.1:6969"
const initMsg =
`This is a client for connecting to GoTel chat server.
=======================================================`

func main() {
	initCommands();
	gui, _ = gocui.NewGui(gocui.OutputNormal)
	// if err != nil {
	// 	log.Panicln(err)
	// }
	defer gui.Close()

	gui.Cursor = true;
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
		fmt.Fprintln(chatLog, initMsg)
	}
	if prompt, promptErr := g.SetView("prompt", promptX1, promptY1, promptX2, promptY2); promptErr != nil {
		if promptErr != gocui.ErrUnknownView {
			return promptErr
		}
		prompt.Editable = true;
		prompt.Wrap = true;
		if _,err := g.SetCurrentView("prompt"); err != nil {
			return err;
		}
	}
	return nil
}

func getCommandArg(items []string) string {
	if len(items) >= 2 {
		return items[1]
	}
	return ""
}

func getInput(g *gocui.Gui, v *gocui.View) error {
	input := strings.TrimRight(v.Buffer(), "\r\n");
	items := strings.Split(input, " ")
	v.Clear();
	v.SetCursor(0, 0);
	chatLog, _ := g.View("chatLog");
	for _, cmd := range commands {
		if strings.HasPrefix(cmd.Signature, items[0]) {
			cmd.Function(chatLog, getCommandArg(items));
			return nil;
		}
	}
	// fmt.Fprintf(chatLog, "you entered: %s\n", input);
	serverConn.Write([]byte(input))
	return nil;
}

func initCommands() {
	commands = [commandCnt]Command {

		Command {
			Name: "help",
			Description: "Print help menu",
			Signature: "/help",
			Function: printHelp,
		},

		Command {
			Name: "join",
			Description: "Join the chat",
			Signature: "/join <Token>",
			Function: sendJoin,
		},

		Command {
			Name: "quit",
			Description: "Logout from server",
			Signature: "/quit",
			Function: sendQuit,
		},
	}
}

func printHelp(v *gocui.View, input string) error {
	for _, cmd := range commands {
		fmt.Fprintf(v, "%s - %s\n", cmd.Signature, cmd.Description)
	}
	return nil
}

func sendJoin(v *gocui.View, input string) error {
	serverConn, _ = net.Dial("tcp", serverAddr);
	// if err != nil {
	// 	return err
	// }
	_, err := serverConn.Write([]byte(input));
	if err != nil {
		return err
	}
	go getMsg(serverConn)
	return nil
}

func sendQuit (v *gocui.View, input string) error {
	fmt.Fprintln(v, "Quiting from server");
	serverConn.Close()
	return nil
}

func getMsg (conn net.Conn) {
	readBuf := make([]byte, 512)
	for {
		n, readErr := conn.Read(readBuf);

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
