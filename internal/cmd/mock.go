// SPDX-License-Identifier: MIT

package cmd

import (
	"bytes"
	"flag"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/caixw/apidoc/v5/internal/locale"
	"github.com/caixw/apidoc/v5/internal/mock"
	"github.com/caixw/apidoc/v5/message"
)

var mockFlagSet *flag.FlagSet

// servers 参数
type servers map[string]string

func (srv servers) Get() interface{} {
	return map[string]string(srv)
}

func (srv servers) Set(v string) error {
	if index := strings.IndexByte(v, ','); index <= 0 {
		return locale.Errorf(locale.ErrInvalidFormat)
	}

	pairs := strings.Split(v, ",")
	for _, pair := range pairs {
		index := strings.IndexByte(pair, '=')
		if index <= 0 {
			return locale.Errorf(locale.ErrInvalidValue)
		}

		var v string
		if index < len(pair) {
			v = pair[index+1:]
		}
		srv[strings.TrimSpace(pair[:index])] = v
	}

	return nil
}

func (srv servers) String() string {
	if len(srv) == 0 {
		return ""
	}

	buf := new(bytes.Buffer)

	for k, v := range srv {
		buf.WriteString(k)
		buf.WriteByte('=')
		buf.WriteString(v)
		buf.WriteByte(',')
	}
	buf.Truncate(buf.Len() - 1)
	return buf.String()
}

var mockPort string
var mockServers servers = make(servers, 0)

func initMock() {
	mockFlagSet = command.New("mock", doMock, mockUsage)
	mockFlagSet.StringVar(&mockPort, "p", ":8080", locale.Sprintf(locale.FlagMockPortUsage))
	mockFlagSet.Var(mockServers, "s", locale.Sprintf(locale.FlagMockServersUsage))
}

func doMock(io.Writer) error {
	path := getPath(mockFlagSet)

	h := message.NewHandler(newHandlerFunc())
	defer h.Stop()

	handler, err := mock.Load(h, path, mockServers)
	if err != nil {
		return err
	}

	return http.ListenAndServe(mockPort, handler)
}

func mockUsage(w io.Writer) error {
	buf := new(bytes.Buffer)
	mockFlagSet.SetOutput(buf)
	mockFlagSet.PrintDefaults()
	mockFlagSet.SetOutput(w)

	_, err := fmt.Fprintln(w, locale.Sprintf(locale.CmdMockUsage, buf.String()))
	return err
}
