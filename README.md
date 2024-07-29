[![Contributors][contributors-shield]][contributors-url]
[![Forks][forks-shield]][forks-url]
[![Stargazers][stars-shield]][stars-url]
[![Issues][issues-shield]][issues-url]
[![MIT License][license-shield]][license-url]
[![LinkedIn][linkedin-shield]][linkedin-url]

# gowebrtc
Setup Webrtc for video and audio streaming from local media devices

# Build and Install
To build the code execute command `make build`. This will build executable: `bin/gowebrtc`. Executing command `./bin/gowebrtc` will print the command line usage instructions.

To build deb installer package (which can be installed on Debian, Ubuntu, Raspbian OS among others) execute the command `make debian`. The package gets built in the directory `build/debian/gowebrtc.deb`.

To install the above package using **sudo** execute the command `make debian-install`.

# Configuring

The default configuration file resides in `/etc/gowebrtc/config.yaml`. Sample configuration file is:

```yaml
port: 8080
url: /stream
image_width: 640
image_height: 480
framerate: 30
log_file: /var/log/gowebrtc.log
audio_device: alsasrc device=plughw:CARD=I930,DEV=0
video_device: autovideosrc
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
