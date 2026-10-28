package main

import (
	"io"
	"os"
	"strings"
)

// 日志尾部读取上限。
//
// 这些页面（运行观测、日志、WAF 审计）只关心最近的内容，
// 因此从文件末尾读取固定大小的窗口，而不是用 os.ReadFile 把整个文件读进内存。
// 生产环境里 caddy.log / 访问日志可以长到数百 MB，
// 全量读取再 strings.Split 会把内存放大数倍并直接 OOM。
const (
	logTailDefaultMaxBytes = int64(16 << 20) // 16MB
	logTailDefaultMaxLines = 20000
)

// tailFileLines 从文件末尾向前读取最多 maxLines 行，且读取字节数不超过 maxBytes。
// 窗口左边界若落在行中间，会丢弃该残行，保证返回的都是完整行。
// maxLines <= 0 表示不限制行数（仍受 maxBytes 限制）。
func tailFileLines(path string, maxLines int, maxBytes int64) ([]string, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	info, err := f.Stat()
	if err != nil {
		return nil, err
	}
	size := info.Size()
	if size <= 0 {
		return []string{}, nil
	}

	if maxBytes <= 0 {
		maxBytes = logTailDefaultMaxBytes
	}
	start := size - maxBytes
	if start < 0 {
		start = 0
	}

	buf := make([]byte, size-start)
	if _, err := f.ReadAt(buf, start); err != nil && err != io.EOF {
		return nil, err
	}

	text := string(buf)
	if start > 0 {
		// 窗口左边界可能把一行截断，丢弃这个残行。
		if idx := strings.IndexByte(text, '\n'); idx >= 0 {
			text = text[idx+1:]
		} else {
			text = ""
		}
	}
	text = strings.TrimRight(text, "\n")
	if text == "" {
		return []string{}, nil
	}

	lines := strings.Split(text, "\n")
	if maxLines > 0 && len(lines) > maxLines {
		lines = lines[len(lines)-maxLines:]
	}
	return lines, nil
}
