// SPDX-License-Identifier: MIT

package main

import (
	"io/ioutil"
	"os"
	"path/filepath"
	"strconv"

	"github.com/issue9/version"
	"gopkg.in/yaml.v2"

	"github.com/caixw/apidoc/v5"
	"github.com/caixw/apidoc/v5/input"
	"github.com/caixw/apidoc/v5/internal/locale"
	"github.com/caixw/apidoc/v5/message"
	"github.com/caixw/apidoc/v5/output"
)

const (
	configFilename = ".apidoc.yaml"
	docFilename    = "apidoc.xml"
)

// 项目的配置内容
type config struct {
	// 产生此配置文件的程序版本号
	//
	// 程序会用此来判断程序的兼容性。
	Version string `yaml:"version"`

	// 输入的配置项，可以指定多个项目
	//
	// 多语言项目，可能需要用到多个输入面。
	Inputs []*input.Options `yaml:"inputs"`

	// 输出配置项
	Output *output.Options `yaml:"output"`

	// 配置文件所在的目录
	//
	// 如果 input 和 output 中涉及到地址为非绝对目录，则使用此值作为基地址。
	wd string
}

// 加载 wd 目录下的配置文件到 *config 实例。
func loadConfig(wd string) (*config, *message.SyntaxError) {
	data, err := ioutil.ReadFile(filepath.Join(wd, configFilename))
	if err != nil {
		return nil, message.WithError(configFilename, "", 0, err)
	}

	cfg := &config{}
	if err = yaml.Unmarshal(data, cfg); err != nil {
		return nil, message.WithError(configFilename, "", 0, err)
	}
	cfg.wd = wd

	if err := cfg.sanitize(); err != nil {
		return nil, err
	}

	return cfg, nil
}

func (cfg *config) sanitize() *message.SyntaxError {
	if !version.SemVerValid(cfg.Version) {
		return message.NewLocaleError(configFilename, "version", 0, locale.ErrInvalidFormat)
	}

	// 比较版本号兼容问题
	compatible, err := version.SemVerCompatible(apidoc.Version(), cfg.Version)
	if err != nil {
		return message.WithError(configFilename, "version", 0, err)
	}
	if !compatible {
		return message.NewLocaleError(configFilename, "version", 0, locale.VersionInCompatible)
	}

	if len(cfg.Inputs) == 0 {
		return message.NewLocaleError(configFilename, "inputs", 0, locale.ErrRequired)
	}

	if cfg.Output == nil {
		return message.NewLocaleError(configFilename, "output", 0, locale.ErrRequired)
	}

	for index, i := range cfg.Inputs {
		field := "inputs[" + strconv.Itoa(index) + "]"

		if i.Dir, err = abs(i.Dir, cfg.wd); err != nil {
			return message.WithError(configFilename, field+".path", 0, err)
		}
	}

	if !filepath.IsAbs(cfg.Output.Path) {
		if cfg.Output.Path, err = abs(cfg.Output.Path, cfg.wd); err != nil {
			return message.WithError(configFilename, "output.path", 0, err)
		}
	}

	return nil
}

func fixedSyntaxError(err *message.SyntaxError, field string) *message.SyntaxError {
	err.File = configFilename
	if err.Field == "" {
		err.Field = field
	} else {
		err.Field = field + "." + err.Field
	}
	return err
}

func getConfig(wd string) (*config, error) {
	inputs, err := input.Detect(wd, true)
	if err != nil {
		return nil, err
	}
	if len(inputs) == 0 {
		return nil, message.NewLocaleError("", "", 0, locale.ErrNotFoundSupportedLang)
	}

	for _, i := range inputs {
		i.Dir, err = filepath.Rel(wd, i.Dir)
		if err != nil {
			return nil, err
		}
	}

	outputFile, err := filepath.Rel(wd, filepath.Join(wd, docFilename))
	if err != nil {
		return nil, err
	}
	return &config{
		Version: apidoc.Version(),
		Inputs:  inputs,
		Output: &output.Options{
			Type: output.ApidocXML,
			Path: outputFile,
		},
	}, nil
}

// 根据 wd 所在目录的内容生成一个配置文件，并写入到 path  中
//
// wd 表示当前程序的工作目录，根据此目录的内容检测其语言特性。
// path 表示生成的配置文件存放的路径。
func generateConfig(wd, path string) error {
	cfg, err := getConfig(wd)
	if err != nil {
		return err
	}

	data, err := yaml.Marshal(cfg)
	if err != nil {
		return err
	}

	return ioutil.WriteFile(path, data, os.ModePerm)
}
