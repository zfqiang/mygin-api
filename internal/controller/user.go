package controller

import (
	"mygin-api/pkg/config"
)

type userController struct {
	appCfg *config.AppConfig
}

func NewuserController(appCfg *config.AppConfig) *userController {
	return &userController{appCfg: appCfg}
}
