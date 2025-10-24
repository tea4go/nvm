// Package semver 提供语义化版本(Semantic Versioning)解析和比较功能
// 主要功能包括：
// - 解析和验证语义化版本字符串
// - 版本号比较和排序
// - 预发布版本和构建元数据处理
// 基于MIT License，原始代码来自Benedikt Lang(https://github.com/blang)
package semver

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
)

const (
	numbers  string = "0123456789"
	alphas          = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ-"
	alphanum        = alphas + numbers
	dot             = "."
	hyphen          = "-"
	plus            = "+"
)

// Latest fully supported spec version
var SPEC_VERSION = Version{
	Major: 2,
	Minor: 0,
	Patch: 0,
}

// Version 表示一个语义化版本
type Version struct {
	Major uint64       // 主版本号(不兼容的API修改)
	Minor uint64       // 次版本号(向下兼容的功能新增)
	Patch uint64       // 修订号(向下兼容的问题修正)
	Pre   []*PRVersion // 预发布版本标识
	Build []string     // 构建元数据(不参与版本比较)
}

// String 将Version结构体转换为语义化版本字符串
// 返回值: 格式为"Major.Minor.Patch[-PreRelease][+BuildMetadata]"的字符串
func (v *Version) String() string {
	versionArray := []string{
		strconv.FormatUint(v.Major, 10),
		dot,
		strconv.FormatUint(v.Minor, 10),
		dot,
		strconv.FormatUint(v.Patch, 10),
	}
	if len(v.Pre) > 0 {
		versionArray = append(versionArray, hyphen)
		for i, pre := range v.Pre {
			if i > 0 {
				versionArray = append(versionArray, dot)
			}
			versionArray = append(versionArray, pre.String())
		}
	}
	if len(v.Build) > 0 {
		versionArray = append(versionArray, plus, strings.Join(v.Build, dot))
	}
	return strings.Join(versionArray, "")
}

// GT 检查当前版本是否大于目标版本
// 参数:
//
//	o: 要比较的目标版本
//
// 返回值: 如果当前版本大于目标版本则返回true
func (v *Version) GT(o *Version) bool {
	return (v.Compare(o) == 1)
}

// GTE 检查当前版本是否大于或等于目标版本
// 参数:
//
//	o: 要比较的目标版本
//
// 返回值: 如果当前版本大于或等于目标版本则返回true
func (v *Version) GTE(o *Version) bool {
	return (v.Compare(o) >= 0)
}

// LT 检查当前版本是否小于目标版本
// 参数:
//
//	o: 要比较的目标版本
//
// 返回值: 如果当前版本小于目标版本则返回true
func (v *Version) LT(o *Version) bool {
	return (v.Compare(o) == -1)
}

// LTE 检查当前版本是否小于或等于目标版本
// 参数:
//
//	o: 要比较的目标版本
//
// 返回值: 如果当前版本小于或等于目标版本则返回true
func (v *Version) LTE(o *Version) bool {
	return (v.Compare(o) <= 0)
}

// Compare 比较两个版本
// 参数:
//
//	o: 要比较的目标版本
//
// 返回值:
//
//	-1: 当前版本小于目标版本
//	 0: 两个版本相等
//	 1: 当前版本大于目标版本
func (v *Version) Compare(o *Version) int {
	if v.Major != o.Major {
		if v.Major > o.Major {
			return 1
		} else {
			return -1
		}
	}
	if v.Minor != o.Minor {
		if v.Minor > o.Minor {
			return 1
		} else {
			return -1
		}
	}
	if v.Patch != o.Patch {
		if v.Patch > o.Patch {
			return 1
		} else {
			return -1
		}
	}

	// Quick comparison if a version has no prerelease versions
	if len(v.Pre) == 0 && len(o.Pre) == 0 {
		return 0
	} else if len(v.Pre) == 0 && len(o.Pre) > 0 {
		return 1
	} else if len(v.Pre) > 0 && len(o.Pre) == 0 {
		return -1
	} else {

		i := 0
		for ; i < len(v.Pre) && i < len(o.Pre); i++ {
			if comp := v.Pre[i].Compare(o.Pre[i]); comp == 0 {
				continue
			} else if comp == 1 {
				return 1
			} else {
				return -1
			}
		}

		// If all pr versions are the equal but one has further pr version, this one greater
		if i == len(v.Pre) && i == len(o.Pre) {
			return 0
		} else if i == len(v.Pre) && i < len(o.Pre) {
			return -1
		} else {
			return 1
		}

	}
}

