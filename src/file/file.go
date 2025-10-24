// Package file 提供文件操作相关功能
// 主要功能包括：
// - 解压zip文件
// - 按行读取文件内容
// - 检查文件是否存在
package file

import (
	"archive/zip"
	"bufio"
	"io"
	"log"
	"os"
	"path/filepath"
	"strings"
)

// Unzip 解压zip文件到指定目录
// 参数:
//
//	src: zip文件路径
//	dest: 解压目标目录
//
// 返回值: 解压过程中遇到的错误
// 注意: 防止目录遍历攻击，拒绝包含".."的路径
// Unzip 解压zip文件到目标目录
func Unzip(src, dest string) error {
	// 打开zip文件
	r, err := zip.OpenReader(src)
	if err != nil {
		return err
	}
	defer r.Close()

	// 遍历zip中的文件
	for _, f := range r.File {
		// 安全检查：防止路径穿越攻击
		if !strings.Contains(f.Name, "..") {
			// 打开zip中的文件
			rc, err := f.Open()
			if err != nil {
				return err
			}
			defer rc.Close()

			// 构建目标路径
			fpath := filepath.Join(dest, f.Name)
			if f.FileInfo().IsDir() {
				// 创建目录
				os.MkdirAll(fpath, f.Mode())
			} else {
				// 获取文件所在目录
				var fdir string
				if lastIndex := strings.LastIndex(fpath, string(os.PathSeparator)); lastIndex > -1 {
					fdir = fpath[:lastIndex]
				}

				// 创建父目录
				err = os.MkdirAll(fdir, f.Mode())
				if err != nil {
					log.Fatal(err)
					return err
				}
				// 创建目标文件
				f, err := os.OpenFile(fpath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, f.Mode())
				if err != nil {
					return err
				}
				defer f.Close()

				// 复制文件内容
				_, err = io.Copy(f, rc)
				if err != nil {
					return err
				}
			}
		} else {
			// 记录无效文件
			log.Printf("failed to extract file: %s (cannot validate)\n", f.Name)
		}
	}

	return nil
}

// ReadLines 按行读取文件内容
// 参数:
//
//	path: 文件路径
//
// 返回值:
//
//	[]string: 文件各行内容
//	error: 读取过程中遇到的错误
//
// ReadLines 读取指定文件的所有行
// path: 文件路径
// 返回: 字符串切片(每行内容)和可能的错误
func ReadLines(path string) ([]string, error) {
	// 打开文件
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	// 确保文件关闭
	defer file.Close()

	// 初始化行切片
	var lines []string
	// 创建文件扫描器
	scanner := bufio.NewScanner(file)
	// 逐行扫描文件
	for scanner.Scan() {
		lines = append(lines, scanner.Text())
	}
	// 返回行内容和可能的扫描错误
	return lines, scanner.Err()
}

// Exists 检查文件是否存在
// 参数:
//
//	filename: 文件路径
//
// 返回值: 文件是否存在
func Exists(filename string) bool {
	_, err := os.Stat(filename)
	return err == nil
}
