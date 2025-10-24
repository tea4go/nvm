// Package node 提供Node.js版本管理相关功能
// 主要功能包括：
// - 获取当前安装的Node.js版本信息
// - 检查版本是否已安装/可用
// - 管理已安装版本列表
// - 获取远程可用版本信息
package node

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"nvm/arch"
	"nvm/file"
	"nvm/web"
	"os"
	"os/exec"
	"regexp"
	"strings"

	// "../semver"
	"github.com/blang/semver"
)

// GetCurrentVersion 获取当前使用的Node.js版本和架构信息
// 返回值:
//
//	string: 版本号(如"12.18.3")，如果获取失败返回"Unknown"
//	string: 架构("32"/"64"/"arm64")，如果获取失败返回空字符串
func GetCurrentVersion() (string, string) {
	// 获取Node.js版本号
	cmd := exec.Command("node", "-v")
	str, err := cmd.Output()
	if err == nil {
		// 清理版本号字符串，去除"v"前缀和后续描述
		v := strings.Trim(regexp.MustCompile("-.*$").ReplaceAllString(regexp.MustCompile("v").ReplaceAllString(strings.Trim(string(str), " \n\r"), ""), ""), " \n\r")

		// 获取Node.js可执行文件路径
		cmd := exec.Command("node", "-p", "console.log(process.execPath)")
		str, _ := cmd.Output()
		file := strings.Trim(regexp.MustCompile("undefined").ReplaceAllString(string(str), ""), " \n\r")

		// 通过文件路径获取架构信息
		bit := arch.Bit(file)
		if bit == "?" {
			// 如果无法通过文件获取架构，则直接查询Node.js进程架构
			cmd := exec.Command("node", "-e", "console.log(process.arch)")
			str, err := cmd.Output()
			if err == nil {
				if string(str) == "x64" {
					bit = "64"
				} else if string(str) == "arm64" {
					bit = "arm64"
				} else {
					bit = "32"
				}
			} else {
				return v, "Unknown"
			}
		}
		return v, bit
	}
	return "Unknown", ""
}

// IsVersionInstalled 检查指定版本的Node.js是否已安装
// 参数:
//
//	root: NVM安装根目录
//	version: 要检查的版本号
//	cpu: 架构类型("32"/"64"/"arm64"/"all")
//
// 返回值: 是否已安装
func IsVersionInstalled(root string, version string, cpu string) bool {
	e32 := file.Exists(root + "\\v" + version + "\\node32.exe")
	e64 := file.Exists(root + "\\v" + version + "\\node64.exe")
	used := file.Exists(root + "\\v" + version + "\\node.exe")
	if cpu == "all" {
		return ((e32 || e64) && used) || e32 && e64
	}
	if file.Exists(root + "\\v" + version + "\\node" + cpu + ".exe") {
		return true
	}
	if ((e32 || e64) && used) || (e32 && e64) {
		return true
	}
	if !e32 && !e64 && used && arch.Validate(cpu) == arch.Bit(root+"\\v"+version+"\\node.exe") {
		return true
	}
	if cpu == "32" {
		return e32
	}
	if cpu == "64" {
		return e64
	}
	return false
}

// IsVersionAvailable 检查指定版本的Node.js是否可从远程获取
// 参数:
//
//	v: 要检查的版本号
//
// 返回值: 是否可用
func IsVersionAvailable(v string) bool {
	// Check the service to make sure the version is available
	avail, _, _, _, _, _ := GetAvailable()

	for _, b := range avail {
		if b == v {
			return true
		}
	}
	return false
}

func reverseStringArray(str []string) []string {
	for i := 0; i < len(str)/2; i++ {
		j := len(str) - i - 1
		str[i], str[j] = str[j], str[i]
	}

	return str
}

// GetInstalled 获取已安装的所有Node.js版本列表(按版本号降序排列)
// 参数:
//
//	root: NVM安装根目录
//
// 返回值: 已安装版本列表(格式如["v12.18.3", "v10.22.0"])
// GetInstalled 获取指定目录下安装的所有Node.js版本
// 返回格式为 ["v1.2.3", "v4.5.6"] 的字符串数组
func GetInstalled(root string) []string {
	// 初始化版本列表
	list := make([]semver.Version, 0)
	// 读取目录下所有文件
	files, _ := ioutil.ReadDir(root)

	// 倒序遍历文件，确保最新版本在前
	for i := len(files) - 1; i >= 0; i-- {
		// 检查是否为目录或符号链接
		if files[i].IsDir() || (files[i].Mode()&os.ModeSymlink == os.ModeSymlink) {
			// 检查文件名是否以"v"开头(表示Node.js版本目录)
			isnode, _ := regexp.MatchString("v", files[i].Name())

			if isnode {
				// 移除"v"前缀并解析为语义化版本
				currentVersionString := strings.Replace(files[i].Name(), "v", "", 1)
				currentVersion, _ := semver.Make(currentVersionString)

				list = append(list, currentVersion)
			}
		}
	}

	// 对版本进行排序
	semver.Sort(list)

	// 准备可输出的版本字符串列表
	loggableList := make([]string, 0)
	// 为每个版本添加"v"前缀
	for _, version := range list {
		loggableList = append(loggableList, "v"+version.String())
	}

	// 反转数组使最新版本在前
	loggableList = reverseStringArray(loggableList)

	return loggableList
}

