import {handleFetchResponse} from "./common.js";


export function playerLogin(data){
  return fetch("/api/players",{
    method: "POST",
    headers: {
      "Content-Type": "application/json"
    },
    body: JSON.stringify(data)
  })
      .then(handleFetchResponse)
}