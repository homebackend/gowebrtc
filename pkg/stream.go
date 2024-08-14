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
	"fmt"
	"io"
	"log"
	"net/http"
	"os"

	"github.com/go-gst/go-gst/gst"
	"github.com/go-gst/go-gst/gst/app"
	"github.com/pion/interceptor"
	"github.com/pion/webrtc/v3"
	"github.com/pion/webrtc/v3/pkg/media"
)

type CandidateHandler func(i *webrtc.ICECandidate, peerConnection *webrtc.PeerConnection)
type EndHandler func(peerConnection *webrtc.PeerConnection)

func StartStreaming(conf *Configuration, videoSrc, audioSrc, sdpFile string, wait bool) {
	startExecuting(conf, videoSrc, audioSrc, sdpFile, wait)
}

func getWebrtcPeerConfiguration(conf *Configuration) (*webrtc.PeerConnection, error) {
	config := webrtc.Configuration{}
	if conf.UseInternalTurn {
		if conf.TurnConfiguration.TurnType == TurnInternal {
			config.ICEServers = make([]webrtc.ICEServer, 2*len(conf.TurnConfiguration.Users))

			for i, user := range conf.TurnConfiguration.Users {
				config.ICEServers[2*i].URLs = make([]string, 1)
				config.ICEServers[2*i].URLs[0] = fmt.Sprintf("stun:%s:%d", "127.0.0.1", conf.TurnConfiguration.UdpPort)
				config.ICEServers[2*i].Username = user.User
				config.ICEServers[2*i].Credential = user.Password
				config.ICEServers[2*i+1].URLs = make([]string, 1)
				config.ICEServers[2*i+1].URLs[0] = fmt.Sprintf("turn:%s:%d", "127.0.0.1", conf.TurnConfiguration.UdpPort)
				config.ICEServers[2*i+1].Username = user.User
				config.ICEServers[2*i+1].Credential = user.Password
			}
		} else {
			m := &webrtc.MediaEngine{}
			if err := m.RegisterDefaultCodecs(); err != nil {
				return nil, err
			}

			i := &interceptor.Registry{}
			if err := webrtc.RegisterDefaultInterceptors(m, i); err != nil {
				return nil, err
			}

			s := webrtc.SettingEngine{}
			s.SetNAT1To1IPs([]string{conf.TurnConfiguration.PublicIp}, webrtc.ICECandidateTypeSrflx)

			config.ICEServers = make([]webrtc.ICEServer, 1)
			config.ICEServers[0].URLs = make([]string, 1)
			config.ICEServers[0].URLs[0] = "stun:stun.l.google.com:19302"

			api := webrtc.NewAPI(webrtc.WithMediaEngine(m), webrtc.WithInterceptorRegistry(i), webrtc.WithSettingEngine(s))
			return api.NewPeerConnection(config)
		}
	} else if conf.OpenRelayConfig != nil {
		fmt.Println("Found Open Relay Config")
		url := fmt.Sprintf("https://%s/api/v1/turn/credentials?apiKey=%s", conf.OpenRelayConfig.AppName, conf.OpenRelayConfig.ApiKey)
		fmt.Printf("Url: %s\n", url)
		response, err := http.Get(url)
		if err != nil {
			log.Fatalln(err)
		}

		responseData, err := io.ReadAll(response.Body)
		if err != nil {
			log.Fatalln(err)
		}

		var iceServers []ICEServer
		if err := json.Unmarshal(responseData, &iceServers); err != nil {
			log.Fatalln(err)
		}

		config.ICEServers = make([]webrtc.ICEServer, len(iceServers))
		for i, iceServer := range iceServers {
			config.ICEServers[i].URLs = make([]string, 1)
			config.ICEServers[i].URLs[0] = iceServer.URLs
			config.ICEServers[i].Username = iceServer.Username
			config.ICEServers[i].Credential = iceServer.Credential
			config.ICEServers[i].CredentialType = iceServer.CredentialType
		}
	} else if len(conf.IceServers) > 0 {
		fmt.Println("Found ICE Servers")
		config.ICEServers = conf.IceServers
	} else {
		fmt.Println("Using default Ice Servers")
		config.ICEServers = make([]webrtc.ICEServer, 1)
		config.ICEServers[0].URLs = make([]string, 1)
		config.ICEServers[0].URLs[0] = "stun:stun.l.google.com:19302"
	}

	fmt.Printf("Webrtc config: %v\n", config)
	return webrtc.NewPeerConnection(config)
}

func printAnswer(answer webrtc.SessionDescription) {
	localDescription := encode(answer)
	fmt.Println(ANSWER + localDescription)
}

