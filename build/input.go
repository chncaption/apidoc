// SPDX-License-Identifier: MIT

package build

import (
	"os"
	"path/filepath"
	"sync"

	"github.com/issue9/utils"
	"golang.org/x/text/encoding"
	"golang.org/x/text/encoding/ianaindex"

	"github.com/caixw/apidoc/v6/core"
	"github.com/caixw/apidoc/v6/internal/lang"
	"github.com/caixw/apidoc/v6/internal/locale"
	"github.com/caixw/apidoc/v6/spec"
)

// Input 指定输入内容的相关信息。
type Input struct {
	// 输入的目标语言
	//
	// 取值为 lang.Language.Name
	Lang string `yaml:"lang"`

	// 源代码目录
	Dir string `yaml:"dir"`

	// 需要扫描的文件扩展名
	//
	// 若未指定，则根据 Lang 选项获取其默认的扩展名作为过滤条件。
	Exts []string `yaml:"exts,omitempty"`

	// 是否查找 Dir 的子目录
	Recursive bool `yaml:"recursive"`

	// 源文件的编码，默认为 UTF-8
	Encoding string `yaml:"encoding,omitempty"`

	blocks   []lang.Blocker    // 根据 Lang 生成
	paths    []core.URI        // 根据 Dir、Exts 和 Recursive 生成
	encoding encoding.Encoding // 根据 Encoding 生成
}

// Sanitize 验证参数正确性
func (o *Input) Sanitize() error {
	if o == nil {
		return core.NewLocaleError("", "", 0, locale.ErrRequired)
	}

	if len(o.Dir) == 0 {
		return core.NewLocaleError("", "dir", 0, locale.ErrRequired)
	}

	if !utils.FileExists(o.Dir) {
		return core.NewLocaleError("", "dir", 0, locale.ErrDirNotExists)
	}

	if len(o.Lang) == 0 {
		return core.NewLocaleError("", "lang", 0, locale.ErrRequired)
	}

	language := lang.Get(o.Lang)
	if language == nil {
		return core.NewLocaleError("", "lang", 0, locale.ErrInvalidValue)
	}
	o.blocks = language.Blocks

	if len(o.Exts) > 0 {
		exts := make([]string, 0, len(o.Exts))
		for _, ext := range o.Exts {
			if len(ext) == 0 {
				continue
			}

			if ext[0] != '.' {
				ext = "." + ext
			}
			exts = append(exts, ext)
		}
		o.Exts = exts
	} else {
		o.Exts = language.Exts
	}

	// 生成 paths
	paths, err := recursivePath(o)
	if err != nil {
		return core.WithError("", "dir", 0, err)
	}
	if len(paths) == 0 {
		return core.NewLocaleError("", "dir", 0, locale.ErrDirIsEmpty)
	}
	o.paths = paths

	// 生成 encoding
	if o.Encoding != "" {
		o.encoding, err = ianaindex.IANA.Encoding(o.Encoding)
		if err != nil {
			return core.WithError("", "encoding", 0, err)
		}
	}

	return nil
}

// 按 Input 中的规则查找所有符合条件的文件列表。
func recursivePath(o *Input) ([]core.URI, error) {
	var uris []core.URI

	extIsEnabled := func(ext string) bool {
		for _, v := range o.Exts {
			if ext == v {
				return true
			}
		}
		return false
	}

	walk := func(path string, fi os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if fi.IsDir() && !o.Recursive && path != o.Dir {
			return filepath.SkipDir
		} else if extIsEnabled(filepath.Ext(path)) {
			uri, err := core.FileURI(path)
			if err != nil {
				return err
			}
			uris = append(uris, uri)
		}
		return nil
	}

	if err := filepath.Walk(o.Dir, walk); err != nil {
		return nil, err
	}

	return uris, nil
}

// 分析 opt 中所指定的内容
//
// 分析后的内容推送至 blocks 中。
func parseInputs(blocks chan spec.Block, h *core.MessageHandler, opt ...*Input) {
	wg := &sync.WaitGroup{}
	for _, o := range opt {
		for _, path := range o.paths {
			wg.Add(1)
			go func(path core.URI, o *Input) {
				o.parseFile(blocks, h, path)
				wg.Done()
			}(path, o)
		}
	}
	wg.Wait()
}

// 分析 uri 指向的文件
func (o *Input) parseFile(blocks chan spec.Block, h *core.MessageHandler, uri core.URI) {
	data, err := uri.ReadAll(o.encoding)
	if err != nil {
		h.Error(core.Erro, core.WithError(string(uri), "", 0, err))
		return
	}

	l, err := lang.NewLexer(data, o.blocks)
	if err != nil {
		h.Error(core.Erro, core.WithError(string(uri), "", 0, err))
		return
	}

	var block lang.Blocker
	var pos core.Position
	for {
		if l.AtEOF() {
			return
		}

		if block == nil {
			if block, pos = l.Block(); block == nil { // 没有匹配的 block 了
				return
			}
		}

		raw, data, ok := block.EndFunc(l)
		if !ok { // 没有找到结束标签，那肯定是到文件尾了，可以直接返回。
			h.Error(core.Erro, core.NewLocaleError(string(uri), "", pos.Line, locale.ErrNotFoundEndFlag))
			return
		}

		block = nil // 重置 block

		if len(raw) == 0 {
			continue
		}

		blocks <- spec.Block{
			File: string(uri),
			Range: core.Range{
				Start: pos,
				End:   l.Position(),
			},
			Data: data,
			Raw:  raw,
		}
	} // end for
}