package jaegertracing

import (
	"bytes"
	"io"
	"net/http"

	"github.com/labstack/echo/v4"
)

type ResponseDumper struct {
	http.ResponseWriter

	mw  io.Writer
	buf *bytes.Buffer
}

func NewResponseDumper(resp *echo.Response) *ResponseDumper {
	buf := new(bytes.Buffer)
	return &ResponseDumper{
		ResponseWriter: resp.Writer,

		mw:  io.MultiWriter(resp.Writer, buf),
		buf: buf,
	}
}

func (d *ResponseDumper) Write(b []byte) (int, error) {
	return d.mw.Write(b)
}

func (r *ResponseDumper) GetResponse() string {
	return r.buf.String()
}
