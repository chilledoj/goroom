import { StrictMode } from 'react'
import { createRoot } from 'react-dom/client'

import { PlayerProvider } from './contexts/player.jsx';

import App from './App.jsx'
import 'doodle.css/doodle.css'
import './index.css'

createRoot(document.getElementById('root')).render(
  <StrictMode>
    <PlayerProvider>
      <App />
    </PlayerProvider>
  </StrictMode>,
)
