remoteStream = new MediaStream()
document.getElementById('video-player').srcObject = remoteStream

var pc;

window.init = async function () {
  document.getElementById('start-session').disabled = true;
  document.getElementById('test-open-relay').disabled = true;
  let appName = document.getElementById('app-name').value;
  let apiKey = document.getElementById('api-key').value;

  const response = await fetch("https://" + appName + "/api/v1/turn/credentials?apiKey=" + apiKey);
  const iceServers = await response.json();
  pc = new RTCPeerConnection({
    iceServers: iceServers
  });

  let log = msg => {
    document.getElementById('div').innerHTML += msg + '<br>'
  }

  pc.ontrack = function(event) {
    event.streams[0].getTracks().forEach((track) => {
      remoteStream.addTrack(track)
    })
  }

  pc.oniceconnectionstatechange = e => log(pc.iceConnectionState)
  pc.onicecandidate = event => {
    if (event.candidate) {
      console.log('Updated candidate: ' + btoa(JSON.stringify(pc.localDescription)));
    }
  }

  // Offer to receive 1 audio, and 1 video track
  pc.addTransceiver('video', {
    'direction': 'sendrecv'
  })
  pc.addTransceiver('audio', {
    'direction': 'sendrecv'
  })

  pc.createOffer().then(d => pc.setLocalDescription(d)).catch(log)
  document.getElementById('start-session').disabled = false;
  alert('Click on Start session now');
}

//init();

window.startSession = async () => {
  document.getElementById('start-session').disabled = true;
	let localSessionDescription = btoa(JSON.stringify(pc.localDescription));
  console.log('Local: ' + localSessionDescription);
  
  fetch('/stream', {
    method: 'POST',
    body: JSON.stringify({"sdp": localSessionDescription}),
    headers: {
      'Content-Type': 'application/json'
    }
  }).then(response => {
    if (!response.ok) {
      alert('Failed to call /stream api');
      throw new Error('Error calling api');
    }

    return response.json();
  }).then(data => {
    console.log('Remote: ' + data);
    pc.setRemoteDescription(new RTCSessionDescription(JSON.parse(atob(data.sdp))));
    document.getElementById('stop-session').disabled = false;
  }).catch(error => {
    alert('Error' + error);
  });
}

window.stopSession = () => {
  document.getElementById('stop-session').disabled = true;
  document.getElementById('reload-session').disabled = false;
  fetch('/stream', {
    method: 'DELETE',
  }).then(response => {
    if (!response.ok) {
      alert('Failed to call /stream api');
      throw new Error('Error calling api');
    }
  }).catch(error => {
    alert('Error' + error);
  });
}

