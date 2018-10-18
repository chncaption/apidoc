// Copyright 2018 by caixw, All rights reserved.
// Use of this source code is governed by a MIT
// license that can be found in the LICENSE file.

package syntax

import (
	"github.com/caixw/apidoc/locale"
	"golang.org/x/text/message"
)

// Error 语法错误类型
type Error struct {
	File        string
	Line        int
	MessageKey  message.Reference
	MessageArgs []interface{}
}

func (err *Error) Error() string {
	msg := locale.Sprintf(err.MessageKey, err.MessageArgs...)
	return locale.Sprintf(locale.ErrSyntax, err.File, err.Line, msg)
}

func newError(file string, line int, msg message.Reference, vals ...interface{}) error {
	return &Error{
		File:        file,
		Line:        line,
		MessageKey:  msg,
		MessageArgs: vals,
	}
}

// Error 输出语言错误内容
func (t *Tag) Error(key message.Reference, vals ...interface{}) error {
	return newError(t.File, t.Line, key, vals...)
}

// ErrInvalidFormat 返回格式无效的错误信息
func (t *Tag) ErrInvalidFormat() error {
	return t.Error(locale.ErrInvalidFormat, string(t.Name))
}

// ErrDuplicateTag 返回标签重复的错误信息
func (t *Tag) ErrDuplicateTag() error {
	return t.Error(locale.ErrDuplicateTag, string(t.Name))
}

// ErrInvalidTag 返回无效的标签错误
func (t *Tag) ErrInvalidTag() error {
	return t.Error(locale.ErrInvalidTag, string(t.Name))
}
