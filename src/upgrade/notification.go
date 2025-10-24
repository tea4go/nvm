package upgrade

import (
	"encoding/json"
	"os"
	"path/filepath"
	"time"
)

// LastNotification 存储最后一次通知的信息
type LastNotification struct {
	outpath string // 通知文件存储路径
	LTS     string `json:"lts,omitempty"`     // 最后一次LTS版本通知日期
	Current string `json:"current,omitempty"` // 最后一次Current版本通知日期
	NVM4W   string `json:"nvm4w,omitempty"`   // 最后一次nvm4w更新通知日期
	Author  string `json:"author,omitempty"`  // 作者通知信息
}

// LoadNotices 从文件中加载通知信息
func LoadNotices() *LastNotification {
	ln := &LastNotification{}
	// 读取通知文件
	noticedata, err := os.ReadFile(ln.File())
	if err != nil {
		// 文件不存在时不报错
		if !os.IsNotExist(err) {
			abortOnError(err)
		}
	}

	// 解析JSON数据
	if noticedata != nil {
		abortOnError(json.Unmarshal(noticedata, &ln))
	}

	return ln
}

// Path 获取通知文件存储目录
func (ln *LastNotification) Path() string {
	// 如果路径未设置，使用默认路径
	if ln.outpath == "" {
		ln.outpath = filepath.Join(os.Getenv("APPDATA"), ".nvm")
	}
	return ln.outpath
}

// File 获取通知文件完整路径
func (ln *LastNotification) File() string {
	return filepath.Join(ln.Path(), ".updates.json")
}

// Save 将通知信息保存到文件
func (ln *LastNotification) Save() {
	// 序列化为JSON
	output, err := json.Marshal(ln)
	abortOnError(err)

	// 确保目录存在
	abortOnError(os.MkdirAll(ln.Path(), os.ModePerm))

	// 写入文件
	abortOnError(os.WriteFile(ln.File(), output, os.ModePerm))

	// 设置隐藏属性
	abortOnError(setHidden(ln.Path()))
}

// LastLTS 获取最后一次LTS通知的时间
func (ln *LastNotification) LastLTS() time.Time {
	// 如果没有记录，返回当前时间
	if ln.LTS == "" {
		return time.Now()
	}

	// 解析日期字符串
	t, _ := time.Parse("2006-01-02", ln.LTS)
	return t
}

// LastCurrent 获取最后一次Current通知的时间
func (ln *LastNotification) LastCurrent() time.Time {
	// 如果没有记录，返回当前时间
	if ln.Current == "" {
		return time.Now()
	}

	// 解析日期字符串
	t, _ := time.Parse("2006-01-02", ln.Current)
	return t
}
