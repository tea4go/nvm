// Package encoding 提供字符编码检测和转换功能
// 主要功能包括：
// - 检测字节内容的字符编码
// - 将字符串转换为UTF-8编码的字节数组
package encoding

import (
	"strings"
	"unicode/utf8"

	"github.com/saintfish/chardet"
)

// DetectCharset 检测字节内容的字符编码
// 参数:
//
//	content: 要检测的字节内容
//
// 返回值:
//
//	string: 检测到的字符编码名称(如UTF-8, GBK等)
//	error: 检测过程中遇到的错误
func DetectCharset(content []byte) (string, error) {
	detector := chardet.NewTextDetector()
	result, err := detector.DetectBest(content)
	if err != nil {
		return "", err
	}

	return strings.ToUpper(result.Charset), nil
}

// ToUTF8 将字符串转换为UTF-8编码的字节数组
// 参数:
//
//	content: 要转换的字符串
//
// 返回值: UTF-8编码的字节数组
func ToUTF8(content string) []byte {
	b := make([]byte, len(content))
	i := 0
	for _, r := range content {
		i += utf8.EncodeRune(b[i:], r)
	}

	return b[:i]
}

// func ToUTF8(content []byte, ignoreInvalidITF8Chars ...bool) (string, error) {
// 	ignore := false
// 	if len(ignoreInvalidITF8Chars) > 0 {
// 		ignore = ignoreInvalidITF8Chars[0]
// 	}

// 	cs, err := DetectCharset(content)
// 	if err != nil {
// 		if !ignore {
// 			return "", err
// 		}
// 		cs = "UTF-8"
// 	}

// 	bs := string(content)
// 	if ignore {
// 		if !utf8.ValidString(bs) {
// 			v := make([]rune, 0, len(bs))
// 			for i, r := range bs {
// 				if r == utf8.RuneError {
// 					_, size := utf8.DecodeRuneInString(bs[i:])
// 					if size == 1 {
// 						continue
// 					}
// 				}
// 				v = append(v, r)
// 			}
// 			bs = string(v)
// 		}
// 	}

// 	if cs == "UTF-8" {
// 		return bs, nil
// 	}

// 	converter, err := iconv.NewConverter(cs, "UTF-8")
// 	if err != nil {
// 		err = errors.New("Failed to convert " + cs + " to UTF-8: " + err.Error())
// 		return bs, err
// 	}

// 	return converter.ConvertString(bs)
// }
