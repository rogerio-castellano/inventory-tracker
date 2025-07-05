package repo

import "github.com/rogerio-castellano/inventory-tracker/internal/models"

type UserRepository interface {
	GetByUsername(username string) (models.User, error)
	CreateUser(u models.User) (models.User, error)
}
