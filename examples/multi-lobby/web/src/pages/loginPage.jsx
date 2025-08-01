import {usePlayer} from "../contexts/player.jsx";


function LoginPage(){
  const { login } = usePlayer();

  const onSubmit = (e) => {
    e.preventDefault();
    let formData = new FormData(e.target);
    const username = formData.get("username");
    if(!username || username==""){
      return
    }

    login(username);
  }

  return (
      <div className="doodle-border">
        <h1>Login</h1>
        <form onSubmit={onSubmit}>
          <div style={{marginBottom: "1rem", display: "flex", flexDirection: "row", alignItems: "center", gap: "1rem"}}>
            <label htmlFor="username">Username</label>
            <input type="text" name="username" required />
          </div>
          <button>Login</button>
        </form>
      </div>
  )
}

export default LoginPage;