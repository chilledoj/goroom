import {useEffect, useState} from "react";
import {createLobby, getLobbies} from "../services/lobbies.js";

function useLobbies(seed) {
  const [lobbies, setLobbies] = useState([]);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState(null);

  const addLobby = (playerId)=>{
    setLoading(true)
    return createLobby(playerId)
        .then((lobby)=>{
          console.log(lobby)
          setLobbies([...lobbies, lobby])
          return lobby
    })
    .catch(setError)
    .finally(() => setLoading(false))
  }


  useEffect(() => {
    setLoading(true)
    setError(null)
    getLobbies()
        .then(setLobbies)
        .catch(setError)
        .finally(() => setLoading(false))
  }, [seed, setLobbies, setError, setLoading])
  return {lobbies, loading, error, addLobby}
}

export default useLobbies;