// BySemanticVersion 用于按语义化版本排序的字符串切片类型
type BySemanticVersion []string

func (s BySemanticVersion) Len() int {
	return len(s)
}

func (s BySemanticVersion) Swap(i, j int) {
	s[i], s[j] = s[j], s[i]
}

func (s BySemanticVersion) Less(i, j int) bool {
	v1, _ := semver.Make(s[i])
	v2, _ := semver.Make(s[j])
	return v1.GTE(v2)
}

// isLTS 检查版本是否为LTS(长期支持)版本(内部函数)
// 参数:
//
//	element: 版本信息map
//
// 返回值: 是否为LTS版本
func isLTS(element map[string]interface{}) bool {
	switch datatype := element["lts"].(type) {
	case bool:
		return datatype
	case string:
		return true
	}
	return false
}

// isCurrent 检查版本是否为当前版本(非LTS的最新版本)(内部函数)
// 参数:
//
//	element: 版本信息map
//
// 返回值: 是否为当前版本
func isCurrent(element map[string]interface{}) bool {
	if isLTS(element) {
		return false
	}

	version, _ := semver.Make(element["version"].(string)[1:])
	benchmark, _ := semver.Make("1.0.0")

	if version.LT(benchmark) {
		return false
	}

	return true
	// return version.Major%2 == 1
}

// isStable 检查版本是否为稳定旧版本(内部函数)
// 参数:
//
//	element: 版本信息map
//
// 返回值: 是否为稳定旧版本
// isStable 判断给定的Node版本是否是稳定版
// 稳定版的条件：1.不是当前版本 2.主版本号为0 3.次版本号为偶数
func isStable(element map[string]interface{}) bool {
	if isCurrent(element) {
		return false
	}

	version, _ := semver.Make(element["version"].(string)[1:])

	if version.Major != 0 {
		return false
	}

	return version.Minor%2 == 0
}

// isUnstable 检查版本是否为不稳定旧版本(内部函数)
// 参数:
//
//	element: 版本信息map
//
// 返回值: 是否为不稳定旧版本
func isUnstable(element map[string]interface{}) bool {
	if isStable(element) {
		return false
	}

	version, _ := semver.Make(element["version"].(string)[1:])

	if version.Major != 0 {
		return false
	}

	return version.Minor%2 != 0
}

// GetAvailable 获取远程可用的Node.js版本信息
// 返回值:
//
//	[]string: 所有可用版本
//	[]string: LTS版本
//	[]string: 当前版本
//	[]string: 稳定旧版本
//	[]string: 不稳定旧版本
//	map[string]string: 各版本对应的npm版本
//
// GetAvailable 获取所有可用的Node.js版本信息
// 返回: all(所有版本), lts(长期支持版), current(当前版), stable(稳定版), unstable(不稳定版), npm(版本对应的npm版本)
func GetAvailable() ([]string, []string, []string, []string, []string, map[string]string) {
	all := make([]string, 0)
	lts := make([]string, 0)
	current := make([]string, 0)
	stable := make([]string, 0)
	unstable := make([]string, 0)
	npm := make(map[string]string)
	url := web.GetFullNodeUrl("index.json")

	// 从远程获取版本列表JSON文件
	text, err := web.GetRemoteTextFile(url)
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
	if len(text) == 0 {
		fmt.Println("Error retrieving version list: \"" + url + "\" returned blank results. This can happen when the remote file is being updated. Please try again in a few minutes.")
		os.Exit(0)
	}

	// 解析JSON数据到map切片
	var data = make([]map[string]interface{}, 0)
	err = json.Unmarshal([]byte(text), &data)
	if err != nil {
		fmt.Printf("Error retrieving versions from \"%s\": %v", url, err.Error())
		os.Exit(1)
	}

	// 遍历所有版本数据并分类
	for _, element := range data {
		var version = element["version"].(string)[1:] // 去掉版本号前的'v'
		all = append(all, version)

		if val, ok := element["npm"].(string); ok {
			npm[version] = val // 记录版本对应的npm版本
		}

		// 根据版本类型分类
		if isLTS(element) {
			lts = append(lts, version)
		} else if isCurrent(element) {
			current = append(current, version)
		} else if isStable(element) {
			stable = append(stable, version)
		} else if isUnstable(element) {
			unstable = append(unstable, version)
		}
	}

	return all, lts, current, stable, unstable, npm
}
