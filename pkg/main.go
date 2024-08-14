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
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/akamensky/argparse"
	"github.com/gin-gonic/gin"
	homecommon "github.com/homebackend/go-homebackend-common/pkg"
	"github.com/pion/turn/v2"
	"golang.org/x/sys/unix"
)

const (
	CONF_FILE = "/etc/gowebrtc/config.yaml"
	ANSWER    = "Answer: "
	CANDIDATE = "Candidate: "
	EOF       = "::EOF::"
)

type StreamAnswerHandler func(string)

type StreamCandidateHandler func(string)

type StreamErrorHandler func(string)

type Request struct {
	SDP string `json:"sdp"`
}

type Response struct {
	SDP string `json:"sdp"`
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

	w := executeCommand.Flag("w", "wait", &argparse.Options{
		Default: false,
		Help:    "Wait for candidates before printing Answer",
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
		if config.Signalling == "http" {
			setupRouter(c, config)
		} else if config.Signalling == "websocket" {
			setupWebsocketServer(c, config)
		}
	} else if executeCommand.Happened() {
		StartStreaming(config, *v, *a, *s, *w)
	}
}

func setupCommon(config *Configuration) *os.File {
	f := setupLogging(config.LogFile)

	if config.UseInternalTurn {
		if config.TurnConfiguration == nil {
			log.Fatalln("Turn server is enabled but configuration not provided")
		}

		if len(config.TurnConfiguration.Users) == 0 {
			log.Fatalln("At least one user needs to be provided for server")
		}

		if config.TurnConfiguration.TurnType == TurnInternal {
			go setupTurnServer(config)
		}
	}

	return f
}

func setupRouter(c *string, config *Configuration) {
	f := setupCommon(config)
	if f != nil {
		defer f.Close()
	}

	streaming := false
	pid := 0

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
}

func setupTurnServer(config *Configuration) {
	addr, err := net.ResolveUDPAddr("udp", "0.0.0.0:"+strconv.Itoa(config.TurnConfiguration.UdpPort))
	if err != nil {
		log.Fatalf("Failed to parse server address: %s", err)
	}

	usersMap := map[string][]byte{}
	for _, userPassword := range config.TurnConfiguration.Users {
		usersMap[userPassword.User] = turn.GenerateAuthKey(userPassword.User, config.TurnConfiguration.Realm, userPassword.Password)
	}

	listenerConfig := &net.ListenConfig{
		Control: func(network, address string, conn syscall.RawConn) error {
			var operr error
			if err = conn.Control(func(fd uintptr) {
				operr = syscall.SetsockoptInt(int(fd), syscall.SOL_SOCKET, unix.SO_REUSEPORT, 1)
			}); err != nil {
				return err
			}

			return operr
		},
	}

	relayAddressGenerator := &turn.RelayAddressGeneratorStatic{
		RelayAddress: net.ParseIP(config.TurnConfiguration.PublicIp),
		Address:      "0.0.0.0",
	}

	packetConnConfigs := make([]turn.PacketConnConfig, config.TurnConfiguration.Threads)
	for i := 0; i < config.TurnConfiguration.Threads; i++ {
		log.Printf("Network: %s, address: %s\n", addr.Network(), addr.String())
		conn, listErr := listenerConfig.ListenPacket(context.Background(), addr.Network(), addr.String())
		if listErr != nil {
			log.Fatalf("Failed to allocate UDP listener at %s:%s", addr.Network(), addr.String())
		}

		packetConnConfigs[i] = turn.PacketConnConfig{
			PacketConn:            conn,
			RelayAddressGenerator: relayAddressGenerator,
		}

		log.Printf("Server %d listening on %s\n", i, conn.LocalAddr().String())
	}

	s, err := turn.NewServer(turn.ServerConfig{
		Realm: config.TurnConfiguration.Realm,
		AuthHandler: func(username string, realm string, srcAddr net.Addr) ([]byte, bool) { // nolint: revive
			if key, ok := usersMap[username]; ok {
				return key, true
			}
			return nil, false
		},
		PacketConnConfigs: packetConnConfigs,
	})
	if err != nil {
		log.Panicf("Failed to create TURN server: %s", err)
	}

	defer s.Close()
	select {}
}

func setupWebsocketServer(c *string, config *Configuration) {
	f := setupCommon(config)
	if f != nil {
		defer f.Close()
	}

	rootCtx := context.Background()
	ctx, cancel := context.WithCancel(rootCtx)

	defer cancel()

	manager := NewManager(ctx, *c, config)
	go manager.processConnection()

	// Serve the ./frontend directory at Route /
	http.HandleFunc("/", serveHome)
	http.HandleFunc(config.Url, manager.serveWS)

	// Serve on port :8080, fudge yeah hardcoded port
	var err error
	addr := fmt.Sprintf("0.0.0.0:%d", config.Port)
	log.Printf("WS Address: %s\n", addr)
	if config.SignallingUsesTls {
		err = http.ListenAndServeTLS(addr, config.SignallingTlsCert, config.SignallingTlsKey, nil)
	} else {
		err = http.ListenAndServe(addr, nil)
	}
	if err != nil {
		log.Fatal("ListenAndServe: ", err)
	}
}

func serveHome(w http.ResponseWriter, r *http.Request) {
	log.Println(r.URL)
	if r.URL.Path != "/" {
		http.Error(w, "Not found", http.StatusNotFound)
		return
	}
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	http.ServeFile(w, r, "home.html")
}

func killStream(pid *int) {
	syscall.Kill(*pid, syscall.SIGINT)
}

func deleteStream(streaming *bool, pid *int) gin.HandlerFunc {
	fn := func(c *gin.Context) {
		if *streaming {
			killStream(pid)
			*streaming = false
		}

		c.Writer.WriteHeader(http.StatusNoContent)
	}

	return fn
}

func executeSelfAsExecutor(wait bool, configFile, videoSrc, audioSrc, sdpFileName string, pid chan int, answer chan string, candidate chan string, end chan bool) int {
	var cmd *exec.Cmd
	if wait {
		cmd = exec.Command(os.Args[0], "execute", "-c", configFile, "-v", videoSrc, "-a", audioSrc, "-s", sdpFileName, "-w")
	} else {
		cmd = exec.Command(os.Args[0], "execute", "-c", configFile, "-v", videoSrc, "-a", audioSrc, "-s", sdpFileName)
	}

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

	go func() {
		scanner := bufio.NewScanner(stdout)
		for scanner.Scan() {
			m := scanner.Text()
			if strings.HasPrefix(m, ANSWER) {
				answerText := m[len(ANSWER):]
				log.Printf("Answer found to be %s", answerText)
				answer <- answerText
			} else if strings.HasPrefix(m, CANDIDATE) {
				candidateText := m[len(CANDIDATE):]
				log.Printf("Candidate found to be %s", candidateText)
				candidate <- candidateText
			} else if strings.HasPrefix(m, EOF) {
				end <- true
			} else {
				log.Println(m)
			}
		}
	}()

	go func() {
		scanerr := bufio.NewScanner(stderr)
		for scanerr.Scan() {
			m := scanerr.Text()
			log.Println(m)
		}
	}()

	if err := cmd.Wait(); err != nil {
		if exiterr, ok := err.(*exec.ExitError); ok {
			return exiterr.ExitCode()
		} else {
			log.Fatalf("cmd.Wait: %v", err)
		}
	}

	return 0
}

func HandleStreamingRequest(configFile string, config *Configuration, streaming *bool, child *int, sdp string,
	answerHandler StreamAnswerHandler, candidateHandler StreamCandidateHandler, errorHandler StreamErrorHandler) {
	if *streaming {
		if !config.DisconnectOnReconnect {
			errorHandler("Service unavailable as streaming is in progress")
			return
		}

		killStream(child)
		// Allow some time to let things settle down
		time.Sleep(1 * time.Second)
	}

	*streaming = true

	videoSrc := fmt.Sprintf("%s ! video/x-raw, width=%d, height=%d, framerate=%d/1 ! videoconvert ! queue", config.VideoDevice, config.ImageWidth, config.ImageHeight, config.FrameRate)
	audioSrc := fmt.Sprintf("%s ! audioconvert ! queue", config.AudioDevice)

	log.Println(videoSrc)
	log.Println(audioSrc)

	file, err := os.CreateTemp("/tmp", "gowebrtc")
	if err != nil {
		log.Println(err)
		errorHandler("Unable to create sdp file")
		return
	}
	sdpFileName := file.Name()
	defer os.Remove(sdpFileName)

	file.WriteString(sdp)
	file.Close()

	answer := make(chan string)
	candidate := make(chan string)
	end := make(chan bool)
	pid := make(chan int)
	failure := make(chan int)

	go func() {
		log.Println("About to execute streaming process")
		code := executeSelfAsExecutor(config.IceTrickling, configFile, videoSrc, audioSrc, sdpFileName, pid, answer, candidate, end)
		log.Printf("Child process with PID: %d exited with code: %d", *child, code)
		*streaming = false
		*child = 0
		failure <- code
	}()

	for {
		select {
		case s := <-answer:
			log.Println("Got result")
			answerHandler(s)
			log.Println("Sent response")
		case c := <-candidate:
			candidateHandler(c)
		case <-end:
			return
		case code := <-failure:
			log.Println("Got error while starting streaming")
			errorHandler(fmt.Sprintf("Error while creating stream: %d", code))
			return
		case p := <-pid:
			log.Printf("Started process with pid: %d", p)
			*child = p
			break
		}
	}
}

func createStream(configFile string, config *Configuration, streaming *bool, child *int) gin.HandlerFunc {
	fn := func(c *gin.Context) {
		var request Request
		if err := c.BindJSON(&request); err != nil {
			log.Println(err)
			return
		}

		HandleStreamingRequest(configFile, config, streaming, child, request.SDP, func(s string) {
			var response Response
			log.Println("Got result")
			response.SDP = s
			c.IndentedJSON(http.StatusOK, response)
			log.Println("Sent response")
		}, func(string) {

		}, func(e string) {
			c.IndentedJSON(http.StatusInternalServerError, map[string]string{"message": e})
		})
	}

	return fn
}

func setupLogging(logFile string) *os.File {
	if logFile != "none" {
		f, err := os.OpenFile(logFile, os.O_RDWR|os.O_CREATE|os.O_APPEND, 0666)
		if err != nil {
			panic(fmt.Sprintf("Error opening file: %v", err))
		}

		log.SetOutput(f)

		return f
	}

	return nil
}
