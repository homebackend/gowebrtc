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
	"context"
	"errors"
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

const (
	readBufferSize  = 1024
	writeBufferSize = 1024
)

var (
	ErrAuthorizationNotDone = errors.New("not authorized")
	ErrEventNotSupported    = errors.New("event type is not supported")
)

type Manager struct {
	clients ClientList
	sync.RWMutex
	handlers          map[string]EventHandler
	websocketUpgrader websocket.Upgrader
	configFile        string
	config            *Configuration
	streaming         bool
	pid               int
	clientConnect     chan *Client
}

func NewManager(ctx context.Context, configFile string, config *Configuration) *Manager {
	m := &Manager{
		clients:  make(ClientList),
		handlers: make(map[string]EventHandler),
		websocketUpgrader: websocket.Upgrader{
			CheckOrigin: func(r *http.Request) bool {
				if config.SignallingOrigin == "" {
					return true
				}

				origin := r.Header.Get("Origin")

				return origin == config.SignallingOrigin
			},
			ReadBufferSize:  readBufferSize,
			WriteBufferSize: writeBufferSize,
		},
		configFile:    configFile,
		config:        config,
		streaming:     false,
		pid:           0,
		clientConnect: make(chan *Client),
	}
	m.setupEventHandlers()
	return m
}

func (m *Manager) setupEventHandlers() {
	m.handlers[EventDisconnect] = DisconnectHandler
}

func (m *Manager) routeEvent(event Event, c *Client) error {
	log.Printf("Event type to be routed: %s\n", event.Type)
	if !c.authorized {
		if event.Type == EventConnect {
			if err := ConnectHandler(event, c); err != nil {
				return err
			} else {
				return nil
			}
		} else {
			return ErrAuthorizationNotDone
		}
	}

	if handler, ok := m.handlers[event.Type]; ok {
		if err := handler(event, c); err != nil {
			return err
		}
		return nil
	} else {
		return ErrEventNotSupported
	}
}

func (m *Manager) serveWS(w http.ResponseWriter, r *http.Request) {
	log.Println("New connection")
	conn, err := m.websocketUpgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Println(err)
		return
	}
	log.Println("Connection upgrade done")

	client := NewClient(conn, m)
	m.addClient(client)

	go client.readMessages()
	go client.writeMessages()
}

func (m *Manager) processConnection() {
	ticker := time.NewTicker(pingInterval)
	defer func() {
		log.Println("Exiting process connection")
		ticker.Stop()
	}()

	for {
		select {
		case c := <-m.clientConnect:
			log.Println("Handling streaming request")
			HandleStreamingRequest(m.configFile, m.config, &m.streaming, &m.pid, c.sdp,
				func(answer string) {
					log.Printf("Answer: %s\n", answer)
					c.egress <- GetAnswerEvent(answer)
				}, func(candidate string) {
					log.Printf("Candidate: %s\n", candidate)
					c.egress <- GetNewCandidateEvent(candidate)
				}, func(error string) {
					log.Printf("Error: %s\n", error)
					m.removeClient(c)
				})
		case <-ticker.C:
			for c := range m.clients {
				if c.hasAuthTimedOut() {
					m.removeClient(c)
				}
			}
		}
	}
}

func (m *Manager) addClient(client *Client) {
	m.Lock()
	defer m.Unlock()

	m.clients[client] = true
}

func (m *Manager) removeClient(client *Client) {
	m.Lock()
	defer m.Unlock()

	if _, ok := m.clients[client]; ok {
		client.connection.Close()
		delete(m.clients, client)
	}
}
