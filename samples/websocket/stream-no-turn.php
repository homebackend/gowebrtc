<?php

const SERVER = '<ip>';
const HTTP_PORT = '<port>';
const WS_USERNAME = '<username>';
const WS_PASSWORD = '<password>';

?>

<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>Video Stream</title>
    <style>
        body {
            display: flex;
            justify-content: center;
            align-items: center;
            height: 100vh;
            margin: 0;
            background-color: #000;
        }
        .video-container .image-container {
            width: 640px;
            height: 480px;
            background-color: #000;
            display: flex;
            justify-content: center;
            align-items: center;
        }

        .video-container video {
            width: 100%;
            height: 100%;
            object-fit: contain;
        }
        @media (max-width: 768px) {
            .video-container {
                width: 100%;
                height: auto;
                aspect-ratio: 4 / 3;
            }
            .video-container video {
                width: 100%;
                height: 100%;
            }
    }

    .image-container {
           z-index: 10;
        }
    </style>
</head>
<body>
<script>

window.startVideo = async function () {
  let remoteStream = new MediaStream()
  //document.getElementById('video-player').srcObject = remoteStream

  let wsConnected = false;

  function wsConnect() {
    if (wsConnected) {
      return;
    }
    
    wsConnected = true;
    let localSessionDescription = btoa(JSON.stringify(pc.localDescription));
    console.log('Local: ' + localSessionDescription);
    connectWebsocket(pc, localSessionDescription, remoteStream);
  }


  let pc = new RTCPeerConnection({
    iceServers: [
      {"urls":['stun:stun.l.google.com:19302']},
      {"urls":['stun:stun1.l.google.com:19302']},
      {"urls":['stun:stun2.l.google.com:19302']},
      {"urls":['stun:stun3.l.google.com:19302']},
      {"urls":['stun:stun4.l.google.com:19302']},
    ]
  }); 

  let log = msg => {
    console.log(msg);
  }

  pc.ontrack = function(event) {
    event.streams[0].getTracks().forEach((track) => {
      console.log('Track added');
      remoteStream.addTrack(track)
    })
  }

  pc.oniceconnectionstatechange = e => log(pc.iceConnectionState)
  pc.onicecandidate = async (event) => {
    if (event.candidate) {
      console.log(JSON.stringify(event.candidate));
      //console.log('Updated candidate: ' + btoa(JSON.stringify(pc.localDescription)));
      setTimeout(wsConnect, 2000);
    } else {
      wsConnect();
    }
  }

  // Offer to receive 1 audio, and 1 video track
  pc.addTransceiver('video', {
    'direction': 'sendrecv'
  })
  pc.addTransceiver('audio', {
    'direction': 'sendrecv'
  })

  pc.createOffer().then(d => {
    pc.setLocalDescription(d);
  }).catch(log)
}

class Event {
  constructor(type, payload) {
    this.type = type;
    this.payload = payload;
  }
}

class ConnectEvent {
  constructor(sdp, user, password) {
    this.sdp = sdp;
    this.user = user;
    this.password = password;
  }
}

class AnswerEvent {
  constructor(answer){
    this.answer = answer
  }
}

class NewCandidateEvent {
  constructor() {
  }
}

function routeEvent(event, pc, remoteStream, conn) {
  if (event.type === undefined) {
    alert("no 'type' field in event");
  }
  console.log("Event type is: " + event.type)
  switch (event.type) {
    case 'answer':
      const answerEvent = Object.assign(new AnswerEvent, event.payload);
      pc.setRemoteDescription(new RTCSessionDescription(JSON.parse(atob(answerEvent.answer))));
      let p = document.getElementById('video-player')
      p.srcObject = remoteStream;
      p.onclick = function() {
        p.play();
        p.onclick = null;
      }
      break;
    case 'new_candidate':
      const candidateEvent = Object.assign(new NewCandidateEvent, event.payload);
      pc.addIceCandidate(candidateEvent).catch((e) => {
        console.log(`Failure during addIceCandidate(): ${e.name}`);
      });
      break;
    case 'disconnect':
      console.log('Closing connection');
      conn.close()
      break;
    default:
      alert("unsupported message type");
      break;
  }
}

function connectWebsocket(pc, sdp, remoteStream) {
  if (window["WebSocket"]) {
    console.log("supports websockets");
    conn = new WebSocket("ws://" + <?php echo SERVER;?> + ":" + <?php echo HTTP_PORT;?> + "/ws");

    conn.onopen = function (evt) {
      console.log('Connected to Websocket: true');
      let connectEvent = new ConnectEvent(sdp, '<?php echo WS_USERNAME;?>', '<?php echo WS_PASSWORD;?>');
      let event = new Event('connect', connectEvent);
      conn.send(JSON.stringify(event))
    }

    conn.onclose = function (evt) {
      console.log('Connected to Websocket: false');
    }

    conn.onmessage = function (evt) {
      console.log(evt);
      const eventData = JSON.parse(evt.data);
      const event = Object.assign(new Event, eventData);
      routeEvent(event, pc, remoteStream, conn);
    }

  } else {
      alert("Not supporting websockets");
  }
}

window.onload = function () {
  window.startVideo();
}

</script>

    <div class="video-container">
      <video preload="metadata" id="video-player"></video>
    </div>
</body>
</html>