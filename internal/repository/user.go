package repository

import (
	"mygin-api/internal/model"
	"mygin-api/pkg/config"
)

type UserRepository struct {
	appCfg *config.AppConfig
}

func NewUserRepository(appCfg *config.AppConfig) *UserRepository {
	return &UserRepository{
		appCfg: appCfg,
	}
}

func (ur *UserRepository) GetUserByUsername(username string) (model.User, error) {
	var user model.User
	err := ur.appCfg.DB.Where("username = ?", username).First(&user).Error
	return user, err
}
