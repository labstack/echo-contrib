// SPDX-License-Identifier: MIT
// SPDX-FileCopyrightText: © 2017 LabStack and Echo contributors

package jaegertracing

import (
	"bytes"
	"io"
	"net/http"

	"github.com/labstack/echo/v4"
)

type responseDumper struct {
	http.ResponseWriter

	mw  io.Writer
	buf *bytes.Buffer
}

func newResponseDumper(resp *echo.Response) *responseDumper {
	buf := new(bytes.Buffer)
	return &responseDumper{
		ResponseWriter: resp.Writer,

		mw:  io.MultiWriter(resp.Writer, buf),
		buf: buf,
	}
}

func (d *responseDumper) Write(b []byte) (int, error) {
	return d.mw.Write(b)
}

func (d *responseDumper) GetResponse() string {
	return d.buf.String()
}
