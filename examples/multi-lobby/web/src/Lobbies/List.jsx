import useLobbies from "../hooks/useLobbies.js";
import {useRef, useState} from "react";
import {usePlayer} from "../contexts/player.jsx";
import LobbyView from "./LobbyView.jsx";


function LobbiesList() {
  const [seed, setSeed] = useState(0)
  const {lobbies, loading, error, addLobby} = useLobbies(seed);
  const {player} = usePlayer();
  const detailRef = useRef(null);

  const onRefresh = () => {
    setSeed(seed+1)
  }
  const onCreateLobby = () => {
    addLobby(player.id)
        .then((lobby)=>{
          setSeed(seed+1)
        })
  }
  const showDialog = () => {
    detailRef.current.showModal()
  }
  return (
    <>
      <div>
      {loading && (<div>Loading...</div>)}
      {error && <div>Error: {error.message}</div>}
      </div>
      <div style={{display: "flex", flexDirection: "row", justifyContent: "space-between"}}>
        <div>
          <button onClick={onRefresh}>refresh</button>
          <button onClick={onCreateLobby}>Create Lobby</button>
        </div>
        <div><button onClick={showDialog}>Show Details</button></div>
      </div>
      <dialog ref={detailRef}>
        <div className="doodle-border" style={{minWidth: "50vw"}}>
          <pre><code>{JSON.stringify(lobbies,null,2)}</code></pre>
        </div>
      </dialog>

      <div>
        {lobbies.map(lobby => (<LobbyView key={lobby.id} lobby={lobby} playerId={player.id} />))}
      </div>
    </>
  )
}

export default LobbiesList;