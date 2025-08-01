import {useEffect, useState} from "react";
import useWebSocket from "react-use-websocket";
import {usePlayer} from "../contexts/player.jsx";


function LobbyView({lobby, playerId}) {
  const [socketUrl, setSocketUrl] = useState(`ws://${window.location.host}/api/lobbies/${lobby.id}/ws?playerId=${playerId}`);
  const [messageHistory, setMessageHistory] = useState([])
  const [players,setPlayers] = useState(lobby.players)
  const [lobbyStatus, setLobbyStatus] = useState(lobby.status)
  const {logout} = usePlayer();

  const { sendMessage, readyState,getWebSocket } = useWebSocket(socketUrl,{
    onMessage: (event) => {
      console.log(event)
      if (event.data instanceof Blob) {
        const reader = new FileReader();

        reader.onload = () => {
          console.log("Result: " + reader.result);
          const msg = JSON.parse(reader.result)
          setPlayers(msg.players)
          setMessageHistory((prev) => prev.concat(msg));
          console.log("BLOB: " , msg);
        };

        reader.readAsText(event.data);
      } else {
        console.log("Not BLOB: " + event.data);
        const bytes = new Uint8Array(event.data).reduce((a,b)=> a+ String.fromCharCode(b),'');
        const msg = JSON.parse(bytes)
        setPlayers(msg.players)
        setLobbyStatus(msg.status)
        setMessageHistory((prev) => prev.concat(msg));

        console.log("Not BLOB: ",msg)
      }
    }
  });

  useEffect(()=>{
    const socket = getWebSocket()
    if(socket == null){
      return
    }
    getWebSocket().binaryType = 'arraybuffer'
    if(readyState === 3){
      socket.close()
      logout()
    }
  },[readyState])

  console.log("lobbyStatus", lobby.status)
  const onToggleStatus = (e) => {
    e.preventDefault();
    sendMessage(JSON.stringify({action: "toggleStatus", lobbyId: lobby.id, playerId: playerId, status: lobbyStatus=="Open"?"Locked":"Open"}))
  }

  return (
      <div style={{margin: "1rem", display: "flex", flexDirection: "row", gap: "1rem", alignItems: "center"}}>
        <div style={{width: "200px", overflow: "hidden", display: 'flex', gap: '2rem'}}>
          <div style={{display: "flex", flexDirection: "column", gap: "1rem"}}>
            <h4>{lobby.id}</h4>
            <p>{lobbyStatus}</p>
          </div>
          <div>
            <button onClick={onToggleStatus}>{lobbyStatus =="Open"?"Lock":"Unlock"}</button>
          </div>
        </div>
        <div className={"doodle-border"} style={{flex: 1, minHeight: "2rem"}}>
          <h3>Players {players.length}</h3>
          <div style={{display:"flex"}}>
            {players.map(player => (
                <div key={lobby.id+(''+player.id)}>
                  <div>
                    <img
                        src={`https://api.dicebear.com/9.x/croodles/svg?size=128&seed=${player.username}`}
                        alt="avatar"
                        className={player.isConnected ? "avatar-online" : "avatar-offline" }
                    />
                  </div>
                  <div>{player.id}:{player.username}</div>
                </div>))}
          </div>
        </div>
      </div>
  )
}

export default LobbyView;