func startExecuting(conf *Configuration, videoSrc, audioSrc, sdpFile string, wait bool) {
	gst.Init(nil)

	peerConnection, err := getWebrtcPeerConfiguration(conf)
	if err != nil {
		log.Fatalln(err)
	}

	defer func() {
		peerConnection.Close()
	}()

	peerConnection.OnICEConnectionStateChange(func(connectionState webrtc.ICEConnectionState) {
		fmt.Printf("Connection State has changed %s \n", connectionState.String())
		if connectionState == webrtc.ICEConnectionStateFailed {
			log.Fatalln("Exiting as connection could not be established")
		}
	})

	audioTrack, err := webrtc.NewTrackLocalStaticSample(webrtc.RTPCodecCapability{MimeType: "audio/opus"}, "audio", "pion1")
	if err != nil {
		log.Fatalln(err)
	}
	_, err = peerConnection.AddTrack(audioTrack)
	if err != nil {
		log.Fatalln(err)
	}

	// Create a video track
	videoTrack, err := webrtc.NewTrackLocalStaticSample(webrtc.RTPCodecCapability{MimeType: "video/vp8"}, "video", "pion2")
	if err != nil {
		log.Fatalln(err)
	}
	_, err = peerConnection.AddTrack(videoTrack)
	if err != nil {
		log.Fatalln(err)
	}

	buf, err := os.ReadFile(sdpFile)
	if err != nil {
		log.Fatalln(err)
	}

	offer := webrtc.SessionDescription{}
	decode(string(buf), &offer)

	err = peerConnection.SetRemoteDescription(offer)
	if err != nil {
		log.Fatalln(err)
	}

	peerConnection.OnICECandidate(func(i *webrtc.ICECandidate) {
		if wait {
			if i == nil {
				fmt.Println(EOF)
			} else {
				fmt.Printf("Gathered candidate: %s\n", i.String())
				if c, err := json.Marshal(i.ToJSON()); err == nil {
					fmt.Println(CANDIDATE + string(c))
				} else {
					log.Fatalln(err)
				}
			}
		} else {
			if i == nil {
				fmt.Println("All candidates have been gathered")
				printAnswer(*peerConnection.LocalDescription())
				fmt.Println(EOF)
			}
		}
	})

	if answer, err := peerConnection.CreateAnswer(nil); err != nil {
		log.Fatalln(err)
	} else {
		if wait {
			printAnswer(answer)
		}
		peerConnection.SetLocalDescription(answer)
	}

	pipelineForCodec("opus", []*webrtc.TrackLocalStaticSample{audioTrack}, audioSrc)
	pipelineForCodec("vp8", []*webrtc.TrackLocalStaticSample{videoTrack}, videoSrc)

	select {}
}

// Create the appropriate GStreamer pipeline depending on what codec we are working with
func pipelineForCodec(codecName string, tracks []*webrtc.TrackLocalStaticSample, pipelineSrc string) {
	pipelineStr := "appsink name=appsink"
	switch codecName {
	case "vp8":
		pipelineStr = pipelineSrc + " ! vp8enc error-resilient=partitions keyframe-max-dist=10 auto-alt-ref=true cpu-used=5 deadline=1 ! " + pipelineStr
	case "vp9":
		pipelineStr = pipelineSrc + " ! vp9enc ! " + pipelineStr
	case "h264":
		pipelineStr = pipelineSrc + " ! video/x-raw,format=I420 ! x264enc speed-preset=ultrafast tune=zerolatency key-int-max=20 ! video/x-h264,stream-format=byte-stream ! " + pipelineStr
	case "opus":
		pipelineStr = pipelineSrc + " ! opusenc ! " + pipelineStr
	case "pcmu":
		pipelineStr = pipelineSrc + " ! audio/x-raw, rate=8000 ! mulawenc ! " + pipelineStr
	case "pcma":
		pipelineStr = pipelineSrc + " ! audio/x-raw, rate=8000 ! alawenc ! " + pipelineStr
	default:
		log.Fatalln("Unhandled codec " + codecName) //nolint
	}

	log.Println(pipelineStr)
	pipeline, err := gst.NewPipelineFromString(pipelineStr)
	if err != nil {
		log.Fatalln(err)
	}

	if err = pipeline.SetState(gst.StatePlaying); err != nil {
		log.Fatalln(err)
	}

	appSink, err := pipeline.GetElementByName("appsink")
	if err != nil {
		log.Fatalln(err)
	}

	app.SinkFromElement(appSink).SetCallbacks(&app.SinkCallbacks{
		NewSampleFunc: func(sink *app.Sink) gst.FlowReturn {
			sample := sink.PullSample()
			if sample == nil {
				return gst.FlowEOS
			}

			buffer := sample.GetBuffer()
			if buffer == nil {
				return gst.FlowError
			}

			samples := buffer.Map(gst.MapRead).Bytes()
			defer buffer.Unmap()

			for _, t := range tracks {
				if err := t.WriteSample(media.Sample{Data: samples, Duration: *buffer.Duration().AsDuration()}); err != nil {
					log.Fatalln(err) //nolint
				}
			}

			return gst.FlowOK
		},
	})
}
