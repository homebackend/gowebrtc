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

import "github.com/pion/webrtc/v3"

type OpenRelay struct {
	AppName string `yaml:"app_name"`
	ApiKey  string `yaml:"api_key"`
}

type ICEServer struct {
	URLs           string                   `json:"urls"`
	Username       string                   `json:"username,omitempty"`
	Credential     interface{}              `json:"credential,omitempty"`
	CredentialType webrtc.ICECredentialType `json:"credentialType,omitempty"`
}

type UserCredentials struct {
	User     string `yaml:"user" validate:"required"`
	Password string `yaml:"password" validate:"required"`
}

type TurnConfiguration struct {
	PublicIp string            `yaml:"public_ip" validate:"required"`
	UdpPort  int               `yaml:"port" validate:"required,number,gte=1,lte=65535" default:"8080"`
	Users    []UserCredentials `yaml:"users" validate:"required"`
	Realm    string            `yaml:"realm" default:"default"`
	Threads  int               `yaml:"threads" validate:"required,gte=1,lte=20"`
}

type Configuration struct {
	Port                  int                `yaml:"port" validate:"number,gte=1,lte=65535" default:"8080"`
	Url                   string             `yaml:"url" default:"/stream"`
	ImageWidth            uint               `yaml:"image_width" default:"640"`
	ImageHeight           uint               `yaml:"image_height" default:"480"`
	FrameRate             uint               `yaml:"framerate" default:"30"`
	LogFile               string             `yaml:"log_file" default:"none"`
	AudioDevice           string             `yaml:"audio_device" validate:"required"`
	VideoDevice           string             `yaml:"video_device" validate:"required"`
	Signalling            string             `yaml:"signalling" validate:"oneof=http websocket" default:"websocket"`
	SignallingUsesTls     bool               `yaml:"signalling_uses_tls" default:"false"`
	SignallingTlsCert     string             `yaml:"signalling_tls_cert"`
	SignallingTlsKey      string             `yaml:"signalling_tls_key"`
	SignallingCredentials []UserCredentials  `yaml:"signalling_credentials"`
	SignallingOrigin      string             `yaml:"signalling_origin" default:""`
	IceTrickling          bool               `yaml:"ice_trickling" default:"false"`
	DisconnectOnReconnect bool               `yaml:"disconnect_on_reconnect" default:"false"`
	IceServers            []webrtc.ICEServer `yaml:"ice_servers,omitempty"`
	OpenRelayConfig       *OpenRelay         `yaml:"open_relay_config,omitempty"`
	UseInternalTurn       bool               `yaml:"use_internal_turn" default:"false"`
	TurnConfiguration     *TurnConfiguration `yaml:"turn_configuration"`
}
