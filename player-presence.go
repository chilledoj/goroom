package goroom

import "time"

type PlayerPresence[PlayerId comparable] struct {
	ID          PlayerId
	IsConnected bool
	LastSeen    time.Time
}
