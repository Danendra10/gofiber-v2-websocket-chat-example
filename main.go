package main

import (
	"flag"
	"log"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/websocket/v2"
)

func main() {
	app := fiber.New()

	app.Static("/", "./index.html")

	app.Use("/ws", func(c *fiber.Ctx) error {
		if websocket.IsWebSocketUpgrade(c) {
			c.Locals("allowed", true)
			return c.Next()
		}
		return fiber.ErrUpgradeRequired
	})

	go runHub()

	app.Get("/ws/:username", websocket.New(func(c *websocket.Conn) {
		username := c.Params("username")

		defer func() {
			unregister <- c
			c.Close()
		}()

		register <- client{Conn: c, Name: username}

		for {
			messageType, message, err := c.ReadMessage()
			if err != nil {
				if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
					log.Println("read error:", err)
				}

				return
			}

			if messageType == websocket.TextMessage {
				broadcast <- messageData{Username: username, Message: string(message)}
			} else {
				log.Println("websocket message received of type", messageType)
			}
		}
	}))

	port := flag.String("port", ":8888", "Address to listen on")
	flag.Parse()
	app.Listen(*port)
}

type client struct {
	Conn *websocket.Conn
	Name string
}

type messageData struct {
	Username string
	Message  string
}

var clients = make(map[*websocket.Conn]client)
var register = make(chan client)
var broadcast = make(chan messageData)
var unregister = make(chan *websocket.Conn)

func runHub() {
	for {
		select {
		case connection := <-register:
			clients[connection.Conn] = connection
			log.Println("connection registered")

		case message := <-broadcast:
			log.Println("message received:", message.Message)

			for connection := range clients {
				if err := connection.WriteMessage(websocket.TextMessage, []byte(message.Username+": "+message.Message)); err != nil {
					log.Println("write error:", err)

					unregister <- connection
					connection.WriteMessage(websocket.CloseMessage, []byte{})
					connection.Close()
				}
			}

		case connection := <-unregister:
			delete(clients, connection)
			log.Println("connection unregistered")
		}
	}
}
