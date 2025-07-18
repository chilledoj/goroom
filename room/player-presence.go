package room

type PlayerPresence[PlayerId comparable] struct {
	ID          PlayerId
	IsConnected bool
}
