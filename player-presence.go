package goroom

type PlayerPresence[PlayerId comparable] struct {
	ID          PlayerId
	IsConnected bool
}
