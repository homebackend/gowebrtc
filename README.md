[![Contributors][contributors-shield]][contributors-url]
[![Forks][forks-shield]][forks-url]
[![Stargazers][stars-shield]][stars-url]
[![Issues][issues-shield]][issues-url]
[![MIT License][license-shield]][license-url]
[![LinkedIn][linkedin-shield]][linkedin-url]

# gowebrtc
Setup Webrtc for video and audio streaming from local media devices

# Build and Install
# Prerequisites

Install the following packages: **libgstreamer1.0-dev**, **libgstreamer-plugins-base1.0-dev**, **libunwind-dev**.

Download latest go lang from the following [link](https://go.dev/dl/). Specifically for Raspbian OS download: go1.22.5.linux-arm64.tar.gz.

# Build

To build the code execute command `make build`. This will build executable: `bin/gowebrtc`. Executing command `./bin/gowebrtc` will print the command line usage instructions.

# Installation

To build deb installer package (which can be installed on Debian, Ubuntu, Raspbian OS among others) execute the command `make debian`. The package gets built in the directory `build/debian/gowebrtc.deb`.

To install the above package using **sudo** execute the command `make debian-install`.

# Configuring

The default configuration file resides in `/etc/gowebrtc/config.yaml`. 

## General configuration option

| Option | Type | Default | Required | Description |
| -- | -- | -- | -- | -- |
| image_width | number | 640 | No | Specifies the video width |
| image_height | number | 480 | No | Specifies the video height |
| framerate | number | 30 | No | Specifies the fps value |
| log_file | string | none | No | Log file location. Needs to be writable |
| audio_device | string | | Yes | Gstream pipeline to be used for audio stream |
| video_device | string | | Yes | Gstream pipeline to be used for video stream |

## Signalling configuration
Either REST HTTP API or websockets can be used for exchanging SDP and Candidates.

| Option | Type | Default | Required | Description |
| -- | -- | -- | -- | -- |
| port | number | 8080 | No | Port to be used for running signalling service. |
| url | string | /stream | No | Url path to be used by signalling service. |
| signalling | string | websocket | No | The value can be one of http or websocket. |
| signalling_uses_tls | bool | false | No | If you want to run signalling server in TLS mode. Currently only websocket supports this. |
| signalling_tls_cert | string | | No | Server certificate |
| signalling_tls_key | string | | No | Server certificate key |

### Websocket specific configuration options

| Option | Type | Default | Required | Description |
| -- | -- | -- | -- | -- |
| signalling_origin | string | | No | If provided allows requests from specific origin. Requests from other origina are denied. |
| SignallingCredentials | array | | No | Array of credentials. Any of the given credentials must be provided. If no credentials is specified in configuration file, credential are not required. |

#### Specifying credentials

| Option | Type | Default | Required | Description |
| -- | -- | -- | -- | -- |
| user | string | | Yes | Credential user name |
| password | string | | Yes | Credential user password |

## Turn server configuration
Webrtc requires turn servers to function in some scenarios. Gowebrtc service has internal turn server. Also it supports using Open Relay and other turn servers.

| Option | Type | Default | Required | Description |
| -- | -- | -- | -- | -- |
| use_internal_turn | bool | false | No | Use internal turn server of external turn server |

### Internal turn server options

Following is sample configuration portion for internal turn server configuration:

```yaml
use_internal_turn: true
turn_configuration:
  public_ip: <public-ip>
  port: <turn-port>
  users:
    - user: <turn-user>
      password: <turn-password>
```

### Open Relay turn server options

If **open_relay_config** attribute is defined, Open relay will be used as turn server.

```yaml
open_relay_config:
  app_name: <app-name>
  api_key: <api-key>
```

### Other turn servers

Using **ice_servers** any other ice server(s) can be used.

```yaml
ice_servers:
  - urls:
    - "stun:<server>"
  - urls:
    - "turn:<server>:80"
    username: <username>
    credential: <password>
  - urls:
    - "turn:<server>:80?transport=tcp"
    username: <username>
    credential: <password>
  - urls:
    - "turn:<server>:443"
    username: <username>
    credential: <password>
  - urls:
    - "turns:<server>:443?transport=tcp"
    username: <username>
    credential: <password>
```

## Samples
### Running the service with internal turn server
If you have public and IP and would like to host your own turn server, the following configuration can be used:

```yaml
port: 8080
url: /ws
image_width: 640
image_height: 480
framerate: 30
log_file: /var/log/gowebrtc.log
audio_device: alsasrc device=plughw:CARD=I930,DEV=0
video_device: autovideosrc
use_internal_turn: true
signalling: websocket
turn_configuration:
  public_ip: <public-ip>
  port: <turn-port>
  users:
    - user: <turn-user>
      password: <turn-password>
```

### Runnig the service with external turn server
External turn server configuration is specified by providing relevant information in the config file. Relevant section is as follows:

```yaml
port: 8080
url: /stream
image_width: 640
image_height: 480
framerate: 30
log_file: /var/log/gowebrtc.log
audio_device: alsasrc device=plughw:CARD=I930,DEV=0
video_device: autovideosrc
signalling: http
open_relay_config:
  app_name: <app-name>
  api_key: <api-key>
ice_servers:
  - urls:
    - "stun:stun.relay.metered.ca:80"
  - urls:
    - "turn:global.relay.metered.ca:80"
    username: <username>
    credential: <password>
  - urls:
    - "turn:global.relay.metered.ca:80?transport=tcp"
    username: <username>
    credential: <password>
  - urls:
    - "turn:global.relay.metered.ca:443"
    username: <username>
    credential: <password>
  - urls:
    - "turns:global.relay.metered.ca:443?transport=tcp"
    username: <username>
    credential: <password>
```

# Running and Enabling Service
`sudo systemctl start gowebrtc` will start the service.
`sudo systemctl enable gowebrtc` will configure service startup on each reboot.

# APIS

| URL | Method | Payload | Description | Response | Error Response |
| -- | -- | -- | -- | -- | -- |
| /stream | POST | `{"sdp": "localSessionDescription"}` | This API call initiates SDP exchange more generally known as Signalling. In response the API returns the Remote SDP. In case of error error message is returned with status code as 500. | `{"sdp": "remoteSessionDescription"}` | `{"error": "error message"}` |
| /stream | DELETE | `none` | This API call terminates any existing streaming session. | `none` | `{"error": "error message"}` |

Remember at any given time only one webrtc streaming is possible. An attempt to initiate another streaming will result in an error.


<!-- MARKDOWN LINKS & IMAGES -->
<!-- https://www.markdownguide.org/basic-syntax/#reference-style-links -->
[contributors-shield]: https://img.shields.io/github/contributors/homebackend/gowebrtc.svg?style=for-the-badge
[contributors-url]: https://github.com/homebackend/gowebrtc/graphs/contributors
[forks-shield]: https://img.shields.io/github/forks/homebackend/gowebrtc.svg?style=for-the-badge
[forks-url]: https://github.com/homebackend/gowebrtc/network/members
[stars-shield]: https://img.shields.io/github/stars/homebackend/gowebrtc.svg?style=for-the-badge
[stars-url]: https://github.com/homebackend/gowebrtc/stargazers
[issues-shield]: https://img.shields.io/github/issues/homebackend/gowebrtc.svg?style=for-the-badge
[issues-url]: https://github.com/homebackend/gowebrtc/issues
[license-shield]: https://img.shields.io/github/license/homebackend/gowebrtc.svg?style=for-the-badge
[license-url]: https://github.com/homebackend/gowebrtc/blob/master/LICENSE
[linkedin-shield]: https://img.shields.io/badge/-LinkedIn-black.svg?style=for-the-badge&logo=linkedin&colorB=555
[linkedin-url]: https://linkedin.com/in/neeraj-jakhar-39686212b