// Validate 检查版本是否有效
// 返回值: 如果版本无效则返回错误
func (v *Version) Validate() error {
	// Major, Minor, Patch already validated using uint64

	if len(v.Pre) > 0 {
		for _, pre := range v.Pre {
			if !pre.IsNum { //Numeric prerelease versions already uint64
				if len(pre.VersionStr) == 0 {
					return fmt.Errorf("Prerelease can not be empty %q", pre.VersionStr)
				}
				if !containsOnly(pre.VersionStr, alphanum) {
					return fmt.Errorf("Invalid character(s) found in prerelease %q", pre.VersionStr)
				}
			}
		}
	}

	if len(v.Build) > 0 {
		for _, build := range v.Build {
			if len(build) == 0 {
				return fmt.Errorf("Build meta data can not be empty %q", build)
			}
			if !containsOnly(build, alphanum) {
				return fmt.Errorf("Invalid character(s) found in build meta data %q", build)
			}
		}
	}

	return nil
}

// New 解析版本字符串并返回Version对象(Parse的别名)
// 参数:
//
//	s: 要解析的版本字符串
//
// 返回值:
//
//	*Version: 解析后的版本对象
//	error: 解析过程中遇到的错误
func New(s string) (*Version, error) {
	return Parse(s)
}

// Parse 解析版本字符串并返回Version对象
// 参数:
//
//	s: 要解析的版本字符串
//
// 返回值:
//
//	*Version: 解析后的版本对象
//	error: 解析过程中遇到的错误
//
// Parse 解析语义版本字符串并返回 Version 结构体
// 支持的格式: vX.Y.Z[-PR][+build]
func Parse(s string) (*Version, error) {
	// 移除版本号前的'v'前缀
	s = strings.Replace(s, "v", "", 1)
	if len(s) == 0 {
		return nil, errors.New("Version string empty")
	}

	// 将版本号分割为 major.minor.(patch+pr+meta) 三部分
	parts := strings.SplitN(s, ".", 3)
	if len(parts) != 3 {
		return nil, errors.New("No Major.Minor.Patch elements found")
	}

	// 解析 major 版本号
	if !containsOnly(parts[0], numbers) {
		return nil, fmt.Errorf("Invalid character(s) found in major number %q", parts[0])
	}
	if hasLeadingZeroes(parts[0]) {
		return nil, fmt.Errorf("Major number must not contain leading zeroes %q", parts[0])
	}
	major, err := strconv.ParseUint(parts[0], 10, 64)
	if err != nil {
		return nil, err
	}

	// 解析 minor 版本号
	if !containsOnly(parts[1], numbers) {
		return nil, fmt.Errorf("Invalid character(s) found in minor number %q", parts[1])
	}
	if hasLeadingZeroes(parts[1]) {
		return nil, fmt.Errorf("Minor number must not contain leading zeroes %q", parts[1])
	}
	minor, err := strconv.ParseUint(parts[1], 10, 64)
	if err != nil {
		return nil, err
	}

	// 查找预发布版本和构建元数据的分隔符位置
	preIndex := strings.Index(parts[2], "-")
	buildIndex := strings.Index(parts[2], "+")

	// 确定 patch 版本的结束位置(预发布版本或构建元数据的开始位置)
	var subVersionIndex int
	if preIndex != -1 && buildIndex == -1 {
		subVersionIndex = preIndex
	} else if preIndex == -1 && buildIndex != -1 {
		subVersionIndex = buildIndex
	} else if preIndex == -1 && buildIndex == -1 {
		subVersionIndex = len(parts[2])
	} else {
		// 处理构建元数据中包含连字符的情况
		if buildIndex < preIndex {
			subVersionIndex = buildIndex
			preIndex = -1 // 构建元数据在预发布版本之前表示没有预发布版本
		} else {
			subVersionIndex = preIndex
		}
	}

	// 解析 patch 版本号
	if !containsOnly(parts[2][:subVersionIndex], numbers) {
		return nil, fmt.Errorf("Invalid character(s) found in patch number %q", parts[2][:subVersionIndex])
	}
	if hasLeadingZeroes(parts[2][:subVersionIndex]) {
		return nil, fmt.Errorf("Patch number must not contain leading zeroes %q", parts[2][:subVersionIndex])
	}
	patch, err := strconv.ParseUint(parts[2][:subVersionIndex], 10, 64)
	if err != nil {
		return nil, err
	}
	v := &Version{}
	v.Major = major
	v.Minor = minor
	v.Patch = patch

	// 解析预发布版本(如果有)
	if preIndex != -1 {
		var preRels string
		if buildIndex != -1 {
			preRels = parts[2][subVersionIndex+1 : buildIndex]
		} else {
			preRels = parts[2][subVersionIndex+1:]
		}
		prparts := strings.Split(preRels, ".")
		for _, prstr := range prparts {
			parsedPR, err := NewPRVersion(prstr)
			if err != nil {
				return nil, err
			}
			v.Pre = append(v.Pre, parsedPR)
		}
	}

	// 解析构建元数据(如果有)
	if buildIndex != -1 {
		buildStr := parts[2][buildIndex+1:]
		buildParts := strings.Split(buildStr, ".")
		for _, str := range buildParts {
			if len(str) == 0 {
				return nil, errors.New("Build meta data is empty")
			}
			if !containsOnly(str, alphanum) {
				return nil, fmt.Errorf("Invalid character(s) found in build meta data %q", str)
			}
			v.Build = append(v.Build, str)
		}
	}

	return v, nil
}

