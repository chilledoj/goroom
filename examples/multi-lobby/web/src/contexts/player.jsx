import { createContext, useState, useContext } from "react";
import {playerLogin} from "../services/player.js";

const PlayerContext = createContext();

export const usePlayer = () => {
    return useContext(PlayerContext);
}

export const PlayerProvider = ({ children }) => {
    const [player, setPlayer] = useState(null);

    const login = (username) => {
        playerLogin({username})
            .then(userData=>{
                setPlayer(userData);
            })
    };

    const logout = () => {
        setPlayer(null);
    }

    const value = {
        player,
        login,
        logout
    };

    return (
        <PlayerContext.Provider value={value}>
            {children}
        </PlayerContext.Provider>
    );
}

export default PlayerContext;