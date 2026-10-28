package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// TestTailFileLinesReturnsLastLines 基本行为：返回最后 N 行。
func TestTailFileLinesReturnsLastLines(t *testing.T) {
	path := filepath.Join(t.TempDir(), "test.log")
	content := "line1\nline2\nline3\nline4\nline5\n"
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("write error = %v", err)
	}

	lines, err := tailFileLines(path, 2, logTailDefaultMaxBytes)
	if err != nil {
		t.Fatalf("tailFileLines error = %v", err)
	}
	if len(lines) != 2 || lines[0] != "line4" || lines[1] != "line5" {
		t.Fatalf("lines = %#v, want [line4 line5]", lines)
	}
}

// TestTailFileLinesBoundedByBytes 字节上限生效，且不会返回被截断的残行。
func TestTailFileLinesBoundedByBytes(t *testing.T) {
	path := filepath.Join(t.TempDir(), "big.log")
	// 每行 100 字节，共 1000 行 = 100KB。
	var sb strings.Builder
	for i := 0; i < 1000; i++ {
		sb.WriteString(strings.Repeat("x", 99))
		sb.WriteString("\n")
	}
	if err := os.WriteFile(path, []byte(sb.String()), 0644); err != nil {
		t.Fatalf("write error = %v", err)
	}

	// 只允许读 1KB，应拿到约 10 行，且每行都是完整的 99 个 x。
	lines, err := tailFileLines(path, 0, 1024)
	if err != nil {
		t.Fatalf("tailFileLines error = %v", err)
	}
	if len(lines) == 0 {
		t.Fatal("lines is empty")
	}
	if len(lines) > 11 {
		t.Fatalf("lines count = %d, want <= 11 for 1KB window", len(lines))
	}
	for i, line := range lines {
		if line != strings.Repeat("x", 99) {
			t.Fatalf("line %d is truncated: len=%d", i, len(line))
		}
	}
}

// TestTailFileLinesDropsTruncatedHead 窗口左边界截断的残行必须被丢弃。
func TestTailFileLinesDropsTruncatedHead(t *testing.T) {
	path := filepath.Join(t.TempDir(), "head.log")
	// 第一行很长，窗口从它中间开始。
	content := strings.Repeat("A", 500) + "\nshort1\nshort2\n"
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("write error = %v", err)
	}

	lines, err := tailFileLines(path, 0, 20) // 只够覆盖尾部
	if err != nil {
		t.Fatalf("tailFileLines error = %v", err)
	}
	for _, line := range lines {
		if strings.HasPrefix(line, "A") {
			t.Fatalf("truncated head line leaked into result: %q", line)
		}
	}
	if len(lines) == 0 || lines[len(lines)-1] != "short2" {
		t.Fatalf("lines = %#v, want last line short2", lines)
	}
}

// TestTailFileLinesHandlesEmptyAndMissing 空文件和不存在文件的处理。
func TestTailFileLinesHandlesEmptyAndMissing(t *testing.T) {
	dir := t.TempDir()

	empty := filepath.Join(dir, "empty.log")
	if err := os.WriteFile(empty, []byte{}, 0644); err != nil {
		t.Fatalf("write error = %v", err)
	}
	lines, err := tailFileLines(empty, 10, logTailDefaultMaxBytes)
	if err != nil {
		t.Fatalf("empty file error = %v", err)
	}
	if len(lines) != 0 {
		t.Fatalf("empty file lines = %#v, want empty", lines)
	}

	if _, err := tailFileLines(filepath.Join(dir, "missing.log"), 10, logTailDefaultMaxBytes); err == nil {
		t.Fatal("missing file should return error")
	}
}

// TestTailFileLinesSmallFileWholeContent 文件小于窗口时返回全部内容。
func TestTailFileLinesSmallFileWholeContent(t *testing.T) {
	path := filepath.Join(t.TempDir(), "small.log")
	if err := os.WriteFile(path, []byte("a\nb\nc\n"), 0644); err != nil {
		t.Fatalf("write error = %v", err)
	}
	lines, err := tailFileLines(path, 100, logTailDefaultMaxBytes)
	if err != nil {
		t.Fatalf("tailFileLines error = %v", err)
	}
	if len(lines) != 3 || lines[0] != "a" || lines[2] != "c" {
		t.Fatalf("lines = %#v, want [a b c]", lines)
	}
}

// TestReadRuntimeLogEventsBoundedToTail 观测页面只扫描日志尾部，
// 且日志过大时不会把整个文件读进内存。
func TestReadRuntimeLogEventsBoundedToTail(t *testing.T) {
	tmp := t.TempDir()
	withTempPaths(t, tmp)

	// 构造一个比尾部窗口大得多的日志文件。
	var sb strings.Builder
	// 前面塞大量无关行（模拟健康检查噪音）。
	noise := `{"level":"error","ts":"2020-01-01T00:00:00+08:00","logger":"http.log.access","msg":"status code out of tolerances"}` + "\n"
	for i := 0; i < 20000; i++ {
		sb.WriteString(noise)
	}
	// 尾部放一条新鲜的可解析事件。
	now := time.Now()
	tailTS := now.Format(time.RFC3339)
	sb.WriteString(`{"level":"error","ts":"` + tailTS + `","logger":"http","msg":"tail event"}` + "\n")

	if err := os.WriteFile(logFile, []byte(sb.String()), 0644); err != nil {
		t.Fatalf("write log error = %v", err)
	}

	events := readRuntimeLogEvents(0, now, 24*time.Hour)
	foundTail := false
	for _, ev := range events {
		if strings.Contains(ev.Message, "tail event") {
			foundTail = true
		}
	}
	if !foundTail {
		t.Fatalf("tail event not found in %d events", len(events))
	}
}
