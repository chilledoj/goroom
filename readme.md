# GoRoom

A helper library hub to manage websockets, with the main use case being game servers.

## About

I didn't like the way that some of the other game server libraries were so opinionated in their design

It works differently to other game server libraries
+ It is just the websocket hub, not a full server.
+ It expects the action of connecting via websocket to be the join action. In other libraries the join request is 
usually sent after the websocket connection is made.
+ It uses generics so that the room identifier and player identifier can be which ever type you like.
+ A player can only have one websocket connection


## Usage

See the `examples` directory for extend working examples of how to use this library.

The `goroom.Room` type uses generics to allow you to define which ever type is to be used for the identifiers for
+ this room ID
+ the identifier for the conencted player

You can provide any type (basic, custom, aliased) for these identifiers as long as they are `comparable`. 
You can then embed `goroom.Room` (as a pointer) in a container struct:

```go
type RoomId uint64
type PlayerId = string

type ChatRoom struct {
	*goroom.Room[string, string]

	// Other details specifically for your use case
}
```

To create a new `goroom.Room` you must use the `NewRoom` function to ensure the internals are created correctly. 
You must also start the room go routine.

```go
func NewChatRoom() *ChatRoom {
    cr := &ChatRoom{
    users:   NewUserStore(),
    msgs:    make([]ChatMessage, 0),
    Slogger: slog.Default(),
    }
    
    cr.Room = goroom.NewRoom[string, UserId](ctx, "room", goroom.Options[UserId]{
    OnConnect:    cr.OnConnect,
    OnDisconnect: cr.OnDisconnect,
    OnMessage:    cr.OnMessage,
    Slogger:      cr.Slogger,
    })

    return cr
}

func (cr *ChatRoom) Start() { go cr.Room.Start() }
func (cr *ChatRoom) Stop() { cr.Room.Stop() }
```