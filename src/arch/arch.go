// Package arch 提供与系统架构相关的功能
// 主要功能包括：
// - 检测可执行文件的架构类型
// Package arch 提供与系统架构相关的功能
// 主要功能包括：
// - 检测可执行文件的架构类型
// - 验证和规范化架构字符串
package arch

import (
	"encoding/hex"
	"os"
	"strings"
)

// SearchBytesInFile 在文件中搜索指定的字节序列
// 参数:
//
//	path: 文件路径
//	match: 要匹配的16进制字符串
//	limit: 最大搜索字节数
//
// 返回值: 是否找到匹配
func SearchBytesInFile(path string, match string, limit int) bool {
	// 将16进制字符串转换为字节数组
	toMatch, err := hex.DecodeString(match)
	if err != nil {
		return false
	}

	// 打开文件
	file, err := os.Open(path)
	if err != nil {
		return false
	}

	// 确保文件关闭
	defer file.Close()

	// 创建1字节缓冲区用于读取
	bit := make([]byte, 1)
	j := 0
	for i := 0; i < limit; i++ {
		file.Read(bit)

		// 匹配失败则重置匹配位置
		if bit[0] != toMatch[j] {
			j = 0
		}
		// 匹配成功则前进位置
		if bit[0] == toMatch[j] {
			j++
			// 完全匹配则返回成功
			if j >= len(toMatch) {
				file.Close()
				return true
			}
		}
	}
	file.Close()
	return false
}

// Bit 检测可执行文件的架构类型
// 参数:
//
//	path: 可执行文件路径
//
// 返回值: 架构类型("arm64"/"64"/"32"/"?")
func Bit(path string) string {
	// 通过文件头特征检测架构类型
	isarm64 := SearchBytesInFile(path, "5045000064AA", 400)
	is64 := SearchBytesInFile(path, "504500006486", 400)
	is32 := SearchBytesInFile(path, "504500004C", 400)
	if isarm64 {
		return "arm64"
	} else if is64 {
		return "64"
	} else if is32 {
		return "32"
	}
	return "?"
}

// Validate 验证和规范化架构字符串
// 参数:
//
//	str: 原始架构字符串
//
// 返回值: 规范化后的架构("arm64"/"64"/"32")
func Validate(str string) string {
	// 如果未提供则从环境变量获取
	if str == "" {
		str = strings.ToLower(os.Getenv("PROCESSOR_ARCHITECTURE"))
	}
	// 检查ARM64架构
	if strings.Contains(str, "arm64") {
		return "arm64"
	}
	// 检查64位架构
	if strings.Contains(str, "64") {
		return "64"
	}
	// 默认为32位
	return "32"
}
