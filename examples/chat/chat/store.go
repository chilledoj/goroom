package chat

import "sync"

type UserStore interface {
	GetUser(id UserId) *User
	GetUserByName(name string) *User
	AddUser(name string) *User
	RemoveUser(id UserId)
}

type UserStoreImpl struct {
	mu    sync.RWMutex
	pk    int
	users map[UserId]*User
}

func (us *UserStoreImpl) GetUser(id UserId) *User {
	us.mu.RLock()
	defer us.mu.RUnlock()
	return us.users[id]
}

func (us *UserStoreImpl) GetUserByName(name string) *User {
	us.mu.RLock()
	defer us.mu.RUnlock()
	for _, user := range us.users {
		if user.Name == name {
			return user
		}
	}
	return nil
}

func (us *UserStoreImpl) AddUser(name string) *User {
	us.mu.Lock()
	defer us.mu.Unlock()
	us.pk += 1
	id := us.pk // copy pk to id
	user := &User{
		Id:   id,
		Name: name,
	}
	us.users[user.Id] = user
	return user
}

func (us *UserStoreImpl) RemoveUser(id UserId) {
	us.mu.Lock()
	defer us.mu.Unlock()
	delete(us.users, id)
}

func NewUserStore() *UserStoreImpl {
	return &UserStoreImpl{
		users: make(map[UserId]*User),
	}
}
