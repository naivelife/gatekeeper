package util

import (
	"encoding/json"
	"github.com/e421083458/golang_common/lib"
	"github.com/gin-gonic/gin"
	"net/http"
)

type ResponseCode int

//1000以下为通用码，1000以上为用户自定义码
const (
	SuccessCode ResponseCode = iota
	UndefErrorCode
	ValidErrorCode
	InternalErrorCode

	InvalidRequestErrorCode ResponseCode = 401
	CustomizeCode           ResponseCode = 1000
)

//Response 返回值结构体
type Response struct {
	ErrorCode ResponseCode `json:"errno"`
	ErrorMsg  string       `json:"errmsg"`
	Data      interface{}  `json:"data"`
	TraceID   interface{}  `json:"trace_id"`
}

//ResponseError 错误输出
func ResponseError(c *gin.Context, code ResponseCode, err error) {
	trace, ok := c.Get("trace")
	traceID := ""
	if ok {
		traceContext := trace.(*lib.TraceContext)
		if traceContext != nil {
			traceID = traceContext.TraceId
		}
	}

	resp := &Response{ErrorCode: code, ErrorMsg: err.Error(), Data: "", TraceID: traceID}
	c.JSON(int(code), resp)
	response, jerr := json.Marshal(resp)
	if jerr != nil {
		// todo
	}
	c.Set("response", string(response))
	c.Abort()
	//c.AbortWithError(int(code), err)
}

//ResponseSuccess 正确输出
func ResponseSuccess(c *gin.Context, data interface{}) {
	trace, ok := c.Get("trace")
	traceID := ""
	if ok {
		traceContext := trace.(*lib.TraceContext)
		if traceContext != nil {
			traceID = traceContext.TraceId
		}
	}

	resp := &Response{ErrorCode: SuccessCode, ErrorMsg: "", Data: data, TraceID: traceID}
	c.JSON(200, resp)
	response, jerr := json.Marshal(resp)
	if jerr != nil {
		// todo
	}
	c.Set("response", string(response))
}

//HTTPError 错误输出
func HTTPError(errcode ResponseCode, message string, w http.ResponseWriter, r *http.Request) {
	var resp *Response
	resp = &Response{ErrorCode: errcode, ErrorMsg: message, Data: "", TraceID: 1}
	w.Header().Set("Content-Type", "application/json")
	response, jerr := json.Marshal(resp)
	if jerr != nil {
		// todo
	}
	http.Error(w, string(response), int(errcode))
}

//HTTPSuccess 正确输出
func HTTPSuccess(message string, w http.ResponseWriter, r *http.Request) {
	var resp *Response
	resp = &Response{ErrorCode: 0, ErrorMsg: "", Data: message, TraceID: 1}
	w.WriteHeader(200)
	w.Header().Set("Content-Type", "application/json")
	response, jerr := json.Marshal(resp)
	if jerr != nil {
		// todo
	}
	w.Write(response)
}
