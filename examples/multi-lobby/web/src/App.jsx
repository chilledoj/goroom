import './App.css'
import { usePlayer } from "./contexts/player.jsx";
import LoginPage from "./pages/loginPage.jsx";
import LobbyPage from "./pages/lobbyPage.jsx";

function App() {
  const { player } = usePlayer();


  if(player == null){
    return <LoginPage />
  }
  console.log("player", player)
  return (
    <LobbyPage />
  )
}

export default App
