/*
 Copyright (c) 2024 Neeraj Jakhar

 This program is free software: you can redistribute it and/or modify
 it under the terms of the GNU General Public License as published by
 the Free Software Foundation, either version 3 of the License, or
 (at your option) any later version.

 This program is distributed in the hope that it will be useful,
 but WITHOUT ANY WARRANTY; without even the implied warranty of
 MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
 GNU General Public License for more details.

 You should have received a copy of the GNU General Public License
 along with this program. If not, see <https://www.gnu.org/licenses/>.
*/

package main

import (
	"encoding/json"
	"log"
	"time"

	"github.com/gorilla/websocket"
)

const (
	clientReadLimit = 512
	pongWait        = 10 * time.Second
	pingInterval    = (pongWait * 9) / 10
)

type ClientList map[*Client]bool

type Client struct {
	authorized   bool
	authDeadline time.Time
	sdp          string
	connection   *websocket.Conn
	manager      *Manager
	egress       chan Event
}

func NewClient(conn *websocket.Conn, manager *Manager) *Client {
	return &Client{
		authorized:   false,
		connection:   conn,
		manager:      manager,
		egress:       make(chan Event),
		authDeadline: time.Now().Add(time.Second * time.Duration(60)),
	}
}

func (c *Client) hasAuthTimedOut() bool {
	return time.Now().After(c.authDeadline)
}

func (c *Client) readMessages() {
	defer func() {
		log.Println("Exiting read message")
		c.manager.removeClient(c)
	}()

	if err := c.connection.SetReadDeadline(time.Now().Add(pongWait)); err != nil {
		log.Println(err)
		return
	}

	c.connection.SetPongHandler(func(pongMsg string) error {
		log.Println("Received pong message")
		return c.connection.SetReadDeadline(time.Now().Add(pongWait))
	})

	log.Println("Entering client read loop")

	for {
		_, payload, err := c.connection.ReadMessage()
		if err != nil {
			log.Printf("Error reading message: %v\n", err)
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				log.Printf("error reading message: %v", err)
			}
			break
		}

		log.Println("Payload received")

		var request Event
		if err := json.Unmarshal(payload, &request); err != nil {
			log.Printf("Error processing event: %v", err)
			break
		}

		if err := c.manager.routeEvent(request, c); err != nil {
			log.Println("Error processing event payload: ", err)
		}
	}
}

func (c *Client) writeMessages() {
	ticker := time.NewTicker(pingInterval)
	defer func() {
		log.Println("Exiting write message")
		ticker.Stop()
		c.manager.removeClient(c)
	}()

	log.Println("Entering client write loop")

	for {
		select {
		case message, ok := <-c.egress:
			if !ok {
				log.Println("connection closed")
				if err := c.connection.WriteMessage(websocket.CloseMessage, nil); err != nil {
					log.Println("connection closed: ", err)
				}
				return
			}

			log.Println("Egress event")
			data, err := json.Marshal(message)
			if err != nil {
				// This should never happen
				log.Println(err)
				return
			}

			log.Println("Sending message")
			if err := c.connection.WriteMessage(websocket.TextMessage, data); err != nil {
				log.Println(err)
			}
		case <-ticker.C:
			log.Println("Sending ping message")
			if err := c.connection.WriteMessage(websocket.PingMessage, []byte{}); err != nil {
				log.Println(err)
				return
			}
		}
	}
}
