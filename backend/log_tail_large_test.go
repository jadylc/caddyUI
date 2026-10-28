package main

import (
	"os"
	"runtime"
	"sync/atomic"
	"testing"
	"time"
)

// TestTailFileLinesAllocationBounded tailFileLines 自身的分配量必须限制在窗口内。
// 这是「点开运行观测就 OOM」的核心回归点。
func TestTailFileLinesAllocationBounded(t *testing.T) {
	tmp := t.TempDir()

	// 构造 64MB 日志，模拟生产环境被健康检查和公网扫描灌大的 caddy.log。
	const targetBytes = 64 << 20
	line := `{"level":"error","ts":"2026-01-01T00:00:00+08:00","logger":"http.log.access","msg":"status code out of tolerances"}` + "\n"

	path := tmp + string(os.PathSeparator) + "big.log"
	f, err := os.Create(path)
	if err != nil {
		t.Fatalf("create error = %v", err)
	}
	for written := 0; written < targetBytes; {
		n, err := f.WriteString(line)
		if err != nil {
			f.Close()
			t.Fatalf("write error = %v", err)
		}
		written += n
	}
	if err := f.Close(); err != nil {
		t.Fatalf("close error = %v", err)
	}

	var before runtime.MemStats
	runtime.GC()
	runtime.ReadMemStats(&before)

	lines, err := tailFileLines(path, logTailDefaultMaxLines, logTailDefaultMaxBytes)
	if err != nil {
		t.Fatalf("tailFileLines error = %v", err)
	}

	var after runtime.MemStats
	runtime.ReadMemStats(&after)
	allocated := after.TotalAlloc - before.TotalAlloc

	t.Logf("file=%.1fMB window=%.1fMB lines=%d allocated=%.1fMB",
		float64(targetBytes)/(1<<20), float64(logTailDefaultMaxBytes)/(1<<20),
		len(lines), float64(allocated)/(1<<20))

	// 分配量应接近窗口大小，而不是文件大小。
	// 留 4 倍余量覆盖解码/切片开销，但仍必须远小于文件体积。
	budget := uint64(logTailDefaultMaxBytes) * 4
	if allocated > budget {
		t.Fatalf("allocated %.1f MB, want <= %.1f MB (window-bounded)",
			float64(allocated)/(1<<20), float64(budget)/(1<<20))
	}
	if allocated >= uint64(targetBytes) {
		t.Fatalf("allocated %.1f MB for a %.1f MB file: full-file read regression",
			float64(allocated)/(1<<20), float64(targetBytes)/(1<<20))
	}
	if len(lines) == 0 {
		t.Fatal("no lines returned")
	}
}

// TestObservabilityScanPeakBounded 完整观测扫描的峰值堆占用必须远小于日志体积。
func TestObservabilityScanPeakBounded(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping peak memory test in short mode")
	}

	tmp := t.TempDir()
	withTempPaths(t, tmp)

	const targetBytes = 64 << 20
	line := `{"level":"error","ts":"2026-01-01T00:00:00+08:00","logger":"http.log.access","msg":"status code out of tolerances","request":{"remote_ip":"192.168.1.76","uri":"/health"}}` + "\n"

	f, err := os.Create(logFile)
	if err != nil {
		t.Fatalf("create error = %v", err)
	}
	for written := 0; written < targetBytes; {
		n, err := f.WriteString(line)
		if err != nil {
			f.Close()
			t.Fatalf("write error = %v", err)
		}
		written += n
	}
	if err := f.Close(); err != nil {
		t.Fatalf("close error = %v", err)
	}

	runtime.GC()
	var before runtime.MemStats
	runtime.ReadMemStats(&before)

	// 采样扫描期间的峰值堆占用。
	var peak uint64
	done := make(chan struct{})
	go func() {
		ticker := time.NewTicker(2 * time.Millisecond)
		defer ticker.Stop()
		for {
			select {
			case <-done:
				return
			case <-ticker.C:
				var m runtime.MemStats
				runtime.ReadMemStats(&m)
				for {
					old := atomic.LoadUint64(&peak)
					if m.HeapAlloc <= old || atomic.CompareAndSwapUint64(&peak, old, m.HeapAlloc) {
						break
					}
				}
			}
		}
	}()

	start := time.Now()
	events := readRuntimeLogEvents(0, time.Now(), 24*365*time.Hour)
	elapsed := time.Since(start)
	close(done)

	peakHeap := atomic.LoadUint64(&peak)
	t.Logf("scan %v events=%d peakHeap=%.1fMB baseline=%.1fMB file=%.1fMB",
		elapsed, len(events), float64(peakHeap)/(1<<20),
		float64(before.HeapAlloc)/(1<<20), float64(targetBytes)/(1<<20))

	if len(events) == 0 {
		t.Fatal("no events parsed")
	}
	// 峰值堆占用不应达到文件体积的量级（旧实现会读全量 + 复制 + 切分）。
	if peakHeap >= uint64(targetBytes) {
		t.Fatalf("peak heap %.1f MB >= file %.1f MB: full-file read regression",
			float64(peakHeap)/(1<<20), float64(targetBytes)/(1<<20))
	}
	if elapsed > 30*time.Second {
		t.Fatalf("scan too slow: %v", elapsed)
	}
}
