// SPDX-License-Identifier: MIT

package token

import (
	"fmt"
	"io"
	"reflect"

	"github.com/caixw/apidoc/v7/core"
	"github.com/caixw/apidoc/v7/internal/locale"
)

// Decoder 实现从 p 中解码内容到当前对象的值
type Decoder interface {
	// 从 p 中读取内容并实例化到当前对象中
	//
	// 必须要同时从 p 中读取相应的 EndElement 才能返回。
	// end 表示 EndElement.End 的值。
	//
	// NOTE: 如果是自闭合标签，则不会调用该接口。
	DecodeXML(p *Parser, start *StartElement) (end *EndElement, err error)
}

// AttrDecoder 实现从 attr 中解码内容到当前对象的值
type AttrDecoder interface {
	DecodeXMLAttr(attr *Attribute) error
}

var (
	attrDecoderType = reflect.TypeOf((*AttrDecoder)(nil)).Elem()
	decoderType     = reflect.TypeOf((*Decoder)(nil)).Elem()
)

// Decode 将 p 中的 XML 内容解码至 v 对象中
func Decode(p *Parser, v interface{}) error {
	var hasRoot bool
	for {
		t, err := p.Token()
		if err == io.EOF {
			return nil
		} else if err != nil {
			return err
		}

		switch elem := t.(type) {
		case *StartElement:
			if hasRoot { // 多个根元素
				return p.NewError(elem.Start, elem.End, locale.ErrInvalidXML)
			}
			hasRoot = true

			o := newNode(elem.Name.Value, reflect.ValueOf(v))
			if _, err := o.decode(p, elem); err != nil {
				return err
			}
		case *EndElement:
			return p.NewError(elem.Start, elem.End, locale.ErrInvalidXML)
		}
	}
}

func (n *node) decode(p *Parser, start *StartElement) (*EndElement, error) {
	if err := n.decodeAttributes(start); err != nil {
		return nil, err
	}

	if start.Close {
		return nil, nil
	}

	return n.decodeElements(p)
}

// 将 start 的属性内容解码到 obj.attrs 之中
func (n *node) decodeAttributes(start *StartElement) error {
	if start == nil {
		return nil
	}

	for _, attr := range start.Attributes {
		item, found := n.attr(attr.Name.Value)
		if !found {
			continue
		}
		v := getRealValue(item.Value)
		v.Set(reflect.New(v.Type()).Elem())

		var impl bool
		if item.CanInterface() && item.Type().Implements(attrDecoderType) {
			if err := item.Interface().(AttrDecoder).DecodeXMLAttr(attr); err != nil {
				return err
			}
			impl = true
		} else if item.CanAddr() {
			pv := item.Addr()
			if pv.CanInterface() && pv.Type().Implements(attrDecoderType) {
				if err := pv.Interface().(AttrDecoder).DecodeXMLAttr(attr); err != nil {
					return err
				}
				impl = true
			}
		}

		if !impl {
			panic(fmt.Sprintf("当前属性 %s 未实现 AttrDecoder 接口", attr.Name.Value))
		}

		setValue(item.Value, item.usage, attr.Start, attr.End, attr.Name, String{})
	}

	return nil
}

func (n *node) decodeElements(p *Parser) (*EndElement, error) {
	for {
		t, err := p.Token()
		if err == io.EOF {
			// 应该只有 EndElement 才能返回，否则就不完整的 XML
			return nil, p.NewError(p.Position().Position, p.Position().Position, locale.ErrInvalidXML)
		} else if err != nil {
			return nil, err
		}

		switch elem := t.(type) {
		case *EndElement: // 找到当前对象的结束标签
			if elem.Name.Value != n.name {
				return nil, p.NewError(elem.Start, elem.End, locale.ErrInvalidXML)
			}
			return elem, nil
		case *CData:
			if n.cdata.IsValid() {
				getRealValue(n.cdata.Value).Set(getRealValue(reflect.ValueOf(elem)))
				setValue(n.cdata.Value, n.cdata.usage, elem.Start, elem.End, String{}, String{})
			}
		case *String:
			if n.content.IsValid() {
				getRealValue(n.content.Value).Set(getRealValue(reflect.ValueOf(elem)))
				setValue(n.content.Value, n.content.usage, elem.Start, elem.End, String{}, String{})
			}
		case *StartElement:
			item, found := n.elem(elem.Name.Value)
			if !found {
				panic(fmt.Sprintf("不存在的子元素 %s", elem.Name.Value))
			}

			if err = decodeElement(p, elem, item); err != nil {
				return nil, err
			}
		}
	} // end for
}

