package accesslog

import (
	"bytes"
	"context"
	"github.com/gin-gonic/gin"
	"io"
	"time"

	"go.uber.org/atomic"
)

type AccessLog struct {
	// HTTP请求方法, GET, POST, PUT, DELETE
	Method string
	// 请求的URL
	URL string
	// 请求的body
	ReqBody string
	// 响应的body
	RespBody string
	// HTTP状态码
	Status int
	// 请求耗时
	Duration string
}

type Builder struct {
	// 是否允许记录请求body
	allowReqBody *atomic.Bool
	// 是否允许记录响应body
	allowRespBody *atomic.Bool

	// 记录日志的函数
	loggerFunc func(ctx context.Context, al *AccessLog)

	maxLength *atomic.Int64
}

func NewBuilder(fn func(ctx context.Context, al *AccessLog)) *Builder {
	return &Builder{
		loggerFunc:    fn,
		allowReqBody:  atomic.NewBool(false),
		allowRespBody: atomic.NewBool(false),
		maxLength:     atomic.NewInt64(1024),
	}
}

// AllowReqBody 是否打印请求体
func (b *Builder) AllowReqBody() *Builder {
	b.allowReqBody.Store(true)
	return b
}

// AllowRespBody 是否打印响应体
func (b *Builder) AllowRespBody() *Builder {
	b.allowRespBody.Store(true)
	return b
}

// MaxLength 打印的最大长度
func (b *Builder) MaxLength(maxLength int64) *Builder {
	b.maxLength.Store(maxLength)
	return b
}

func (b *Builder) Builder() gin.HandlerFunc {
	return func(ctx *gin.Context) {
		var (
			// 记录请求开始时间
			start = time.Now()
			// 记录请求url
			url = ctx.Request.URL.String()
			// url的长度
			curLen = int64(len(url))
			// 运行打印的最大长度
			maxLength = b.maxLength.Load()
			// 是否打印请求体
			allowReqBody = b.allowReqBody.Load()
			// 是否打印响应体
			allowRespBody = b.allowRespBody.Load()
		)
		// 如果url长度超过最大长度，截取0-maxLength位置
		if curLen > maxLength {
			url = url[:maxLength]
		}

		accessLog := &AccessLog{
			Method: ctx.Request.Method,
			URL:    url,
		}

		// 记录请求体
		if ctx.Request.Body != nil && allowReqBody {
			// 读取body
			body, _ := ctx.GetRawData()
			// 读取完body后，需要重新写入
			ctx.Request.Body = io.NopCloser(bytes.NewReader(body))
			// 如果body长度超过最大长度，截取0-maxLength位置
			if int64(len(body)) > maxLength {
				body = body[:maxLength]
			}
			// 注意资源的消耗
			accessLog.ReqBody = string(body)
		}
		if allowRespBody {
			ctx.Writer = responseWriter{
				ResponseWriter: ctx.Writer,
				al:             accessLog,
				maxLength:      maxLength,
			}
		}
		defer func() {
			accessLog.Duration = time.Since(start).String()
			// 日志打印
			b.loggerFunc(ctx, accessLog)
		}()

		// 执行下一个中间件
		ctx.Next()

	}
}

// responseWriter 重写gin.ResponseWriter的Write方法，用于记录响应体
type responseWriter struct {
	gin.ResponseWriter
	al        *AccessLog
	maxLength int64
}

func (r responseWriter) WriteHeader(statusCode int) {
	r.al.Status = statusCode
	r.ResponseWriter.WriteHeader(statusCode)
}
func (r responseWriter) Write(data []byte) (int, error) {
	curLen := int64(len(data))
	if curLen >= r.maxLength {
		data = data[:r.maxLength]
	}
	r.al.RespBody = string(data)
	return r.ResponseWriter.Write(data)
}
