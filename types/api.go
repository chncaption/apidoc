// Copyright 2017 by caixw, All rights reserved.
// Use of this source code is governed by a MIT
// license that can be found in the LICENSE file.

// Package types 一些公用类型的定义
package types

import (
	"errors"
	"net/http"
	"strings"

	"github.com/caixw/apidoc/types/openapi"
)

// API 文档内容
type API struct {
	API         string              // @api 后面的内容，包含了 method, url 和 summary
	Group       string              `yaml:"group,omitempty"`
	Tags        []string            `yaml:"tags,omitempty"`
	Description openapi.Description `yaml:"description,omitempty"`
	Deprecated  bool                `yaml:"deprecated,omitempty"`
	OperationID string              `yaml:"operationId,omitempty" `
	Queries     []string            `yaml:"queries,omitempty"`
	Params      []string            `yaml:"params,omitempty"`
	Headers     []string            `yaml:"header,omitempty"`
	Request     *Request            `yaml:"request,omitempty"` // GET 此值可能为空
	Responses   []*Response         `yaml:"responses"`
}

// Request 表示请求内容
type Request struct {
	Schema   *openapi.Schema                 `yaml:"schema"`
	Examples map[string]openapi.ExampleValue `yaml:"examples,omitempty"`
}

// Response 表示返回的内容
type Response struct {
	Schema   *openapi.Schema                 `yaml:"schema"`
	Examples map[string]openapi.ExampleValue `yaml:"examples,omitempty"`
}

func (doc *Doc) parseAPI(api *API) error {
	o, err := doc.getOperation(api)
	if err != nil {
		return err
	}

	o.Tags = api.Tags
	o.Description = api.Description
	o.Deprecated = api.Deprecated
	o.OperationID = api.OperationID

	if err := api.parseParameter(o); err != nil {
		return err
	}

	o.RequestBody = &openapi.RequestBody{
		// TODO
	}

	o.Responses = make(map[string]*openapi.Response, 3)
	// TODO

	return nil
}

func (api *API) parseParameter(o *openapi.Operation) error {
	// TODO
	return nil
}

func (doc *Doc) getOperation(api *API) (*openapi.Operation, error) {
	doc.locker.Lock()
	defer doc.locker.Unlock()

	if doc.OpenAPI.Paths == nil {
		doc.OpenAPI.Paths = make(map[string]*openapi.PathItem, 10)
	}

	strs := strings.SplitN(api.API, " ", 3)
	if len(strs) != 3 {
		return nil, errors.New("缺少参数")
	}

	path, found := doc.OpenAPI.Paths[strs[1]]
	if !found {
		path = &openapi.PathItem{}
		doc.OpenAPI.Paths[strs[1]] = path
	}

	switch strings.ToUpper(strs[0]) {
	case http.MethodGet:
		if path.Get != nil {
			return nil, errors.New("已经存在一个相同的 Get 路由")
		}
		path.Get = &openapi.Operation{}
		return path.Get, nil
	case http.MethodDelete:
		if path.Delete != nil {
			return nil, errors.New("已经存在一个相同的 Delete 路由")
		}
		path.Delete = &openapi.Operation{}
		return path.Delete, nil
	case http.MethodPost:
		if path.Post != nil {
			return nil, errors.New("已经存在一个相同的 Post 路由")
		}
		path.Post = &openapi.Operation{}
		return path.Post, nil
	case http.MethodPut:
		if path.Put != nil {
			return nil, errors.New("已经存在一个相同的 Put 路由")
		}
		path.Put = &openapi.Operation{}
		return path.Put, nil
	case http.MethodPatch:
		if path.Patch != nil {
			return nil, errors.New("已经存在一个相同的 Patch 路由")
		}
		path.Patch = &openapi.Operation{}
		return path.Patch, nil
	case http.MethodOptions:
		if path.Options != nil {
			return nil, errors.New("已经存在一个相同的 Options 路由")
		}
		path.Options = &openapi.Operation{}
		return path.Options, nil
	case http.MethodHead:
		if path.Head != nil {
			return nil, errors.New("已经存在一个相同的 Head 路由")
		}
		path.Head = &openapi.Operation{}
		return path.Head, nil
	case http.MethodTrace:
		if path.Trace != nil {
			return nil, errors.New("已经存在一个相同的 Trace 路由")
		}
		path.Trace = &openapi.Operation{}
		return path.Trace, nil
	}

	return nil, errors.New("无效的请法语方法")
}
