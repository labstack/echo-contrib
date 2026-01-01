// SPDX-License-Identifier: MIT
// SPDX-FileCopyrightText: © 2017 LabStack and Echo contributors

package jaegertracing

import (
	"bytes"
	"io"
	"net/http"
)

type responseDumper struct {
	http.ResponseWriter

	mw  io.Writer
	buf *bytes.Buffer
}

func newResponseDumper(resp http.ResponseWriter) *responseDumper {
	buf := new(bytes.Buffer)
	return &responseDumper{
		ResponseWriter: resp,

		mw:  io.MultiWriter(resp, buf),
		buf: buf,
	}
}

func (d *responseDumper) Write(b []byte) (int, error) {
	return d.mw.Write(b)
}

func (d *responseDumper) GetResponse() string {
	return d.buf.String()
}

func (d *responseDumper) Unwrap() http.ResponseWriter {
	return d.ResponseWriter
}
