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
	"bufio"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/exec"
	"strings"
	"syscall"
	"time"

	"github.com/akamensky/argparse"
	"github.com/gin-gonic/gin"
	"github.com/go-gst/go-gst/gst"
	"github.com/go-gst/go-gst/gst/app"
	homecommon "github.com/homebackend/go-homebackend-common/pkg"
	"github.com/pion/webrtc/v3"
	"github.com/pion/webrtc/v3/pkg/media"
)

const (
	CONF_FILE = "/etc/gowebrtc/config.yaml"
	ANSWER    = "Answer: "
)

type Request struct {
	SDP string `json:"sdp"`
}

type Response struct {
	SDP string `json:"sdp"`
}

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

type Configuration struct {
	Port            int                `yaml:"port" validate:"number,gte=1,lte=65535" default:"8080"`
	Url             string             `yaml:"url" default:"/stream"`
	ImageWidth      uint               `yaml:"image_width" default:"640"`
	ImageHeight     uint               `yaml:"image_height" default:"480"`
	FrameRate       uint               `yaml:"framerate" default:"30"`
	LogFile         string             `yaml:"log_file" default:"none"`
	AudioDevice     string             `yaml:"audio_device" validate:"required"`
	VideoDevice     string             `yaml:"video_device" validate:"required"`
	IceServers      []webrtc.ICEServer `yaml:"ice_servers,omitempty"`
	OpenRelayConfig *OpenRelay         `yaml:"open_relay_config,omitempty"`
}

// Encode encodes the input in base64
func encode(obj interface{}) string {
	b, err := json.Marshal(obj)
	if err != nil {
		log.Fatalln(err)
	}

	return base64.StdEncoding.EncodeToString(b)
}

func decode(in string, obj interface{}) {
	b, err := base64.StdEncoding.DecodeString(in)
	if err != nil {
		log.Fatalln(err)
	}

	err = json.Unmarshal(b, obj)
	if err != nil {
		log.Fatalln(err)
	}
}

func main() {
	parser := argparse.NewParser(os.Args[0], "Setup Webrtc for video streaming from local camera")

	serverCommand := parser.NewCommand("server", "Start webrtc service")
	executeCommand := parser.NewCommand("execute", "Execute webrtc streaming")

	c := parser.String("c", "configuration-file", &argparse.Options{
		Required: false,
		Default:  CONF_FILE,
		Help:     "Configuration File",
	})

	a := executeCommand.String("a", "audio-pipeline", &argparse.Options{
		Required: true,
		Help:     "GStreamer audio pipeline to use",
	})

	v := executeCommand.String("v", "video-pipeline", &argparse.Options{
		Required: true,
		Help:     "GStreamer video pipeline to use",
	})

	s := executeCommand.String("s", "sdp-file", &argparse.Options{
		Required: true,
		Help:     "File containing SDP data",
	})

	err := parser.Parse(os.Args)
	if err != nil {
		fmt.Print(parser.Usage(err))
		os.Exit(1)
	}

	config := homecommon.GetConf[Configuration](*c)

	if serverCommand.Happened() {
		streaming := false
		pid := 0

		f := setupLogging(config.LogFile)
		if f != nil {
			defer f.Close()
		}

		var htmldir string
		if _, err := os.Stat("./html"); err == nil {
			htmldir = "./html"
		} else {
			htmldir = "/usr/share/gowebrtc/html"
		}

		gin.DisableConsoleColor()
		gin.DefaultWriter = log.Writer()

		router := gin.Default()
		router.Static("/", htmldir)
		router.POST(config.Url, createStream(*c, config, &streaming, &pid))
		router.DELETE(config.Url, deleteStream(&streaming, &pid))
		router.Run(fmt.Sprintf("0.0.0.0:%d", config.Port))
	} else if executeCommand.Happened() {
		startExecuting(config, *v, *a, *s)
	}
}

func deleteStream(streaming *bool, pid *int) gin.HandlerFunc {
	fn := func(c *gin.Context) {
		if *streaming {
			syscall.Kill(*pid, syscall.SIGINT)
			*streaming = false
		}

		c.Writer.WriteHeader(http.StatusNoContent)
	}

	return fn
}