// PRVersion 表示预发布版本信息
type PRVersion struct {
	VersionStr string // 字符串形式的版本标识
	VersionNum uint64 // 数字形式的版本标识
	IsNum      bool   // 是否为数字版本
}

// NewPRVersion 创建新的预发布版本对象
// 参数:
//
//	s: 预发布版本字符串
//
// 返回值:
//
//	*PRVersion: 创建的预发布版本对象
//	error: 创建过程中遇到的错误
func NewPRVersion(s string) (*PRVersion, error) {
	if len(s) == 0 {
		return nil, errors.New("Prerelease is empty")
	}
	v := &PRVersion{}
	if containsOnly(s, numbers) {
		if hasLeadingZeroes(s) {
			return nil, fmt.Errorf("Numeric PreRelease version must not contain leading zeroes %q", s)
		}
		num, err := strconv.ParseUint(s, 10, 64)

		// Might never be hit, but just in case
		if err != nil {
			return nil, err
		}
		v.VersionNum = num
		v.IsNum = true
	} else if containsOnly(s, alphanum) {
		v.VersionStr = s
		v.IsNum = false
	} else {
		return nil, fmt.Errorf("Invalid character(s) found in prerelease %q", s)
	}
	return v, nil
}

// IsNumeric 检查预发布版本是否为数字版本
// 返回值: 如果是数字版本则返回true
func (v *PRVersion) IsNumeric() bool {
	return v.IsNum
}

// Compare 比较两个预发布版本
// 参数:
//
//	o: 要比较的目标预发布版本
//
// 返回值:
//
//	-1: 当前版本小于目标版本
//	 0: 两个版本相等
//	 1: 当前版本大于目标版本
func (v *PRVersion) Compare(o *PRVersion) int {
	if v.IsNum && !o.IsNum {
		return -1
	} else if !v.IsNum && o.IsNum {
		return 1
	} else if v.IsNum && o.IsNum {
		if v.VersionNum == o.VersionNum {
			return 0
		} else if v.VersionNum > o.VersionNum {
			return 1
		} else {
			return -1
		}
	} else { // both are Alphas
		if v.VersionStr == o.VersionStr {
			return 0
		} else if v.VersionStr > o.VersionStr {
			return 1
		} else {
			return -1
		}
	}
}

// String 将预发布版本转换为字符串
// 返回值: 预发布版本的字符串表示
func (v *PRVersion) String() string {
	if v.IsNum {
		return strconv.FormatUint(v.VersionNum, 10)
	}
	return v.VersionStr
}

// containsOnly 检查字符串是否只包含指定字符集中的字符(内部函数)
// 参数:
//
//	s: 要检查的字符串
//	set: 允许的字符集
//
// 返回值: 如果字符串只包含指定字符则返回true
func containsOnly(s string, set string) bool {
	return strings.IndexFunc(s, func(r rune) bool {
		return !strings.ContainsRune(set, r)
	}) == -1
}

// hasLeadingZeroes 检查字符串是否有前导零(内部函数)
// 参数:
//
//	s: 要检查的字符串
//
// 返回值: 如果有前导零则返回true
func hasLeadingZeroes(s string) bool {
	return len(s) > 1 && s[0] == '0'
}

// NewBuildVersion 创建新的构建版本元数据(内部函数)
// 参数:
//
//	s: 构建元数据字符串
//
// 返回值:
//
//	string: 验证后的构建元数据
//	error: 创建过程中遇到的错误
func NewBuildVersion(s string) (string, error) {
	if len(s) == 0 {
		return "", errors.New("Buildversion is empty")
	}
	if !containsOnly(s, alphanum) {
		return "", fmt.Errorf("Invalid character(s) found in build meta data %q", s)
	}
	return s, nil
}
