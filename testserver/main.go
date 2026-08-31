package main

import (
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/gorilla/websocket"
)

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
}

func main() {
	name := os.Getenv("SERVER_NAME")

	if name == "" {
		name = "unknown"
	}

	port := os.Getenv("PORT")

	if port == "" {
		port = "9001"
	}

	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w, "Hello from %s\n", name)
	})

	http.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		fmt.Fprintln(w, "OK")
	})

	http.HandleFunc("/slow", func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(5 * time.Second)
		fmt.Fprintf(w, "Slow response from %s\n", name)
	})

	http.HandleFunc("/ws/game/", func(w http.ResponseWriter, r *http.Request) {
		gameID := strings.TrimPrefix(r.URL.Path, "/ws/game/")

		if gameID == "" {
			http.Error(
				w,
				"missing game ID",
				http.StatusBadRequest,
			)
			return
		}

		conn, err := upgrader.Upgrade(w, r, nil)

		if err != nil {
			log.Printf(
				"WebSocket upgrade failed: %v",
				err,
			)
			return
		}

		defer conn.Close()

		log.Printf(
			"WebSocket game=%s connected to %s",
			gameID,
			name,
		)

		for {
			messageType, message, err := conn.ReadMessage()

			if err != nil {
				log.Printf(
					"WebSocket game=%s closed on %s: %v",
					gameID,
					name,
					err,
				)
				return
			}

			response := fmt.Sprintf(
				"%s game=%s received: %s",
				name,
				gameID,
				string(message),
			)

			if err := conn.WriteMessage(
				messageType,
				[]byte(response),
			); err != nil {
				return
			}
		}
	})

	log.Printf("%s listening on :%s", name, port)
	log.Fatal(http.ListenAndServe(":"+port, nil))
}
