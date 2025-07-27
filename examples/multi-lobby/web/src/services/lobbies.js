import {handleFetchResponse} from "./common.js";


export function getLobbies() {
  return fetch("/api/lobbies")
      .then(handleFetchResponse)
}

export function createLobby(playerId, data){
  const url = `/api/lobbies?playerId=${playerId}`
  return fetch(url,{
    method: "POST",
    headers: {
      "Content-Type": "application/json"
    },
    body: JSON.stringify(data)
  })
      .then(handleFetchResponse)
}