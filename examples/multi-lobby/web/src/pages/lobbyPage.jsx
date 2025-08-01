import LobbiesList from "../Lobbies/List.jsx";
import {usePlayer} from "../contexts/player.jsx";

function LobbyPage() {
  const {player} = usePlayer();
  return (
      <div className="doodle-border">
        <div style={{display: "flex", alignItems: "center", justifyContent: "space-between"}}>
          <h1>Lobbies</h1>
          <p>Logged in as {player.username}</p>
        </div>
        <LobbiesList />
      </div>
  )
}

export default LobbyPage;