func executeSelfAsExecutor(configFile, videoSrc, audioSrc, sdpFileName string, pid chan int, answer chan string) int {
	cmd := exec.Command(os.Args[0], "execute", "-c", configFile, "-v", videoSrc, "-a", audioSrc, "-s", sdpFileName)

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		log.Fatalln(err)
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		log.Fatalln(err)
	}

	cmd.Start()
	pid <- cmd.Process.Pid

	scanner := bufio.NewScanner(stdout)
	for scanner.Scan() {
		m := scanner.Text()
		if strings.HasPrefix(m, "Answer: ") {
			answerText := m[len(ANSWER):]
			log.Printf("Answer found to be %s", answerText)
			answer <- answerText
		} else {
			log.Println(m)
		}
	}

	scanerr := bufio.NewScanner(stderr)
	for scanerr.Scan() {
		m := scanerr.Text()
		log.Println(m)
	}

	if err := cmd.Wait(); err != nil {
		if exiterr, ok := err.(*exec.ExitError); ok {
			return exiterr.ExitCode()
		} else {
			log.Fatalf("cmd.Wait: %v", err)
		}
	}

	return 0
}

func createStream(configFile string, config *Configuration, streaming *bool, child *int) gin.HandlerFunc {
	fn := func(c *gin.Context) {
		if *streaming {
			c.AbortWithStatusJSON(http.StatusInternalServerError, map[string]string{"message": "Service unavailable as streaming is in progress"})
			return
		}

		*streaming = true

		var request Request
		if err := c.BindJSON(&request); err != nil {
			log.Println(err)
			return
		}

		videoSrc := fmt.Sprintf("%s ! video/x-raw, width=%d, height=%d, framerate=%d/1 ! videoconvert ! queue", config.VideoDevice, config.ImageWidth, config.ImageHeight, config.FrameRate)
		audioSrc := fmt.Sprintf("%s ! audioconvert ! queue", config.AudioDevice)

		log.Println(videoSrc)
		log.Println(audioSrc)

		file, err := os.CreateTemp("/tmp", "gowebrtc")
		if err != nil {
			log.Println(err)
			c.IndentedJSON(http.StatusInternalServerError, map[string]string{"message": "Unable to create sdp file"})
		}
		sdpFileName := file.Name()
		defer os.Remove(sdpFileName)

		file.WriteString(request.SDP)
		file.Close()

		var response Response
		answer := make(chan string)
		pid := make(chan int)
		failure := make(chan int)

		go func() {
			log.Println("About to execute streaming process")
			code := executeSelfAsExecutor(configFile, videoSrc, audioSrc, sdpFileName, pid, answer)
			log.Printf("Child process with PID: %d exited with code: %d", *child, code)
			*streaming = false
			*child = 0
			failure <- code
		}()

		for {
			select {
			case s := <-answer:
				log.Println("Got result")
				response.SDP = s
				c.IndentedJSON(http.StatusOK, response)
				log.Println("Sent response")
				return
			case code := <-failure:
				log.Println("Got error while starting streaming")
				c.IndentedJSON(http.StatusInternalServerError, map[string]string{"message": fmt.Sprintf("Error while creating stream: %d", code)})
				return
			case p := <-pid:
				log.Printf("Started process with pid: %d", p)
				*child = p
				break
			default:
				time.Sleep(100 * time.Millisecond)
			}
		}
	}

	return fn
}

func setupLogging(logFile string) *os.File {
	if logFile != "none" {
		f, err := os.OpenFile(logFile, os.O_RDWR|os.O_CREATE|os.O_APPEND, 0666)
		if err != nil {
			log.Fatalf("Error opening file: %v", err)
		}

		log.SetOutput(f)

		return f
	}

	return nil
}

func startExecuting(conf *Configuration, videoSrc, audioSrc, sdpFile string) {
	gst.Init(nil)

	config := webrtc.Configuration{}
	if conf.OpenRelayConfig != nil {
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

	peerConnection, err := webrtc.NewPeerConnection(config)
	if err != nil {
		log.Fatalln(err)
	}

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
	firstVideoTrack, err := webrtc.NewTrackLocalStaticSample(webrtc.RTPCodecCapability{MimeType: "video/vp8"}, "video", "pion2")
	if err != nil {
		log.Fatalln(err)
	}
	_, err = peerConnection.AddTrack(firstVideoTrack)
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

	answer, err := peerConnection.CreateAnswer(nil)
	if err != nil {
		log.Fatalln(err)
	}

	gatherComplete := webrtc.GatheringCompletePromise(peerConnection)

	err = peerConnection.SetLocalDescription(answer)
	if err != nil {
		log.Fatalln(err)
	}

	fmt.Println("Gathering candidates")
	<-gatherComplete

	localDescription := encode(*peerConnection.LocalDescription())
	fmt.Println(ANSWER + localDescription)

	pipelineForCodec("opus", []*webrtc.TrackLocalStaticSample{audioTrack}, audioSrc)
	pipelineForCodec("vp8", []*webrtc.TrackLocalStaticSample{firstVideoTrack}, videoSrc)

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
