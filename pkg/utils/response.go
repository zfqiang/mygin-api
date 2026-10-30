package utils

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

type Response struct {
	Code int    `json:"code"`
	Msg  string `json:"msg"`
	Data any    `json:"data"`
}

func Respond(c *gin.Context, httpStatus, code int, msg string, data any) {
	c.JSON(httpStatus, Response{
		Code: code,
		Msg:  msg,
		Data: data,
	})
}

func Success(c *gin.Context, data any) {
	Respond(c, http.StatusOK, http.StatusOK, "success", data)
}

func Failure(c *gin.Context, httpStatus int, msg string) {
	Respond(c, httpStatus, httpStatus, msg, nil)
}

func AbortFailure(c *gin.Context, httpStatus int, msg string) {
	Failure(c, httpStatus, msg)
	c.Abort()
}
