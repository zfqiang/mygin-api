package utils

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

const (
	successStatus = 1 // code 成功
	failureStatus = 0 // code 失败
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
	Respond(c, http.StatusOK, successStatus, "success", data)
}

func Fail(c *gin.Context, msg string) {
	Respond(c, http.StatusOK, failureStatus, msg, nil)
}

func Failure(c *gin.Context, httpStatus int, msg string) {
	Respond(c, http.StatusOK, failureStatus, msg, nil)
}

func Error(c *gin.Context, httpStatus int, code int, msg string) {
	Respond(c, httpStatus, code, msg, nil)
}

func AbortFailure(c *gin.Context, httpStatus int, msg string) {
	Error(c, httpStatus, failureStatus, msg)
	c.Abort()
}
