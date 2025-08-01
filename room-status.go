package goroom

type RoomStatus int8

const (
	Inactive RoomStatus = iota - 1
	Open
	Locked
)

func (r RoomStatus) String() string {
	switch r {
	case Inactive:
		return "Inactive"
	case Open:
		return "Open"
	case Locked:
		return "Locked"
	default:
		return "Unknown"
	}
}
