export async function handleFetchResponse(res){
  const body = await res.text()
  if(res.status >=400){
    throw new Error(body)
  }
  return JSON.parse(body)
}