func decodeElement(p *Parser, start *StartElement, v value) (err error) {
	v.Value = getRealValue(v.Value)
	k := v.Kind()
	switch {
	case k == reflect.Ptr, k == reflect.Func, k == reflect.Chan, k == reflect.Array, isPrimitive(v.Value):
		panic(fmt.Sprintf("%s 是无效的类型", v.Value.Type()))
	case k == reflect.Slice:
		return decodeSlice(p, start, v)
	}

	end, impl, err := callDecodeXML(v.Value, p, start)
	if !impl {
		end, err = newNode(start.Name.Value, v.Value).decode(p, start)
	}

	if err != nil {
		return err
	}
	setElementValue(v.Value, v.usage, start, end)
	return nil
}

func decodeSlice(p *Parser, start *StartElement, slice value) (err error) {
	// 不相配，表示当前元素找不到与之相配的元素，需要忽略这个元素，
	// 所以要过滤与 start 想匹配的结束符号才算结束。
	if !start.Close && (start.Name.Value != slice.name) {
		return findEndElement(p, start)
	}

	elem := reflect.New(slice.Type().Elem()).Elem()
	if elem.Kind() == reflect.Ptr && elem.IsNil() {
		elem.Set(reflect.New(elem.Type().Elem()))
	}

	end, impl, err := callDecodeXML(elem, p, start)
	if !impl {
		if isPrimitive(elem) {
			panic(fmt.Sprintf("%s:%s 必须实现 Decoder 接口", slice.name, elem.Type()))
		}
		end, err = newNode(start.Name.Value, elem).decode(p, start)
	}

	if err != nil {
		return err
	}
	setElementValue(elem, slice.usage, start, end)
	slice.Value.Set(reflect.Append(slice.Value, elem))
	return nil
}

// 调用 v 的 DecodeXML 接口方法
func callDecodeXML(v reflect.Value, p *Parser, start *StartElement) (end *EndElement, impl bool, err error) {
	if v.CanInterface() && v.Type().Implements(decoderType) {
		if !start.Close {
			end, err = v.Interface().(Decoder).DecodeXML(p, start)
		}
		return end, true, err
	} else if v.CanAddr() {
		pv := v.Addr()
		if pv.CanInterface() && pv.Type().Implements(decoderType) {
			if !start.Close {
				end, err = pv.Interface().(Decoder).DecodeXML(p, start)
			}
			return end, true, err
		}
	}
	return nil, false, nil
}

// 找到与 start 相对应的结束符号位置
func findEndElement(p *Parser, start *StartElement) error {
	level := 0
	for {
		t, err := p.Token()
		if err == io.EOF {
			return p.NewError(start.Start, start.End, locale.ErrInvalidXML) // 找不到相配的结束符号
		} else if err != nil {
			return err
		}

		switch elem := t.(type) {
		case *StartElement:
			if elem.Name.Value == start.Name.Value {
				level++
			}
		case *EndElement:
			if level == 0 && (elem.Name.Value == start.Name.Value) {
				return nil
			}
			level--
		}
	}
}

func setElementValue(v reflect.Value, usage string, start *StartElement, end *EndElement) {
	if end == nil {
		setValue(v, usage, start.Start, start.End, start.Name, String{})
		return
	}
	setValue(v, usage, start.Start, end.End, start.Name, end.Name)
}

func setValue(v reflect.Value, usage string, start, end core.Position, xmlName, xmlNameEnd String) {
	v = getRealValue(v)
	if v.Kind() != reflect.Struct {
		panic(fmt.Sprintf("无效的 kind 类型: %s:%s", v.Type(), v.Kind()))
	}

	v.FieldByName(rangeName).Set(reflect.ValueOf(core.Range{Start: start, End: end}))

	if usage != "" { // CDATA 和 content 节点类型的 usage 内容为空
		v.FieldByName(usageKeyName).Set(reflect.ValueOf(usage))
	}

	if xmlName.Value != "" { // CDATA 和 content 节点的 XMLName 肯定为空
		v.FieldByName(elementTagName).Set(reflect.ValueOf(xmlName))
	}

	if xmlNameEnd.Value != "" {
		v.FieldByName(elementTagEndName).Set(reflect.ValueOf(xmlNameEnd))
	}
}