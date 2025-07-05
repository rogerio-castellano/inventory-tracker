package repo

import (
	"errors"

	"github.com/rogerio-castellano/inventory-tracker/internal/models"
)

type InMemoryUserRepository struct {
	users []models.User
}

func NewInMemoryUserRepository() *InMemoryUserRepository {
	return &InMemoryUserRepository{
		users: []models.User{},
	}
}

func (r *InMemoryUserRepository) GetByUsername(username string) (models.User, error) {
	for _, user := range r.users {
		if user.Username == username {
			return user, nil
		}
	}

	return models.User{}, nil
}

func (r *InMemoryUserRepository) CreateUser(u models.User) (models.User, error) {
	for _, user := range r.users {
		if user.Username == u.Username {
			return models.User{}, errors.New("unique constraint violation: username already exists")
		}
	}

	u.ID = len(r.users) + 1
	r.users = append(r.users, u)
	return u, nil
}
