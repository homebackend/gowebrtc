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
	"errors"
	"fmt"
	"log"

	"github.com/pion/webrtc/v3"
)

var (
	ErrorInvalidCredentials = errors.New("invalid credentials")
	ErrorUnauthorized       = errors.New("unauthorized")
)

type Event struct {
	Type    string          `json:"type" validate:"oneof:connect candidate disconnect"`
	Payload json.RawMessage `json:"payload"`
}

type EventHandler func(event Event, c *Client) error

const (
	EventConnect      = "connect"
	EventAnswer       = "answer"
	EventNewCandidate = "new_candidate"
	EventDisconnect   = "disconnect"
)

type ConnectEvent struct {
	SDP      string `json:"sdp" validate:"required"`
	User     string `json:"user"`
	Password string `json:"password"`
}

type AnswerEvent struct {
	Answer string `json:"answer" validate:"required"`
}

type DisconnectEvent struct {
	Message string `json:"message" validate:"required"`
}

type NewCandidateEvent struct {
	Candidate webrtc.ICECandidate `json:"candidate" validate:"required"`
}

func ConnectHandler(event Event, c *Client) error {
	if c.authorized {
		log.Println("Already authorized")
		return nil
	}

	var connectEvent ConnectEvent
	if err := json.Unmarshal(event.Payload, &connectEvent); err != nil {
		return fmt.Errorf("invalid connect request: %v", err)
	}

	if len(c.manager.config.SignallingCredentials) == 0 {
		log.Println("No signalling credentials: authorized")
		c.authorized = true
	}

	for _, credential := range c.manager.config.SignallingCredentials {
		if credential.User == connectEvent.User && credential.Password == connectEvent.Password {
			c.authorized = true
			log.Printf("Credential match success for: %s\n", connectEvent.User)
			break
		}
	}

	if !c.authorized {
		log.Printf("Authorization failure for: %s\n", connectEvent.User)
		c.egress <- GetDisconnectEvent(ErrorInvalidCredentials.Error())
		return ErrorInvalidCredentials
	} else {
		c.sdp = connectEvent.SDP
		c.manager.clientConnect <- c
		return nil
	}
}

func DisconnectHandler(event Event, c *Client) error {
	if !c.authorized {
		return ErrorUnauthorized
	}

	c.manager.removeClient(c)
	return nil
}

func GetDisconnectEvent(message string) Event {
	var disconnectEvent DisconnectEvent
	disconnectEvent.Message = message

	var event Event
	event.Type = EventDisconnect
	if payload, err := json.Marshal(disconnectEvent); err != nil {
		log.Fatalln(err)
	} else {
		event.Payload = payload
	}

	log.Println(event)
	return event
}

func GetAnswerEvent(answer string) Event {
	var answerEvent AnswerEvent
	answerEvent.Answer = answer

	var event Event
	event.Type = EventAnswer
	if payload, err := json.Marshal(answerEvent); err != nil {
		log.Fatalln(err)
	} else {
		event.Payload = payload
	}

	log.Println(event)
	return event
}

func GetNewCandidateEvent(c string) Event {
	var candidate webrtc.ICECandidate

	if err := json.Unmarshal([]byte(c), &candidate); err != nil {
		log.Fatalln(err)
	}

	var candidateEvent NewCandidateEvent
	candidateEvent.Candidate = candidate

	var event Event
	event.Type = EventNewCandidate
	if payload, err := json.Marshal(candidateEvent); err != nil {
		log.Fatalln(err)
	} else {
		event.Payload = payload
	}

	log.Println(event)
	return event
}
